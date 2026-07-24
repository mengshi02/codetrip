package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMCPSetupResolveClientsDetectsInstalledClients(t *testing.T) {
	options := &mcpSetupOptions{
		homeDir: t.TempDir(),
		lookPath: func(command string) (string, error) {
			if command == "codex" || command == "code" {
				return "/bin/" + command, nil
			}
			return "", os.ErrNotExist
		},
	}
	clients, err := options.resolveClients()
	if err != nil {
		t.Fatal(err)
	}
	if expected := []string{"codex", "vscode"}; !reflect.DeepEqual(clients, expected) {
		t.Fatalf("clients = %v, want %v", clients, expected)
	}
}

func TestMCPSetupAliasesAndDeduplicatesClients(t *testing.T) {
	options := &mcpSetupOptions{
		clients: []string{"claude-code", "code", "vscode", "github-copilot"},
		lookPath: func(command string) (string, error) {
			return "/bin/" + command, nil
		},
	}
	clients, err := options.resolveClients()
	if err != nil {
		t.Fatal(err)
	}
	expected := []string{"claude", "vscode", "copilot"}
	if !reflect.DeepEqual(clients, expected) {
		t.Fatalf("clients = %v, want %v", clients, expected)
	}
}

func TestMCPSetupAddCommands(t *testing.T) {
	options := &mcpSetupOptions{
		executable: "/opt/codetrip",
		dataDir:    "/data/codetrip",
	}
	tests := map[string][]string{
		"codex":   {"mcp", "add", "codetrip", "--", "/opt/codetrip", "mcp", "--dir", "/data/codetrip"},
		"claude":  {"mcp", "add", "--scope", "user", "codetrip", "--", "/opt/codetrip", "mcp", "--dir", "/data/codetrip"},
		"copilot": {"mcp", "add", "codetrip", "--", "/opt/codetrip", "mcp", "--dir", "/data/codetrip"},
	}
	for client, expected := range tests {
		command, args := options.addCommand(client)
		if command != client || !reflect.DeepEqual(args, expected) {
			t.Errorf("%s command = %q %v, want %q %v", client, command, args, client, expected)
		}
	}
	command, args := options.addCommand("vscode")
	if command != "code" || len(args) != 2 || args[0] != "--add-mcp" {
		t.Fatalf("vscode command = %q %v", command, args)
	}
	var config map[string]any
	if err := json.Unmarshal([]byte(args[1]), &config); err != nil {
		t.Fatal(err)
	}
	if config["name"] != "codetrip" || config["command"] != "/opt/codetrip" {
		t.Fatalf("unexpected VS Code configuration: %#v", config)
	}
}

func TestMCPSetupCursorPreservesExistingServers(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	initial := []byte("{\n  \"mcpServers\": {\n    \"existing\": {\"command\": \"example\"}\n  }\n}\n")
	if err := os.WriteFile(configPath, initial, 0o600); err != nil {
		t.Fatal(err)
	}
	options := &mcpSetupOptions{
		homeDir:    home,
		executable: "/opt/codetrip",
		dataDir:    "/data/codetrip",
	}
	var output bytes.Buffer
	status, err := options.installCursor(&output)
	if err != nil {
		t.Fatal(err)
	}
	if status != "configured" {
		t.Fatalf("status = %q", status)
	}
	encoded, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		Servers map[string]json.RawMessage `json:"mcpServers"`
	}
	if err := json.Unmarshal(encoded, &config); err != nil {
		t.Fatal(err)
	}
	if config.Servers["existing"] == nil || config.Servers["codetrip"] == nil {
		t.Fatalf("servers were not preserved and merged: %s", encoded)
	}
	backups, err := filepath.Glob(configPath + ".bak-*")
	if err != nil || len(backups) != 1 {
		t.Fatalf("backups = %v, err = %v", backups, err)
	}
}

func TestMCPSetupCursorDryRunDoesNotWrite(t *testing.T) {
	home := t.TempDir()
	options := &mcpSetupOptions{
		homeDir:    home,
		executable: "/opt/codetrip",
		dataDir:    "/data/codetrip",
		dryRun:     true,
	}
	var output bytes.Buffer
	status, err := options.installCursor(&output)
	if err != nil {
		t.Fatal(err)
	}
	if status != "would configure" {
		t.Fatalf("status = %q", status)
	}
	if _, err := os.Stat(filepath.Join(home, ".cursor", "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("dry run created a configuration file: %v", err)
	}
}
