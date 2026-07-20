package main

import (
	"context"

	"github.com/mengshi02/codetrip"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

type listInput struct{}

type listOutput struct {
	Repositories []codetrip.RepoInfo `json:"repositories"`
}

func newMCPCmd(flags *cliFlags) *cobra.Command {
	return &cobra.Command{
		Use: "mcp", Short: "Start the codetrip MCP server", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer e.Close()
			return newMCPServer(e).Run(cmd.Context(), &protocol.StdioTransport{})
		},
	}
}

func newMCPServer(e *codetrip.Engine) *protocol.Server {
	server := protocol.NewServer(&protocol.Implementation{Name: "codetrip", Version: codetrip.Version}, nil)

	protocol.AddTool(server, &protocol.Tool{
		Name:        "list",
		Description: "List repositories indexed in the codetrip data directory.",
	}, func(_ context.Context, _ *protocol.CallToolRequest, _ listInput) (*protocol.CallToolResult, listOutput, error) {
		repositories, err := e.ListRepos()
		return nil, listOutput{Repositories: repositories}, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "search",
		Description: "Search indexed code symbols by name and metadata.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.SearchRequest) (*protocol.CallToolResult, codetrip.SearchResult, error) {
		result, err := e.Search(ctx, &input)
		if err != nil {
			return nil, codetrip.SearchResult{}, err
		}
		return nil, *result, nil
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "source",
		Description: "Search file names and source contents using literal, regular-expression, file, and language filters.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.SourceSearchRequest) (*protocol.CallToolResult, codetrip.SourceSearchResult, error) {
		result, err := e.SearchSource(ctx, &input)
		if err != nil {
			return nil, codetrip.SourceSearchResult{}, err
		}
		return nil, *result, nil
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "traverse",
		Description: "Run a bounded breadth-first traversal from a code graph node.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.TraverseRequest) (*protocol.CallToolResult, codetrip.TraverseResult, error) {
		result, err := e.Traverse(ctx, &input)
		if err != nil {
			return nil, codetrip.TraverseResult{}, err
		}
		return nil, *result, nil
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "path",
		Description: "Find the shortest directed path between two code graph nodes.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.PathRequest) (*protocol.CallToolResult, codetrip.PathResult, error) {
		result, err := e.ShortestPath(ctx, &input)
		if err != nil {
			return nil, codetrip.PathResult{}, err
		}
		return nil, *result, nil
	})

	return server
}
