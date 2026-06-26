package main

import (
	"context"
	"strings"

	"github.com/mengshi02/codetrip"
	"github.com/mengshi02/codetrip/cmd/codetrip/utils"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func newMCPCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Start MCP server for AI coding agent integration",
		Long:  "Start a Model Context Protocol (MCP) server that exposes codetrip's code analysis capabilities as tools for AI coding agents like Claude, Codex, etc.",
		Example: `  # Start MCP server via stdio (for Claude Desktop, Cursor, etc.)
  codetrip mcp

  # Start with a specific trip directory
  codetrip mcp --trip-dir /custom/trip`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runMCPServer(flags)
		},
	}

	return cmd
}

func runMCPServer(flags *cliFlags) error {
	s := mcp.NewServer(&mcp.Implementation{Name: "codetrip", Version: codetrip.Version}, nil)
	registerMCPTools(s, flags)
	fmt.Println("Starting codetrip MCP server — Hybrid Graph-Augmented Code Intelligence Engine")
	return s.Run(context.Background(), &mcp.StdioTransport{})
}

func registerMCPTools(s *mcp.Server, flags *cliFlags) {
	// Index repo tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "index_repo",
		Description: "Index a code repository into the codetrip graph database. This scans the repository, builds the code graph, and creates search indexes.",
	}, makeIndexRepoHandler(flags))

	// Search tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "search_symbols",
		Description: "Search for code symbols using BM25 text search or hybrid semantic search.",
	}, makeSearchHandler(flags))

	// Impact analysis tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "impact_analysis",
		Description: "Analyze the upstream/downstream impact of modifying a code symbol. Shows risk level and affected symbols at each depth.",
	}, makeImpactHandler(flags))

	// Context tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "symbol_context",
		Description: "Get a 360-degree view of a code symbol including incoming/outgoing references and disambiguation candidates.",
	}, makeContextHandler(flags))

	// Detect changes tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "detect_changes",
		Description: "Detect file changes and affected symbols using incremental indexing. Shows risk summary and affected processes.",
	}, makeDetectChangesHandler(flags))

	// Check tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "structural_check",
		Description: "Perform structural checks on a repository's code graph, such as cycle detection.",
	}, makeCheckHandler(flags))

	// Explain (taint tracking) tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "explain_taint",
		Description: "Perform taint analysis on a symbol and trace data flow paths from source to sink.",
	}, makeExplainHandler(flags))

	// Route map tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "route_map",
		Description: "List API routes with their handlers and consumers.",
	}, makeRouteMapHandler(flags))

	// Tool map tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "tool_map",
		Description: "List MCP/RPC tools with their handlers.",
	}, makeToolMapHandler(flags))

	// Shape check tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "shape_check",
		Description: "Check if API route response shapes match consumer expectations.",
	}, makeShapeCheckHandler(flags))

	// API impact tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "api_impact",
		Description: "Analyze API route impact combining route map + impact + shape check.",
	}, makeApiImpactHandler(flags))

	// Rename tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "rename_symbol",
		Description: "Rename a code symbol across all files based on graph reference analysis. By default runs in dry-run mode.",
	}, makeRenameHandler(flags))

	// Stats tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "index_stats",
		Description: "Show index statistics for a repository.",
	}, makeStatsHandler(flags))

	// List repos tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_repos",
		Description: "List all indexed repositories.",
	}, makeListReposHandler(flags))

	// Repo status tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "repo_status",
		Description: "Show node and edge counts for a repository.",
	}, makeRepoStatusHandler(flags))

	// Drop index tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "drop_index",
		Description: "Delete all index data for a repository.",
	}, makeDropHandler(flags))

	// Group list tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "group_list",
		Description: "List all cross-repo groups.",
	}, makeGroupListHandler(flags))

	// Group sync tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "group_sync",
		Description: "Sync a cross-repo group by indexing multiple repositories together.",
	}, makeGroupSyncHandler(flags))

	// Group impact tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "group_impact",
		Description: "Perform cross-repo impact analysis across a group of repositories.",
	}, makeGroupImpactHandler(flags))

	// Embed repo tool
	mcp.AddTool(s, &mcp.Tool{
		Name:        "embed_repo",
		Description: "Generate dual-modal (description + code) vector embeddings for an already indexed repository. Requires prior index_repo.",
	}, makeEmbedRepoHandler(flags))
}

