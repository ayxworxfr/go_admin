package config

// OpenTelemetryConfig 存储OpenTelemetry相关配置
type OpenTelemetryConfig struct {
	Enable   bool    `yaml:"enable"`   // 是否启用
	Service  string  `yaml:"service"`  // 服务名称
	Endpoint string  `yaml:"endpoint"` // Jaeger上报地址
	Protocol string  `yaml:"protocol"` // 上报协议
	Sampling float64 `yaml:"sampling"` // 采样率（0.0-1.0）
	Timeout  int     `yaml:"timeout"`  // 超时时间（秒）
}

// 默认OpenTelemetry配置
func NewOpenTelemetryConfig() OpenTelemetryConfig {
	return OpenTelemetryConfig{
		Enable:   false,            // 默认不启用
		Service:  "hertz-service",  // 默认服务名
		Endpoint: "localhost:4317", // 默认Jaeger地址
		Protocol: "grpc",           // 默认上报协议
		Sampling: 0.1,              // 默认10%采样率
		Timeout:  3,                // 默认超时3秒
	}
}
