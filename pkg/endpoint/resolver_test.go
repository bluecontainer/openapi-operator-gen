/*
Copyright 2024 openapi-operator-gen authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package endpoint

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// =============================================================================
// Configuration Tests
// =============================================================================

func TestNewResolver_DefaultValues(t *testing.T) {
	cfg := Config{
		StatefulSetName: "my-sts",
		Namespace:       "default",
		Port:            8080,
	}

	resolver := NewResolver(nil, cfg)

	if resolver.config.RefreshInterval != 30*time.Second {
		t.Errorf("expected RefreshInterval 30s, got %v", resolver.config.RefreshInterval)
	}
	if resolver.config.HealthCheckInterval != 10*time.Second {
		t.Errorf("expected HealthCheckInterval 10s, got %v", resolver.config.HealthCheckInterval)
	}
	if resolver.config.Scheme != "http" {
		t.Errorf("expected Scheme 'http', got %s", resolver.config.Scheme)
	}
	if resolver.config.Strategy != RoundRobin {
		t.Errorf("expected Strategy RoundRobin, got %s", resolver.config.Strategy)
	}
	if resolver.config.WorkloadKind != AutoKind {
		t.Errorf("expected WorkloadKind AutoKind, got %s", resolver.config.WorkloadKind)
	}
	if resolver.config.DiscoveryMode != DNSMode {
		t.Errorf("expected DiscoveryMode DNSMode for StatefulSet, got %s", resolver.config.DiscoveryMode)
	}
}

func TestNewResolver_DeploymentDefaults(t *testing.T) {
	cfg := Config{
		DeploymentName: "my-deploy",
		Namespace:      "default",
		Port:           8080,
	}

	resolver := NewResolver(nil, cfg)

	if resolver.config.DiscoveryMode != PodIPMode {
		t.Errorf("expected DiscoveryMode PodIPMode for Deployment, got %s", resolver.config.DiscoveryMode)
	}
}

func TestNewResolver_DeploymentStrategyFallback(t *testing.T) {
	tests := []struct {
		name             string
		inputStrategy    Strategy
		expectedStrategy Strategy
	}{
		{"LeaderOnly falls back to RoundRobin", LeaderOnly, RoundRobin},
		{"ByOrdinal falls back to RoundRobin", ByOrdinal, RoundRobin},
		{"RoundRobin stays RoundRobin", RoundRobin, RoundRobin},
		{"AnyHealthy stays AnyHealthy", AnyHealthy, AnyHealthy},
		{"AllHealthy stays AllHealthy", AllHealthy, AllHealthy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				DeploymentName: "my-deploy",
				Namespace:      "default",
				Port:           8080,
				Strategy:       tt.inputStrategy,
			}

			resolver := NewResolver(nil, cfg)

			if resolver.config.Strategy != tt.expectedStrategy {
				t.Errorf("expected Strategy %s, got %s", tt.expectedStrategy, resolver.config.Strategy)
			}
		})
	}
}

func TestNewResolver_CustomValues(t *testing.T) {
	cfg := Config{
		StatefulSetName:     "my-sts",
		Namespace:           "custom-ns",
		Port:                9090,
		Scheme:              "https",
		Strategy:            LeaderOnly,
		DiscoveryMode:       PodIPMode,
		RefreshInterval:     60 * time.Second,
		HealthCheckInterval: 30 * time.Second,
		BasePath:            "/api/v1",
	}

	resolver := NewResolver(nil, cfg)

	if resolver.config.RefreshInterval != 60*time.Second {
		t.Errorf("expected RefreshInterval 60s, got %v", resolver.config.RefreshInterval)
	}
	if resolver.config.HealthCheckInterval != 30*time.Second {
		t.Errorf("expected HealthCheckInterval 30s, got %v", resolver.config.HealthCheckInterval)
	}
	if resolver.config.Scheme != "https" {
		t.Errorf("expected Scheme 'https', got %s", resolver.config.Scheme)
	}
	if resolver.config.Strategy != LeaderOnly {
		t.Errorf("expected Strategy LeaderOnly, got %s", resolver.config.Strategy)
	}
	if resolver.config.DiscoveryMode != PodIPMode {
		t.Errorf("expected DiscoveryMode PodIPMode, got %s", resolver.config.DiscoveryMode)
	}
	if resolver.config.BasePath != "/api/v1" {
		t.Errorf("expected BasePath '/api/v1', got %s", resolver.config.BasePath)
	}
}

// =============================================================================
// Strategy Selection Tests
// =============================================================================

func TestGetEndpoint_RoundRobin(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy: RoundRobin,
			BasePath: "",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
			{URL: "http://pod-1:8080", Healthy: true, PodName: "sts-1"},
			{URL: "http://pod-2:8080", Healthy: true, PodName: "sts-2"},
		},
		counter: 0,
	}

	// Track which endpoints are returned
	counts := make(map[string]int)
	for i := 0; i < 9; i++ {
		url, err := resolver.GetEndpoint()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		counts[url]++
	}

	// Each endpoint should be called 3 times
	for url, count := range counts {
		if count != 3 {
			t.Errorf("expected endpoint %s to be called 3 times, got %d", url, count)
		}
	}
}

func TestGetEndpoint_LeaderOnly(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy:        LeaderOnly,
			StatefulSetName: "my-sts",
			BasePath:        "",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "my-sts-0"},
			{URL: "http://pod-1:8080", Healthy: true, PodName: "my-sts-1"},
			{URL: "http://pod-2:8080", Healthy: true, PodName: "my-sts-2"},
		},
	}

	// Should always return pod-0
	for i := 0; i < 5; i++ {
		url, err := resolver.GetEndpoint()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if url != "http://pod-0:8080" {
			t.Errorf("expected leader endpoint http://pod-0:8080, got %s", url)
		}
	}
}

func TestGetEndpoint_LeaderOnly_UnhealthyLeader(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy:        LeaderOnly,
			StatefulSetName: "my-sts",
			BasePath:        "",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: false, PodName: "my-sts-0"},
			{URL: "http://pod-1:8080", Healthy: true, PodName: "my-sts-1"},
		},
	}

	_, err := resolver.GetEndpoint()
	if err == nil {
		t.Fatal("expected error when leader is unhealthy")
	}
}

func TestGetEndpoint_AnyHealthy(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy: AnyHealthy,
			BasePath: "",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: false, PodName: "sts-0"},
			{URL: "http://pod-1:8080", Healthy: true, PodName: "sts-1"},
			{URL: "http://pod-2:8080", Healthy: true, PodName: "sts-2"},
		},
	}

	url, err := resolver.GetEndpoint()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return first healthy (pod-1)
	if url != "http://pod-1:8080" {
		t.Errorf("expected first healthy endpoint http://pod-1:8080, got %s", url)
	}
}

func TestGetEndpoint_NoHealthyEndpoints(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy: RoundRobin,
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: false, PodName: "sts-0"},
			{URL: "http://pod-1:8080", Healthy: false, PodName: "sts-1"},
		},
	}

	_, err := resolver.GetEndpoint()
	if err == nil {
		t.Fatal("expected error when no healthy endpoints")
	}
}

func TestGetEndpoint_AllHealthyStrategy_Error(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy: AllHealthy,
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
		},
	}

	_, err := resolver.GetEndpoint()
	if err == nil {
		t.Fatal("expected error for AllHealthy strategy")
	}
}

func TestGetEndpoint_ByOrdinalStrategy_Error(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy: ByOrdinal,
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
		},
	}

	_, err := resolver.GetEndpoint()
	if err == nil {
		t.Fatal("expected error for ByOrdinal strategy")
	}
}

func TestGetEndpoint_WithBasePath(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy: AnyHealthy,
			BasePath: "/api/v1",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
		},
	}

	url, err := resolver.GetEndpoint()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url != "http://pod-0:8080/api/v1" {
		t.Errorf("expected URL with basepath http://pod-0:8080/api/v1, got %s", url)
	}
}

// =============================================================================
// GetAllHealthyEndpoints Tests
// =============================================================================

func TestGetAllHealthyEndpoints(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			BasePath: "/api",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
			{URL: "http://pod-1:8080", Healthy: false, PodName: "sts-1"},
			{URL: "http://pod-2:8080", Healthy: true, PodName: "sts-2"},
		},
	}

	eps, err := resolver.GetAllHealthyEndpoints()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(eps) != 2 {
		t.Fatalf("expected 2 healthy endpoints, got %d", len(eps))
	}

	expectedURLs := []string{"http://pod-0:8080/api", "http://pod-2:8080/api"}
	expectedPods := []string{"sts-0", "sts-2"}
	for i, ep := range eps {
		if ep.URL != expectedURLs[i] {
			t.Errorf("endpoint %d: expected URL %s, got %s", i, expectedURLs[i], ep.URL)
		}
		if ep.PodName != expectedPods[i] {
			t.Errorf("endpoint %d: expected PodName %s, got %s", i, expectedPods[i], ep.PodName)
		}
	}
}

func TestGetAllHealthyEndpoints_NoHealthy(t *testing.T) {
	resolver := &Resolver{
		config: Config{},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: false, PodName: "sts-0"},
		},
	}

	_, err := resolver.GetAllHealthyEndpoints()
	if err == nil {
		t.Fatal("expected error when no healthy endpoints")
	}
}

func TestGetAllEndpointsForStatefulSet_ResolvedEndpoint(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "target-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(sts, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			DiscoveryMode: PodIPMode,
			BasePath:      "/api",
		},
	}

	eps, err := resolver.GetAllEndpointsForStatefulSet(context.Background(), "target-sts", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}

	for _, ep := range eps {
		if ep.PodName == "" {
			t.Errorf("expected PodName to be set, got empty for URL %s", ep.URL)
		}
		if ep.URL == "" {
			t.Errorf("expected URL to be set, got empty for pod %s", ep.PodName)
		}
	}
}

func TestGetAllEndpointsForDeployment_ResolvedEndpoint(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-deploy",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "target-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-deploy-abc",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-deploy-def",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(deploy, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
		},
	}

	eps, err := resolver.GetAllEndpointsForDeployment(context.Background(), "target-deploy", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}

	for _, ep := range eps {
		if ep.PodName == "" {
			t.Errorf("expected PodName to be set, got empty for URL %s", ep.URL)
		}
		if ep.URL == "" {
			t.Errorf("expected URL to be set, got empty for pod %s", ep.PodName)
		}
	}
}

func TestGetAllEndpointsForDaemonSet_ResolvedEndpoint(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds-node2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(ds, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
			BasePath:  "/api",
		},
	}

	eps, err := resolver.GetAllEndpointsForDaemonSet(context.Background(), "target-ds", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(eps) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(eps))
	}

	for _, ep := range eps {
		if ep.PodName == "" {
			t.Errorf("expected PodName to be set, got empty for URL %s", ep.URL)
		}
		if ep.URL == "" {
			t.Errorf("expected URL to be set, got empty for pod %s", ep.PodName)
		}
	}
}

// =============================================================================
// Ordinal Selection Tests
// =============================================================================

func TestGetEndpointByOrdinal(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			StatefulSetName: "my-sts",
			BasePath:        "/api",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "my-sts-0"},
			{URL: "http://pod-1:8080", Healthy: true, PodName: "my-sts-1"},
			{URL: "http://pod-2:8080", Healthy: true, PodName: "my-sts-2"},
		},
	}

	tests := []struct {
		ordinal     int
		expectedURL string
	}{
		{0, "http://pod-0:8080/api"},
		{1, "http://pod-1:8080/api"},
		{2, "http://pod-2:8080/api"},
	}

	for _, tt := range tests {
		url, err := resolver.GetEndpointByOrdinal(tt.ordinal)
		if err != nil {
			t.Fatalf("unexpected error for ordinal %d: %v", tt.ordinal, err)
		}
		if url != tt.expectedURL {
			t.Errorf("ordinal %d: expected %s, got %s", tt.ordinal, tt.expectedURL, url)
		}
	}
}

func TestGetEndpointByOrdinal_NegativeOrdinal(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			StatefulSetName: "my-sts",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "my-sts-0"},
		},
	}

	_, err := resolver.GetEndpointByOrdinal(-1)
	if err == nil {
		t.Fatal("expected error for negative ordinal")
	}
}

func TestGetEndpointByOrdinal_NotFound(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			StatefulSetName: "my-sts",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "my-sts-0"},
		},
	}

	_, err := resolver.GetEndpointByOrdinal(5)
	if err == nil {
		t.Fatal("expected error for non-existent ordinal")
	}
}

func TestGetEndpointByOrdinal_UnhealthyPod(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			StatefulSetName: "my-sts",
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: false, PodName: "my-sts-0"},
		},
	}

	_, err := resolver.GetEndpointByOrdinal(0)
	if err == nil {
		t.Fatal("expected error for unhealthy pod")
	}
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestGetAllEndpoints(t *testing.T) {
	endpoints := []Endpoint{
		{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
		{URL: "http://pod-1:8080", Healthy: false, PodName: "sts-1"},
	}

	resolver := &Resolver{
		endpoints: endpoints,
	}

	result := resolver.GetAllEndpoints()

	if len(result) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(result))
	}

	// Verify it's a copy
	result[0].URL = "modified"
	if resolver.endpoints[0].URL == "modified" {
		t.Error("GetAllEndpoints should return a copy, not the original")
	}
}

func TestIsAllHealthyStrategy(t *testing.T) {
	tests := []struct {
		strategy Strategy
		expected bool
	}{
		{AllHealthy, true},
		{RoundRobin, false},
		{LeaderOnly, false},
		{AnyHealthy, false},
		{ByOrdinal, false},
	}

	for _, tt := range tests {
		resolver := &Resolver{config: Config{Strategy: tt.strategy}}
		if resolver.IsAllHealthyStrategy() != tt.expected {
			t.Errorf("Strategy %s: expected IsAllHealthyStrategy=%v", tt.strategy, tt.expected)
		}
	}
}

func TestIsByOrdinalStrategy(t *testing.T) {
	tests := []struct {
		strategy Strategy
		expected bool
	}{
		{ByOrdinal, true},
		{RoundRobin, false},
		{LeaderOnly, false},
		{AnyHealthy, false},
		{AllHealthy, false},
	}

	for _, tt := range tests {
		resolver := &Resolver{config: Config{Strategy: tt.strategy}}
		if resolver.IsByOrdinalStrategy() != tt.expected {
			t.Errorf("Strategy %s: expected IsByOrdinalStrategy=%v", tt.strategy, tt.expected)
		}
	}
}

func TestGetNamespace(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		expected  string
	}{
		{"configured namespace", "my-ns", "my-ns"},
		{"empty namespace defaults", "", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{config: Config{Namespace: tt.namespace}}
			if resolver.GetNamespace() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, resolver.GetNamespace())
			}
		})
	}
}

func TestIsUsingDeployment(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name:     "DeploymentName configured",
			config:   Config{DeploymentName: "my-deploy"},
			expected: true,
		},
		{
			name:     "WorkloadKind is Deployment",
			config:   Config{WorkloadKind: DeploymentKind},
			expected: true,
		},
		{
			name:     "StatefulSet configured",
			config:   Config{StatefulSetName: "my-sts"},
			expected: false,
		},
		{
			name:     "Neither configured",
			config:   Config{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{config: tt.config}
			if resolver.isUsingDeployment() != tt.expected {
				t.Errorf("expected isUsingDeployment=%v", tt.expected)
			}
		})
	}
}

func TestIsUsingDeployment_DiscoveredDeployment(t *testing.T) {
	resolver := &Resolver{
		config:               Config{},
		discoveredDeployName: "discovered-deploy",
	}

	if !resolver.isUsingDeployment() {
		t.Error("expected isUsingDeployment=true when discoveredDeployName is set")
	}
}

func TestGetWorkloadKind(t *testing.T) {
	tests := []struct {
		name     string
		resolver *Resolver
		expected WorkloadKind
	}{
		{
			name:     "Deployment",
			resolver: &Resolver{config: Config{DeploymentName: "my-deploy"}},
			expected: DeploymentKind,
		},
		{
			name:     "StatefulSet",
			resolver: &Resolver{config: Config{StatefulSetName: "my-sts"}},
			expected: StatefulSetKind,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.resolver.getWorkloadKind() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, tt.resolver.getWorkloadKind())
			}
		})
	}
}

func TestGetStatefulSetName(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		discovered string
		expected   string
	}{
		{"configured name", Config{StatefulSetName: "configured-sts"}, "", "configured-sts"},
		{"discovered name", Config{}, "discovered-sts", "discovered-sts"},
		{"configured takes precedence", Config{StatefulSetName: "configured-sts"}, "discovered-sts", "configured-sts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{
				config:            tt.config,
				discoveredStsName: tt.discovered,
			}
			if resolver.getStatefulSetName() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, resolver.getStatefulSetName())
			}
		})
	}
}

func TestGetDeploymentName(t *testing.T) {
	tests := []struct {
		name       string
		config     Config
		discovered string
		expected   string
	}{
		{"configured name", Config{DeploymentName: "configured-deploy"}, "", "configured-deploy"},
		{"discovered name", Config{}, "discovered-deploy", "discovered-deploy"},
		{"configured takes precedence", Config{DeploymentName: "configured-deploy"}, "discovered-deploy", "configured-deploy"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{
				config:               tt.config,
				discoveredDeployName: tt.discovered,
			}
			if resolver.getDeploymentName() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, resolver.getDeploymentName())
			}
		})
	}
}

func TestGetServiceName(t *testing.T) {
	tests := []struct {
		name          string
		config        Config
		discoveredSvc string
		discoveredSts string
		expected      string
	}{
		{"configured service", Config{ServiceName: "my-svc", StatefulSetName: "my-sts"}, "", "", "my-svc"},
		{"discovered service", Config{StatefulSetName: "my-sts"}, "discovered-svc", "", "discovered-svc"},
		{"fallback to sts name", Config{StatefulSetName: "my-sts"}, "", "", "my-sts"},
		{"fallback to discovered sts", Config{}, "", "discovered-sts", "discovered-sts"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{
				config:            tt.config,
				discoveredSvcName: tt.discoveredSvc,
				discoveredStsName: tt.discoveredSts,
			}
			if resolver.getServiceName() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, resolver.getServiceName())
			}
		})
	}
}

// =============================================================================
// selectEndpoint Tests
// =============================================================================

func TestSelectEndpoint_RoundRobin(t *testing.T) {
	resolver := &Resolver{
		config:  Config{Strategy: RoundRobin, BasePath: ""},
		counter: 0,
	}

	endpoints := []Endpoint{
		{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
		{URL: "http://pod-1:8080", Healthy: true, PodName: "sts-1"},
	}

	// First call
	url1, err := resolver.selectEndpoint(endpoints, "sts", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call should be different
	url2, err := resolver.selectEndpoint(endpoints, "sts", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url1 == url2 {
		t.Error("round-robin should return different endpoints on consecutive calls")
	}
}

func TestSelectEndpoint_WithOrdinal(t *testing.T) {
	resolver := &Resolver{
		config: Config{Strategy: RoundRobin, BasePath: "/api"},
	}

	endpoints := []Endpoint{
		{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
		{URL: "http://pod-1:8080", Healthy: true, PodName: "sts-1"},
		{URL: "http://pod-2:8080", Healthy: true, PodName: "sts-2"},
	}

	ordinal := int32(1)
	url, err := resolver.selectEndpoint(endpoints, "sts", &ordinal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url != "http://pod-1:8080/api" {
		t.Errorf("expected http://pod-1:8080/api, got %s", url)
	}
}

func TestSelectEndpoint_OrdinalUnhealthy(t *testing.T) {
	resolver := &Resolver{
		config: Config{Strategy: RoundRobin},
	}

	endpoints := []Endpoint{
		{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
		{URL: "http://pod-1:8080", Healthy: false, PodName: "sts-1"},
	}

	ordinal := int32(1)
	_, err := resolver.selectEndpoint(endpoints, "sts", &ordinal)
	if err == nil {
		t.Fatal("expected error for unhealthy ordinal target")
	}
}

func TestSelectEndpoint_OrdinalNotFound(t *testing.T) {
	resolver := &Resolver{
		config: Config{Strategy: RoundRobin},
	}

	endpoints := []Endpoint{
		{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
	}

	ordinal := int32(5)
	_, err := resolver.selectEndpoint(endpoints, "sts", &ordinal)
	if err == nil {
		t.Fatal("expected error for non-existent ordinal")
	}
}

func TestSelectEndpoint_EmptyEndpoints(t *testing.T) {
	resolver := &Resolver{
		config: Config{Strategy: RoundRobin},
	}

	_, err := resolver.selectEndpoint([]Endpoint{}, "sts", nil)
	if err == nil {
		t.Fatal("expected error for empty endpoints")
	}
}

func TestSelectEndpoint_AllUnhealthy(t *testing.T) {
	resolver := &Resolver{
		config: Config{Strategy: RoundRobin},
	}

	endpoints := []Endpoint{
		{URL: "http://pod-0:8080", Healthy: false, PodName: "sts-0"},
		{URL: "http://pod-1:8080", Healthy: false, PodName: "sts-1"},
	}

	_, err := resolver.selectEndpoint(endpoints, "sts", nil)
	if err == nil {
		t.Fatal("expected error when all endpoints unhealthy")
	}
}

func TestSelectEndpoint_LeaderOnly_Deployment(t *testing.T) {
	resolver := &Resolver{
		config: Config{Strategy: LeaderOnly, BasePath: ""},
	}

	endpoints := []Endpoint{
		{URL: "http://pod-abc:8080", Healthy: true, PodName: "deploy-abc"},
		{URL: "http://pod-def:8080", Healthy: true, PodName: "deploy-def"},
	}

	// With empty stsName (deployment), should return first healthy
	url, err := resolver.selectEndpoint(endpoints, "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url != "http://pod-abc:8080" {
		t.Errorf("expected first healthy for deployment, got %s", url)
	}
}

func TestSelectEndpoint_AllHealthyStrategy_Error(t *testing.T) {
	resolver := &Resolver{
		config: Config{Strategy: AllHealthy},
	}

	endpoints := []Endpoint{
		{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
	}

	_, err := resolver.selectEndpoint(endpoints, "sts", nil)
	if err == nil {
		t.Fatal("expected error for AllHealthy strategy")
	}
}

func TestSelectEndpoint_ByOrdinalStrategy_Error(t *testing.T) {
	resolver := &Resolver{
		config: Config{Strategy: ByOrdinal},
	}

	endpoints := []Endpoint{
		{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
	}

	_, err := resolver.selectEndpoint(endpoints, "sts", nil)
	if err == nil {
		t.Fatal("expected error for ByOrdinal strategy without ordinal")
	}
}

// =============================================================================
// Integration Tests with Fake Client
// =============================================================================

func newFakeClient(objs ...runtime.Object) *fake.ClientBuilder {
	scheme := runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	return fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objs...)
}

func int32Ptr(i int32) *int32 {
	return &i
}

func TestDiscoverByDNS(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	client := newFakeClient(sts).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			StatefulSetName: "my-sts",
			Namespace:       "default",
			ServiceName:     "my-svc",
			Port:            8080,
			Scheme:          "http",
			DiscoveryMode:   DNSMode,
		},
	}

	endpoints, err := resolver.discoverByDNS(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(endpoints))
	}

	expectedURLs := []string{
		"http://my-sts-0.my-svc.default.svc.cluster.local:8080",
		"http://my-sts-1.my-svc.default.svc.cluster.local:8080",
		"http://my-sts-2.my-svc.default.svc.cluster.local:8080",
	}

	for i, ep := range endpoints {
		if ep.URL != expectedURLs[i] {
			t.Errorf("endpoint %d: expected %s, got %s", i, expectedURLs[i], ep.URL)
		}
		if ep.PodName != "my-sts-"+string(rune('0'+i)) {
			t.Errorf("endpoint %d: expected pod name my-sts-%d, got %s", i, i, ep.PodName)
		}
	}
}

func TestDiscoverByPodIP(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(sts, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			StatefulSetName: "my-sts",
			Namespace:       "default",
			Port:            8080,
			Scheme:          "http",
			DiscoveryMode:   PodIPMode,
		},
	}

	endpoints, err := resolver.discoverByPodIP(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(endpoints))
	}

	// Check that pods were discovered (order may vary)
	foundPod0 := false
	foundPod1 := false
	for _, ep := range endpoints {
		if ep.PodIP == "10.0.0.1" {
			foundPod0 = true
			if ep.URL != "http://10.0.0.1:8080" {
				t.Errorf("expected URL http://10.0.0.1:8080, got %s", ep.URL)
			}
		}
		if ep.PodIP == "10.0.0.2" {
			foundPod1 = true
			if ep.URL != "http://10.0.0.2:8080" {
				t.Errorf("expected URL http://10.0.0.2:8080, got %s", ep.URL)
			}
		}
	}

	if !foundPod0 || !foundPod1 {
		t.Error("not all pods were discovered")
	}
}

func TestDiscoverByPodIP_SkipsNonRunningPods(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pendingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			PodIP: "",
		},
	}

	failedPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-sts-2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
			PodIP: "10.0.0.3",
		},
	}

	client := newFakeClient(sts, runningPod, pendingPod, failedPod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			StatefulSetName: "my-sts",
			Namespace:       "default",
			Port:            8080,
			Scheme:          "http",
			DiscoveryMode:   PodIPMode,
		},
	}

	endpoints, err := resolver.discoverByPodIP(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 1 {
		t.Errorf("expected 1 running endpoint, got %d", len(endpoints))
	}

	if endpoints[0].PodIP != "10.0.0.1" {
		t.Errorf("expected running pod IP 10.0.0.1, got %s", endpoints[0].PodIP)
	}
}

func TestDiscoverDeploymentPods(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deploy",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deploy-abc123",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deploy-def456",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(deploy, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			DeploymentName: "my-deploy",
			Namespace:      "default",
			Port:           8080,
			Scheme:         "http",
		},
	}

	endpoints, err := resolver.discoverDeploymentPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(endpoints))
	}
}

func TestDiscoverDeploymentPods_SkipsDeletingPods(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deploy",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	now := metav1.Now()
	deletingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "my-deploy-deleting",
			Namespace:         "default",
			Labels:            map[string]string{"app": "my-app"},
			DeletionTimestamp: &now,
			Finalizers:        []string{"test-finalizer"}, // Required by fake client
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deploy-running",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(deploy, deletingPod, runningPod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			DeploymentName: "my-deploy",
			Namespace:      "default",
			Port:           8080,
			Scheme:         "http",
		},
	}

	endpoints, err := resolver.discoverDeploymentPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 1 {
		t.Errorf("expected 1 endpoint (excluding deleting), got %d", len(endpoints))
	}

	if endpoints[0].PodIP != "10.0.0.2" {
		t.Errorf("expected non-deleting pod, got %s", endpoints[0].PodIP)
	}
}

func TestDiscoverFromHelmRelease_StatefulSet(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-sts",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	headlessSvc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-headless",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "None",
		},
	}

	client := newFakeClient(sts, headlessSvc).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			HelmRelease: "my-release",
			Namespace:   "default",
		},
	}

	err := resolver.discoverFromHelmRelease(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolver.discoveredStsName != "my-release-sts" {
		t.Errorf("expected discoveredStsName 'my-release-sts', got %s", resolver.discoveredStsName)
	}

	if resolver.discoveredSvcName != "my-release-headless" {
		t.Errorf("expected discoveredSvcName 'my-release-headless', got %s", resolver.discoveredSvcName)
	}

	if resolver.discoveredKind != StatefulSetKind {
		t.Errorf("expected discoveredKind StatefulSetKind, got %s", resolver.discoveredKind)
	}
}

func TestDiscoverFromHelmRelease_Deployment(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-deploy",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	client := newFakeClient(deploy).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			HelmRelease: "my-release",
			Namespace:   "default",
		},
	}

	err := resolver.discoverFromHelmRelease(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolver.discoveredDeployName != "my-release-deploy" {
		t.Errorf("expected discoveredDeployName 'my-release-deploy', got %s", resolver.discoveredDeployName)
	}

	if resolver.discoveredKind != DeploymentKind {
		t.Errorf("expected discoveredKind DeploymentKind, got %s", resolver.discoveredKind)
	}
}

func TestDiscoverFromHelmRelease_NotFound(t *testing.T) {
	client := newFakeClient().Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			HelmRelease: "non-existent",
			Namespace:   "default",
		},
	}

	err := resolver.discoverFromHelmRelease(context.Background())
	if err == nil {
		t.Fatal("expected error when no workload found")
	}
}

func TestGetEndpointForStatefulSet(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "target-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(sts, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			Strategy:      RoundRobin,
			DiscoveryMode: PodIPMode,
			BasePath:      "/api",
		},
	}

	url, err := resolver.GetEndpointForStatefulSet(context.Background(), "target-sts", "default", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return one of the endpoints with basepath
	if url != "http://10.0.0.1:8080/api" && url != "http://10.0.0.2:8080/api" {
		t.Errorf("unexpected URL: %s", url)
	}
}

func TestGetEndpointForStatefulSet_WithOrdinal(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "target-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(sts, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			Strategy:      ByOrdinal,
			DiscoveryMode: PodIPMode,
		},
	}

	ordinal := int32(1)
	url, err := resolver.GetEndpointForStatefulSet(context.Background(), "target-sts", "default", &ordinal)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url != "http://10.0.0.2:8080" {
		t.Errorf("expected http://10.0.0.2:8080, got %s", url)
	}
}

func TestGetEndpointForDeployment(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-deploy",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "target-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-deploy-abc",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	client := newFakeClient(deploy, pod0).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
			Strategy:  AnyHealthy,
			BasePath:  "/api",
		},
	}

	url, err := resolver.GetEndpointForDeployment(context.Background(), "target-deploy", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url != "http://10.0.0.1:8080/api" {
		t.Errorf("expected http://10.0.0.1:8080/api, got %s", url)
	}
}

func TestGetAllEndpointsForStatefulSet(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts",
			Namespace: "default",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "target-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-sts-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(sts, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			DiscoveryMode: PodIPMode,
			BasePath:      "/api",
		},
	}

	eps, err := resolver.GetAllEndpointsForStatefulSet(context.Background(), "target-sts", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(eps) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(eps))
	}
}

func TestGetAllEndpointsForDeployment(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-deploy",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "target-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-deploy-abc",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-deploy-def",
			Namespace: "default",
			Labels:    map[string]string{"app": "target-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(deploy, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
		},
	}

	eps, err := resolver.GetAllEndpointsForDeployment(context.Background(), "target-deploy", "default")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(eps) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(eps))
	}
}

func TestDiscoverHelmReleaseEndpoint_StatefulSet(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-sts",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-sts-0",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-sts-1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(sts, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			DiscoveryMode: PodIPMode,
		},
	}

	result, err := resolver.DiscoverHelmReleaseEndpoint(context.Background(), "my-release", "default", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatefulSetName != "my-release-sts" {
		t.Errorf("expected StatefulSetName 'my-release-sts', got %s", result.StatefulSetName)
	}

	if result.WorkloadKind != StatefulSetKind {
		t.Errorf("expected WorkloadKind StatefulSetKind, got %s", result.WorkloadKind)
	}

	if len(result.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(result.Endpoints))
	}
}

func TestDiscoverHelmReleaseEndpoint_Deployment(t *testing.T) {
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-deploy",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-app"},
			},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-deploy-abc",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	client := newFakeClient(deploy, pod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
		},
	}

	result, err := resolver.DiscoverHelmReleaseEndpoint(context.Background(), "my-release", "default", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DeploymentName != "my-release-deploy" {
		t.Errorf("expected DeploymentName 'my-release-deploy', got %s", result.DeploymentName)
	}

	if result.WorkloadKind != DeploymentKind {
		t.Errorf("expected WorkloadKind DeploymentKind, got %s", result.WorkloadKind)
	}

	if len(result.Endpoints) != 1 {
		t.Errorf("expected 1 endpoint, got %d", len(result.Endpoints))
	}
}

func TestDiscoverHelmReleaseEndpoint_NarrowByStatefulSetName(t *testing.T) {
	// Create two StatefulSets with the same Helm release label
	sts1 := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-api",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "api"},
			},
		},
	}
	sts2 := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-worker",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(5), // More replicas — would be selected without narrowing
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "worker"},
			},
		},
	}

	apiPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-release-api-0", Namespace: "default",
			Labels: map[string]string{"app": "api"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.1"},
	}

	client := newFakeClient(sts1, sts2, apiPod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			DiscoveryMode: PodIPMode,
		},
	}

	// Narrow to the "api" StatefulSet by name
	opts := &HelmReleaseDiscoveryOptions{
		StatefulSetName: "my-release-api",
	}
	result, err := resolver.DiscoverHelmReleaseEndpoint(context.Background(), "my-release", "default", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatefulSetName != "my-release-api" {
		t.Errorf("expected StatefulSetName 'my-release-api', got %s", result.StatefulSetName)
	}
	if result.WorkloadKind != StatefulSetKind {
		t.Errorf("expected WorkloadKind StatefulSetKind, got %s", result.WorkloadKind)
	}
}

func TestDiscoverHelmReleaseEndpoint_NarrowByDeploymentName(t *testing.T) {
	// Create a StatefulSet and a Deployment with the same Helm release label
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-sts",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "sts-app"},
			},
		},
	}
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-web",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(2),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "web"},
			},
		},
	}

	webPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-release-web-abc", Namespace: "default",
			Labels: map[string]string{"app": "web"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.10"},
	}

	client := newFakeClient(sts, deploy, webPod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			DiscoveryMode: PodIPMode,
		},
	}

	// Narrow to just the Deployment — should skip StatefulSet entirely
	opts := &HelmReleaseDiscoveryOptions{
		DeploymentName: "my-release-web",
	}
	result, err := resolver.DiscoverHelmReleaseEndpoint(context.Background(), "my-release", "default", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DeploymentName != "my-release-web" {
		t.Errorf("expected DeploymentName 'my-release-web', got %s", result.DeploymentName)
	}
	if result.WorkloadKind != DeploymentKind {
		t.Errorf("expected WorkloadKind DeploymentKind, got %s", result.WorkloadKind)
	}
}

func TestDiscoverHelmReleaseEndpoint_NarrowByLabels(t *testing.T) {
	// Two StatefulSets: one with extra label "component=api", one without
	sts1 := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-api",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance":  "my-release",
				"app.kubernetes.io/component": "api",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "api"},
			},
		},
	}
	sts2 := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-worker",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance":  "my-release",
				"app.kubernetes.io/component": "worker",
			},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: int32Ptr(5), // More replicas, would be selected without label narrowing
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "worker"},
			},
		},
	}

	apiPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "my-release-api-0", Namespace: "default",
			Labels: map[string]string{"app": "api"},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning, PodIP: "10.0.0.1"},
	}

	client := newFakeClient(sts1, sts2, apiPod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			DiscoveryMode: PodIPMode,
		},
	}

	// Narrow by label to get only the "api" component
	opts := &HelmReleaseDiscoveryOptions{
		Labels: map[string]string{
			"app.kubernetes.io/component": "api",
		},
	}
	result, err := resolver.DiscoverHelmReleaseEndpoint(context.Background(), "my-release", "default", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.StatefulSetName != "my-release-api" {
		t.Errorf("expected StatefulSetName 'my-release-api', got %s", result.StatefulSetName)
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestGetEndpoint_ConcurrentAccess(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Strategy: RoundRobin,
		},
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
			{URL: "http://pod-1:8080", Healthy: true, PodName: "sts-1"},
			{URL: "http://pod-2:8080", Healthy: true, PodName: "sts-2"},
		},
	}

	// Run concurrent calls
	done := make(chan bool)
	for i := 0; i < 100; i++ {
		go func() {
			_, err := resolver.GetEndpoint()
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 100; i++ {
		<-done
	}

	// Just verify no panics or race conditions occurred
	// The counter should have incremented 100 times
	if resolver.counter != 100 {
		t.Errorf("expected counter to be 100, got %d", resolver.counter)
	}
}

func TestGetAllEndpoints_ReturnsCopy(t *testing.T) {
	resolver := &Resolver{
		endpoints: []Endpoint{
			{URL: "http://pod-0:8080", Healthy: true, PodName: "sts-0"},
		},
	}

	// Get endpoints
	result := resolver.GetAllEndpoints()

	// Modify the result
	result[0].URL = "modified"

	// Original should be unchanged
	if resolver.endpoints[0].URL == "modified" {
		t.Error("GetAllEndpoints should return a copy")
	}
}

// =============================================================================
// DaemonSet Tests
// =============================================================================

func TestNewResolver_DaemonSetDefaults(t *testing.T) {
	cfg := Config{
		DaemonSetName: "my-ds",
		Namespace:     "default",
		Port:          8080,
	}

	resolver := NewResolver(nil, cfg)

	if resolver.config.DiscoveryMode != PodIPMode {
		t.Errorf("expected DiscoveryMode PodIPMode for DaemonSet, got %s", resolver.config.DiscoveryMode)
	}
}

func TestNewResolver_DaemonSetStrategyFallback(t *testing.T) {
	tests := []struct {
		name             string
		inputStrategy    Strategy
		expectedStrategy Strategy
	}{
		{"LeaderOnly falls back to RoundRobin", LeaderOnly, RoundRobin},
		{"ByOrdinal falls back to RoundRobin", ByOrdinal, RoundRobin},
		{"RoundRobin stays RoundRobin", RoundRobin, RoundRobin},
		{"AnyHealthy stays AnyHealthy", AnyHealthy, AnyHealthy},
		{"AllHealthy stays AllHealthy", AllHealthy, AllHealthy},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				DaemonSetName: "my-ds",
				Namespace:     "default",
				Port:          8080,
				Strategy:      tt.inputStrategy,
			}

			resolver := NewResolver(nil, cfg)

			if resolver.config.Strategy != tt.expectedStrategy {
				t.Errorf("expected Strategy %s, got %s", tt.expectedStrategy, resolver.config.Strategy)
			}
		})
	}
}

func TestIsUsingDaemonSet(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		expected bool
	}{
		{
			name:     "DaemonSetName configured",
			config:   Config{DaemonSetName: "my-ds"},
			expected: true,
		},
		{
			name:     "WorkloadKind DaemonSet",
			config:   Config{WorkloadKind: DaemonSetKind},
			expected: true,
		},
		{
			name:     "DeploymentName configured",
			config:   Config{DeploymentName: "my-deploy"},
			expected: false,
		},
		{
			name:     "StatefulSetName configured",
			config:   Config{StatefulSetName: "my-sts"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &Resolver{config: tt.config}
			if got := resolver.isUsingDaemonSet(); got != tt.expected {
				t.Errorf("isUsingDaemonSet() = %v, expected %v", got, tt.expected)
			}
		})
	}
}

func TestIsUsingDaemonSet_DiscoveredDaemonSet(t *testing.T) {
	resolver := &Resolver{
		config:           Config{},
		discoveredDSName: "discovered-ds",
	}

	if !resolver.isUsingDaemonSet() {
		t.Error("isUsingDaemonSet should return true when discoveredDSName is set")
	}
}

func TestGetWorkloadKind_DaemonSet(t *testing.T) {
	resolver := &Resolver{config: Config{DaemonSetName: "my-ds"}}
	if kind := resolver.getWorkloadKind(); kind != DaemonSetKind {
		t.Errorf("expected DaemonSetKind, got %s", kind)
	}
}

func TestGetDaemonSetName(t *testing.T) {
	tests := []struct {
		name     string
		resolver *Resolver
		expected string
	}{
		{
			name: "configured name",
			resolver: &Resolver{
				config:           Config{DaemonSetName: "configured-ds"},
				discoveredDSName: "discovered-ds",
			},
			expected: "configured-ds",
		},
		{
			name: "discovered name",
			resolver: &Resolver{
				config:           Config{},
				discoveredDSName: "discovered-ds",
			},
			expected: "discovered-ds",
		},
		{
			name:     "empty",
			resolver: &Resolver{config: Config{}},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resolver.getDaemonSetName(); got != tt.expected {
				t.Errorf("getDaemonSetName() = %q, expected %q", got, tt.expected)
			}
		})
	}
}

func TestDiscoverDaemonSetPods(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds-node2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds-node3",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.3",
		},
	}

	client := newFakeClient(ds, pod0, pod1, pod2).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			DaemonSetName: "my-ds",
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
		},
	}

	endpoints, err := resolver.discoverDaemonSetPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(endpoints))
	}
}

func TestDiscoverDaemonSetPods_SkipsDeletingPods(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
	}

	now := metav1.Now()
	deletingPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "my-ds-deleting",
			Namespace:         "default",
			Labels:            map[string]string{"app": "my-ds"},
			DeletionTimestamp: &now,
			Finalizers:        []string{"test-finalizer"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	runningPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds-running",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(ds, deletingPod, runningPod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			DaemonSetName: "my-ds",
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
		},
	}

	endpoints, err := resolver.discoverDaemonSetPods(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 1 {
		t.Errorf("expected 1 endpoint (skipping deleting pod), got %d", len(endpoints))
	}
}

func TestDiscoverByServiceDNS(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.10",
			Selector:  map[string]string{"app": "my-app"},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app-node2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-app"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(svc, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			ServiceName: "my-svc",
			Namespace:   "default",
			Port:        8080,
			Scheme:      "http",
		},
	}

	endpoints, err := resolver.discoverByServiceDNS(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 2 {
		t.Fatalf("expected 2 endpoints, got %d", len(endpoints))
	}

	// Verify endpoints are pod IPs, not Service DNS
	for _, ep := range endpoints {
		if ep.PodIP == "" {
			t.Errorf("expected PodIP to be set, got empty for %s", ep.PodName)
		}
	}
}

func TestDiscoverByServiceDNS_DiscoveredDSSvc(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "discovered-svc",
			Namespace: "prod",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.20",
			Selector:  map[string]string{"app": "discovered"},
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "discovered-pod",
			Namespace: "prod",
			Labels:    map[string]string{"app": "discovered"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.1.1",
		},
	}

	client := newFakeClient(svc, pod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "prod",
			Port:      9090,
			Scheme:    "https",
		},
		discoveredDSSvcName: "discovered-svc",
	}

	endpoints, err := resolver.discoverByServiceDNS(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}

	expectedURL := "https://10.0.1.1:9090"
	if endpoints[0].URL != expectedURL {
		t.Errorf("expected URL %q, got %q", expectedURL, endpoints[0].URL)
	}
}

func TestDiscoverByServiceDNS_NoService(t *testing.T) {
	resolver := &Resolver{
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
		},
	}

	_, err := resolver.discoverByServiceDNS(context.Background())
	if err == nil {
		t.Fatal("expected error when no ServiceName is configured")
	}
}

func TestDiscoverServiceEndpoints_NoSelector(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "no-selector-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.10",
		},
	}

	client := newFakeClient(svc).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
		},
	}

	_, err := resolver.discoverServiceEndpoints(context.Background(), "no-selector-svc", "default")
	if err == nil {
		t.Fatal("expected error when Service has no selector")
	}
}

func TestDiscoverFromHelmRelease_DaemonSet(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
		Status: appsv1.DaemonSetStatus{
			NumberReady: 3,
		},
	}

	client := newFakeClient(ds).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			HelmRelease: "my-release",
			Namespace:   "default",
		},
	}

	err := resolver.discoverFromHelmRelease(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolver.discoveredDSName != "my-release-ds" {
		t.Errorf("expected discoveredDSName 'my-release-ds', got %s", resolver.discoveredDSName)
	}

	if resolver.discoveredKind != DaemonSetKind {
		t.Errorf("expected discoveredKind DaemonSetKind, got %s", resolver.discoveredKind)
	}
}

func TestDiscoverFromHelmRelease_DaemonSet_WithService(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
		Status: appsv1.DaemonSetStatus{
			NumberReady: 3,
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-svc",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.10",
		},
	}

	client := newFakeClient(ds, svc).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			HelmRelease: "my-release",
			Namespace:   "default",
		},
	}

	err := resolver.discoverFromHelmRelease(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resolver.discoveredDSName != "my-release-ds" {
		t.Errorf("expected discoveredDSName 'my-release-ds', got %s", resolver.discoveredDSName)
	}

	if resolver.discoveredDSSvcName != "my-release-svc" {
		t.Errorf("expected discoveredDSSvcName 'my-release-svc', got %s", resolver.discoveredDSSvcName)
	}
}

func TestGetEndpointForDaemonSet(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds-node2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(ds, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
			Strategy:  RoundRobin,
			BasePath:  "/api",
		},
	}

	url, err := resolver.GetEndpointForDaemonSet(context.Background(), "target-ds", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url == "" {
		t.Fatal("expected non-empty URL")
	}

	// Verify it includes the base path
	if len(url) < 4 || url[len(url)-4:] != "/api" {
		t.Errorf("expected URL to end with /api, got %s", url)
	}
}

func TestGetEndpointForDaemonSet_WithService(t *testing.T) {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds-svc",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.10",
			Selector:  map[string]string{"app": "my-ds"},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-ds-node2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(svc, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
			BasePath:  "/api",
			Strategy:  RoundRobin,
		},
	}

	url, err := resolver.GetEndpointForDaemonSet(context.Background(), "my-ds", "", "my-ds-svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if url == "" {
		t.Fatal("expected non-empty URL")
	}

	// Verify it includes the base path and uses a pod IP (not Service DNS)
	if len(url) < 4 || url[len(url)-4:] != "/api" {
		t.Errorf("expected URL to end with /api, got %s", url)
	}
}

func TestGetAllEndpointsForDaemonSet(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds",
			Namespace: "default",
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds-node2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	pod2 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "target-ds-node3",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.3",
		},
	}

	client := newFakeClient(ds, pod0, pod1, pod2).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
			BasePath:  "/api",
		},
	}

	eps, err := resolver.GetAllEndpointsForDaemonSet(context.Background(), "target-ds", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(eps) != 3 {
		t.Errorf("expected 3 endpoints, got %d", len(eps))
	}

	for _, ep := range eps {
		if len(ep.URL) < 4 || ep.URL[len(ep.URL)-4:] != "/api" {
			t.Errorf("expected URL to end with /api, got %s", ep.URL)
		}
	}
}

func TestDiscoverHelmReleaseEndpoint_DaemonSet(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
		Status: appsv1.DaemonSetStatus{
			NumberReady: 2,
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds-node2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(ds, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
		},
	}

	result, err := resolver.DiscoverHelmReleaseEndpoint(context.Background(), "my-release", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DaemonSetName != "my-release-ds" {
		t.Errorf("expected DaemonSetName 'my-release-ds', got %s", result.DaemonSetName)
	}

	if result.WorkloadKind != DaemonSetKind {
		t.Errorf("expected WorkloadKind DaemonSetKind, got %s", result.WorkloadKind)
	}

	if len(result.Endpoints) != 2 {
		t.Errorf("expected 2 endpoints, got %d", len(result.Endpoints))
	}
}

func TestDiscoverHelmReleaseEndpoint_DaemonSet_ServiceDNS(t *testing.T) {
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "my-ds"},
			},
		},
		Status: appsv1.DaemonSetStatus{
			NumberReady: 2,
		},
	}

	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-svc",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: corev1.ServiceSpec{
			ClusterIP: "10.96.0.10",
			Selector:  map[string]string{"app": "my-ds"},
		},
	}

	pod0 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	pod1 := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds-node2",
			Namespace: "default",
			Labels:    map[string]string{"app": "my-ds"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.2",
		},
	}

	client := newFakeClient(ds, svc, pod0, pod1).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace:     "default",
			Port:          8080,
			Scheme:        "http",
			DiscoveryMode: ServiceDNSMode,
		},
	}

	result, err := resolver.DiscoverHelmReleaseEndpoint(context.Background(), "my-release", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DaemonSetName != "my-release-ds" {
		t.Errorf("expected DaemonSetName 'my-release-ds', got %s", result.DaemonSetName)
	}

	if result.ServiceName != "my-release-svc" {
		t.Errorf("expected ServiceName 'my-release-svc', got %s", result.ServiceName)
	}

	// ServiceDNS now discovers individual pods behind the Service
	if len(result.Endpoints) != 2 {
		t.Fatalf("expected 2 endpoints (pods behind service), got %d", len(result.Endpoints))
	}

	// Verify endpoints use pod IPs
	for _, ep := range result.Endpoints {
		if ep.PodIP == "" {
			t.Errorf("expected PodIP to be set for endpoint %s", ep.PodName)
		}
	}
}

func TestDiscoverHelmReleaseEndpoint_NarrowByDaemonSetName(t *testing.T) {
	ds1 := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds-agent",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "agent"},
			},
		},
		Status: appsv1.DaemonSetStatus{
			NumberReady: 5,
		},
	}

	ds2 := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds-collector",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/instance": "my-release",
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "collector"},
			},
		},
		Status: appsv1.DaemonSetStatus{
			NumberReady: 3,
		},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-release-ds-collector-node1",
			Namespace: "default",
			Labels:    map[string]string{"app": "collector"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			PodIP: "10.0.0.1",
		},
	}

	client := newFakeClient(ds1, ds2, pod).Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			Namespace: "default",
			Port:      8080,
			Scheme:    "http",
		},
	}

	// Narrow to the collector DaemonSet
	result, err := resolver.DiscoverHelmReleaseEndpoint(context.Background(), "my-release", "", &HelmReleaseDiscoveryOptions{
		DaemonSetName: "my-release-ds-collector",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DaemonSetName != "my-release-ds-collector" {
		t.Errorf("expected DaemonSetName 'my-release-ds-collector', got %s", result.DaemonSetName)
	}
}

func TestDiscoverHelmRelease_NotFound_IncludesDaemonSet(t *testing.T) {
	client := newFakeClient().Build()

	resolver := &Resolver{
		client: client,
		config: Config{
			HelmRelease: "non-existent",
			Namespace:   "default",
		},
	}

	err := resolver.discoverFromHelmRelease(context.Background())
	if err == nil {
		t.Fatal("expected error when no workload found")
	}

	// Error message should mention DaemonSet
	if got := err.Error(); !contains(got, "DaemonSet") {
		t.Errorf("error message should mention DaemonSet, got: %s", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
