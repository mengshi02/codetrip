package main

import (
	"context"
	"testing"

	"github.com/mengshi02/codetrip"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestMCPServerTools(t *testing.T) {
	trip, err := codetrip.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := trip.Close(); err != nil {
			t.Errorf("close trip: %v", err)
		}
	})

	ctx := context.Background()
	clientTransport, serverTransport := protocol.NewInMemoryTransports()
	serverSession, err := newMCPServer(trip).Connect(ctx, serverTransport, nil)
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
		"list_repositories": false,
		"search_symbols":    false,
		"search_source":     false,
		"traverse_graph":    false,
		"shortest_path":     false,
	}
	for _, tool := range listed.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q was not advertised", name)
		}
	}

	result, err := clientSession.CallTool(ctx, &protocol.CallToolParams{Name: "list_repositories", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("list_repositories returned tool error: %v", result.Content)
	}
}
