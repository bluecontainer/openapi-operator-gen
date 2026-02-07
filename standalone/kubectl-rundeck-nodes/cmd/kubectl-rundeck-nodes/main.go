// kubectl-rundeck-nodes discovers Kubernetes workloads and outputs them
// as Rundeck resource model JSON for use as a script-based node source.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/bluecontainer/kubectl-rundeck-nodes/pkg/nodes"
)

var (
	version = "dev"

	// Flags
	kubeconfig         string
	server             string
	token              string
	insecure           bool
	namespace          string
	allNamespaces      bool
	labelSelector      string
	clusterName        string
	clusterURL         string
	clusterTokenSuffix string
	defaultTokenSuffix string
	output             string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

var rootCmd = &cobra.Command{
	Use:   "kubectl-rundeck-nodes",
	Short: "Discover Kubernetes workloads as Rundeck nodes",
	Long: `Discover Kubernetes workloads (Helm releases, StatefulSets, Deployments) and
output them as Rundeck resource model JSON. This command is designed to be used
as a Rundeck script-based resource model source.

Each discovered workload becomes a Rundeck node with attributes that map to
kubectl plugin --target-* flags:

  targetType:      helm-release, statefulset, or deployment
  targetValue:     the workload or release name
  targetNamespace: the workload's namespace
  workloadKind:    StatefulSet or Deployment
  workloadName:    the underlying workload name
  podCount:        total pod count
  healthyPods:     running pod count

Examples:
  # Discover workloads in the default namespace
  kubectl rundeck-nodes

  # Discover workloads across all namespaces
  kubectl rundeck-nodes --all-namespaces

  # Filter by label selector
  kubectl rundeck-nodes -l app.kubernetes.io/part-of=myapp

  # Multi-cluster setup with token suffix
  kubectl rundeck-nodes --cluster-name=prod --cluster-token-suffix=clusters/prod/token`,
	Version: version,
	RunE:    run,
}

func init() {
	rootCmd.Flags().StringVar(&kubeconfig, "kubeconfig", "", "Path to kubeconfig file")
	rootCmd.Flags().StringVar(&server, "server", "", "Kubernetes API server URL")
	rootCmd.Flags().StringVar(&token, "token", "", "Bearer token for authentication")
	rootCmd.Flags().BoolVar(&insecure, "insecure-skip-tls-verify", false, "Skip TLS certificate verification")
	rootCmd.Flags().StringVarP(&namespace, "namespace", "n", "", "Namespace to discover workloads in")
	rootCmd.Flags().BoolVarP(&allNamespaces, "all-namespaces", "A", false, "Discover workloads across all namespaces")
	rootCmd.Flags().StringVarP(&labelSelector, "selector", "l", "", "Label selector to filter workloads")
	rootCmd.Flags().StringVar(&clusterName, "cluster-name", "", "Cluster identifier for multi-cluster discovery")
	rootCmd.Flags().StringVar(&clusterURL, "cluster-url", "", "Cluster API URL to embed in node attributes")
	rootCmd.Flags().StringVar(&clusterTokenSuffix, "cluster-token-suffix", "", "Key Storage path suffix for cluster token")
	rootCmd.Flags().StringVar(&defaultTokenSuffix, "default-token-suffix", nodes.DefaultTokenSuffix, "Default Key Storage path suffix")
	rootCmd.Flags().StringVarP(&output, "output", "o", "json", "Output format: json, yaml, table")
}

func run(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	client, ns, err := buildClient()
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Use namespace from flag or kubeconfig
	if namespace != "" {
		ns = namespace
	}
	if ns == "" {
		ns = "default"
	}

	opts := nodes.DiscoverOptions{
		Namespace:          ns,
		AllNamespaces:      allNamespaces,
		LabelSelector:      labelSelector,
		ClusterName:        clusterName,
		ClusterURL:         clusterURL,
		ClusterTokenSuffix: clusterTokenSuffix,
		DefaultTokenSuffix: defaultTokenSuffix,
	}

	discovered, err := nodes.Discover(ctx, client, opts)
	if err != nil {
		return fmt.Errorf("discovery failed: %w", err)
	}

	format := nodes.OutputFormat(output)
	return nodes.Write(os.Stdout, discovered, format)
}

func buildClient() (dynamic.Interface, string, error) {
	var config *rest.Config
	var ns string
	var err error

	if server != "" && token != "" {
		// Direct connection with server/token
		config = &rest.Config{
			Host:        server,
			BearerToken: token,
			TLSClientConfig: rest.TLSClientConfig{
				Insecure: insecure,
			},
		}
		ns = namespace
	} else {
		// Load from kubeconfig
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		if kubeconfig != "" {
			loadingRules.ExplicitPath = kubeconfig
		}

		configOverrides := &clientcmd.ConfigOverrides{}
		if namespace != "" {
			configOverrides.Context.Namespace = namespace
		}

		clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

		config, err = clientConfig.ClientConfig()
		if err != nil {
			return nil, "", err
		}

		ns, _, err = clientConfig.Namespace()
		if err != nil {
			ns = "default"
		}
	}

	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, "", err
	}

	return client, ns, nil
}
