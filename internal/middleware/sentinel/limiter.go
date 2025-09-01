package sentinel

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/ayxworxfr/go_admin/pkg/logger"
	"github.com/cloudwego/hertz/pkg/app"
	"github.com/go-redis/redis/v8"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

// RateLimiterConfig 限流配置
type RateLimiterConfig struct {
	RefreshInterval time.Duration // 清理间隔
	ExpiryTime      time.Duration // 过期时间
	EnableMetrics   bool          // 是否启用指标收集
}

// IPRateLimiterMiddleware 基于IP的限流中间件
func IPRateLimiterMiddleware(rps, burst int, cfg *RateLimiterConfig) app.HandlerFunc {
	if cfg == nil {
		cfg = &RateLimiterConfig{
			RefreshInterval: 10 * time.Minute,
			ExpiryTime:      30 * time.Minute,
			EnableMetrics:   false,
		}
	}

	limiterMap := make(map[string]*rate.Limiter)
	lastSeen := make(map[string]time.Time)
	mu := &sync.RWMutex{}

	// 指标记录器
	var (
		requestCounter metric.Int64Counter
		blockedCounter metric.Int64Counter
		initOnce       sync.Once
	)

	// 启动清理器，移除过期的limiter
	go func() {
		ticker := time.NewTicker(cfg.RefreshInterval)
		defer ticker.Stop()
		for range ticker.C {
			mu.Lock()
			now := time.Now()
			for ip, last := range lastSeen {
				if now.Sub(last) > cfg.ExpiryTime {
					delete(limiterMap, ip)
					delete(lastSeen, ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(ctx context.Context, c *app.RequestContext) {
		start := time.Now()
		ip := c.ClientIP()
		path := string(c.Path())

		// 初始化指标（延迟初始化）
		initOnce.Do(func() {
			if !cfg.EnableMetrics {
				return
			}

			meter := otel.GetMeterProvider().Meter("hertz-middleware")

			var err error
			requestCounter, err = meter.Int64Counter(
				"rate_limiter.requests",
				metric.WithDescription("Total number of requests"),
			)
			if err != nil {
				logger.Error(ctx, "Failed to create request counter", zap.Error(err))
			}

			blockedCounter, err = meter.Int64Counter(
				"rate_limiter.blocked",
				metric.WithDescription("Total number of blocked requests"),
			)
			if err != nil {
				logger.Error(ctx, "Failed to create blocked counter", zap.Error(err))
			}
		})

		// 快速读取检查
		mu.RLock()
		limiter, exists := limiterMap[ip]
		mu.RUnlock()

		if !exists {
			// 获取写锁创建新限流器
			mu.Lock()
			// 双重检查
			if limiter, exists = limiterMap[ip]; !exists {
				limiter = rate.NewLimiter(rate.Limit(rps), burst)
				limiterMap[ip] = limiter
				lastSeen[ip] = start

				logger.FromContext(ctx).Info("New rate limiter created",
					zap.String("ip", ip),
					zap.Int("rps", rps),
					zap.Int("burst", burst))
			}
			mu.Unlock()
		} else {
			// 异步更新最后访问时间，减少锁持有时间
			go func() {
				mu.Lock()
				lastSeen[ip] = start
				mu.Unlock()
			}()
		}

		// 执行限流检查
		if !limiter.Allow() {
			logger.FromContext(ctx).Warn("Request blocked by rate limiter",
				zap.String("ip", ip),
				zap.String("path", path))

			// 记录限流指标
			if cfg.EnableMetrics && requestCounter != nil && blockedCounter != nil {
				requestCounter.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("ip", ip),
						attribute.String("path", path),
						attribute.Bool("blocked", true),
					),
				)
				blockedCounter.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("ip", ip),
						attribute.String("path", path),
					),
				)
			}

			c.AbortWithStatusJSON(http.StatusTooManyRequests, map[string]string{
				"code":    "429",
				"message": "Too many requests, please try again later",
			})
			return
		}

		// 记录正常请求指标
		if cfg.EnableMetrics && requestCounter != nil {
			requestCounter.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("ip", ip),
					attribute.String("path", path),
					attribute.Bool("blocked", false),
				),
			)
		}

		c.Next(ctx)
	}
}

