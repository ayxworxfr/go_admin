package app

import (
	"context"
	"fmt"

	"github.com/ayxworxfr/go_admin/internal/config"
	"github.com/ayxworxfr/go_admin/pkg/logger"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0" // 确保版本匹配
)

type Shutdownable interface {
	Shutdown(context.Context) error
}

func InitOpenTelemetry(cfg config.OpenTelemetryConfig) (Shutdownable, error) {
	if !cfg.Enable {
		return nil, nil
	}

	ctx := context.Background()
	var exporter trace.SpanExporter
	var err error
	var protocol string

	// 根据配置选择协议和编码器
	switch cfg.Protocol {
	case "grpc":
		protocol = "grpc"
		client := otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
			otlptracegrpc.WithInsecure(),
		)
		exporter, err = otlptrace.New(ctx, client)
	default: // http/protobuf
		protocol = "http/protobuf"
		client := otlptracehttp.NewClient(
			otlptracehttp.WithEndpoint(cfg.Endpoint),
			otlptracehttp.WithInsecure(),
		)
		exporter, err = otlptrace.New(ctx, client)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// 创建资源
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(cfg.Service),
			semconv.ServiceVersion("1.0.0"), // 可选：添加服务版本
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 创建采样器
	sampler := trace.TraceIDRatioBased(cfg.Sampling)

	// 创建追踪提供器
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(exporter), // 使用批量导出提高性能
		trace.WithResource(res),
		trace.WithSampler(sampler),
	)

	// 设置全局传播器（支持TraceContext和Baggage）
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// 设置全局追踪器
	otel.SetTracerProvider(tracerProvider)

	logger.Infof(context.Background(), "OpenTelemetry initialized: service=%s, endpoint=%s, protocol=%s, sampling=%.2f",
		cfg.Service, cfg.Endpoint, protocol, cfg.Sampling)

	return tracerProvider, nil
}
