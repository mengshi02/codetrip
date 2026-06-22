package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/mengshi02/codetrip"
	"github.com/mengshi02/codetrip/cmd/codetrip/utils"
	"github.com/spf13/cobra"
)

// mustMarkFlagRequired marks a flag as required and panics on error.
func mustMarkFlagRequired(cmd *cobra.Command, flag string) {
	if err := cmd.MarkFlagRequired(flag); err != nil {
		panic(fmt.Sprintf("failed to mark flag %q as required: %v", flag, err))
	}
}

// cliFlags holds all CLI flag values, shared across command definitions.
type cliFlags struct {
	// Global
	tripDir string

	// Index
	indexRepoName   string
	indexWorkers    int
	indexByteBudget int64
	indexWithCFG    bool
	indexWithPDG    bool

	// Query
	queryRepo string

	// Impact
	impactDirection string
	impactMaxDepth  int

	// Context
	contextFile string

	// Search
	searchLimit    int
	searchRepo     string
	searchSemantic bool

	// Embed
	embedEndpoint  string
	embedModel     string
	embedAPIKey    string
	embedDims      int
	embedBatchSize int
	embedIncremental bool
	embedQuantInt8 bool
	embedTwoStage  bool
	embedTimeout   time.Duration

	// Rename
	renameNewName string
	renameDryRun  bool

	// Detect-changes
	changesRepo string

	// Reindex
	reindexRepoName string

	// Route-map
	routeName string

	// Tool-map
	toolName string

	// Shape-check
	shapeRoute string

	// API-impact
	apiRoute string

	// Explain
	explainLimit int

	// Common repo
	commonRepo string

	// Group sync
	groupSyncRepos []string

	// Group impact
	groupImpactDirection string
}

// newCLIFlags returns a new cliFlags instance with defaults.
func newCLIFlags() *cliFlags {
	return &cliFlags{
		indexByteBudget:      20 << 20,
		impactDirection:      "downstream",
		impactMaxDepth:       3,
		searchLimit:          20,
		renameDryRun:         true,
		explainLimit:         100,
		groupImpactDirection: "downstream",
	}
}

// openTripForRepo is a CLI convenience that uses flags to resolve the trip directory.
func (f *cliFlags) openTripForRepo(repo string) (*codetrip.Trip, error) {
	return utils.OpenTripWithDir(f.tripDir)
}

// openTrip is a CLI convenience that uses flags to resolve the trip directory.
func (f *cliFlags) openTrip() (*codetrip.Trip, error) {
	return utils.OpenTripWithDir(f.tripDir)
}

// ============ Root Command ============

func newRootCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "codetrip",
		Short: "codetrip - Hybrid Graph-Augmented Code Intelligence Engine",
		Long:  "codetrip is a Hybrid Graph-Augmented Code Intelligence Engine that builds knowledge graphs from code and augments retrieval with BM25 + semantic vectors.",
		Example: `  # Index a repository
  codetrip index /path/to/my-project

  # Query with Cypher
  codetrip query "MATCH (n:Function) RETURN n.name LIMIT 10" --repo redis

  # Analyze impact of a symbol change
  codetrip impact User --repo redis --direction upstream --max-depth 5`,
	}

	cmd.PersistentFlags().StringVar(&flags.tripDir, "trip-dir", "", "Trip directory (default: ~/.codetrip)")

	cmd.AddCommand(
		newIndexCmd(flags),
		newReIndexCmd(flags),
		newQueryCmd(flags),
		newInfoCmd(flags),
		newVersionCmd(),
		newImpactCmd(flags),
		newContextCmd(flags),
		newCheckCmd(flags),
		newSearchCmd(flags),
		newEmbedCmd(flags),
		newRenameCmd(flags),
		newDetectChangesCmd(flags),
		newRouteMapCmd(flags),
		newToolMapCmd(flags),
		newShapeCheckCmd(flags),
		newApiImpactCmd(flags),
		newExplainCmd(flags),
		newGroupCmd(flags),
		newDropCmd(flags),
		newListReposCmd(flags),
		newRepoStatusCmd(flags),
	)

	return cmd
}

// ============ Index Command ============

func newIndexCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index <repo-path>",
		Short: "Index a code repository",
		Long:  "Scan and index a repository, building the code graph and search index.",
		Example: `  # Basic indexing (stores data in ~/.codetrip/<repo>)
  codetrip index /path/to/my-project

  # Index with a custom repo name and CFG enabled
  codetrip index /path/to/my-project --name my-service --cfg

  # Index with limited workers and a custom trip directory
  codetrip index /path/to/my-project --name redis --workers 4 --trip-dir /custom/trip`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}

			repoName := flags.indexRepoName
			if repoName == "" {
				repoName = filepath.Base(repoPath)
			}

			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()

			fmt.Printf("Indexing %s\n", repoPath)
			start := time.Now()

			var opts []codetrip.IndexOption
			if flags.indexRepoName != "" {
				opts = append(opts, codetrip.WithRepoName(flags.indexRepoName))
			} else {
				opts = append(opts, codetrip.WithRepoName(filepath.Base(repoPath)))
			}
			if flags.indexWorkers > 0 {
				opts = append(opts, codetrip.WithMaxWorkers(flags.indexWorkers))
			}
			if flags.indexByteBudget > 0 {
				opts = append(opts, codetrip.WithByteBudget(flags.indexByteBudget))
			}
			if flags.indexWithCFG {
				opts = append(opts, codetrip.WithCFG(true))
			}
			if flags.indexWithPDG {
				opts = append(opts, codetrip.WithPDG(true))
			}

			ctx := context.Background()
			stats, err := utils.IndexRepo(ctx, trip, repoPath, repoName, opts)
			if err != nil {
				return err
			}

			elapsed := time.Since(start)
			fmt.Printf("\nIndexing complete in %s\n", elapsed.Round(time.Millisecond))
			fmt.Printf("  Files scanned: %d\n", stats.Files)
			fmt.Printf("  Symbols found: %d\n", stats.Nodes)
			fmt.Printf("  Edges created: %d\n", stats.Edges)
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.indexRepoName, "name", "", "Repository name (default: base directory name)")
	cmd.Flags().IntVar(&flags.indexWorkers, "workers", 0, "Max concurrent workers (0=CPU count)")
	cmd.Flags().Int64Var(&flags.indexByteBudget, "byte-budget", 20<<20, "Byte budget per chunk")
	cmd.Flags().BoolVar(&flags.indexWithCFG, "cfg", false, "Enable CFG construction")
	cmd.Flags().BoolVar(&flags.indexWithPDG, "pdg", false, "Enable PDG construction")

	return cmd
}

// ============ Query Command ============

func newQueryCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <cypher-query>",
		Short: "Execute a Cypher query",
		Long:  "Execute a Cypher query against the specified repository.",
		Example: `  # List all function names
  codetrip query "MATCH (n:Function) RETURN n.name LIMIT 10" --repo redis

  # Find call relationships
  codetrip query "MATCH (a)-[:CALLS]->(b) WHERE a.name = 'handleRequest' RETURN b.name" --repo my-project`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			queryStr := args[0]

			trip, err := flags.openTripForRepo(flags.queryRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			result, err := utils.Query(ctx, trip, queryStr, flags.queryRepo)
			if err != nil {
				return err
			}

			if len(result.Rows) == 0 {
				fmt.Println("(no results)")
				return nil
			}

			if len(result.Columns) > 0 {
				fmt.Println(strings.Join(result.Columns, "\t"))
				fmt.Println(strings.Repeat("-", 40))
			}

			for _, row := range result.Rows {
				vals := make([]string, 0, len(result.Columns))
				for _, col := range result.Columns {
					v := row[col]
					vals = append(vals, utils.FormatValue(v))
				}
				fmt.Println(strings.Join(vals, "\t"))
			}

			fmt.Printf("\n(%d rows)\n", len(result.Rows))
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.queryRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Search Command ============

func newSearchCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search symbols",
		Long:  "Search symbols using BM25 or hybrid (semantic) search.",
		Example: `  # BM25 text search
  codetrip search "hash table" --repo redis

  # Semantic hybrid search
  codetrip search "user login validation" --repo redis --semantic --limit 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := args[0]

			trip, err := flags.openTripForRepo(flags.searchRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.SearchRequest{
				Query:    query,
				Limit:    flags.searchLimit,
				Repo:     flags.searchRepo,
				Semantic: flags.searchSemantic,
			}

			result, err := utils.Search(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("repository %q not found (use --repo to specify)", flags.searchRepo)
				}
				return err
			}

			if len(result.Results) == 0 {
				fmt.Println("(no results)")
				return nil
			}

			// Show fallback message if semantic search fell back to BM25
			if result.Fallback == "bm25" {
				fmt.Printf("Note: No embedding data found. Falling back to BM25 search.\n")
				fmt.Printf("Run \"codetrip embed --repo %s --endpoint <url>\" to enable semantic search.\n\n", flags.searchRepo)
			}

			fmt.Printf("Search Results (%d):\n", len(result.Results))
			for i, item := range result.Results {
				fmt.Printf("  %d. %s [%s] - %s", i+1, item.Name, item.Kind, item.FilePath)
				if item.StartLine > 0 {
					fmt.Printf(":%d", item.StartLine)
				}
				fmt.Printf(" (score: %.4f)\n", item.Score)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&flags.searchLimit, "limit", 20, "Maximum number of results")
	cmd.Flags().StringVar(&flags.searchRepo, "repo", "", "Repository name (required)")
	cmd.Flags().BoolVar(&flags.searchSemantic, "semantic", false, "Enable semantic (hybrid) search")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Embed Command ============

func newEmbedCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "embed",
		Short: "Generate vector embeddings for semantic search",
		Long:  "Generate vector embeddings for an already indexed repository to enable hybrid (BM25 + semantic) search. Requires prior 'codetrip index'.",
		Example: `  # Generate embeddings for hybrid search (requires prior index)
  codetrip embed --repo my-project --endpoint http://localhost:11434/v1/embeddings

  # Incremental embedding (only changed nodes)
  codetrip embed --repo my-project --endpoint http://localhost:11434/v1/embeddings --incremental

  # With int8 quantization and two-stage search
  codetrip embed --repo my-project --endpoint http://localhost:11434/v1/embeddings --quant-int8 --two-stage`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			opts := []codetrip.EmbedOption{
				codetrip.WithEmbedEndpoint(flags.embedEndpoint),
			}
			if flags.embedModel != "" {
				opts = append(opts, codetrip.WithEmbedModel(flags.embedModel))
			}
			if flags.embedAPIKey != "" {
				opts = append(opts, codetrip.WithEmbedAPIKey(flags.embedAPIKey))
			}
			if flags.embedDims > 0 {
				opts = append(opts, codetrip.WithEmbedDimensions(flags.embedDims))
			}
			if flags.embedBatchSize > 0 {
				opts = append(opts, codetrip.WithEmbedBatchSize(flags.embedBatchSize))
			}
			if flags.embedIncremental {
				opts = append(opts, codetrip.WithEmbedIncremental(true))
			}
			if flags.embedQuantInt8 {
				opts = append(opts, codetrip.WithEmbedQuantInt8(true))
			}
			if flags.embedTwoStage {
				opts = append(opts, codetrip.WithEmbedTwoStageSearch(true))
			}
			if flags.embedTimeout > 0 {
				opts = append(opts, codetrip.WithEmbedTimeout(flags.embedTimeout))
			}

			result, err := trip.EmbedRepo(ctx, flags.searchRepo, opts...)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotIndexed) {
					return fmt.Errorf("repository %q has not been indexed. Run \"codetrip index\" first.", flags.searchRepo)
				}
				return err
			}

			fmt.Printf("\nEmbedding complete in %s\n", time.Duration(result.Duration*float64(time.Second)).Round(time.Millisecond))
			fmt.Printf("  Nodes embedded: %d\n", result.NodesEmbedded)
			fmt.Printf("  Description chunks: %d\n", result.DescChunks)
			fmt.Printf("  Code chunks: %d\n", result.CodeChunks)
			if result.Skipped > 0 {
				fmt.Printf("  Skipped (unchanged): %d\n", result.Skipped)
			}
			if result.Errors > 0 {
				fmt.Printf("  Errors: %d\n", result.Errors)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.embedEndpoint, "endpoint", "", "HTTP embedding service endpoint (required, e.g. http://localhost:11434/v1/embeddings)")
	cmd.Flags().StringVar(&flags.embedModel, "model", "", "Model name (default: auto-detect from endpoint)")
	cmd.Flags().StringVar(&flags.embedAPIKey, "api-key", "", "API key for the embedding service")
	cmd.Flags().IntVar(&flags.embedDims, "dimensions", 0, "Vector dimensions (default: auto-detect)")
	cmd.Flags().IntVar(&flags.embedBatchSize, "batch-size", 16, "Batch size for embedding requests")
	cmd.Flags().BoolVar(&flags.embedIncremental, "incremental", false, "Skip nodes with unchanged content hash")
	cmd.Flags().BoolVar(&flags.embedQuantInt8, "quant-int8", false, "Enable int8 vector quantization")
	cmd.Flags().BoolVar(&flags.embedTwoStage, "two-stage", false, "Enable two-stage search (int8 coarse + float32 refine)")
	cmd.Flags().DurationVar(&flags.embedTimeout, "timeout", 30*time.Minute, "Embedding timeout")
	cmd.Flags().StringVar(&flags.searchRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "endpoint")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Impact Command ============

func newImpactCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "impact <target>",
		Short: "Impact analysis",
		Long:  "Analyze upstream/downstream impact of modifying a symbol.",
		Example: `  # Downstream impact analysis (default)
  codetrip impact dictFind --repo redis

  # Upstream impact analysis with a deeper traversal
  codetrip impact User --repo redis --direction upstream --max-depth 5`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.ImpactRequest{
				Target:    target,
				Repo:      flags.commonRepo,
				Direction: flags.impactDirection,
				MaxDepth:  flags.impactMaxDepth,
			}

			result, err := utils.Impact(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("repository %q not found (use --repo to specify)", flags.commonRepo)
				}
				return err
			}

			totalAffected := 0
			for _, count := range result.ByDepthCounts {
				totalAffected += count
			}

			fmt.Printf("Impact Analysis: %s\n", target)
			fmt.Printf("  Risk Level:      %s\n", result.Risk)
			fmt.Printf("  Total Affected:  %d symbols\n", totalAffected)

			for _, dg := range result.ByDepth {
				fmt.Printf("\n  Depth %d (%d symbols):\n", dg.Depth, len(dg.Symbols))
				for _, sym := range dg.Symbols {
					name := sym.Name
					if name == "" {
						name = sym.NodeID
					}
					fp := sym.FilePath
					if fp == "" {
						fp = "<no file>"
					}
					fmt.Printf("    - %s (%s) in %s\n", name, sym.Kind, fp)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.impactDirection, "direction", "downstream", "Traversal direction: downstream or upstream")
	cmd.Flags().IntVar(&flags.impactMaxDepth, "max-depth", 3, "Maximum traversal depth")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Context Command ============

func newContextCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "context <symbol-name>",
		Short: "360-degree symbol view",
		Long:  "Show full context for a symbol including incoming/outgoing references and disambiguation.",
		Example: `  # View context for a symbol
  codetrip context dictFind --repo redis

  # Disambiguate by specifying the file path
  codetrip context User --repo my-project --file pkg/models/user.go`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbolName := args[0]

			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.ContextRequest{
				Name:     symbolName,
				Repo:     flags.commonRepo,
				FilePath: flags.contextFile,
			}

			result, err := utils.Context(ctx, trip, req)
			if err != nil {
				return err
			}

			fmt.Printf("360° Context: %s\n", result.Symbol.Name)
			fmt.Printf("  Kind: %s\n", result.Symbol.Kind)
			fmt.Printf("  File: %s\n", result.Symbol.FilePath)

			if len(result.Incoming) > 0 {
				fmt.Println("\n  Incoming References:")
				for _, rg := range result.Incoming {
					fmt.Printf("    [%s] %d refs\n", rg.Type, len(rg.Refs))
					for _, ref := range rg.Refs {
						fmt.Printf("      - %s (%s)\n", ref.Name, ref.FilePath)
					}
				}
			}

			if len(result.Outgoing) > 0 {
				fmt.Println("\n  Outgoing References:")
				for _, rg := range result.Outgoing {
					fmt.Printf("    [%s] %d refs\n", rg.Type, len(rg.Refs))
					for _, ref := range rg.Refs {
						fmt.Printf("      - %s (%s)\n", ref.Name, ref.FilePath)
					}
				}
			}

			if len(result.Disambiguation) > 1 {
				fmt.Printf("\n  Disambiguation (%d candidates):\n", len(result.Disambiguation))
				for _, c := range result.Disambiguation {
					fmt.Printf("    - %s (%s) in %s\n", c.Name, c.Kind, c.FilePath)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.contextFile, "file", "", "File path to disambiguate the symbol")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Detect Changes Command ============

func newDetectChangesCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect-changes <scope>",
		Short: "Detect code changes",
		Long:  "Detect file changes and affected symbols using incremental indexing.",
		Example: `  # Detect changes in a specific directory
  codetrip detect-changes /Users/me/c/redis --repo redis`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			scope := args[0]

			trip, err := flags.openTripForRepo(flags.changesRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.DetectChangesRequest{
				Scope: scope,
				Repo:  flags.changesRepo,
			}

			result, err := utils.DetectChanges(ctx, trip, req)
			if err != nil {
				return err
			}

			fmt.Printf("Change Detection: %s\n", scope)
			fmt.Printf("  Risk Level:    %s\n", result.RiskSummary.Level)
			fmt.Printf("  Total Changes: %d\n", result.RiskSummary.TotalChanges)
			fmt.Printf("  High Risk:     %d\n", result.RiskSummary.HighRisk)

			if len(result.ChangedSymbols) > 0 {
				fmt.Println("\n  Changed Symbols:")
				for _, sym := range result.ChangedSymbols {
					fmt.Printf("    [%s] %s in %s\n", sym.ChangeType, sym.Name, sym.FilePath)
				}
			}

			if len(result.AffectedProcesses) > 0 {
				fmt.Println("\n  Affected Processes:")
				for _, p := range result.AffectedProcesses {
					fmt.Printf("    - %s (%s)\n", p.ID, p.Label)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.changesRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ ReIndex Command ============

func newReIndexCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reindex <repo-path>",
		Short: "Incremental re-index an already indexed repository",
		Long:  "Detect changes (added/modified/deleted files) and incrementally update the code graph, BM25, and vector indexes. Must have been indexed first with 'codetrip index'.",
		Example: `  # Re-index with default repo name
  codetrip reindex /path/to/my-project

  # Re-index with a custom repo name
  codetrip reindex /path/to/my-project --name my-service`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := filepath.Abs(args[0])
			if err != nil {
				return fmt.Errorf("failed to resolve path: %w", err)
			}

			repoName := flags.reindexRepoName
			if repoName == "" {
				repoName = filepath.Base(repoPath)
			}

			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()

			fmt.Printf("Re-indexing %s (repo: %s)\n", repoPath, repoName)
			start := time.Now()

			var opts []codetrip.IndexOption
			if flags.reindexRepoName != "" {
				opts = append(opts, codetrip.WithRepoName(flags.reindexRepoName))
			}

			ctx := context.Background()
			result, err := utils.ReIndex(ctx, trip, repoPath, opts)
			if err != nil {
				return err
			}

			elapsed := time.Since(start)
			fmt.Printf("\nRe-indexing complete in %s\n", elapsed.Round(time.Millisecond))
			fmt.Printf("  Files added:    %d\n", result.Added)
			fmt.Printf("  Files modified: %d\n", result.Modified)
			fmt.Printf("  Files deleted:  %d\n", result.Deleted)
			fmt.Printf("  Files unchanged: %d\n", result.Unchanged)
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.reindexRepoName, "name", "", "Repository name (default: base directory name)")

	return cmd
}

// ============ Check Command ============

func newCheckCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "check",
		Short:   "Structural checks (cycle detection, etc.)",
		Example: "  codetrip check --repo redis",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			cycles, _ := cmd.Flags().GetBool("cycles")

			ctx := context.Background()
			req := &codetrip.CheckRequest{Repo: flags.commonRepo, Cycles: cycles}

			result, err := utils.Check(ctx, trip, req)
			if err != nil {
				return err
			}

			fmt.Println("Structural Check Results:")
			if len(result.Cycles) == 0 {
				fmt.Println("  No cycles detected.")
			} else {
				fmt.Printf("  %d cycles detected:\n", len(result.Cycles))
				for i, cycle := range result.Cycles {
					fmt.Printf("    Cycle %d: %s\n", i+1, strings.Join(cycle, " -> "))
				}
			}
			return nil
		},
	}

	cmd.Flags().Bool("cycles", true, "Detect circular dependencies")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Explain Command ============

func newExplainCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "explain <target>",
		Short: "Taint tracking explanation",
		Long:  "Perform taint analysis on a symbol and trace data flow paths.",
		Example: `  # Trace taint paths for a symbol
  codetrip explain dictFind --repo redis

  # Limit the number of findings
  codetrip explain dictFind --repo redis --limit 20`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.ExplainRequest{
				Target: target,
				Repo:   flags.commonRepo,
				Limit:  flags.explainLimit,
			}

			result, err := utils.Explain(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("repository %q not found (use --repo to specify)", flags.commonRepo)
				}
				return err
			}

			fmt.Printf("Taint Explanation: %s\n", target)
			fmt.Printf("  Total Findings: %d\n", result.TotalFindings)

			if result.Truncated {
				fmt.Println("  (results truncated)")
			}

			for i, f := range result.Findings {
				fmt.Printf("\n  Finding %d:\n", i+1)
				fmt.Printf("    Category:  %s\n", f.Category)
				fmt.Printf("    Source:    line %d -> Sink: line %d\n", f.SourceLine, f.SinkLine)
				if len(f.HopPath) > 0 {
					fmt.Print("    Path: ")
					for j, hop := range f.HopPath {
						if j > 0 {
							fmt.Print(" -> ")
						}
						fmt.Printf("%s(L%d)", hop.NodeID, hop.Line)
					}
					fmt.Println()
				}
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&flags.explainLimit, "limit", 100, "Maximum number of findings")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Route Map Command ============

func newRouteMapCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "route-map",
		Short: "API route mapping",
		Long:  "List API routes with their handlers and consumers.",
		Example: `  # List all API routes
  codetrip route-map --repo my-project`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.RouteMapRequest{Route: flags.routeName, Repo: flags.commonRepo}

			result, err := utils.RouteMap(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("repository %q not found (use --repo to specify)", flags.commonRepo)
				}
				return err
			}

			if len(result.Routes) == 0 {
				fmt.Println("(no routes)")
				return nil
			}

			fmt.Printf("Route Map (%d routes):\n", len(result.Routes))
			for _, r := range result.Routes {
				fmt.Printf("  %s %s", r.Method, r.Path)
				if r.HandlerID != "" {
					fmt.Printf(" -> handler: %s", r.HandlerID)
				}
				if len(r.Middleware) > 0 {
					fmt.Printf(" | middleware: %s", strings.Join(r.Middleware, ", "))
				}
				fmt.Println()
				if len(r.Consumers) > 0 {
					fmt.Printf("    consumers: %s\n", strings.Join(r.Consumers, ", "))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.routeName, "route", "", "Filter by route name")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Tool Map Command ============

func newToolMapCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tool-map",
		Short: "MCP/RPC tool mapping",
		Example: `  # List all MCP/RPC tools
  codetrip tool-map --repo my-project`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.ToolMapRequest{Tool: flags.toolName, Repo: flags.commonRepo}

			result, err := utils.ToolMap(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("repository %q not found (use --repo to specify)", flags.commonRepo)
				}
				return err
			}

			if len(result.Tools) == 0 {
				fmt.Println("(no tools)")
				return nil
			}

			fmt.Printf("Tool Map (%d tools):\n", len(result.Tools))
			for _, t := range result.Tools {
				fmt.Printf("  %s", t.Name)
				if t.Description != "" {
					fmt.Printf(" - %s", t.Description)
				}
				if t.HandlerID != "" {
					fmt.Printf(" -> handler: %s", t.HandlerID)
				}
				fmt.Println()
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.toolName, "tool", "", "Filter by tool name")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Shape Check Command ============

func newShapeCheckCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shape-check",
		Short: "Response shape checking",
		Long:  "Check if API route response shapes match consumer expectations.",
		Example: `  # Check all route response shapes
  codetrip shape-check --repo my-project`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.ShapeCheckRequest{Route: flags.shapeRoute, Repo: flags.commonRepo}

			result, err := utils.ShapeCheck(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("repository %q not found (use --repo to specify)", flags.commonRepo)
				}
				return err
			}

			if len(result.Mismatches) == 0 {
				fmt.Println("Shape Check: no mismatches found.")
			} else {
				fmt.Printf("Shape Mismatches (%d):\n", len(result.Mismatches))
				for _, m := range result.Mismatches {
					fmt.Printf("  Route %s field %s: producer=%s consumer=%s\n", m.Route, m.Field, m.Producer, m.Consumer)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.shapeRoute, "route", "", "Filter by route name")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ API Impact Command ============

func newApiImpactCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "api-impact",
		Short: "API impact analysis (route map + impact + shape check)",
		Example: `  # Analyze all API routes for change impact
  codetrip api-impact --repo my-project`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.ApiImpactRequest{Route: flags.apiRoute, Repo: flags.commonRepo}

			result, err := utils.ApiImpact(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("repository %q not found (use --repo to specify)", flags.commonRepo)
				}
				return err
			}

			fmt.Printf("API Impact Analysis: %s\n", flags.apiRoute)
			fmt.Printf("  Risk Level: %s\n", result.Risk)

			if len(result.Consumers) > 0 {
				fmt.Println("\n  Consumers:")
				for _, c := range result.Consumers {
					fmt.Printf("    - %s (%s)\n", c.Name, c.FilePath)
				}
			}

			if len(result.Middleware) > 0 {
				fmt.Printf("\n  Middleware: %s\n", strings.Join(result.Middleware, ", "))
			}

			if len(result.Processes) > 0 {
				fmt.Println("\n  Affected Processes:")
				for _, p := range result.Processes {
					fmt.Printf("    - %s (%s)\n", p.ID, p.Label)
				}
			}

			if len(result.Mismatches) > 0 {
				fmt.Printf("\n  Shape Mismatches (%d):\n", len(result.Mismatches))
				for _, m := range result.Mismatches {
					fmt.Printf("    Route %s field %s: producer=%s consumer=%s\n", m.Route, m.Field, m.Producer, m.Consumer)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.apiRoute, "route", "", "Analyze specific API route")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Rename Command ============

func newRenameCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rename <symbol-name>",
		Short: "Multi-file coordinated renaming",
		Long:  "Rename a symbol across all files based on graph reference analysis.",
		Example: `  # Dry-run mode (default, no files modified)
  codetrip rename dictFind --repo redis --new-name myDictFind`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			symbolName := args[0]

			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.RenameRequest{
				SymbolName: symbolName,
				Repo:       flags.commonRepo,
				NewName:    flags.renameNewName,
				DryRun:     flags.renameDryRun,
			}

			result, err := utils.Rename(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrSymbolNotFound) {
					return fmt.Errorf("symbol %q not found", symbolName)
				}
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("repository %q not found (use --repo to specify)", flags.commonRepo)
				}
				return err
			}

			fmt.Printf("Rename: %s -> %s\n", symbolName, flags.renameNewName)
			if flags.renameDryRun {
				fmt.Println("(dry-run mode, no files modified)")
			}
			fmt.Printf("  Found %d edits:\n", len(result.Edits))

			highCount := 0
			for _, edit := range result.Edits {
				conf := "low"
				if edit.Confidence == "high" {
					conf = "high"
					highCount++
				}
				fp := edit.FilePath
				if fp == "" {
					fp = "<no file>"
				}
				fmt.Printf("    [%s confidence] %s: %s -> %s\n", conf, fp, edit.OldText, edit.NewText)
			}

			fmt.Printf("\n  High confidence: %d, Low confidence: %d\n", highCount, len(result.Edits)-highCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.renameNewName, "new-name", "", "New name for the symbol (required)")
	cmd.Flags().BoolVar(&flags.renameDryRun, "dry-run", true, "Dry-run mode (do not modify files)")
	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")
	mustMarkFlagRequired(cmd, "new-name")

	return cmd
}

