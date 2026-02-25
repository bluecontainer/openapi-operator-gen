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
	Long:  `Configure openapi-operator-gen integrations with external tools like Claude Code and GitHub Copilot.`,
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

var setupCopilotCmd = &cobra.Command{
	Use:   "copilot",
	Short: "Configure MCP server in GitHub Copilot (VS Code)",
	Long: `Add the openapi-operator-gen MCP server to GitHub Copilot in VS Code.

This configures the MCP server so GitHub Copilot can use openapi-operator-gen
tools (validate, preview, generate) directly.

Scopes:
  - "project" (default): Creates .vscode/mcp.json in the current directory.
    This is checked into version control so anyone cloning the repo gets
    the MCP server configured automatically.
  - "user": Writes to the VS Code user-level mcp.json.
    This makes the server available in all VS Code workspaces.

Examples:
  # Add to current project (.vscode/mcp.json)
  openapi-operator-gen setup copilot

  # Add globally to VS Code user config
  openapi-operator-gen setup copilot --scope user

  # Use a specific binary path
  openapi-operator-gen setup copilot --binary /usr/local/bin/openapi-operator-gen`,
	RunE: runSetupCopilot,
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.AddCommand(setupClaudeCodeCmd)
	setupCmd.AddCommand(setupCopilotCmd)

	setupClaudeCodeCmd.Flags().StringVar(&setupBinary, "binary", "", "Path to openapi-operator-gen binary (default: auto-detect)")
	setupClaudeCodeCmd.Flags().StringVar(&setupScope, "scope", "project", "Scope: \"project\" (writes .mcp.json) or \"user\" (runs claude mcp add)")

	setupCopilotCmd.Flags().StringVar(&setupBinary, "binary", "", "Path to openapi-operator-gen binary (default: auto-detect)")
	setupCopilotCmd.Flags().StringVar(&setupScope, "scope", "project", "Scope: \"project\" (writes .vscode/mcp.json) or \"user\" (writes to VS Code user config)")
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

func runSetupCopilot(cmd *cobra.Command, args []string) error {
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
		return setupCopilotProjectMCP(binaryPath)
	case "user":
		return setupCopilotUserMCP(binaryPath)
	default:
		return fmt.Errorf("invalid scope %q: must be \"project\" or \"user\"", setupScope)
	}
}

// setupCopilotProjectMCP writes a .vscode/mcp.json file in the current directory.
func setupCopilotProjectMCP(binaryPath string) error {
	mcpDir := ".vscode"
	mcpFile := filepath.Join(mcpDir, "mcp.json")

	// Create .vscode/ directory if needed
	if err := os.MkdirAll(mcpDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s directory: %w", mcpDir, err)
	}

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

	// Get or create servers (VS Code uses "servers", not "mcpServers")
	var servers map[string]any
	if existing, ok := settings["servers"]; ok {
		servers, ok = existing.(map[string]any)
		if !ok {
			return fmt.Errorf("servers in %s has unexpected type", mcpFile)
		}
	} else {
		servers = make(map[string]any)
		settings["servers"] = servers
	}

	// Check if already configured
	_, existed := servers["openapi-operator-gen"]

	// Set the MCP server entry (VS Code requires "type": "stdio")
	servers["openapi-operator-gen"] = map[string]any{
		"type":    "stdio",
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
	fmt.Println("GitHub Copilot will discover this MCP server when VS Code opens this project.")
	fmt.Println("Commit .vscode/mcp.json to share the configuration with your team.")

	return nil
}

// setupCopilotUserMCP writes the MCP server config to the VS Code user-level mcp.json.
func setupCopilotUserMCP(binaryPath string) error {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return fmt.Errorf("failed to determine user config directory: %w", err)
	}

	mcpFile := filepath.Join(configDir, "Code", "User", "mcp.json")

	// Create parent directories if needed
	dir := filepath.Dir(mcpFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

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

	// Get or create servers
	var servers map[string]any
	if existing, ok := settings["servers"]; ok {
		servers, ok = existing.(map[string]any)
		if !ok {
			return fmt.Errorf("servers in %s has unexpected type", mcpFile)
		}
	} else {
		servers = make(map[string]any)
		settings["servers"] = servers
	}

	_, existed := servers["openapi-operator-gen"]

	servers["openapi-operator-gen"] = map[string]any{
		"type":    "stdio",
		"command": binaryPath,
		"args":    []string{"mcp"},
	}

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
	fmt.Println("GitHub Copilot will now have access to openapi-operator-gen tools in all VS Code workspaces.")

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
