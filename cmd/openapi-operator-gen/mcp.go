package main

import (
	"fmt"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	mcpserver "github.com/bluecontainer/openapi-operator-gen/pkg/mcp"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Start MCP (Model Context Protocol) server for AI assistant integration",
	Long: `Start a stdio-based MCP server that exposes openapi-operator-gen capabilities
as tools for AI assistants like Claude Code.

Available tools:
  - validate: Check if an OpenAPI spec is parseable
  - preview:  Show what CRDs would be generated (dry run)
  - generate: Generate a complete Kubernetes operator

Usage with Claude Code - add to your settings:
  {
    "mcpServers": {
      "openapi-operator-gen": {
        "command": "openapi-operator-gen",
        "args": ["mcp"]
      }
    }
  }`,
	RunE: runMCP,
}

func init() {
	rootCmd.AddCommand(mcpCmd)
}

func runMCP(cmd *cobra.Command, args []string) error {
	s := mcpserver.NewServer(version, commit, date)
	if err := server.ServeStdio(s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	return nil
}
