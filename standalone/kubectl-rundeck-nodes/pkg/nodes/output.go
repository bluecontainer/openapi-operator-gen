package nodes

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"text/tabwriter"

	"gopkg.in/yaml.v3"
)

// OutputFormat specifies the output format for discovered nodes.
type OutputFormat string

const (
	FormatJSON  OutputFormat = "json"
	FormatYAML  OutputFormat = "yaml"
	FormatTable OutputFormat = "table"
)

// Write outputs the discovered nodes in the specified format.
func Write(w io.Writer, nodes map[string]*RundeckNode, format OutputFormat) error {
	// Sort keys for deterministic output
	keys := make([]string, 0, len(nodes))
	for k := range nodes {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	switch format {
	case FormatJSON:
		return writeJSON(w, nodes, keys)
	case FormatYAML:
		return writeYAML(w, nodes, keys)
	case FormatTable:
		return writeTable(w, nodes, keys)
	default:
		return fmt.Errorf("unknown output format: %s", format)
	}
}

func writeJSON(w io.Writer, nodes map[string]*RundeckNode, keys []string) error {
	// Build ordered output
	output := make(map[string]*RundeckNode, len(nodes))
	for _, k := range keys {
		output[k] = nodes[k]
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(output)
}

func writeYAML(w io.Writer, nodes map[string]*RundeckNode, keys []string) error {
	// Build ordered output
	output := make(map[string]*RundeckNode, len(nodes))
	for _, k := range keys {
		output[k] = nodes[k]
	}

	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	return enc.Encode(output)
}

func writeTable(w io.Writer, nodes map[string]*RundeckNode, keys []string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NODE\tTYPE\tWORKLOAD\tNAMESPACE\tPODS\tCLUSTER")

	for _, k := range keys {
		n := nodes[k]
		cluster := n.Cluster
		if cluster == "" {
			cluster = "-"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s/%s\t%s\n",
			n.TargetValue,
			n.TargetType,
			n.WorkloadKind,
			n.TargetNamespace,
			n.HealthyPods,
			n.PodCount,
			cluster,
		)
	}

	return tw.Flush()
}
