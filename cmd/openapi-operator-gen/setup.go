package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var (
	setupBinary string
	setupScope  string
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Configure integrations with external tools",
	Long:  `Configure openapi-operator-gen integrations with external tools like Claude Code.`,
}

var setupClaudeCodeCmd = &cobra.Command{
	Use:   "claude-code",
	Short: "Configure MCP server in Claude Code",
	Long: `Add the openapi-operator-gen MCP server to Claude Code.

This configures the MCP server so Claude Code can use openapi-operator-gen
tools (validate, preview, generate) directly.

Scopes:
  - "project" (default): Creates .mcp.json in the current directory.
    This is checked into version control so anyone cloning the repo gets
    the MCP server configured automatically.
  - "user": Runs "claude mcp add" to register the server globally.
    Requires the Claude Code CLI to be installed.

Examples:
  # Add to current project (.mcp.json)
  openapi-operator-gen setup claude-code

  # Add globally via Claude Code CLI
  openapi-operator-gen setup claude-code --scope user

  # Use a specific binary path
  openapi-operator-gen setup claude-code --binary /usr/local/bin/openapi-operator-gen`,
	RunE: runSetupClaudeCode,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.AddCommand(setupClaudeCodeCmd)

	setupClaudeCodeCmd.Flags().StringVar(&setupBinary, "binary", "", "Path to openapi-operator-gen binary (default: auto-detect)")
	setupClaudeCodeCmd.Flags().StringVar(&setupScope, "scope", "project", "Scope: \"project\" (writes .mcp.json) or \"user\" (runs claude mcp add)")
}

func runSetupClaudeCode(cmd *cobra.Command, args []string) error {
	// Determine binary path
	binaryPath := setupBinary
	if binaryPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to detect binary path: %w (use --binary to specify)", err)
		}
		binaryPath, err = filepath.EvalSymlinks(exe)
		if err != nil {
			binaryPath = exe
		}
	}

	switch setupScope {
	case "project":
		return setupProjectMCP(binaryPath)
	case "user":
		return setupUserMCP(binaryPath)
	default:
		return fmt.Errorf("invalid scope %q: must be \"project\" or \"user\"", setupScope)
	}
}

// setupProjectMCP writes a .mcp.json file in the current directory.
func setupProjectMCP(binaryPath string) error {
	mcpFile := ".mcp.json"

	// Read existing file or start fresh
	settings := make(map[string]any)
	data, err := os.ReadFile(mcpFile)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("failed to parse %s: %w", mcpFile, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", mcpFile, err)
	}

	// Get or create mcpServers
	var mcpServers map[string]any
	if existing, ok := settings["mcpServers"]; ok {
		mcpServers, ok = existing.(map[string]any)
		if !ok {
			return fmt.Errorf("mcpServers in %s has unexpected type", mcpFile)
		}
	} else {
		mcpServers = make(map[string]any)
		settings["mcpServers"] = mcpServers
	}

	// Check if already configured
	_, existed := mcpServers["openapi-operator-gen"]

	// Set the MCP server entry
	mcpServers["openapi-operator-gen"] = map[string]any{
		"command": binaryPath,
		"args":    []string{"mcp"},
	}

	// Write file
	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}
	output = append(output, '\n')

	if err := os.WriteFile(mcpFile, output, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", mcpFile, err)
	}

	if existed {
		fmt.Printf("Updated openapi-operator-gen MCP server in %s\n", mcpFile)
	} else {
		fmt.Printf("Added openapi-operator-gen MCP server to %s\n", mcpFile)
	}
	fmt.Printf("  Binary: %s\n", binaryPath)
	fmt.Println()
	fmt.Println("Claude Code will discover this MCP server when opened in this project.")
	fmt.Println("Commit .mcp.json to share the configuration with your team.")

	return nil
}

// setupUserMCP runs "claude mcp add" to register the server globally.
func setupUserMCP(binaryPath string) error {
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return fmt.Errorf("claude CLI not found in PATH: %w\n\nInstall Claude Code first: https://docs.anthropic.com/en/docs/claude-code", err)
	}

	cmd := exec.Command(claudePath, "mcp", "add", "openapi-operator-gen", "--transport", "stdio", "--", binaryPath, "mcp")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude mcp add failed: %w", err)
	}

	fmt.Println()
	fmt.Println("Claude Code will now have access to openapi-operator-gen tools in all projects.")

	return nil
}
