package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/mengshi02/codetrip"
	protocol "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

type listInput struct{}

type listOutput struct {
	Repositories []codetrip.RepoInfo `json:"repositories"`
}

// engineAccess serializes MCP requests and keeps the durable store open only
// for the duration of one request. A long-running stdio server therefore does
// not prevent CLI commands from opening the same data directory.
type engineAccess struct {
	gate chan struct{}
	open func() (*codetrip.Engine, error)
}

func newEngineAccess(open func() (*codetrip.Engine, error)) *engineAccess {
	gate := make(chan struct{}, 1)
	gate <- struct{}{}
	return &engineAccess{gate: gate, open: open}
}

func (access *engineAccess) use(ctx context.Context, operation func(*codetrip.Engine) error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-access.gate:
	}
	defer func() { access.gate <- struct{}{} }()

	engine, err := access.open()
	if err != nil {
		return fmt.Errorf("open codetrip engine for MCP request: %w", err)
	}
	operationErr := operation(engine)
	closeErr := engine.Close()
	return errors.Join(operationErr, closeErr)
}

func newMCPCmd(flags *cliFlags) *cobra.Command {
	return &cobra.Command{
		Use: "mcp", Short: "Start the codetrip MCP server", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			dir, err := flags.resolvedTripDir()
			if err != nil {
				return err
			}
			access := newEngineAccess(func() (*codetrip.Engine, error) {
				return codetrip.Open(dir)
			})
			return newMCPServer(access).Run(cmd.Context(), &protocol.StdioTransport{})
		},
	}
}

func newMCPServer(access *engineAccess) *protocol.Server {
	server := protocol.NewServer(&protocol.Implementation{Name: "codetrip", Version: codetrip.Version}, nil)

	protocol.AddTool(server, &protocol.Tool{
		Name:        "list",
		Description: "List repositories indexed in the codetrip data directory.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, _ listInput) (*protocol.CallToolResult, listOutput, error) {
		var output listOutput
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			repositories, err := engine.ListRepos()
			output.Repositories = repositories
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "search",
		Description: "Search indexed code symbols by name and metadata.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.SearchRequest) (*protocol.CallToolResult, codetrip.SearchResult, error) {
		var output codetrip.SearchResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.Search(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "source",
		Description: "Search repository contents using literal, regular-expression, file, and language filters. scope accepts code (default, including engineering configuration), docs, or all.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.SourceSearchRequest) (*protocol.CallToolResult, codetrip.SourceSearchResult, error) {
		var output codetrip.SourceSearchResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.SearchSource(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "traverse",
		Description: "Run bounded BFS from a code graph node. Canonical directions are out, in, and both; common aliases are accepted. Use relationTypes such as CALLS to restrict traversed edges.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.TraverseRequest) (*protocol.CallToolResult, codetrip.TraverseResult, error) {
		var output codetrip.TraverseResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.Traverse(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "path",
		Description: "Find the shortest directed path between two code graph nodes.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.PathRequest) (*protocol.CallToolResult, codetrip.PathResult, error) {
		var output codetrip.PathResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.ShortestPath(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "context",
		Description: "Explain a code symbol with source content and direct semantic relationships while excluding structural graph noise by default.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.ContextRequest) (*protocol.CallToolResult, codetrip.ContextResult, error) {
		var output codetrip.ContextResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.Context(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "impact",
		Description: "Find callers, importers, implementations, derived types, overrides, and bound entry points affected by changing a graph node.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.ImpactRequest) (*protocol.CallToolResult, codetrip.ImpactResult, error) {
		var output codetrip.ImpactResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.Impact(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "check",
		Description: "Check graph integrity, inheritance and import cycles, and optionally low-confidence semantic relationships.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.CheckRequest) (*protocol.CallToolResult, codetrip.CheckResult, error) {
		var output codetrip.CheckResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.Check(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "diff",
		Description: "Map Git changes to persisted symbols and aggregate reverse semantic impact. Defaults to comparing HEAD with the working tree.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.DiffRequest) (*protocol.CallToolResult, codetrip.DiffResult, error) {
		var output codetrip.DiffResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.Diff(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	protocol.AddTool(server, &protocol.Tool{
		Name:        "rename",
		Description: "Plan a non-mutating symbol rename with conflict detection, semantic references, and exact textual source candidates.",
	}, func(ctx context.Context, _ *protocol.CallToolRequest, input codetrip.RenameRequest) (*protocol.CallToolResult, codetrip.RenameResult, error) {
		var output codetrip.RenameResult
		err := access.use(ctx, func(engine *codetrip.Engine) error {
			result, err := engine.Rename(ctx, &input)
			if err == nil {
				output = *result
			}
			return err
		})
		return nil, output, err
	})

	return server
}
