package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mengshi02/codetrip"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServerTools(t *testing.T) {
	dataDir := t.TempDir()
	access := newEngineAccess(func() (*codetrip.Engine, error) { return codetrip.Open(dataDir) })

	ctx := context.Background()
	clientTransport, serverTransport := protocol.NewInMemoryTransports()
	serverSession, err := newMCPServer(access).Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = serverSession.Close() })

	client := protocol.NewClient(&protocol.Implementation{Name: "codetrip-test", Version: "test"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = clientSession.Close() })

	listed, err := clientSession.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"check":    false,
		"context":  false,
		"diff":     false,
		"impact":   false,
		"list":     false,
		"search":   false,
		"source":   false,
		"traverse": false,
		"path":     false,
		"rename":   false,
	}
	if len(listed.Tools) != len(want) {
		t.Fatalf("advertised %d tools, want exactly %d", len(listed.Tools), len(want))
	}
	for _, tool := range listed.Tools {
		if _, ok := want[tool.Name]; !ok {
			t.Errorf("unexpected tool %q was advertised", tool.Name)
		} else {
			want[tool.Name] = true
		}
		if tool.Name == "traverse" {
			schema, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{"out, in, or both", "forward", "CALLS"} {
				if !strings.Contains(string(schema), text) {
					t.Errorf("traverse input schema does not describe %q: %s", text, schema)
				}
			}
		}
		if tool.Name == "source" {
			schema, err := json.Marshal(tool.InputSchema)
			if err != nil {
				t.Fatal(err)
			}
			for _, text := range []string{"scope", "code", "docs", "all"} {
				if !strings.Contains(string(schema), text) {
					t.Errorf("source input schema does not describe %q: %s", text, schema)
				}
			}
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q was not advertised", name)
		}
	}

	result, err := clientSession.CallTool(ctx, &protocol.CallToolParams{Name: "list", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("list returned tool error: %v", result.Content)
	}

	// An idle MCP server must not retain the Pebble lock. CLI commands open the
	// same directory through a separate Engine instance.
	probe, err := codetrip.Open(dataDir)
	if err != nil {
		t.Fatalf("MCP server retained the engine lock after a request: %v", err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEngineAccessClosesAfterOperationError(t *testing.T) {
	dataDir := t.TempDir()
	access := newEngineAccess(func() (*codetrip.Engine, error) { return codetrip.Open(dataDir) })
	sentinel := errors.New("operation failed")
	err := access.use(context.Background(), func(*codetrip.Engine) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("use error=%v, want sentinel", err)
	}
	probe, err := codetrip.Open(dataDir)
	if err != nil {
		t.Fatalf("engine lock was retained after an operation error: %v", err)
	}
	if err := probe.Close(); err != nil {
		t.Fatal(err)
	}
}
