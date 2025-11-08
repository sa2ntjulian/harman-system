package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	oteltrace "go.opentelemetry.io/otel/trace"
)

var (
	tracer  oteltrace.Tracer
	meter   metric.Meter
	shutdown func(context.Context) error
)

func InitTelemetry(serviceName, nodeName string) error {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			attribute.String("service.name", serviceName),
			attribute.String("service.namespace", "harman-system"),
			attribute.String("k8s.node.name", nodeName),
		),
	)
	if err != nil {
		return fmt.Errorf("리소스 생성 실패: %w", err)
	}

	otlpEndpoint := getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", "opentelemetry-collector.monitoring.svc.cluster.local:4317")

	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("트레이스 익스포터 생성 실패: %w", err)
	}

	metricExporter, err := otlpmetricgrpc.New(ctx,
		otlpmetricgrpc.WithEndpoint(otlpEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("메트릭 익스포터 생성 실패: %w", err)
	}

	tp := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)
	otel.SetTracerProvider(tp)

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExporter,
			sdkmetric.WithInterval(30*time.Second))),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(mp)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	tracer = otel.Tracer(serviceName)
	meter = otel.Meter(serviceName)

	shutdown = func(ctx context.Context) error {
		var errs []error
		if err := tp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("트레이스 프로바이더 종료 실패: %w", err))
		}
		if err := mp.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("메트릭 프로바이더 종료 실패: %w", err))
		}
		if len(errs) > 0 {
			return fmt.Errorf("종료 중 오류 발생: %v", errs)
		}
		return nil
	}

	slog.Info("OpenTelemetry 초기화 완료",
		"service_name", serviceName,
		"node_name", nodeName,
		"otlp_endpoint", otlpEndpoint,
	)

	return nil
}

func Shutdown(ctx context.Context) error {
	if shutdown != nil {
		return shutdown(ctx)
	}
	return nil
}

func Tracer() oteltrace.Tracer {
	return tracer
}

func Meter() metric.Meter {
	return meter
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