// ============ Input/Output Types for MCP Tools ============

type indexRepoInput struct {
	RepoPath string `json:"repo_path" jsonschema:"required,Absolute path to the repository to index"`
	RepoName string `json:"repo_name" jsonschema:"Repository name (default: base directory name)"`
	Workers  int    `json:"workers" jsonschema:"Max concurrent workers (0=CPU count)"`
	CFG      bool   `json:"cfg" jsonschema:"Enable CFG construction"`
	PDG      bool   `json:"pdg" jsonschema:"Enable PDG construction"`
}


type searchSymbolsInput struct {
	Query    string `json:"query" jsonschema:"required,Search query string"`
	Repo     string `json:"repo" jsonschema:"required,Repository name to search in"`
	Limit    int    `json:"limit" jsonschema:"Maximum number of results (default: 20)"`
	Semantic bool   `json:"semantic" jsonschema:"Enable semantic (hybrid) search"`
}

type impactAnalysisInput struct {
	Target      string `json:"target" jsonschema:"required,Symbol name to analyze impact for"`
	Repo        string `json:"repo" jsonschema:"required,Repository name"`
	Direction   string `json:"direction" jsonschema:"Traversal direction: 'downstream' or 'upstream' (default: downstream)"`
	MaxDepth    int    `json:"max_depth" jsonschema:"Maximum traversal depth (default: 3)"`
	Granularity string `json:"granularity" jsonschema:"Output granularity: 'summary' (risk+counts), 'symbol' (per-symbol detail), 'path' (full traversal paths with line numbers)"`
	EdgeFilter  string `json:"edge_filter" jsonschema:"Edge type filter: 'all' (default), 'calls_only' (only CALLS edges), 'semantic' (CALLS+IMPLEMENTS+EXTENDS+ACCESSES etc, excludes DEFINES/CONTAINS)"`
}

type symbolContextInput struct {
	Name     string `json:"name" jsonschema:"required,Symbol name to look up"`
	Repo     string `json:"repo" jsonschema:"required,Repository name"`
	FilePath string `json:"file_path" jsonschema:"File path to disambiguate the symbol"`
}

type detectChangesInput struct {
	Scope   string `json:"scope" jsonschema:"required,Directory path to check for changes"`
	BaseRef string `json:"base_ref" jsonschema:"required,Git ref (branch/commit) to compare against (e.g. main, HEAD~1)"`
	Repo    string `json:"repo" jsonschema:"required,Repository name"`
}

type structuralCheckInput struct {
	Repo   string `json:"repo" jsonschema:"required,Repository name"`
	Cycles bool   `json:"cycles" jsonschema:"Detect circular dependencies (default: true)"`
}

type explainTaintInput struct {
	Target string `json:"target" jsonschema:"required,Symbol name to trace taint for"`
	Repo   string `json:"repo" jsonschema:"required,Repository name"`
	Limit  int    `json:"limit" jsonschema:"Maximum number of findings (default: 100)"`
}

type routeMapInput struct {
	Repo  string `json:"repo" jsonschema:"required,Repository name"`
	Route string `json:"route" jsonschema:"Filter by specific route name"`
}

type toolMapInput struct {
	Repo string `json:"repo" jsonschema:"required,Repository name"`
	Tool string `json:"tool" jsonschema:"Filter by specific tool name"`
}

type shapeCheckInput struct {
	Repo  string `json:"repo" jsonschema:"required,Repository name"`
	Route string `json:"route" jsonschema:"Filter by specific route name"`
}

