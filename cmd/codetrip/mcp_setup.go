package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const mcpServerName = "codetrip"

var supportedMCPClients = []string{"codex", "claude", "cursor", "vscode", "copilot"}

type mcpSetupOptions struct {
	all        bool
	dryRun     bool
	force      bool
	clients    []string
	executable string
	dataDir    string
	homeDir    string
	lookPath   func(string) (string, error)
	run        func(io.Writer, string, ...string) error
}

func newMCPSetupCmd(flags *cliFlags) *cobra.Command {
	options := &mcpSetupOptions{}
	command := &cobra.Command{
		Use:   "setup [client...]",
		Short: "Install Codetrip into supported code agents",
		Long:  "Install the Codetrip stdio MCP server into Codex, Claude Code, Cursor, VS Code, or GitHub Copilot CLI. With no client arguments, all detected clients are configured.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			options.clients = args
			options.dataDir = flags.tripDir
			return options.execute(cmd)
		},
	}
	command.Flags().BoolVar(&options.all, "all", false, "configure every supported client that is installed")
	command.Flags().BoolVar(&options.dryRun, "dry-run", false, "show changes without applying them")
	command.Flags().BoolVar(&options.force, "force", false, "replace an existing Codetrip MCP configuration")
	return command
}

func (options *mcpSetupOptions) execute(cmd *cobra.Command) error {
	if options.lookPath == nil {
		options.lookPath = exec.LookPath
	}
	if options.run == nil {
		options.run = runSetupCommand
	}
	if options.executable == "" {
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve codetrip executable: %w", err)
		}
		options.executable, err = filepath.Abs(executable)
		if err != nil {
			return fmt.Errorf("resolve absolute codetrip executable: %w", err)
		}
	}
	if options.homeDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		options.homeDir = home
	}
	if options.dataDir == "" {
		options.dataDir = filepath.Join(options.homeDir, ".codetrip")
	} else {
		dataDir, err := filepath.Abs(options.dataDir)
		if err != nil {
			return fmt.Errorf("resolve data directory: %w", err)
		}
		options.dataDir = dataDir
	}

	clients, err := options.resolveClients()
	if err != nil {
		return err
	}
	if len(clients) == 0 {
		return errors.New("no supported code agent detected; specify one of: " + strings.Join(supportedMCPClients, ", "))
	}

	var failures []error
	for _, client := range clients {
		status, err := options.install(cmd.OutOrStdout(), client)
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", client, err))
			fmt.Fprintf(cmd.ErrOrStderr(), "✗ %-8s %v\n", client, err)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✓ %-8s %s\n", client, status)
	}
	return errors.Join(failures...)
}

func (options *mcpSetupOptions) resolveClients() ([]string, error) {
	requested := options.clients
	if options.all {
		if len(requested) > 0 {
			return nil, errors.New("--all cannot be combined with client arguments")
		}
		for _, client := range supportedMCPClients {
			if options.clientInstalled(client) {
				requested = append(requested, client)
			}
		}
	}
	if len(requested) == 0 {
		for _, client := range supportedMCPClients {
			if options.clientInstalled(client) {
				requested = append(requested, client)
			}
		}
	}

	seen := make(map[string]bool, len(requested))
	clients := make([]string, 0, len(requested))
	for _, raw := range requested {
		client := normalizeMCPClient(raw)
		if !slices.Contains(supportedMCPClients, client) {
			return nil, fmt.Errorf("unsupported client %q; supported clients: %s", raw, strings.Join(supportedMCPClients, ", "))
		}
		if seen[client] {
			continue
		}
		if !options.clientInstalled(client) {
			return nil, fmt.Errorf("%s is not installed or its command is not available", client)
		}
		seen[client] = true
		clients = append(clients, client)
	}
	return clients, nil
}

func normalizeMCPClient(client string) string {
	client = strings.ToLower(strings.TrimSpace(client))
	switch client {
	case "claude-code":
		return "claude"
	case "vs-code", "code":
		return "vscode"
	case "github-copilot", "github-copilot-cli":
		return "copilot"
	default:
		return client
	}
}

func (options *mcpSetupOptions) clientInstalled(client string) bool {
	switch client {
	case "cursor":
		if _, err := options.lookPath("cursor"); err == nil {
			return true
		}
		_, err := os.Stat(filepath.Join(options.homeDir, ".cursor"))
		return err == nil
	case "vscode":
		_, err := options.lookPath("code")
		return err == nil
	default:
		_, err := options.lookPath(client)
		return err == nil
	}
}

