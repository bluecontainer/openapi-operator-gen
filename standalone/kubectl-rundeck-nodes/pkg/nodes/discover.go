package nodes

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	stsGVR    = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}
	deployGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	podGVR    = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"}
)

// Discover queries the Kubernetes API for workloads and returns Rundeck nodes.
// It discovers StatefulSets and Deployments, with Helm release detection and
// deduplication based on the app.kubernetes.io/instance label.
func Discover(ctx context.Context, client dynamic.Interface, opts DiscoverOptions) (map[string]*RundeckNode, error) {
	listOpts := metav1.ListOptions{}
	if opts.LabelSelector != "" {
		listOpts.LabelSelector = opts.LabelSelector
	}

	nodes := make(map[string]*RundeckNode)
	helmReleases := make(map[string]*helmInfo) // key: "release@namespace"

	// Discover StatefulSets
	if err := discoverStatefulSets(ctx, client, opts, listOpts, nodes, helmReleases); err != nil {
		// Log but don't fail - allow partial discovery
		fmt.Printf("Warning: failed to list StatefulSets: %v\n", err)
	}

	// Discover Deployments
	if err := discoverDeployments(ctx, client, opts, listOpts, nodes, helmReleases); err != nil {
		fmt.Printf("Warning: failed to list Deployments: %v\n", err)
	}

	// Add Helm release nodes
	for _, info := range helmReleases {
		nodeKey := makeNodeKey(opts.ClusterName, "helm", info.release, info.namespace)
		nodes[nodeKey] = &RundeckNode{
			NodeName:           nodeKey,
			Hostname:           "localhost",
			Tags:               joinTags("helm-release", info.namespace, opts.ClusterName),
			OSFamily:           "kubernetes",
			NodeExecutor:       "local",
			FileCopier:         "local",
			Cluster:            opts.ClusterName,
			ClusterURL:         opts.ClusterURL,
			ClusterTokenSuffix: getTokenSuffix(opts),
			TargetType:         "helm-release",
			TargetValue:        info.release,
			TargetNamespace:    info.namespace,
			WorkloadKind:       info.workloadKind,
			WorkloadName:       info.workloadName,
			PodCount:           fmt.Sprintf("%d", info.totalPods),
			HealthyPods:        fmt.Sprintf("%d", info.healthyPods),
		}
	}

	return nodes, nil
}

func discoverStatefulSets(ctx context.Context, client dynamic.Interface, opts DiscoverOptions, listOpts metav1.ListOptions, nodes map[string]*RundeckNode, helmReleases map[string]*helmInfo) error {
	var stsList *unstructured.UnstructuredList
	var err error

	if opts.AllNamespaces {
		stsList, err = client.Resource(stsGVR).Namespace("").List(ctx, listOpts)
	} else {
		stsList, err = client.Resource(stsGVR).Namespace(opts.Namespace).List(ctx, listOpts)
	}
	if err != nil {
		return err
	}

	for _, sts := range stsList.Items {
		stsName := sts.GetName()
		stsNS := sts.GetNamespace()
		labels := sts.GetLabels()
		replicas, _, _ := unstructured.NestedInt64(sts.Object, "spec", "replicas")
		if replicas == 0 {
			replicas = 1
		}

		healthy := countHealthyPods(ctx, client, stsNS, getSelectorLabels(&sts))

		helmRelease := labels["app.kubernetes.io/instance"]
		if helmRelease != "" {
			key := helmRelease + "@" + stsNS
			if existing, ok := helmReleases[key]; ok {
				existing.totalPods += int(replicas)
				existing.healthyPods += healthy
				if int(replicas) > 0 && existing.workloadKind == "" {
					existing.workloadKind = "StatefulSet"
					existing.workloadName = stsName
				}
			} else {
				helmReleases[key] = &helmInfo{
					release:      helmRelease,
					namespace:    stsNS,
					workloadKind: "StatefulSet",
					workloadName: stsName,
					totalPods:    int(replicas),
					healthyPods:  healthy,
				}
			}
		} else {
			nodeKey := makeNodeKey(opts.ClusterName, "sts", stsName, stsNS)
			nodes[nodeKey] = &RundeckNode{
				NodeName:           nodeKey,
				Hostname:           "localhost",
				Tags:               joinTags("statefulset", stsNS, opts.ClusterName),
				OSFamily:           "kubernetes",
				NodeExecutor:       "local",
				FileCopier:         "local",
				Cluster:            opts.ClusterName,
				ClusterURL:         opts.ClusterURL,
				ClusterTokenSuffix: getTokenSuffix(opts),
				TargetType:         "statefulset",
				TargetValue:        stsName,
				TargetNamespace:    stsNS,
				WorkloadKind:       "StatefulSet",
				WorkloadName:       stsName,
				PodCount:           fmt.Sprintf("%d", replicas),
				HealthyPods:        fmt.Sprintf("%d", healthy),
			}
		}
	}

	return nil
}

