package sentinel

import (
	"context"
	"fmt"
	"sync"
	"time"

	sentinelapi "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	sconfig "github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/ayxworxfr/go_admin/internal/platform/config"
	"github.com/ayxworxfr/go_admin/pkg/api"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
)

var SentinelInstance *Sentinel

func SentinelMiddleware() app.HandlerFunc {
	return SentinelInstance.Middleware()
}

func InitSentinel(configPath string) error {
	instance, err := NewSentinelMiddleware(configPath)
	if err != nil {
		return err
	}
	SentinelInstance = instance
	return nil
}

// Sentinel 哨兵中间件
type Sentinel struct {
	configManager *config.ConfigManager
	resourceMap   map[string]string // 路径到资源名的映射
	mutex         sync.RWMutex
}

// NewSentinelMiddleware 创建新的Sentinel中间件
func NewSentinelMiddleware(configPath string) (*Sentinel, error) {
	configManager, err := config.NewConfigManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	mw := &Sentinel{
		configManager: configManager,
		resourceMap:   make(map[string]string),
	}

	// 初始化Sentinel
	if err := mw.initSentinel(); err != nil {
		return nil, fmt.Errorf("failed to initialize Sentinel: %w", err)
	}

	// 加载初始规则
	if err := mw.loadRules(); err != nil {
		return nil, fmt.Errorf("failed to load Sentinel rules: %w", err)
	}

	// 启动配置监控
	configManager.StartWatcher(30 * time.Second)

	// 启动规则定期刷新
	go mw.refreshRulesPeriodically(3 * time.Minute)

	return mw, nil
}

// initSentinel 初始化Sentinel
func (mw *Sentinel) initSentinel() error {
	config := mw.configManager.GetConfig()

	sentinelConfig := sconfig.NewDefaultConfig()
	sentinelConfig.Sentinel.App = struct {
		Name string
		Type int32
	}{
		Name: config.Sentinel.AppName,
		Type: 0, // 默认为普通应用
	}
	if config.Sentinel.Log.Enabled {
		sentinelConfig.Sentinel.Log = sconfig.LogConfig{
			Dir:    config.Sentinel.Log.Dir,
			UsePid: config.Sentinel.Log.UsePid,
			Metric: sconfig.MetricLogConfig{
				SingleFileMaxSize: config.Sentinel.Log.Metric.SingleFileMaxSize,
				MaxFileCount:      config.Sentinel.Log.Metric.MaxFileCount,
				FlushIntervalSec:  config.Sentinel.Log.Metric.FlushIntervalSec,
			},
		}
	}
	sentinelConfig.Sentinel.Exporter.Metric = sconfig.MetricExporterConfig{
		HttpAddr: config.Sentinel.Log.Metric.HttpAddr,
		HttpPath: config.Sentinel.Log.Metric.HttpPath,
	}

	// 设置自定义日志
	logging.ResetGlobalLogger(NewSentinelLogger())

	return sentinelapi.InitWithConfig(sentinelConfig)
}

// loadRules 加载Sentinel规则
func (mw *Sentinel) loadRules() error {
	config := mw.configManager.GetConfig()

	mw.mutex.Lock()
	defer mw.mutex.Unlock()

	// 清空资源映射
	mw.resourceMap = make(map[string]string)

	// 加载限流规则
	var flowRules []*flow.Rule
	// 加载熔断规则
	var cbRules []*circuitbreaker.Rule

	for _, resource := range config.Sentinel.Resources {
		if !resource.Enabled {
			continue
		}

		// 添加路径到资源名的映射
		mw.resourceMap[resource.Path] = resource.Name

		// 添加限流规则
		if fr := resource.ToFlowRules(); fr != nil {
			flowRules = append(flowRules, fr...)
		}

		// 添加熔断规则
		if cbr := resource.ToCircuitBreakerRules(*config); cbr != nil {
			cbRules = append(cbRules, cbr...)
		}
	}

	// 应用规则
	if _, err := flow.LoadRules(flowRules); err != nil {
		return fmt.Errorf("failed to load flow rules: %w", err)
	}

	if _, err := circuitbreaker.LoadRules(cbRules); err != nil {
		return fmt.Errorf("failed to load circuit breaker rules: %w", err)
	}

	logger.Debugf(context.Background(), "Successfully loaded %d flow rules and %d circuit breaker rules", len(flowRules), len(cbRules))
	return nil
}

// refreshRulesPeriodically 定期刷新规则
func (mw *Sentinel) refreshRulesPeriodically(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if err := mw.loadRules(); err != nil {
			logger.Errorf(context.Background(), "Failed to refresh Sentinel rules: %v", err)
		}
	}
}

// Middleware 返回Hertz中间件函数
func (mw *Sentinel) Middleware() app.HandlerFunc {
	return func(ctx context.Context, c *app.RequestContext) {
		if mw == nil {
			// 有bug，接口进入时mw为nil
			mw = SentinelInstance
		}
		var resourceName string

		// 先尝试匹配具体路径
		resourceName, exists := mw.matchResource(string(c.Path()))

		// 路径未匹配时，使用全局限流资源名
		if !exists {
			c.Next(ctx)
			return
		}

		// 进入Sentinel保护
		entry, blockErr := sentinelapi.Entry(
			resourceName,
			sentinelapi.WithTrafficType(base.Inbound),
			sentinelapi.WithArgs(c),
		)

		if blockErr != nil {
			// 请求被限流或熔断
			mw.handleBlockedRequest(c, resourceName, blockErr)
			logger.Warn(ctx, "Request blocked by Sentinel",
				logger.String("resource", resourceName),
				logger.String("reason", blockErr.BlockType().String()),
			)
			return
		}

		// 请求通过，继续处理
		defer entry.Exit()
		c.Next(ctx)
	}
}

func (SentinelInstance *Sentinel) matchResource(path string) (string, bool) {
	SentinelInstance.mutex.RLock()
	defer SentinelInstance.mutex.RUnlock()
	resourceName, exists := SentinelInstance.resourceMap[path]
	if !exists {
		// 路径未匹配时，使用全局限流资源名
		return "global_default", true
	}
	return resourceName, exists
}

// handleBlockedRequest 处理被拦截的请求
func (mw *Sentinel) handleBlockedRequest(c *app.RequestContext, resourceName string, blockErr *base.BlockError) {
	// 记录被拦截的请求
	logger.Warnf(context.Background(), "Request blocked for resource: %s, reason: %s", resourceName, blockErr.BlockType().String())

	api.Abort(c, api.RateLimit("Too many requests, please try again later"))
}