func (options *mcpSetupOptions) install(output io.Writer, client string) (string, error) {
	if client == "cursor" {
		return options.installCursor(output)
	}

	command, args := options.addCommand(client)
	if options.dryRun {
		fmt.Fprintf(output, "  %s\n", formatCommand(command, args))
		return "would configure", nil
	}

	exists := options.serverExists(client)
	if exists && !options.force {
		return "already configured (use --force to replace)", nil
	}
	if exists {
		removeCommand, removeArgs := options.removeCommand(client)
		if err := options.run(output, removeCommand, removeArgs...); err != nil {
			return "", fmt.Errorf("remove existing configuration: %w", err)
		}
	}
	if err := options.run(output, command, args...); err != nil {
		return "", err
	}
	return "configured", nil
}

func (options *mcpSetupOptions) addCommand(client string) (string, []string) {
	serverArgs := []string{"mcp", "--dir", options.dataDir}
	switch client {
	case "codex":
		return "codex", append([]string{"mcp", "add", mcpServerName, "--", options.executable}, serverArgs...)
	case "claude":
		return "claude", append([]string{"mcp", "add", "--scope", "user", mcpServerName, "--", options.executable}, serverArgs...)
	case "vscode":
		config := map[string]any{"name": mcpServerName, "type": "stdio", "command": options.executable, "args": serverArgs}
		encoded, _ := json.Marshal(config)
		return "code", []string{"--add-mcp", string(encoded)}
	case "copilot":
		return "copilot", append([]string{"mcp", "add", mcpServerName, "--", options.executable}, serverArgs...)
	default:
		panic("unsupported MCP client: " + client)
	}
}

func (options *mcpSetupOptions) removeCommand(client string) (string, []string) {
	switch client {
	case "vscode":
		return "", nil
	default:
		command := client
		if client == "vscode" {
			command = "code"
		}
		return command, []string{"mcp", "remove", mcpServerName}
	}
}

func (options *mcpSetupOptions) serverExists(client string) bool {
	if client == "vscode" {
		return false
	}
	command := client
	args := []string{"mcp", "get", mcpServerName}
	if client == "claude" {
		args = []string{"mcp", "get", mcpServerName}
	}
	return exec.Command(command, args...).Run() == nil
}

func (options *mcpSetupOptions) installCursor(output io.Writer) (string, error) {
	configPath := filepath.Join(options.homeDir, ".cursor", "mcp.json")
	config := make(map[string]any)
	if encoded, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(encoded, &config); err != nil {
			return "", fmt.Errorf("parse %s: %w", configPath, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}

	servers, _ := config["mcpServers"].(map[string]any)
	if servers == nil {
		servers = make(map[string]any)
		config["mcpServers"] = servers
	}
	server := map[string]any{
		"command": options.executable,
		"args":    []string{"mcp", "--dir", options.dataDir},
	}
	if existing, exists := servers[mcpServerName]; exists {
		if jsonEqual(existing, server) {
			return "already configured", nil
		}
		if !options.force {
			return "existing configuration differs (use --force to replace)", nil
		}
	}

	servers[mcpServerName] = server
	encoded, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}
	encoded = append(encoded, '\n')
	if options.dryRun {
		fmt.Fprintf(output, "  update %s\n%s", configPath, encoded)
		return "would configure", nil
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		return "", err
	}
	if _, err := os.Stat(configPath); err == nil {
		backup := configPath + ".bak-" + time.Now().Format("20060102-150405")
		if err := copyFile(configPath, backup); err != nil {
			return "", fmt.Errorf("back up Cursor configuration: %w", err)
		}
		fmt.Fprintf(output, "  backup %s\n", backup)
	}
	if err := atomicWriteFile(configPath, encoded, 0o600); err != nil {
		return "", err
	}
	return "configured", nil
}

func runSetupCommand(output io.Writer, command string, args ...string) error {
	process := exec.Command(command, args...)
	process.Stdout = output
	process.Stderr = output
	return process.Run()
}

func formatCommand(command string, args []string) string {
	parts := append([]string{command}, args...)
	for index, part := range parts {
		if strings.ContainsAny(part, " \t\"'") {
			parts[index] = fmt.Sprintf("%q", part)
		}
	}
	return strings.Join(parts, " ")
}

func jsonEqual(left, right any) bool {
	leftJSON, _ := json.Marshal(left)
	rightJSON, _ := json.Marshal(right)
	return bytes.Equal(leftJSON, rightJSON)
}

func copyFile(source, target string) error {
	content, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	info, err := os.Stat(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, content, info.Mode().Perm())
}

func atomicWriteFile(path string, content []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer os.Remove(tempPath)
	if err := temp.Chmod(mode); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	return os.Rename(tempPath, path)
}