type apiImpactInput struct {
	Repo  string `json:"repo" jsonschema:"required,Repository name"`
	Route string `json:"route" jsonschema:"Analyze specific API route"`
}

type renameSymbolInput struct {
	SymbolName string `json:"symbol_name" jsonschema:"required,Current symbol name"`
	NewName    string `json:"new_name" jsonschema:"required,New name for the symbol"`
	Repo       string `json:"repo" jsonschema:"required,Repository name"`
	DryRun     bool   `json:"dry_run" jsonschema:"Dry-run mode (do not modify files, default: true)"`
}

type indexStatsInput struct {
	Repo string `json:"repo" jsonschema:"required,Repository name"`
}

type listReposInput struct{}

type repoStatusInput struct {
	Repo string `json:"repo" jsonschema:"required,Repository name"`
}

type dropIndexInput struct {
	Repo string `json:"repo" jsonschema:"required,Repository name to drop"`
}

type groupListInput struct{}

type groupSyncInput struct {
	GroupName string `json:"group_name" jsonschema:"required,Group name to sync"`
	Repos     string `json:"repos" jsonschema:"required,Repository paths as comma-separated path=name pairs"`
}

type groupImpactInput struct {
	GroupName string `json:"group_name" jsonschema:"required,Group name"`
	Target    string `json:"target" jsonschema:"required,Symbol name to analyze"`
	Direction string `json:"direction" jsonschema:"Traversal direction: 'downstream' or 'upstream' (default: downstream)"`
}

type embedRepoInput struct {
	Repo        string `json:"repo" jsonschema:"required,Repository name"`
	Endpoint    string `json:"endpoint" jsonschema:"required,HTTP embedding service endpoint (e.g. http://localhost:11434/v1/embeddings)"`
	Model       string `json:"model" jsonschema:"Model name (default: auto-detect from endpoint)"`
	APIKey      string `json:"api_key" jsonschema:"API key for the embedding service"`
	Dimensions  int    `json:"dimensions" jsonschema:"Vector dimensions (default: auto-detect)"`
	BatchSize   int    `json:"batch_size" jsonschema:"Batch size for embedding requests (default: 16)"`
	Incremental bool   `json:"incremental" jsonschema:"Skip nodes with unchanged content hash"`
	QuantInt8   bool   `json:"quant_int8" jsonschema:"Enable int8 vector quantization"`
	TwoStage    bool   `json:"two_stage" jsonschema:"Enable two-stage search (int8 coarse + float32 refine)"`
}

type textOutput struct {
	Text string `json:"text" jsonschema:"The result text"`
}

// ============ Tool Handlers ============

func makeIndexRepoHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input indexRepoInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input indexRepoInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTrip()
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		repoName := input.RepoName
		if repoName == "" {
			repoName = strings.TrimRight(input.RepoPath, "/")
		}

		var opts []codetrip.IndexOption
		opts = append(opts, codetrip.WithRepoName(repoName))
		if input.Workers > 0 {
			opts = append(opts, codetrip.WithMaxWorkers(input.Workers))
		}
		if input.CFG {
			opts = append(opts, codetrip.WithCFG(true))
		}
		if input.PDG {
			opts = append(opts, codetrip.WithPDG(true))
		}

		stats, err := utils.IndexRepo(ctx, trip, input.RepoPath, repoName, opts)
		if err != nil {
			return nil, textOutput{}, err
		}

		result := fmt.Sprintf("Indexing complete for %s\nFiles: %d, Symbols: %d, Edges: %d",
			input.RepoPath, stats.Files, stats.Nodes, stats.Edges)
		return nil, textOutput{Text: result}, nil
	}
}




func makeSearchHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input searchSymbolsInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input searchSymbolsInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		limit := input.Limit
		if limit <= 0 {
			limit = 20
		}

		searchReq := &codetrip.SearchRequest{
			Query:    input.Query,
			Limit:    limit,
			Repo:     input.Repo,
			Semantic: input.Semantic,
		}

		result, err := utils.Search(ctx, trip, searchReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		var output strings.Builder
		if result.Fallback == "bm25" {
			output.WriteString("Note: No embedding data found. Falling back to BM25 search.\n")
			output.WriteString(fmt.Sprintf("Run \"codetrip embed --repo %s --endpoint <url>\" to enable semantic search.\n\n", input.Repo))
		}
		b, _ := json.MarshalIndent(result.Results, "", "  ")
		output.Write(b)
		return nil, textOutput{Text: output.String()}, nil
	}
}

func makeImpactHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input impactAnalysisInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input impactAnalysisInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		direction := input.Direction
		if direction == "" {
			direction = "downstream"
		}
		maxDepth := input.MaxDepth
		if maxDepth <= 0 {
			maxDepth = 3
		}
		granularity := input.Granularity
		if granularity == "" {
			granularity = "summary"
		}

		impactReq := &codetrip.ImpactRequest{
			Target:      input.Target,
			Repo:        input.Repo,
			Direction:   direction,
			MaxDepth:    maxDepth,
			Granularity: granularity,
			EdgeFilter:  input.EdgeFilter,
		}

		result, err := utils.Impact(ctx, trip, impactReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeContextHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input symbolContextInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input symbolContextInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		ctxReq := &codetrip.ContextRequest{
			Name:     input.Name,
			Repo:     input.Repo,
			FilePath: input.FilePath,
		}

		result, err := utils.Context(ctx, trip, ctxReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeDetectChangesHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input detectChangesInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input detectChangesInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		detectReq := &codetrip.DetectChangesRequest{
			Scope:   input.Scope,
			BaseRef: input.BaseRef,
			Repo:    input.Repo,
		}

		result, err := utils.DetectChanges(ctx, trip, detectReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeCheckHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input structuralCheckInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input structuralCheckInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		checkReq := &codetrip.CheckRequest{Repo: input.Repo, Cycles: input.Cycles}
		if !input.Cycles {
			checkReq.Cycles = true // default true
		}

		result, err := utils.Check(ctx, trip, checkReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeExplainHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input explainTaintInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input explainTaintInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		limit := input.Limit
		if limit <= 0 {
			limit = 100
		}

		explainReq := &codetrip.ExplainRequest{
			Target: input.Target,
			Repo:   input.Repo,
			Limit:  limit,
		}

		result, err := utils.Explain(ctx, trip, explainReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeRouteMapHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input routeMapInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input routeMapInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		rmReq := &codetrip.RouteMapRequest{Route: input.Route, Repo: input.Repo}

		result, err := utils.RouteMap(ctx, trip, rmReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeToolMapHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input toolMapInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input toolMapInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		tmReq := &codetrip.ToolMapRequest{Tool: input.Tool, Repo: input.Repo}

		result, err := utils.ToolMap(ctx, trip, tmReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeShapeCheckHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input shapeCheckInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input shapeCheckInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		scReq := &codetrip.ShapeCheckRequest{Route: input.Route, Repo: input.Repo}

		result, err := utils.ShapeCheck(ctx, trip, scReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeApiImpactHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input apiImpactInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input apiImpactInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		aiReq := &codetrip.ApiImpactRequest{Route: input.Route, Repo: input.Repo}

		result, err := utils.ApiImpact(ctx, trip, aiReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeRenameHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input renameSymbolInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input renameSymbolInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		renameReq := &codetrip.RenameRequest{
			SymbolName: input.SymbolName,
			Repo:       input.Repo,
			NewName:    input.NewName,
			DryRun:     input.DryRun,
		}
		if !renameReq.DryRun {
			// default is true, only set false if explicitly false
		}

		result, err := utils.Rename(ctx, trip, renameReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeStatsHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input indexStatsInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input indexStatsInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		stats, err := utils.Stats(trip, input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(stats, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeListReposHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input listReposInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input listReposInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTrip()
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		repos, err := utils.ListRepos(trip)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(repos, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeRepoStatusHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input repoStatusInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input repoStatusInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		status, err := utils.RepoStatus(trip, input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(status, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeDropHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input dropIndexInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input dropIndexInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTripForRepo(input.Repo)
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		if err := utils.DropIndex(trip, input.Repo); err != nil {
			return nil, textOutput{}, err
		}

		return nil, textOutput{Text: fmt.Sprintf("Dropped index for repository: %s", input.Repo)}, nil
	}
}

func makeGroupListHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input groupListInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input groupListInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTrip()
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		groups, err := utils.GroupList(trip)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(groups, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeGroupSyncHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input groupSyncInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input groupSyncInput) (*mcp.CallToolResult, textOutput, error) {
		repoPaths := make(map[string]string)
		for _, r := range strings.Split(input.Repos, ",") {
			r = strings.TrimSpace(r)
			parts := strings.SplitN(r, "=", 2)
			if len(parts) == 2 {
				repoPaths[parts[0]] = parts[1]
			} else {
				repoPaths[r] = ""
			}
		}

		trip, err := flags.openTrip()
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		syncReq := &codetrip.GroupSyncRequest{
			GroupName: input.GroupName,
			RepoPaths: repoPaths,
		}

		result, err := utils.GroupSync(ctx, trip, syncReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeGroupImpactHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input groupImpactInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input groupImpactInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTrip()
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		direction := input.Direction
		if direction == "" {
			direction = "downstream"
		}

		impactReq := &codetrip.GroupImpactRequest{
			GroupName: input.GroupName,
			Target:    input.Target,
			Direction: direction,
		}

		result, err := utils.GroupImpact(ctx, trip, impactReq)
		if err != nil {
			return nil, textOutput{}, err
		}

		b, _ := json.MarshalIndent(result, "", "  ")
		return nil, textOutput{Text: string(b)}, nil
	}
}

func makeEmbedRepoHandler(flags *cliFlags) func(ctx context.Context, req *mcp.CallToolRequest, input embedRepoInput) (*mcp.CallToolResult, textOutput, error) {
	return func(ctx context.Context, req *mcp.CallToolRequest, input embedRepoInput) (*mcp.CallToolResult, textOutput, error) {
		trip, err := flags.openTrip()
		if err != nil {
			return nil, textOutput{}, err
		}
		defer trip.Close()

		opts := []codetrip.EmbedOption{
			codetrip.WithEmbedEndpoint(input.Endpoint),
		}
		if input.Model != "" {
			opts = append(opts, codetrip.WithEmbedModel(input.Model))
		}
		if input.APIKey != "" {
			opts = append(opts, codetrip.WithEmbedAPIKey(input.APIKey))
		}
		if input.Dimensions > 0 {
			opts = append(opts, codetrip.WithEmbedDimensions(input.Dimensions))
		}
		if input.BatchSize > 0 {
			opts = append(opts, codetrip.WithEmbedBatchSize(input.BatchSize))
		}
		if input.Incremental {
			opts = append(opts, codetrip.WithEmbedIncremental(true))
		}
		if input.QuantInt8 {
			opts = append(opts, codetrip.WithEmbedQuantInt8(true))
		}
		if input.TwoStage {
			opts = append(opts, codetrip.WithEmbedTwoStageSearch(true))
		}

		result, err := trip.EmbedRepo(ctx, input.Repo, opts...)
		if err != nil {
			return nil, textOutput{}, err
		}

		output := fmt.Sprintf("Embedding complete for %s\nNodes: %d, Desc chunks: %d, Code chunks: %d, Skipped: %d, Errors: %d",
			input.Repo, result.NodesEmbedded, result.DescChunks, result.CodeChunks, result.Skipped, result.Errors)
		return nil, textOutput{Text: output}, nil
	}
}
