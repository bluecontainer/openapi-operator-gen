/*
Copyright 2024 openapi-operator-gen authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/

package endpoint

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Strategy defines how to select endpoints from pods
type Strategy string

const (
	// RoundRobin distributes requests evenly across all healthy pods
	RoundRobin Strategy = "round-robin"
	// LeaderOnly always uses pod-0 (ordinal 0) as the primary endpoint (StatefulSet only)
	LeaderOnly Strategy = "leader-only"
	// AnyHealthy uses any single healthy pod, fails over if unhealthy
	AnyHealthy Strategy = "any-healthy"
	// AllHealthy returns all healthy pods for fan-out/broadcast operations
	AllHealthy Strategy = "all-healthy"
	// ByOrdinal routes to a specific pod based on ordinal index specified in the CR (StatefulSet only)
	ByOrdinal Strategy = "by-ordinal"
)

// DiscoveryMode defines how to discover pod endpoints
type DiscoveryMode string

const (
	// DNSMode uses headless service DNS names (pod-N.svc.namespace.svc.cluster.local)
	// Only works with StatefulSets
	DNSMode DiscoveryMode = "dns"
	// PodIPMode queries pods and uses their IP addresses
	PodIPMode DiscoveryMode = "pod-ip"
)

// WorkloadKind defines the type of workload to discover
type WorkloadKind string

const (
	// StatefulSetKind discovers endpoints from a StatefulSet
	StatefulSetKind WorkloadKind = "statefulset"
	// DeploymentKind discovers endpoints from a Deployment
	DeploymentKind WorkloadKind = "deployment"
	// AutoKind automatically detects the workload kind (tries StatefulSet first, then Deployment)
	AutoKind WorkloadKind = "auto"
)

// Config holds endpoint resolver configuration
type Config struct {
	// StatefulSetName is the name of the StatefulSet to discover pods from.
	// Either StatefulSetName, DeploymentName, or HelmRelease must be specified.
	StatefulSetName string
	// DeploymentName is the name of the Deployment to discover pods from.
	// Either StatefulSetName, DeploymentName, or HelmRelease must be specified.
	DeploymentName string
	// Namespace is the namespace of the workload
	Namespace string
	// HelmRelease is the name of the Helm release to discover workload from.
	// The resolver will find StatefulSets or Deployments with label app.kubernetes.io/instance=<HelmRelease>
	HelmRelease string
	// WorkloadKind specifies the type of workload to discover (statefulset, deployment, or auto)
	// When using HelmRelease, "auto" will try StatefulSet first, then Deployment
	WorkloadKind WorkloadKind
	// ServiceName is the headless service name (for DNS mode with StatefulSets)
	ServiceName string
	// Port is the port number for the REST API
	Port int
	// Scheme is the URL scheme (http or https)
	Scheme string
	// BasePath is the base path prefix for all requests
	BasePath string
	// Strategy defines how to select endpoints
	Strategy Strategy
	// DiscoveryMode defines how to discover endpoints
	DiscoveryMode DiscoveryMode
	// RefreshInterval is how often to refresh the pod list
	RefreshInterval time.Duration
	// HealthCheckPath is the path to use for health checks (empty to disable)
	HealthCheckPath string
	// HealthCheckInterval is how often to check endpoint health
	HealthCheckInterval time.Duration
}

// Endpoint represents a single API endpoint
type Endpoint struct {
	URL     string
	Healthy bool
	PodName string
	PodIP   string
}

// Resolver discovers and manages REST API endpoints from StatefulSet or Deployment pods
type Resolver struct {
	client               client.Client
	config               Config
	endpoints            []Endpoint
	mu                   sync.RWMutex
	counter              uint64 // for round-robin
	stopCh               chan struct{}
	discoveredStsName    string       // StatefulSet name when discovered via Helm release
	discoveredDeployName string       // Deployment name when discovered via Helm release
	discoveredSvcName    string       // Service name when discovered via Helm release
	discoveredKind       WorkloadKind // discovered workload kind
}

// NewResolver creates a new endpoint resolver
func NewResolver(c client.Client, cfg Config) *Resolver {
	if cfg.RefreshInterval == 0 {
		cfg.RefreshInterval = 30 * time.Second
	}
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 10 * time.Second
	}
	if cfg.Scheme == "" {
		cfg.Scheme = "http"
	}
	if cfg.Strategy == "" {
		cfg.Strategy = RoundRobin
	}
	if cfg.WorkloadKind == "" {
		cfg.WorkloadKind = AutoKind
	}

	// Determine default discovery mode based on workload kind
	if cfg.DiscoveryMode == "" {
		if cfg.DeploymentName != "" || cfg.WorkloadKind == DeploymentKind {
			// Deployments must use pod-ip mode (no stable DNS names)
			cfg.DiscoveryMode = PodIPMode
		} else {
			cfg.DiscoveryMode = DNSMode
		}
	}

	// Validate strategy for Deployment (ordinal-based strategies don't apply)
	if cfg.DeploymentName != "" || cfg.WorkloadKind == DeploymentKind {
		if cfg.Strategy == LeaderOnly || cfg.Strategy == ByOrdinal {
			// Fall back to round-robin for Deployments
			cfg.Strategy = RoundRobin
		}
	}

	return &Resolver{
		client:    c,
		config:    cfg,
		endpoints: make([]Endpoint, 0),
		stopCh:    make(chan struct{}),
	}
}

// Start begins the background endpoint discovery and health checking
func (r *Resolver) Start(ctx context.Context) error {
	logger := log.FromContext(ctx)

	// Initial discovery
	if err := r.refresh(ctx); err != nil {
		logger.Error(err, "Initial endpoint discovery failed")
	}

	// Start background refresh
	go func() {
		refreshTicker := time.NewTicker(r.config.RefreshInterval)
		healthTicker := time.NewTicker(r.config.HealthCheckInterval)
		defer refreshTicker.Stop()
		defer healthTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-r.stopCh:
				return
			case <-refreshTicker.C:
				if err := r.refresh(ctx); err != nil {
					logger.Error(err, "Failed to refresh endpoints")
				}
			case <-healthTicker.C:
				r.checkHealth(ctx)
			}
		}
	}()

	return nil
}

// Stop stops the background discovery
func (r *Resolver) Stop() {
	close(r.stopCh)
}

// GetEndpoint returns an endpoint based on the configured strategy.
// For AllHealthy strategy, use GetAllHealthyEndpoints instead.
// For ByOrdinal strategy, use GetEndpointByOrdinal instead.
func (r *Resolver) GetEndpoint() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.config.Strategy == AllHealthy {
		return "", fmt.Errorf("AllHealthy strategy requires GetAllHealthyEndpoints()")
	}
	if r.config.Strategy == ByOrdinal {
		return "", fmt.Errorf("ByOrdinal strategy requires GetEndpointByOrdinal(ordinal)")
	}

	healthyEndpoints := r.getHealthyEndpoints()
	if len(healthyEndpoints) == 0 {
		return "", fmt.Errorf("no healthy endpoints available")
	}

	stsName := r.getStatefulSetName()
	var endpoint Endpoint
	switch r.config.Strategy {
	case LeaderOnly:
		// Find pod-0
		for _, ep := range healthyEndpoints {
			if ep.PodName == stsName+"-0" {
				endpoint = ep
				break
			}
		}
		if endpoint.URL == "" {
			return "", fmt.Errorf("leader pod (%s-0) is not healthy", stsName)
		}
	case AnyHealthy:
		endpoint = healthyEndpoints[0]
	case RoundRobin:
		fallthrough
	default:
		idx := atomic.AddUint64(&r.counter, 1) % uint64(len(healthyEndpoints))
		endpoint = healthyEndpoints[idx]
	}

	return endpoint.URL + r.config.BasePath, nil
}

// GetAllHealthyEndpoints returns URLs for all healthy endpoints.
// Use this with AllHealthy strategy for fan-out/broadcast operations.
func (r *Resolver) GetAllHealthyEndpoints() ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	healthyEndpoints := r.getHealthyEndpoints()
	if len(healthyEndpoints) == 0 {
		return nil, fmt.Errorf("no healthy endpoints available")
	}

	urls := make([]string, 0, len(healthyEndpoints))
	for _, ep := range healthyEndpoints {
		urls = append(urls, ep.URL+r.config.BasePath)
	}
	return urls, nil
}

// IsAllHealthyStrategy returns true if the resolver is configured for all-healthy strategy
func (r *Resolver) IsAllHealthyStrategy() bool {
	return r.config.Strategy == AllHealthy
}

// IsByOrdinalStrategy returns true if the resolver is configured for by-ordinal strategy
func (r *Resolver) IsByOrdinalStrategy() bool {
	return r.config.Strategy == ByOrdinal
}

// GetEndpointByOrdinal returns the endpoint for a specific StatefulSet pod ordinal.
// Use this with ByOrdinal strategy when the CR specifies a target pod index.
func (r *Resolver) GetEndpointByOrdinal(ordinal int) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if ordinal < 0 {
		return "", fmt.Errorf("invalid ordinal: %d (must be >= 0)", ordinal)
	}

	stsName := r.getStatefulSetName()
	targetPodName := fmt.Sprintf("%s-%d", stsName, ordinal)

	for _, ep := range r.endpoints {
		if ep.PodName == targetPodName {
			if !ep.Healthy {
				return "", fmt.Errorf("pod %s is not healthy", targetPodName)
			}
			return ep.URL + r.config.BasePath, nil
		}
	}

	return "", fmt.Errorf("pod with ordinal %d not found (expected %s)", ordinal, targetPodName)
}

// GetAllEndpoints returns all discovered endpoints
func (r *Resolver) GetAllEndpoints() []Endpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Endpoint, len(r.endpoints))
	copy(result, r.endpoints)
	return result
}

func (r *Resolver) getHealthyEndpoints() []Endpoint {
	healthy := make([]Endpoint, 0, len(r.endpoints))
	for _, ep := range r.endpoints {
		if ep.Healthy {
			healthy = append(healthy, ep)
		}
	}
	return healthy
}

func (r *Resolver) refresh(ctx context.Context) error {
	logger := log.FromContext(ctx)

	// If using Helm release discovery, find the workload first
	if r.config.HelmRelease != "" && r.discoveredStsName == "" && r.discoveredDeployName == "" {
		if err := r.discoverFromHelmRelease(ctx); err != nil {
			return err
		}
	}

	var endpoints []Endpoint
	var err error

	// Determine which workload type to discover from
	useDeployment := r.isUsingDeployment()

	if useDeployment {
		// Deployments always use pod-ip mode
		endpoints, err = r.discoverDeploymentPods(ctx)
	} else {
		// StatefulSet discovery
		switch r.config.DiscoveryMode {
		case PodIPMode:
			endpoints, err = r.discoverByPodIP(ctx)
		case DNSMode:
			fallthrough
		default:
			endpoints, err = r.discoverByDNS(ctx)
		}
	}

	if err != nil {
		return err
	}

	r.mu.Lock()
	r.endpoints = endpoints
	r.mu.Unlock()

	logger.Info("Refreshed endpoints", "count", len(endpoints), "workloadKind", r.getWorkloadKind())
	return nil
}

// isUsingDeployment returns true if we're discovering from a Deployment
func (r *Resolver) isUsingDeployment() bool {
	if r.config.DeploymentName != "" {
		return true
	}
	if r.discoveredDeployName != "" {
		return true
	}
	if r.config.WorkloadKind == DeploymentKind {
		return true
	}
	return false
}

// getWorkloadKind returns the current workload kind being used
func (r *Resolver) getWorkloadKind() WorkloadKind {
	if r.isUsingDeployment() {
		return DeploymentKind
	}
	return StatefulSetKind
}

func (r *Resolver) discoverByPodIP(ctx context.Context) ([]Endpoint, error) {
	stsName := r.getStatefulSetName()
	if stsName == "" {
		return nil, fmt.Errorf("no StatefulSet name configured or discovered")
	}

	// Get the StatefulSet
	sts := &appsv1.StatefulSet{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      stsName,
		Namespace: r.GetNamespace(),
	}, sts)
	if err != nil {
		return nil, fmt.Errorf("failed to get StatefulSet %s: %w", stsName, err)
	}

	// List pods belonging to this StatefulSet
	podList := &corev1.PodList{}
	err = r.client.List(ctx, podList,
		client.InNamespace(r.GetNamespace()),
		client.MatchingLabels(sts.Spec.Selector.MatchLabels),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	endpoints := make([]Endpoint, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if pod.Status.PodIP == "" {
			continue
		}

		url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, pod.Status.PodIP, r.config.Port)
		endpoints = append(endpoints, Endpoint{
			URL:     url,
			Healthy: true, // Will be updated by health check
			PodName: pod.Name,
			PodIP:   pod.Status.PodIP,
		})
	}

	return endpoints, nil
}

func (r *Resolver) discoverByDNS(ctx context.Context) ([]Endpoint, error) {
	stsName := r.getStatefulSetName()
	if stsName == "" {
		return nil, fmt.Errorf("no StatefulSet name configured or discovered")
	}
	svcName := r.getServiceName()

	// Get the StatefulSet to find replica count
	sts := &appsv1.StatefulSet{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      stsName,
		Namespace: r.GetNamespace(),
	}, sts)
	if err != nil {
		return nil, fmt.Errorf("failed to get StatefulSet %s: %w", stsName, err)
	}

	replicas := int32(1)
	if sts.Spec.Replicas != nil {
		replicas = *sts.Spec.Replicas
	}

	endpoints := make([]Endpoint, 0, replicas)
	for i := int32(0); i < replicas; i++ {
		podName := fmt.Sprintf("%s-%d", stsName, i)
		// DNS format: pod-name.service-name.namespace.svc.cluster.local
		dnsName := fmt.Sprintf("%s.%s.%s.svc.cluster.local",
			podName, svcName, r.GetNamespace())

		url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, dnsName, r.config.Port)
		endpoints = append(endpoints, Endpoint{
			URL:     url,
			Healthy: true, // Will be updated by health check
			PodName: podName,
		})
	}

	return endpoints, nil
}

func (r *Resolver) checkHealth(ctx context.Context) {
	logger := log.FromContext(ctx)

	if r.config.HealthCheckPath == "" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	for i := range r.endpoints {
		ep := &r.endpoints[i]
		healthy := r.isHealthy(ctx, ep.URL+r.config.HealthCheckPath)
		if ep.Healthy != healthy {
			logger.Info("Endpoint health changed", "pod", ep.PodName, "healthy", healthy)
		}
		ep.Healthy = healthy
	}
}

func (r *Resolver) isHealthy(ctx context.Context, url string) bool {
	// Try to connect with a short timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// For DNS mode, first check if DNS resolves
	if r.config.DiscoveryMode == DNSMode {
		host := url[len(r.config.Scheme)+3:] // strip scheme://
		if idx := len(host) - 1; idx > 0 {
			for i := len(host) - 1; i >= 0; i-- {
				if host[i] == ':' {
					host = host[:i]
					break
				}
			}
		}
		_, err := net.DefaultResolver.LookupHost(ctx, host)
		if err != nil {
			return false
		}
	}

	return true
}

// discoverFromHelmRelease finds StatefulSet or Deployment from a Helm release
func (r *Resolver) discoverFromHelmRelease(ctx context.Context) error {
	logger := log.FromContext(ctx)
	namespace := r.GetNamespace()

	// Try to find StatefulSet first (unless explicitly requesting Deployment)
	if r.config.WorkloadKind != DeploymentKind {
		stsList := &appsv1.StatefulSetList{}
		err := r.client.List(ctx, stsList,
			client.InNamespace(namespace),
			client.MatchingLabels{
				"app.kubernetes.io/instance": r.config.HelmRelease,
			},
		)
		if err != nil {
			logger.Error(err, "Failed to list StatefulSets for Helm release")
		} else if len(stsList.Items) > 0 {
			// Use the first StatefulSet found (or the one with most replicas if multiple)
			var selectedSts *appsv1.StatefulSet
			for i := range stsList.Items {
				sts := &stsList.Items[i]
				if selectedSts == nil {
					selectedSts = sts
				} else if sts.Spec.Replicas != nil && selectedSts.Spec.Replicas != nil {
					if *sts.Spec.Replicas > *selectedSts.Spec.Replicas {
						selectedSts = sts
					}
				}
			}

			r.discoveredStsName = selectedSts.Name
			r.discoveredKind = StatefulSetKind
			logger.Info("Discovered StatefulSet from Helm release",
				"helmRelease", r.config.HelmRelease,
				"statefulset", r.discoveredStsName)

			// Try to find a headless service for this StatefulSet
			if r.config.ServiceName == "" {
				r.discoverServiceFromHelmRelease(ctx, namespace)
			}
			return nil
		}
	}

	// Try to find Deployment (if no StatefulSet found or explicitly requesting Deployment)
	if r.config.WorkloadKind != StatefulSetKind {
		deployList := &appsv1.DeploymentList{}
		err := r.client.List(ctx, deployList,
			client.InNamespace(namespace),
			client.MatchingLabels{
				"app.kubernetes.io/instance": r.config.HelmRelease,
			},
		)
		if err != nil {
			logger.Error(err, "Failed to list Deployments for Helm release")
		} else if len(deployList.Items) > 0 {
			// Use the first Deployment found (or the one with most replicas if multiple)
			var selectedDeploy *appsv1.Deployment
			for i := range deployList.Items {
				deploy := &deployList.Items[i]
				if selectedDeploy == nil {
					selectedDeploy = deploy
				} else if deploy.Spec.Replicas != nil && selectedDeploy.Spec.Replicas != nil {
					if *deploy.Spec.Replicas > *selectedDeploy.Spec.Replicas {
						selectedDeploy = deploy
					}
				}
			}

			r.discoveredDeployName = selectedDeploy.Name
			r.discoveredKind = DeploymentKind
			logger.Info("Discovered Deployment from Helm release",
				"helmRelease", r.config.HelmRelease,
				"deployment", r.discoveredDeployName)
			return nil
		}
	}

	return fmt.Errorf("no StatefulSet or Deployment found for Helm release %s in namespace %s",
		r.config.HelmRelease, namespace)
}

// discoverServiceFromHelmRelease finds a headless service for DNS mode
func (r *Resolver) discoverServiceFromHelmRelease(ctx context.Context, namespace string) {
	logger := log.FromContext(ctx)

	svcList := &corev1.ServiceList{}
	err := r.client.List(ctx, svcList,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance": r.config.HelmRelease,
		},
	)
	if err != nil {
		logger.Error(err, "Failed to list Services for Helm release, using StatefulSet name as service name")
		r.discoveredSvcName = r.discoveredStsName
	} else {
		// Look for a headless service (ClusterIP: None)
		for i := range svcList.Items {
			svc := &svcList.Items[i]
			if svc.Spec.ClusterIP == "None" {
				r.discoveredSvcName = svc.Name
				logger.Info("Discovered headless Service from Helm release",
					"helmRelease", r.config.HelmRelease,
					"service", r.discoveredSvcName)
				return
			}
		}
		// If no headless service found, use StatefulSet name
		r.discoveredSvcName = r.discoveredStsName
		logger.Info("No headless service found, using StatefulSet name as service name",
			"service", r.discoveredSvcName)
	}
}

// discoverDeploymentPods discovers pods from a Deployment
func (r *Resolver) discoverDeploymentPods(ctx context.Context) ([]Endpoint, error) {
	deployName := r.getDeploymentName()
	if deployName == "" {
		return nil, fmt.Errorf("no Deployment name configured or discovered")
	}

	// Get the Deployment
	deploy := &appsv1.Deployment{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      deployName,
		Namespace: r.GetNamespace(),
	}, deploy)
	if err != nil {
		return nil, fmt.Errorf("failed to get Deployment %s: %w", deployName, err)
	}

	// List pods belonging to this Deployment
	podList := &corev1.PodList{}
	err = r.client.List(ctx, podList,
		client.InNamespace(r.GetNamespace()),
		client.MatchingLabels(deploy.Spec.Selector.MatchLabels),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	endpoints := make([]Endpoint, 0, len(podList.Items))
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		if pod.Status.PodIP == "" {
			continue
		}
		// Skip pods that are being deleted
		if pod.DeletionTimestamp != nil {
			continue
		}

		url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, pod.Status.PodIP, r.config.Port)
		endpoints = append(endpoints, Endpoint{
			URL:     url,
			Healthy: true, // Will be updated by health check
			PodName: pod.Name,
			PodIP:   pod.Status.PodIP,
		})
	}

	return endpoints, nil
}

// getDeploymentName returns the Deployment name to use (configured or discovered)
func (r *Resolver) getDeploymentName() string {
	if r.config.DeploymentName != "" {
		return r.config.DeploymentName
	}
	return r.discoveredDeployName
}

// GetNamespace returns the namespace to use
func (r *Resolver) GetNamespace() string {
	if r.config.Namespace != "" {
		return r.config.Namespace
	}
	return "default"
}

// getStatefulSetName returns the StatefulSet name to use (configured or discovered)
func (r *Resolver) getStatefulSetName() string {
	if r.config.StatefulSetName != "" {
		return r.config.StatefulSetName
	}
	return r.discoveredStsName
}

// getServiceName returns the Service name to use (configured or discovered)
func (r *Resolver) getServiceName() string {
	if r.config.ServiceName != "" {
		return r.config.ServiceName
	}
	if r.discoveredSvcName != "" {
		return r.discoveredSvcName
	}
	return r.getStatefulSetName()
}

// HelmReleaseEndpoint contains endpoint information discovered from a Helm release
type HelmReleaseEndpoint struct {
	StatefulSetName string
	DeploymentName  string
	ServiceName     string
	Namespace       string
	WorkloadKind    WorkloadKind
	Endpoints       []Endpoint // All discovered pod endpoints
}

// DiscoverHelmReleaseEndpoint discovers all endpoints from a Helm release for per-CR targeting.
// This is used when a CR specifies targetHelmRelease to override global endpoint config.
// It tries StatefulSet first, then falls back to Deployment.
// Returns all pod endpoints so the caller can apply the appropriate strategy.
func (r *Resolver) DiscoverHelmReleaseEndpoint(ctx context.Context, helmRelease, namespace string) (*HelmReleaseEndpoint, error) {
	logger := log.FromContext(ctx)

	if namespace == "" {
		namespace = r.GetNamespace()
	}

	// Try StatefulSet first
	stsList := &appsv1.StatefulSetList{}
	err := r.client.List(ctx, stsList,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance": helmRelease,
		},
	)
	if err == nil && len(stsList.Items) > 0 {
		// Use the first StatefulSet found (or the one with most replicas if multiple)
		var selectedSts *appsv1.StatefulSet
		for i := range stsList.Items {
			sts := &stsList.Items[i]
			if selectedSts == nil {
				selectedSts = sts
			} else if sts.Spec.Replicas != nil && selectedSts.Spec.Replicas != nil {
				if *sts.Spec.Replicas > *selectedSts.Spec.Replicas {
					selectedSts = sts
				}
			}
		}

		stsName := selectedSts.Name
		svcName := stsName // Default service name

		// Try to find a headless service for this Helm release
		svcList := &corev1.ServiceList{}
		err = r.client.List(ctx, svcList,
			client.InNamespace(namespace),
			client.MatchingLabels{
				"app.kubernetes.io/instance": helmRelease,
			},
		)
		if err == nil {
			for i := range svcList.Items {
				svc := &svcList.Items[i]
				if svc.Spec.ClusterIP == "None" {
					svcName = svc.Name
					break
				}
			}
		}

		// Discover all pod endpoints
		var endpoints []Endpoint
		switch r.config.DiscoveryMode {
		case PodIPMode:
			podList := &corev1.PodList{}
			err = r.client.List(ctx, podList,
				client.InNamespace(namespace),
				client.MatchingLabels(selectedSts.Spec.Selector.MatchLabels),
			)
			if err != nil {
				return nil, fmt.Errorf("failed to list pods for StatefulSet %s: %w", stsName, err)
			}
			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
					continue
				}
				url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, pod.Status.PodIP, r.config.Port)
				endpoints = append(endpoints, Endpoint{
					URL:     url,
					Healthy: true,
					PodName: pod.Name,
					PodIP:   pod.Status.PodIP,
				})
			}
		default: // DNS mode
			replicas := int32(1)
			if selectedSts.Spec.Replicas != nil {
				replicas = *selectedSts.Spec.Replicas
			}
			for i := int32(0); i < replicas; i++ {
				podName := fmt.Sprintf("%s-%d", stsName, i)
				dnsName := fmt.Sprintf("%s.%s.%s.svc.cluster.local", podName, svcName, namespace)
				url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, dnsName, r.config.Port)
				endpoints = append(endpoints, Endpoint{
					URL:     url,
					Healthy: true,
					PodName: podName,
				})
			}
		}

		if len(endpoints) == 0 {
			return nil, fmt.Errorf("no running pods found for StatefulSet %s", stsName)
		}

		logger.Info("Discovered StatefulSet endpoints from Helm release",
			"helmRelease", helmRelease,
			"statefulset", stsName,
			"service", svcName,
			"endpointCount", len(endpoints))

		return &HelmReleaseEndpoint{
			StatefulSetName: stsName,
			ServiceName:     svcName,
			Namespace:       namespace,
			WorkloadKind:    StatefulSetKind,
			Endpoints:       endpoints,
		}, nil
	}

	// Try Deployment
	deployList := &appsv1.DeploymentList{}
	err = r.client.List(ctx, deployList,
		client.InNamespace(namespace),
		client.MatchingLabels{
			"app.kubernetes.io/instance": helmRelease,
		},
	)
	if err == nil && len(deployList.Items) > 0 {
		// Use the first Deployment found (or the one with most replicas if multiple)
		var selectedDeploy *appsv1.Deployment
		for i := range deployList.Items {
			deploy := &deployList.Items[i]
			if selectedDeploy == nil {
				selectedDeploy = deploy
			} else if deploy.Spec.Replicas != nil && selectedDeploy.Spec.Replicas != nil {
				if *deploy.Spec.Replicas > *selectedDeploy.Spec.Replicas {
					selectedDeploy = deploy
				}
			}
		}

		deployName := selectedDeploy.Name

		// Discover all pod endpoints
		podList := &corev1.PodList{}
		err = r.client.List(ctx, podList,
			client.InNamespace(namespace),
			client.MatchingLabels(selectedDeploy.Spec.Selector.MatchLabels),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to list pods for Deployment %s: %w", deployName, err)
		}

		var endpoints []Endpoint
		for _, pod := range podList.Items {
			if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
				continue
			}
			url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, pod.Status.PodIP, r.config.Port)
			endpoints = append(endpoints, Endpoint{
				URL:     url,
				Healthy: true,
				PodName: pod.Name,
				PodIP:   pod.Status.PodIP,
			})
		}

		if len(endpoints) == 0 {
			return nil, fmt.Errorf("no running pods found for Deployment %s", deployName)
		}

		logger.Info("Discovered Deployment endpoints from Helm release",
			"helmRelease", helmRelease,
			"deployment", deployName,
			"endpointCount", len(endpoints))

		return &HelmReleaseEndpoint{
			DeploymentName: deployName,
			Namespace:      namespace,
			WorkloadKind:   DeploymentKind,
			Endpoints:      endpoints,
		}, nil
	}

	return nil, fmt.Errorf("no StatefulSet or Deployment found for Helm release %s in namespace %s", helmRelease, namespace)
}

// GetEndpointForHelmRelease returns an endpoint URL for a specific Helm release.
// This supports per-CR targeting when targetHelmRelease is specified.
// It applies the configured strategy to select from available endpoints.
func (r *Resolver) GetEndpointForHelmRelease(ctx context.Context, helmRelease, namespace string, ordinal *int32) (string, error) {
	if namespace == "" {
		namespace = r.GetNamespace()
	}

	// Discover all endpoints from the Helm release
	result, err := r.DiscoverHelmReleaseEndpoint(ctx, helmRelease, namespace)
	if err != nil {
		return "", err
	}

	// Apply strategy to select endpoint
	return r.selectEndpoint(result.Endpoints, result.StatefulSetName, ordinal)
}

// GetAllEndpointsForHelmRelease returns all endpoint URLs for a specific Helm release.
// Use this with AllHealthy strategy for fan-out/broadcast operations.
func (r *Resolver) GetAllEndpointsForHelmRelease(ctx context.Context, helmRelease, namespace string) ([]string, error) {
	if namespace == "" {
		namespace = r.GetNamespace()
	}

	result, err := r.DiscoverHelmReleaseEndpoint(ctx, helmRelease, namespace)
	if err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(result.Endpoints))
	for _, ep := range result.Endpoints {
		if ep.Healthy {
			urls = append(urls, ep.URL+r.config.BasePath)
		}
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("no healthy endpoints available for Helm release %s", helmRelease)
	}
	return urls, nil
}

// selectEndpoint selects an endpoint based on the configured strategy
func (r *Resolver) selectEndpoint(endpoints []Endpoint, stsName string, ordinal *int32) (string, error) {
	if len(endpoints) == 0 {
		return "", fmt.Errorf("no endpoints available")
	}

	// Filter healthy endpoints
	healthyEndpoints := make([]Endpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.Healthy {
			healthyEndpoints = append(healthyEndpoints, ep)
		}
	}
	if len(healthyEndpoints) == 0 {
		return "", fmt.Errorf("no healthy endpoints available")
	}

	// Handle by-ordinal strategy (only for StatefulSets)
	if ordinal != nil && stsName != "" {
		targetPodName := fmt.Sprintf("%s-%d", stsName, *ordinal)
		for _, ep := range endpoints {
			if ep.PodName == targetPodName {
				if !ep.Healthy {
					return "", fmt.Errorf("pod %s is not healthy", targetPodName)
				}
				return ep.URL + r.config.BasePath, nil
			}
		}
		return "", fmt.Errorf("pod with ordinal %d not found (expected %s)", *ordinal, targetPodName)
	}

	// Apply strategy
	var selectedEndpoint Endpoint
	switch r.config.Strategy {
	case LeaderOnly:
		// Find pod-0 (only works for StatefulSets)
		if stsName != "" {
			pod0Name := fmt.Sprintf("%s-0", stsName)
			for _, ep := range healthyEndpoints {
				if ep.PodName == pod0Name {
					selectedEndpoint = ep
					break
				}
			}
			if selectedEndpoint.URL == "" {
				return "", fmt.Errorf("leader pod (%s-0) is not healthy", stsName)
			}
		} else {
			// For Deployments, fall back to first healthy
			selectedEndpoint = healthyEndpoints[0]
		}
	case AnyHealthy:
		selectedEndpoint = healthyEndpoints[0]
	case AllHealthy:
		return "", fmt.Errorf("AllHealthy strategy requires GetAllEndpointsForHelmRelease()")
	case ByOrdinal:
		return "", fmt.Errorf("ByOrdinal strategy requires ordinal parameter")
	case RoundRobin:
		fallthrough
	default:
		idx := atomic.AddUint64(&r.counter, 1) % uint64(len(healthyEndpoints))
		selectedEndpoint = healthyEndpoints[idx]
	}

	return selectedEndpoint.URL + r.config.BasePath, nil
}

// GetEndpointForStatefulSet returns an endpoint URL for a specific StatefulSet.
// This supports per-CR targeting when targetStatefulSet is specified.
// It discovers all pods and applies the configured strategy.
func (r *Resolver) GetEndpointForStatefulSet(ctx context.Context, stsName, namespace string, ordinal *int32) (string, error) {
	logger := log.FromContext(ctx)

	if namespace == "" {
		namespace = r.GetNamespace()
	}

	// Discover all endpoints for this StatefulSet
	endpoints, svcName, err := r.discoverStatefulSetEndpoints(ctx, stsName, namespace)
	if err != nil {
		return "", err
	}

	logger.Info("Discovered StatefulSet endpoints",
		"statefulset", stsName,
		"namespace", namespace,
		"endpointCount", len(endpoints),
		"service", svcName)

	// Apply strategy to select endpoint
	return r.selectEndpoint(endpoints, stsName, ordinal)
}

// GetAllEndpointsForStatefulSet returns all endpoint URLs for a specific StatefulSet.
// Use this with AllHealthy strategy for fan-out/broadcast operations.
func (r *Resolver) GetAllEndpointsForStatefulSet(ctx context.Context, stsName, namespace string) ([]string, error) {
	if namespace == "" {
		namespace = r.GetNamespace()
	}

	endpoints, _, err := r.discoverStatefulSetEndpoints(ctx, stsName, namespace)
	if err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.Healthy {
			urls = append(urls, ep.URL+r.config.BasePath)
		}
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("no healthy endpoints available for StatefulSet %s", stsName)
	}
	return urls, nil
}

// discoverStatefulSetEndpoints discovers all endpoints for a StatefulSet
func (r *Resolver) discoverStatefulSetEndpoints(ctx context.Context, stsName, namespace string) ([]Endpoint, string, error) {
	// Get the StatefulSet
	sts := &appsv1.StatefulSet{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      stsName,
		Namespace: namespace,
	}, sts)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get StatefulSet %s: %w", stsName, err)
	}

	// Try to find a headless service
	svcName := stsName
	svcList := &corev1.ServiceList{}
	err = r.client.List(ctx, svcList,
		client.InNamespace(namespace),
		client.MatchingLabels(sts.Spec.Selector.MatchLabels),
	)
	if err == nil {
		for i := range svcList.Items {
			svc := &svcList.Items[i]
			if svc.Spec.ClusterIP == "None" {
				svcName = svc.Name
				break
			}
		}
	}

	// Discover all pod endpoints
	var endpoints []Endpoint
	switch r.config.DiscoveryMode {
	case PodIPMode:
		podList := &corev1.PodList{}
		err = r.client.List(ctx, podList,
			client.InNamespace(namespace),
			client.MatchingLabels(sts.Spec.Selector.MatchLabels),
		)
		if err != nil {
			return nil, "", fmt.Errorf("failed to list pods: %w", err)
		}
		for _, pod := range podList.Items {
			if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
				continue
			}
			url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, pod.Status.PodIP, r.config.Port)
			endpoints = append(endpoints, Endpoint{
				URL:     url,
				Healthy: true,
				PodName: pod.Name,
				PodIP:   pod.Status.PodIP,
			})
		}
	default: // DNS mode
		replicas := int32(1)
		if sts.Spec.Replicas != nil {
			replicas = *sts.Spec.Replicas
		}
		for i := int32(0); i < replicas; i++ {
			podName := fmt.Sprintf("%s-%d", stsName, i)
			dnsName := fmt.Sprintf("%s.%s.%s.svc.cluster.local", podName, svcName, namespace)
			url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, dnsName, r.config.Port)
			endpoints = append(endpoints, Endpoint{
				URL:     url,
				Healthy: true,
				PodName: podName,
			})
		}
	}

	if len(endpoints) == 0 {
		return nil, "", fmt.Errorf("no running pods found for StatefulSet %s", stsName)
	}

	return endpoints, svcName, nil
}

// GetEndpointForDeployment returns an endpoint URL for a specific Deployment.
// This supports per-CR targeting when targetDeployment is specified.
// It discovers all pods and applies the configured strategy.
func (r *Resolver) GetEndpointForDeployment(ctx context.Context, deployName, namespace string) (string, error) {
	logger := log.FromContext(ctx)

	if namespace == "" {
		namespace = r.GetNamespace()
	}

	// Discover all endpoints for this Deployment
	endpoints, err := r.discoverDeploymentEndpoints(ctx, deployName, namespace)
	if err != nil {
		return "", err
	}

	logger.Info("Discovered Deployment endpoints",
		"deployment", deployName,
		"namespace", namespace,
		"endpointCount", len(endpoints))

	// Apply strategy to select endpoint (no ordinal for Deployments)
	return r.selectEndpoint(endpoints, "", nil)
}

// GetAllEndpointsForDeployment returns all endpoint URLs for a specific Deployment.
// Use this with AllHealthy strategy for fan-out/broadcast operations.
func (r *Resolver) GetAllEndpointsForDeployment(ctx context.Context, deployName, namespace string) ([]string, error) {
	if namespace == "" {
		namespace = r.GetNamespace()
	}

	endpoints, err := r.discoverDeploymentEndpoints(ctx, deployName, namespace)
	if err != nil {
		return nil, err
	}

	urls := make([]string, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.Healthy {
			urls = append(urls, ep.URL+r.config.BasePath)
		}
	}
	if len(urls) == 0 {
		return nil, fmt.Errorf("no healthy endpoints available for Deployment %s", deployName)
	}
	return urls, nil
}

// discoverDeploymentEndpoints discovers all endpoints for a Deployment
func (r *Resolver) discoverDeploymentEndpoints(ctx context.Context, deployName, namespace string) ([]Endpoint, error) {
	// Get the Deployment
	deploy := &appsv1.Deployment{}
	err := r.client.Get(ctx, types.NamespacedName{
		Name:      deployName,
		Namespace: namespace,
	}, deploy)
	if err != nil {
		return nil, fmt.Errorf("failed to get Deployment %s: %w", deployName, err)
	}

	// Discover all pod endpoints
	podList := &corev1.PodList{}
	err = r.client.List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabels(deploy.Spec.Selector.MatchLabels),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods for Deployment %s: %w", deployName, err)
	}

	var endpoints []Endpoint
	for _, pod := range podList.Items {
		if pod.Status.Phase != corev1.PodRunning || pod.Status.PodIP == "" || pod.DeletionTimestamp != nil {
			continue
		}
		url := fmt.Sprintf("%s://%s:%d", r.config.Scheme, pod.Status.PodIP, r.config.Port)
		endpoints = append(endpoints, Endpoint{
			URL:     url,
			Healthy: true,
			PodName: pod.Name,
			PodIP:   pod.Status.PodIP,
		})
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no running pods found for Deployment %s", deployName)
	}

	return endpoints, nil
}
