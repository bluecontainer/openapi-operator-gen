/*
Copyright 2024 openapi-operator-gen authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package telemetry

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// Config holds OpenTelemetry configuration
type Config struct {
	// Endpoint is the OTLP collector endpoint (e.g., "localhost:4317")
	Endpoint string
	// ServiceName is the name of this service
	ServiceName string
	// ServiceVersion is the version of this service
	ServiceVersion string
	// Insecure disables TLS for the OTLP connection
	Insecure bool
}

// Provider wraps the OpenTelemetry providers for cleanup
type Provider struct {
	TracerProvider *trace.TracerProvider
	MeterProvider  *metric.MeterProvider
}

// Shutdown gracefully shuts down the providers
func (p *Provider) Shutdown(ctx context.Context) error {
	var errs []error

	if p.TracerProvider != nil {
		if err := p.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("tracer provider shutdown: %w", err))
		}
	}

	if p.MeterProvider != nil {
		if err := p.MeterProvider.Shutdown(ctx); err != nil {
			errs = append(errs, fmt.Errorf("meter provider shutdown: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

// InitProvider initializes OpenTelemetry with the given configuration
func InitProvider(ctx context.Context, cfg Config) (*Provider, error) {
	// Build attributes list
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
		semconv.ServiceVersion(cfg.ServiceVersion),
	}

	// Add Kubernetes attributes if available
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		attrs = append(attrs, semconv.K8SNamespaceName(ns))
	}
	if podName := os.Getenv("POD_NAME"); podName != "" {
		attrs = append(attrs, semconv.K8SPodName(podName))
	}

	// Create resource with service information
	res := resource.NewWithAttributes(semconv.SchemaURL, attrs...)

	provider := &Provider{}

	// Initialize trace exporter
	traceOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		traceOpts = append(traceOpts, otlptracegrpc.WithInsecure())
	}

	traceExporter, err := otlptracegrpc.New(ctx, traceOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Create tracer provider
	provider.TracerProvider = trace.NewTracerProvider(
		trace.WithBatcher(traceExporter,
			trace.WithBatchTimeout(5*time.Second),
		),
		trace.WithResource(res),
		trace.WithSampler(trace.AlwaysSample()),
	)

	// Set global tracer provider
	otel.SetTracerProvider(provider.TracerProvider)

	// Set global propagator
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Initialize metric exporter
	metricOpts := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
	}
	if cfg.Insecure {
		metricOpts = append(metricOpts, otlpmetricgrpc.WithInsecure())
	}

	metricExporter, err := otlpmetricgrpc.New(ctx, metricOpts...)
	if err != nil {
		// Clean up tracer provider before returning error
		provider.TracerProvider.Shutdown(ctx)
		return nil, fmt.Errorf("failed to create metric exporter: %w", err)
	}

	// Create meter provider
	provider.MeterProvider = metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter,
			metric.WithInterval(30*time.Second),
		)),
		metric.WithResource(res),
	)

	// Set global meter provider
	otel.SetMeterProvider(provider.MeterProvider)

	return provider, nil
}

// InitProviderFromEnv initializes OpenTelemetry from environment variables.
// Environment variables:
//   - OTEL_EXPORTER_OTLP_ENDPOINT: OTLP collector endpoint
//   - OTEL_SERVICE_NAME: Service name (optional, defaults to provided serviceName)
//   - OTEL_INSECURE: Set to "true" to disable TLS
//
// Returns nil provider (no-op) if OTEL_EXPORTER_OTLP_ENDPOINT is not set.
func InitProviderFromEnv(ctx context.Context, serviceName, serviceVersion string) (*Provider, error) {
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		// OpenTelemetry not configured, return nil provider (no-op)
		return nil, nil
	}

	if envServiceName := os.Getenv("OTEL_SERVICE_NAME"); envServiceName != "" {
		serviceName = envServiceName
	}

	insecure := os.Getenv("OTEL_INSECURE") == "true"

	return InitProvider(ctx, Config{
		Endpoint:       endpoint,
		ServiceName:    serviceName,
		ServiceVersion: serviceVersion,
		Insecure:       insecure,
	})
}
