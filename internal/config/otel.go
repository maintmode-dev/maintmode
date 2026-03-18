package config

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/ruko1202/xlog"
	"github.com/ruko1202/xlog/xfield"
)

func InitTracerResource() (*resource.Resource, error) {
	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceNameKey.String(GetAppBuildMeta().AppName),
			semconv.ServiceVersionKey.String(GetAppBuildMeta().Version),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tracer resource: %w", err)
	}

	return res, nil
}

// InitTracerProvider sets up everything: trace exporter and pull-based metric exporter
func InitTracerProvider(ctx context.Context, res *resource.Resource) (*sdktrace.TracerProvider, error) {
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger := xlog.LoggerFromContext(ctx)
		logger.Error("ALERT: Internal OpenTelemetry error", xfield.Error(err))
	}))

	// --- TRACES ---
	// gRPC exporter to OTel Collector
	traceExporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(fmt.Sprintf("%s:%d", GetAppConfig().Tracer.CollectorHost, GetAppConfig().Tracer.CollectorPort)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}
	bsp := sdktrace.NewBatchSpanProcessor(
		traceExporter,
		sdktrace.WithMaxQueueSize(sdktrace.DefaultMaxQueueSize),
		sdktrace.WithMaxExportBatchSize(sdktrace.DefaultMaxExportBatchSize),
	)

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(bsp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tracerProvider)

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tracerProvider, nil
}

// InitMetricExporter sets up pull-based metric exporter.
func InitMetricExporter(res *resource.Resource) (*sdkmetric.MeterProvider, error) {
	// --- METRICS ---
	// Prometheus exporter (exposes /metrics for scraping)
	metricExporter, err := prometheus.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}
	meterProvider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(metricExporter),
		sdkmetric.WithResource(res),
	)
	otel.SetMeterProvider(meterProvider)

	return meterProvider, nil
}