func discoverDeployments(ctx context.Context, client dynamic.Interface, opts DiscoverOptions, listOpts metav1.ListOptions, nodes map[string]*RundeckNode, helmReleases map[string]*helmInfo) error {
	var deployList *unstructured.UnstructuredList
	var err error

	if opts.AllNamespaces {
		deployList, err = client.Resource(deployGVR).Namespace("").List(ctx, listOpts)
	} else {
		deployList, err = client.Resource(deployGVR).Namespace(opts.Namespace).List(ctx, listOpts)
	}
	if err != nil {
		return err
	}

	for _, deploy := range deployList.Items {
		deployName := deploy.GetName()
		deployNS := deploy.GetNamespace()
		labels := deploy.GetLabels()
		replicas, _, _ := unstructured.NestedInt64(deploy.Object, "spec", "replicas")
		if replicas == 0 {
			replicas = 1
		}

		healthy := countHealthyPods(ctx, client, deployNS, getSelectorLabels(&deploy))

		helmRelease := labels["app.kubernetes.io/instance"]
		if helmRelease != "" {
			key := helmRelease + "@" + deployNS
			if existing, ok := helmReleases[key]; ok {
				existing.totalPods += int(replicas)
				existing.healthyPods += healthy
			} else {
				helmReleases[key] = &helmInfo{
					release:      helmRelease,
					namespace:    deployNS,
					workloadKind: "Deployment",
					workloadName: deployName,
					totalPods:    int(replicas),
					healthyPods:  healthy,
				}
			}
		} else {
			nodeKey := makeNodeKey(opts.ClusterName, "deploy", deployName, deployNS)
			nodes[nodeKey] = &RundeckNode{
				NodeName:           nodeKey,
				Hostname:           "localhost",
				Tags:               joinTags("deployment", deployNS, opts.ClusterName),
				OSFamily:           "kubernetes",
				NodeExecutor:       "local",
				FileCopier:         "local",
				Cluster:            opts.ClusterName,
				ClusterURL:         opts.ClusterURL,
				ClusterTokenSuffix: getTokenSuffix(opts),
				TargetType:         "deployment",
				TargetValue:        deployName,
				TargetNamespace:    deployNS,
				WorkloadKind:       "Deployment",
				WorkloadName:       deployName,
				PodCount:           fmt.Sprintf("%d", replicas),
				HealthyPods:        fmt.Sprintf("%d", healthy),
			}
		}
	}

	return nil
}

// getSelectorLabels extracts spec.selector.matchLabels from a workload.
func getSelectorLabels(obj *unstructured.Unstructured) map[string]string {
	labels, found, err := unstructured.NestedStringMap(obj.Object, "spec", "selector", "matchLabels")
	if err != nil || !found {
		return nil
	}
	return labels
}

// countHealthyPods counts running pods matching the given selector labels.
func countHealthyPods(ctx context.Context, client dynamic.Interface, namespace string, selectorLabels map[string]string) int {
	if len(selectorLabels) == 0 {
		return 0
	}

	parts := make([]string, 0, len(selectorLabels))
	for k, v := range selectorLabels {
		parts = append(parts, k+"="+v)
	}
	selector := strings.Join(parts, ",")

	pods, err := client.Resource(podGVR).Namespace(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return 0
	}

	healthy := 0
	for _, pod := range pods.Items {
		phase, _, _ := unstructured.NestedString(pod.Object, "status", "phase")
		if phase == "Running" {
			healthy++
		}
	}
	return healthy
}

// joinTags creates a comma-separated tag string for Rundeck nodes.
func joinTags(workloadType, namespace, cluster string) string {
	if cluster != "" {
		return workloadType + "," + namespace + "," + cluster
	}
	return workloadType + "," + namespace
}

// makeNodeKey creates a unique node key, optionally prefixed with cluster name.
func makeNodeKey(cluster, workloadType, name, namespace string) string {
	baseKey := fmt.Sprintf("%s:%s@%s", workloadType, name, namespace)
	if cluster != "" {
		return cluster + "/" + baseKey
	}
	return baseKey
}

// getTokenSuffix returns the effective token path suffix.
func getTokenSuffix(opts DiscoverOptions) string {
	if opts.ClusterTokenSuffix != "" {
		return opts.ClusterTokenSuffix
	}
	if opts.DefaultTokenSuffix != "" {
		return opts.DefaultTokenSuffix
	}
	return DefaultTokenSuffix
}
