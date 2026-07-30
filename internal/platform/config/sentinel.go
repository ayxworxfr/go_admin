package config

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/alibaba/sentinel-golang/core/circuitbreaker"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"gopkg.in/yaml.v2"
)

// SentinelConfig 主配置结构
type SentinelConfig struct {
	Sentinel struct {
		AppName              string    `yaml:"app_name"`
		Log                  LogConfig `yaml:"log"`
		GlobalCircuitBreaker struct {
			Enabled          bool   `yaml:"enabled"`
			RetryTimeoutMs   uint32 `yaml:"retry_timeout_ms"`
			MinRequestAmount uint64 `yaml:"min_request_amount"`
			StatIntervalMs   uint32 `yaml:"stat_interval_ms"`
		} `yaml:"global_circuit_breaker"`
		Resources []ResourceConfig `yaml:"resources"`
	} `yaml:"sentinel"`
}

type LogConfig struct {
	Enabled bool            `yaml:"enabled"`
	UsePid  bool            `yaml:"usePid"`
	Dir     string          `yaml:"dir"`
	Metric  MetricLogConfig `yaml:"metric"`
}

type MetricLogConfig struct {
	HttpAddr          string `yaml:"httpAddr"`
	HttpPath          string `yaml:"httpPath"`
	SingleFileMaxSize uint64 `yaml:"singleFileMaxSize"`
	MaxFileCount      uint32 `yaml:"maxFileCount"`
	FlushIntervalSec  uint32 `yaml:"flushIntervalSec"`
}

// ResourceConfig 资源配置
type ResourceConfig struct {
	Name               string                   `yaml:"name"`
	Path               string                   `yaml:"path"`
	Enabled            bool                     `yaml:"enabled"`
	FlowRule           FlowRuleConfig           `yaml:"flow_rule"`
	CircuitBreakerRule CircuitBreakerRuleConfig `yaml:"circuit_breaker_rule"`
}

// FlowRuleConfig 限流规则配置
type FlowRuleConfig struct {
	Enabled           bool    `yaml:"enabled"`
	Threshold         float64 `yaml:"threshold"`
	ControlBehavior   string  `yaml:"control_behavior"`
	MaxQueueingTimeMs int     `yaml:"max_queueing_time_ms"`
}

// CircuitBreakerRuleConfig 熔断规则配置
type CircuitBreakerRuleConfig struct {
	Enabled             bool    `yaml:"enabled"`
	Strategy            string  `yaml:"strategy"`
	SlowRtThreshold     int64   `yaml:"slow_rt_threshold"`
	ErrorRatioThreshold float64 `yaml:"error_ratio_threshold"`
	MinRequestAmount    uint64  `yaml:"min_request_amount"`
	StatIntervalMs      uint32  `yaml:"stat_interval_ms"`
	MaxAllowedRtMs      uint64  `yaml:"max_allowed_rt_ms"`
}

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath string
	config     *SentinelConfig
	mutex      sync.RWMutex
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) (*ConfigManager, error) {
	manager := &ConfigManager{
		configPath: configPath,
	}

	err := manager.reloadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return manager, nil
}

// GetConfig 获取当前配置
func (cm *ConfigManager) GetConfig() *SentinelConfig {
	cm.mutex.RLock()
	defer cm.mutex.RUnlock()
	return cm.config
}

// StartWatcher 启动配置文件监控
func (cm *ConfigManager) StartWatcher(interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			err := cm.reloadConfig()
			if err != nil {
				logger.Errorf(context.Background(), "Failed to reload config: %v", err)
			}
		}
	}()
}

// reloadConfig 重新加载配置文件
func (cm *ConfigManager) reloadConfig() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	newConfig := &SentinelConfig{}
	err = yaml.Unmarshal(data, newConfig)
	if err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	cm.mutex.Lock()
	cm.config = newConfig
	cm.mutex.Unlock()

	logger.Debug(context.Background(), "Config reloaded successfully")
	return nil
}

// ToFlowRules 将配置转换为Sentinel流控规则
func (rc *ResourceConfig) ToFlowRules() []*flow.Rule {
	if !rc.Enabled || !rc.FlowRule.Enabled {
		return nil
	}

	return []*flow.Rule{
		{
			Resource:               rc.Name,
			TokenCalculateStrategy: flow.Direct,
			ControlBehavior:        getControlBehavior(rc.FlowRule.ControlBehavior),
			Threshold:              rc.FlowRule.Threshold,
			MaxQueueingTimeMs:      uint32(rc.FlowRule.MaxQueueingTimeMs),
			StatIntervalInMs:       1000,
		},
	}
}

// ToCircuitBreakerRules 将配置转换为Sentinel熔断规则
func (rc *ResourceConfig) ToCircuitBreakerRules(globalConfig SentinelConfig) []*circuitbreaker.Rule {
	if !rc.Enabled || !rc.CircuitBreakerRule.Enabled {
		return nil
	}

	strategy := circuitbreaker.SlowRequestRatio
	switch rc.CircuitBreakerRule.Strategy {
	case "slow_request_ratio":
		strategy = circuitbreaker.SlowRequestRatio
	case "error_ratio":
		strategy = circuitbreaker.ErrorRatio
	case "error_count":
		strategy = circuitbreaker.ErrorCount
	default:
		strategy = circuitbreaker.SlowRequestRatio
	}

	retryTimeoutMs := globalConfig.Sentinel.GlobalCircuitBreaker.RetryTimeoutMs
	if retryTimeoutMs == 0 {
		retryTimeoutMs = 5000 // 默认5秒
	}

	minRequestAmount := rc.CircuitBreakerRule.MinRequestAmount
	if minRequestAmount == 0 {
		minRequestAmount = globalConfig.Sentinel.GlobalCircuitBreaker.MinRequestAmount
		if minRequestAmount == 0 {
			minRequestAmount = 10 // 默认10次请求
		}
	}

	statIntervalMs := rc.CircuitBreakerRule.StatIntervalMs
	if statIntervalMs == 0 {
		statIntervalMs = globalConfig.Sentinel.GlobalCircuitBreaker.StatIntervalMs
		if statIntervalMs == 0 {
			statIntervalMs = 5000 // 默认5秒
		}
	}

	return []*circuitbreaker.Rule{
		{
			Resource:         rc.Name,
			Strategy:         strategy,
			RetryTimeoutMs:   retryTimeoutMs,
			MinRequestAmount: minRequestAmount,
			StatIntervalMs:   statIntervalMs,
			MaxAllowedRtMs:   rc.CircuitBreakerRule.MaxAllowedRtMs,
			Threshold:        rc.CircuitBreakerRule.ErrorRatioThreshold,
		},
	}
}

// getControlBehavior 将字符串控制行为转换为Sentinel控制行为
func getControlBehavior(behavior string) flow.ControlBehavior {
	switch behavior {
	case "reject":
		return flow.Reject
	case "throttle":
		return flow.Throttling
	default:
		return flow.Reject
	}
}