// RedisClusterRateLimiter 基于Redis集群的分布式限流中间件
func RedisClusterRateLimiter(client *redis.ClusterClient, rps, burst int, keyPrefix string, enableMetrics bool) app.HandlerFunc {
	// 预编译Lua脚本
	script := redis.NewScript(`
		local key = KEYS[1]
		local rate = tonumber(ARGV[1])
		local capacity = tonumber(ARGV[2])
		local now = tonumber(ARGV[3])
		local requested = tonumber(ARGV[4])
		
		local fill_time = capacity / rate
		local ttl = math.floor(fill_time * 2)
		
		local last_tokens = tonumber(redis.call('get', key))
		if last_tokens == nil then
			last_tokens = capacity
		end
		
		local last_refreshed = tonumber(redis.call('get', key .. ':ts'))
		if last_refreshed == nil then
			last_refreshed = 0
		end
		
		local delta = math.max(0, now - last_refreshed)
		local filled_tokens = math.min(capacity, last_tokens + (delta * rate))
		local allowed = filled_tokens >= requested
		local new_tokens = filled_tokens
		if allowed then
			new_tokens = filled_tokens - requested
		end
		
		redis.call('set', key, new_tokens)
		redis.call('set', key .. ':ts', now)
		redis.call('pexpire', key, ttl * 1000)
		redis.call('pexpire', key .. ':ts', ttl * 1000)
		
		return allowed
	`)

	// 指标记录器
	var (
		requestCounter metric.Int64Counter
		blockedCounter metric.Int64Counter
		errorCounter   metric.Int64Counter
		initOnce       sync.Once
	)

	return func(ctx context.Context, c *app.RequestContext) {
		ip := c.ClientIP()
		path := string(c.Path())
		key := fmt.Sprintf("%s:%s:%s", keyPrefix, ip, path)
		now := time.Now().UnixNano() / int64(time.Second)

		// 初始化指标（延迟初始化）
		initOnce.Do(func() {
			if !enableMetrics {
				return
			}

			meter := otel.GetMeterProvider().Meter("hertz-middleware")

			var err error
			requestCounter, err = meter.Int64Counter(
				"rate_limiter.requests",
				metric.WithDescription("Total number of requests"),
			)
			if err != nil {
				logger.Error(ctx, "Failed to create request counter", zap.Error(err))
			}

			blockedCounter, err = meter.Int64Counter(
				"rate_limiter.blocked",
				metric.WithDescription("Total number of blocked requests"),
			)
			if err != nil {
				logger.Error(ctx, "Failed to create blocked counter", zap.Error(err))
			}

			errorCounter, err = meter.Int64Counter(
				"rate_limiter.errors",
				metric.WithDescription("Total number of errors"),
			)
			if err != nil {
				logger.Error(ctx, "Failed to create error counter", zap.Error(err))
			}
		})

		// 执行Lua脚本进行限流检查
		result, err := script.Run(ctx, client, []string{key}, rps, burst, now, 1).Result()
		if err != nil {
			logger.FromContext(ctx).Error("Distributed rate limiting evaluation failed",
				zap.Error(err),
				zap.String("ip", ip),
				zap.String("path", path))

			// 记录错误指标
			if errorCounter != nil {
				errorCounter.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("ip", ip),
						attribute.String("path", path),
					),
				)
			}

			// 发生错误时允许请求通过，避免单点故障
			c.Next(ctx)
			return
		}

		// 检查是否允许请求
		allowed, ok := result.(int64)
		if !ok || allowed == 0 {
			logger.FromContext(ctx).Warn("Request blocked by distributed rate limiter",
				zap.String("ip", ip),
				zap.String("path", path))

			// 记录限流指标
			if requestCounter != nil && blockedCounter != nil {
				requestCounter.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("ip", ip),
						attribute.String("path", path),
						attribute.Bool("blocked", true),
					),
				)
				blockedCounter.Add(ctx, 1,
					metric.WithAttributes(
						attribute.String("ip", ip),
						attribute.String("path", path),
					),
				)
			}

			c.AbortWithStatusJSON(http.StatusTooManyRequests, map[string]string{
				"code":    "429",
				"message": "Too many requests, please try again later",
			})
			return
		}

		// 记录正常请求指标
		if requestCounter != nil {
			requestCounter.Add(ctx, 1,
				metric.WithAttributes(
					attribute.String("ip", ip),
					attribute.String("path", path),
					attribute.Bool("blocked", false),
				),
			)
		}

		c.Next(ctx)
	}
}
