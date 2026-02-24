package main

import (
	"encoding/json"
	"fmt"
	"os"
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
	Short: "Configure MCP server in Claude Code settings",
	Long: `Add the openapi-operator-gen MCP server to Claude Code settings.

This writes the MCP server configuration so Claude Code can use
openapi-operator-gen tools (validate, preview, generate) directly.

Examples:
  # Add to user-level settings (~/.claude/settings.json)
  openapi-operator-gen setup claude-code

  # Add to project-level settings (.claude/settings.json)
  openapi-operator-gen setup claude-code --scope project

  # Use a specific binary path
  openapi-operator-gen setup claude-code --binary /usr/local/bin/openapi-operator-gen`,
	RunE: runSetupClaudeCode,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.AddCommand(setupClaudeCodeCmd)

	setupClaudeCodeCmd.Flags().StringVar(&setupBinary, "binary", "", "Path to openapi-operator-gen binary (default: auto-detect)")
	setupClaudeCodeCmd.Flags().StringVar(&setupScope, "scope", "user", "Settings scope: \"user\" (~/.claude/settings.json) or \"project\" (.claude/settings.json)")
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

	// Determine settings file path
	var settingsPath string
	switch setupScope {
	case "user":
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to determine home directory: %w", err)
		}
		settingsPath = filepath.Join(home, ".claude", "settings.json")
	case "project":
		settingsPath = filepath.Join(".claude", "settings.json")
	default:
		return fmt.Errorf("invalid scope %q: must be \"user\" or \"project\"", setupScope)
	}

	// Read existing settings or start fresh
	settings := make(map[string]any)
	data, err := os.ReadFile(settingsPath)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return fmt.Errorf("failed to parse %s: %w", settingsPath, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read %s: %w", settingsPath, err)
	}

	// Get or create mcpServers
	var mcpServers map[string]any
	if existing, ok := settings["mcpServers"]; ok {
		mcpServers, ok = existing.(map[string]any)
		if !ok {
			return fmt.Errorf("mcpServers in %s has unexpected type", settingsPath)
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

	// Write settings back
	dir := filepath.Dir(settingsPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	output, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings: %w", err)
	}
	output = append(output, '\n')

	if err := os.WriteFile(settingsPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write %s: %w", settingsPath, err)
	}

	if existed {
		fmt.Printf("Updated openapi-operator-gen MCP server in %s\n", settingsPath)
	} else {
		fmt.Printf("Added openapi-operator-gen MCP server to %s\n", settingsPath)
	}
	fmt.Printf("  Binary: %s\n", binaryPath)
	fmt.Println()
	fmt.Println("Claude Code will now have access to openapi-operator-gen tools.")
	fmt.Println("Restart Claude Code to pick up the new configuration.")

	return nil
}