// ============ Info/Version/ListRepos/RepoStatus/Drop Commands ============

func newInfoCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "info",
		Short:   "Show index statistics",
		Example: "  codetrip info --repo my-project",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			stats, err := utils.Stats(trip, flags.commonRepo)
			if err != nil {
				return err
			}

			fmt.Printf("Index Statistics: %s\n\n", flags.commonRepo)
			fmt.Printf("  Name index:  %d entries\n", stats.NameCount)
			fmt.Printf("  Label index: %d entries\n", stats.LabelCount)
			fmt.Printf("  File index:  %d entries\n", stats.FileCount)
			fmt.Printf("  UID index:   %d entries\n", stats.UIDCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Show version information",
		Example: "  codetrip version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("codetrip v%s — Hybrid Graph-Augmented Code Intelligence Engine\n", codetrip.Version)
			return nil
		},
	}
}

func newListReposCmd(flags *cliFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list-repos",
		Short:   "List indexed repositories",
		Example: "  codetrip list-repos",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()

			repos, err := utils.ListRepos(trip)
			if err != nil {
				return err
			}

			if len(repos) == 0 {
				fmt.Println("(no indexed repositories)")
				return nil
			}

			fmt.Printf("Indexed repositories (%d):\n", len(repos))
			for _, r := range repos {
				fmt.Printf("  %s", r.Name)
				if r.Path != "" {
					fmt.Printf(" (%s)", r.Path)
				}
				fmt.Println()
			}
			return nil
		},
	}
}

func newRepoStatusCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repo-status",
		Short:   "Show repository status",
		Example: "  codetrip repo-status --repo my-project",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			status, err := utils.RepoStatus(trip, flags.commonRepo)
			if err != nil {
				return err
			}

			fmt.Printf("Repository Status: %s\n", status.Name)
			fmt.Printf("  Nodes: %d\n", status.NodeCount)
			fmt.Printf("  Edges: %d\n", status.EdgeCount)
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

func newDropCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "drop",
		Short:   "Drop a repository index",
		Long:    "Delete all index data for the specified repository.",
		Example: "  codetrip drop --repo old-project",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTripForRepo(flags.commonRepo)
			if err != nil {
				return err
			}
			defer trip.Close()

			if err := utils.DropIndex(trip, flags.commonRepo); err != nil {
				return err
			}

			fmt.Printf("Dropped index for repository: %s\n", flags.commonRepo)
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.commonRepo, "repo", "", "Repository name (required)")
	mustMarkFlagRequired(cmd, "repo")

	return cmd
}

// ============ Group Commands ============

func newGroupCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Cross-repo group management",
		Example: `  # List cross-repo groups
  codetrip group list`,
	}

	cmd.AddCommand(
		newGroupListCmd(flags),
		newGroupSyncCmd(flags),
		newGroupImpactCmd(flags),
	)

	return cmd
}

func newGroupListCmd(flags *cliFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List cross-repo groups",
		Example: "  codetrip group list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()

			groups, err := utils.GroupList(trip)
			if err != nil {
				return err
			}

			if len(groups) == 0 {
				fmt.Println("(no groups)")
				return nil
			}

			fmt.Printf("Groups (%d):\n", len(groups))
			for _, g := range groups {
				fmt.Printf("  %s - %s\n", g.Name, g.Description)
				fmt.Printf("    repos: %s\n", strings.Join(g.Repos, ", "))
			}
			return nil
		},
	}
}

func newGroupSyncCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sync <group-name>",
		Short: "Sync a cross-repo group",
		Example: `  # Sync with explicit repo names
  codetrip group sync microservices --repo /path/to/user-service=user-service`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupName := args[0]

			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()

			repoPaths := make(map[string]string)
			for _, r := range flags.groupSyncRepos {
				parts := strings.SplitN(r, "=", 2)
				if len(parts) == 2 {
					repoPaths[parts[0]] = parts[1]
				} else {
					repoPaths[r] = filepath.Base(r)
				}
			}

			ctx := context.Background()
			req := &codetrip.GroupSyncRequest{
				GroupName: groupName,
				RepoPaths: repoPaths,
			}

			result, err := utils.GroupSync(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("one or more repositories not found in the specified paths")
				}
				return err
			}

			fmt.Printf("Group Sync: %s\n", result.Group)
			fmt.Printf("  Contracts:   %d\n", result.Contracts)
			fmt.Printf("  Bridge Links: %d\n", result.BridgeLinks)
			fmt.Printf("  Duration:    %.2fs\n", result.Duration)
			return nil
		},
	}

	cmd.Flags().StringArrayVar(&flags.groupSyncRepos, "repo", nil, "Repository path (can be repeated, format: path=name)")

	return cmd
}

func newGroupImpactCmd(flags *cliFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "impact <group-name> <target>",
		Short: "Cross-repo impact analysis",
		Example: `  # Downstream cross-repo impact analysis
  codetrip group impact microservices handleRequest`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupName := args[0]
			target := args[1]

			trip, err := flags.openTrip()
			if err != nil {
				return err
			}
			defer trip.Close()

			ctx := context.Background()
			req := &codetrip.GroupImpactRequest{
				GroupName: groupName,
				Target:    target,
				Direction: flags.groupImpactDirection,
			}

			result, err := utils.GroupImpact(ctx, trip, req)
			if err != nil {
				if errors.Is(err, codetrip.ErrRepoNotFound) {
					return fmt.Errorf("group %q or its repositories not found", groupName)
				}
				return err
			}

			fmt.Printf("Cross-Repo Impact: %s / %s\n", groupName, target)
			fmt.Printf("  Risk Level: %s\n", result.Risk)

			if result.LocalImpact != nil {
				fmt.Printf("  Local Risk: %s\n", result.LocalImpact.Risk)
			}

			if len(result.CrossRepoRefs) > 0 {
				fmt.Printf("\n  Cross-Repo References (%d):\n", len(result.CrossRepoRefs))
				for _, ref := range result.CrossRepoRefs {
					fmt.Printf("    [%s] %s/%s -> %s/%s (confidence: %.2f)\n",
						ref.MatchType,
						ref.SourceRepo, ref.SourceSymbol,
						ref.TargetRepo, ref.TargetSymbol,
						ref.Confidence)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&flags.groupImpactDirection, "direction", "downstream", "Traversal direction: downstream or upstream")

	return cmd
}
