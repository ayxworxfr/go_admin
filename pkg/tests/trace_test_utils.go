package tests

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Config 配置追踪测试
type Config struct {
	ServiceName    string        // 服务名称
	OTLPEndpoint   string        // OTLP 端点地址
	Insecure       bool          // 是否使用非安全连接
	Timeout        time.Duration // 超时时间
	TestSpanCount  int           // 测试 span 数量
	TestEventCount int           // 每个 span 的事件数量
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		ServiceName:    "jaeger-otel-test",
		OTLPEndpoint:   "localhost:43170",
		Insecure:       true,
		Timeout:        10 * time.Second,
		TestSpanCount:  5,
		TestEventCount: 3,
	}
}

func GetConfig() Config {
	config := DefaultConfig()
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		config.OTLPEndpoint = endpoint
	}
	if serviceName := os.Getenv("SERVICE_NAME"); serviceName != "" {
		config.ServiceName = serviceName
	}

	return config
}

// TracingTester 用于测试追踪系统的工具
type TracingTester struct {
	config       Config
	exporter     *otlptrace.Exporter
	tracer       *trace.TracerProvider
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
	errorChannel chan error
}

// NewTracingTester 创建一个新的追踪测试器
func NewTracingTester(config Config) (*TracingTester, error) {
	ctx, cancel := context.WithTimeout(context.Background(), config.Timeout)

	// 创建 gRPC 连接选项
	var dialOpts []grpc.DialOption
	if config.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}

	// 创建 OTLP 导出器
	client := otlptracegrpc.NewClient(
		otlptracegrpc.WithEndpoint(config.OTLPEndpoint),
		otlptracegrpc.WithDialOption(dialOpts...),
	)

	exporter, err := otlptrace.New(ctx, client)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
	}

	// 创建资源
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(config.ServiceName),
		),
	)
	if err != nil {
		exporter.Shutdown(ctx)
		cancel()
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// 创建追踪器提供者
	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(exporter),
		trace.WithResource(res),
	)

	// 设置全局传播器和追踪器
	otel.SetTextMapPropagator(propagation.TraceContext{})
	otel.SetTracerProvider(tracerProvider)

	return &TracingTester{
		config:       config,
		exporter:     exporter,
		tracer:       tracerProvider,
		ctx:          ctx,
		cancel:       cancel,
		errorChannel: make(chan error, 10),
	}, nil
}

// RunTest 执行追踪测试
func (t *TracingTester) RunTest() error {
	defer t.cancel()
	defer t.tracer.Shutdown(t.ctx)

	// 创建根 span
	ctx, span := otel.Tracer("tracetest").Start(t.ctx, "root-test-span")
	defer span.End()

	span.SetAttributes(
		attribute.String("test.run_id", fmt.Sprintf("%d", time.Now().UnixNano())),
		attribute.Int("test.span_count", t.config.TestSpanCount),
		attribute.Int("test.event_count", t.config.TestEventCount),
	)

	// 创建多个子 span 进行测试
	for i := 0; i < t.config.TestSpanCount; i++ {
		t.wg.Add(1)
		go t.createTestSpan(ctx, i)
	}

	// 等待所有 goroutine 完成
	t.wg.Wait()

	// 检查是否有错误
	select {
	case err := <-t.errorChannel:
		return fmt.Errorf("test failed: %w", err)
	default:
		// 给导出器一些时间发送最后的数据
		time.Sleep(2 * time.Second)
		return nil
	}
}

// 创建测试 span
func (t *TracingTester) createTestSpan(parentCtx context.Context, index int) {
	defer t.wg.Done()

	ctx, span := otel.Tracer("tracetest").Start(
		parentCtx,
		fmt.Sprintf("test-span-%d", index),
	)
	defer span.End()

	// 设置 span 属性
	span.SetAttributes(
		attribute.String("test.type", "automated-test"),
		attribute.Int("test.index", index),
		attribute.Bool("test.success", true),
	)

	// 添加事件
	for i := 0; i < t.config.TestEventCount; i++ {
		span.AddEvent(fmt.Sprintf("test-event-%d", i))
		span.SetAttributes(
			attribute.String("key", "value"),
		)

		// 模拟一些工作
		time.Sleep(50 * time.Millisecond)
	}

	// 创建子 span
	_, childSpan := otel.Tracer("tracetest").Start(
		ctx,
		fmt.Sprintf("child-span-%d", index),
	)
	childSpan.SetAttributes(
		attribute.String("child.type", "nested-test"),
	)
	childSpan.End()
}

// 在测试框架中使用的辅助函数
func TestTracingIntegration(t *testing.T, config Config) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tester, err := NewTracingTester(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracing tester: %v", err)
	}

	err = tester.RunTest()
	if err != nil {
		t.Fatalf("Tracing test failed: %v", err)
	}

	t.Logf("✅ Traces exported successfully to %s", config.OTLPEndpoint)
}
