/*
Copyright 2024 openapi-operator-gen authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package telemetry

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestInitProviderFromEnv_NoEndpoint(t *testing.T) {
	// Ensure OTEL_EXPORTER_OTLP_ENDPOINT is not set
	os.Unsetenv("OTEL_EXPORTER_OTLP_ENDPOINT")

	ctx := context.Background()
	provider, err := InitProviderFromEnv(ctx, "test-service", "v1.0.0")

	if err != nil {
		t.Errorf("InitProviderFromEnv() unexpected error: %v", err)
	}
	if provider != nil {
		t.Errorf("InitProviderFromEnv() expected nil provider when endpoint not set, got %v", provider)
	}
}

func TestInitProviderFromEnv_WithEndpoint(t *testing.T) {
	// Save original env and restore after test
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	origServiceName := os.Getenv("OTEL_SERVICE_NAME")
	origInsecure := os.Getenv("OTEL_INSECURE")
	defer func() {
		restoreEnv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)
		restoreEnv("OTEL_SERVICE_NAME", origServiceName)
		restoreEnv("OTEL_INSECURE", origInsecure)
	}()

	// Set test environment
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	os.Setenv("OTEL_INSECURE", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider, err := InitProviderFromEnv(ctx, "test-service", "v1.0.0")
	if err != nil {
		t.Fatalf("InitProviderFromEnv() unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("InitProviderFromEnv() expected non-nil provider")
	}

	// Verify providers are created
	if provider.TracerProvider == nil {
		t.Error("InitProviderFromEnv() TracerProvider is nil")
	}
	if provider.MeterProvider == nil {
		t.Error("InitProviderFromEnv() MeterProvider is nil")
	}

	// Clean up - ignore connection errors since no collector is running
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = provider.Shutdown(shutdownCtx)
}

func TestInitProviderFromEnv_ServiceNameOverride(t *testing.T) {
	// Save original env and restore after test
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	origServiceName := os.Getenv("OTEL_SERVICE_NAME")
	origInsecure := os.Getenv("OTEL_INSECURE")
	defer func() {
		restoreEnv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)
		restoreEnv("OTEL_SERVICE_NAME", origServiceName)
		restoreEnv("OTEL_INSECURE", origInsecure)
	}()

	// Set test environment with service name override
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	os.Setenv("OTEL_SERVICE_NAME", "overridden-service")
	os.Setenv("OTEL_INSECURE", "true")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider, err := InitProviderFromEnv(ctx, "default-service", "v1.0.0")
	if err != nil {
		t.Fatalf("InitProviderFromEnv() unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("InitProviderFromEnv() expected non-nil provider")
	}

	// Clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	provider.Shutdown(shutdownCtx)
}

func TestInitProvider(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := Config{
		Endpoint:       "localhost:4317",
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Insecure:       true,
	}

	provider, err := InitProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("InitProvider() unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("InitProvider() expected non-nil provider")
	}

	if provider.TracerProvider == nil {
		t.Error("InitProvider() TracerProvider is nil")
	}
	if provider.MeterProvider == nil {
		t.Error("InitProvider() MeterProvider is nil")
	}

	// Clean up - ignore connection errors since no collector is running
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = provider.Shutdown(shutdownCtx)
}

func TestInitProvider_WithKubernetesEnv(t *testing.T) {
	// Save original env and restore after test
	origPodName := os.Getenv("POD_NAME")
	origPodNamespace := os.Getenv("POD_NAMESPACE")
	defer func() {
		restoreEnv("POD_NAME", origPodName)
		restoreEnv("POD_NAMESPACE", origPodNamespace)
	}()

	// Set Kubernetes env vars
	os.Setenv("POD_NAME", "test-pod-abc123")
	os.Setenv("POD_NAMESPACE", "test-namespace")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := Config{
		Endpoint:       "localhost:4317",
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Insecure:       true,
	}

	provider, err := InitProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("InitProvider() unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("InitProvider() expected non-nil provider")
	}

	// Clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	provider.Shutdown(shutdownCtx)
}

func TestProvider_Shutdown_NilProviders(t *testing.T) {
	provider := &Provider{
		TracerProvider: nil,
		MeterProvider:  nil,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := provider.Shutdown(ctx)
	if err != nil {
		t.Errorf("Provider.Shutdown() with nil providers should not error, got: %v", err)
	}
}

func TestProvider_Shutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := Config{
		Endpoint:       "localhost:4317",
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Insecure:       true,
	}

	provider, err := InitProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("InitProvider() unexpected error: %v", err)
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	// Shutdown may return connection errors since no collector is running - that's ok
	_ = provider.Shutdown(shutdownCtx)
}

func TestConfig_Fields(t *testing.T) {
	cfg := Config{
		Endpoint:       "collector:4317",
		ServiceName:    "my-service",
		ServiceVersion: "v2.0.0",
		Insecure:       true,
	}

	if cfg.Endpoint != "collector:4317" {
		t.Errorf("Config.Endpoint = %q, want %q", cfg.Endpoint, "collector:4317")
	}
	if cfg.ServiceName != "my-service" {
		t.Errorf("Config.ServiceName = %q, want %q", cfg.ServiceName, "my-service")
	}
	if cfg.ServiceVersion != "v2.0.0" {
		t.Errorf("Config.ServiceVersion = %q, want %q", cfg.ServiceVersion, "v2.0.0")
	}
	if cfg.Insecure != true {
		t.Errorf("Config.Insecure = %v, want %v", cfg.Insecure, true)
	}
}

func TestInitProvider_SecureConnection(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := Config{
		Endpoint:       "localhost:4317",
		ServiceName:    "test-service",
		ServiceVersion: "v1.0.0",
		Insecure:       false, // Secure connection
	}

	provider, err := InitProvider(ctx, cfg)
	if err != nil {
		t.Fatalf("InitProvider() unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("InitProvider() expected non-nil provider")
	}

	// Clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	provider.Shutdown(shutdownCtx)
}

func TestInitProviderFromEnv_InsecureFalse(t *testing.T) {
	// Save original env and restore after test
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	origInsecure := os.Getenv("OTEL_INSECURE")
	defer func() {
		restoreEnv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)
		restoreEnv("OTEL_INSECURE", origInsecure)
	}()

	// Set test environment with insecure=false
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	os.Setenv("OTEL_INSECURE", "false")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider, err := InitProviderFromEnv(ctx, "test-service", "v1.0.0")
	if err != nil {
		t.Fatalf("InitProviderFromEnv() unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("InitProviderFromEnv() expected non-nil provider")
	}

	// Clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	provider.Shutdown(shutdownCtx)
}

func TestInitProviderFromEnv_InsecureNotSet(t *testing.T) {
	// Save original env and restore after test
	origEndpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	origInsecure := os.Getenv("OTEL_INSECURE")
	defer func() {
		restoreEnv("OTEL_EXPORTER_OTLP_ENDPOINT", origEndpoint)
		restoreEnv("OTEL_INSECURE", origInsecure)
	}()

	// Set test environment without insecure flag
	os.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:4317")
	os.Unsetenv("OTEL_INSECURE")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	provider, err := InitProviderFromEnv(ctx, "test-service", "v1.0.0")
	if err != nil {
		t.Fatalf("InitProviderFromEnv() unexpected error: %v", err)
	}
	if provider == nil {
		t.Fatal("InitProviderFromEnv() expected non-nil provider")
	}

	// Clean up
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	provider.Shutdown(shutdownCtx)
}

// restoreEnv restores an environment variable to its original value
func restoreEnv(key, value string) {
	if value != "" {
		os.Setenv(key, value)
	} else {
		os.Unsetenv(key)
	}
}
