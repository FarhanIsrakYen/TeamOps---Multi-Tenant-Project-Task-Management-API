package observability

import (
	"context"
	"fmt"
	"net/url"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type TraceConfig struct {
	Endpoint    string
	Insecure    bool
	ServiceName string
	Environment string
	SampleRatio float64
}

type TraceShutdown func(context.Context) error

// SetupTracing configures the process-wide OpenTelemetry provider. An empty
// endpoint intentionally leaves the default no-op provider in place.
func SetupTracing(ctx context.Context, config TraceConfig) (TraceShutdown, error) {
	if config.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}
	endpoint, err := url.Parse(config.Endpoint)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "http" && endpoint.Scheme != "https") {
		return nil, fmt.Errorf("OTLP endpoint must be an absolute HTTP(S) URL")
	}
	options := []otlptracegrpc.Option{otlptracegrpc.WithEndpointURL(config.Endpoint)}
	if config.Insecure || endpoint.Scheme == "http" {
		options = append(options, otlptracegrpc.WithInsecure())
	}
	exporter, err := otlptracegrpc.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("create OTLP trace exporter: %w", err)
	}
	if config.ServiceName == "" {
		config.ServiceName = "teamops-api"
	}
	if config.SampleRatio < 0 || config.SampleRatio > 1 {
		config.SampleRatio = 1
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(
		attribute.String("service.name", config.ServiceName),
		attribute.String("deployment.environment.name", config.Environment),
	))
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, fmt.Errorf("create OpenTelemetry resource: %w", err)
	}
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(config.SampleRatio))),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return provider.Shutdown, nil
}
