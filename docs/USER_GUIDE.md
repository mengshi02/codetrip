# codetrip User Guide

## Overview

codetrip is an embedded Hybrid Graph-Augmented Code Intelligence Engine. It parses code repositories into knowledge graphs and provides rich tools including Cypher queries, impact analysis, hybrid search, renaming, and taint tracking to help developers deeply understand code structure and dependencies.

**Key Features:**
- Embedded deployment, zero external dependencies (Pebble KV storage)
- 14-stage DAG pipeline auto-indexing (Scan → Structure → Markdown → COBOL → Parse → Routes → Tools → ORM → CrossFile → ScopeResolution → PruneLocal → MRO → Communities → Index)
- Multi-language support (Go / TypeScript / JavaScript / Python / Java / C++ / C / C# / Rust / Markdown, etc.)
- Cypher query language with default 30s timeout protection
- BM25 + dual-modal semantic vector (Description + Code) hybrid search with RRF fusion
- int8 scalar quantization + two-stage search (int8 coarse + float32 refine)
- Incremental indexing: SHA1 hash-driven change detection
- Cross-repo grouping: contract detection + bridge links
- Data consistency verification and Pebble Checkpoint backup
- MCP integration: 18 tools exposed via `codetrip mcp` for AI coding assistants
- Extensible Tool / Phase / Embedder / ContractExtractor plugin system

---

## Quick Start

### Installation

```bash
go get github.com/mengshi02/codetrip
```

### 5-Minute Guide

```go
package main

import (
    "context"
    "fmt"
    "log"

    "github.com/mengshi02/codetrip"
)

func main() {
    // 1. Open engine
    trip, err := codetrip.Open("./my-project.codetrip")
    if err != nil {
        log.Fatal(err)
    }
    defer trip.Close()

    ctx := context.Background()

    // 2. Index repository
    result, err := trip.IndexRepo(ctx, "/path/to/my-project",
        codetrip.WithRepoName("my-project"),
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Indexing complete: %d files, %d nodes, %d edges\n", result.Files, result.Nodes, result.Edges)

    // 3. Execute Cypher query
    cypherResult, err := trip.Cypher(ctx,
        "MATCH (n:Function) RETURN n.name LIMIT 5",
        codetrip.Param{Key: "repo", Value: "my-project"},
    )
    if err != nil {
        log.Fatal(err)
    }
    for _, row := range cypherResult.Rows {
        fmt.Println(row)
    }

    // 4. Impact analysis
    impact, err := trip.Impact(ctx, &codetrip.ImpactRequest{
        Target:    "handleRequest",
        Direction: "downstream",
        MaxDepth:  3,
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Risk level: %s, affected symbols: %d\n", impact.Risk, len(impact.ByDepth))

    // 5. Generate vector embeddings (for semantic search)
    embedResult, err := trip.EmbedRepo(ctx, "my-project",
        codetrip.WithEmbedEndpoint("http://localhost:11434/v1/embeddings"),
    )
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Embedding complete: %d nodes, %d desc chunks, %d code chunks\n",
        embedResult.NodesEmbedded, embedResult.DescChunks, embedResult.CodeChunks)
}
```

---

## Engine Lifecycle

### Open — Open Engine

```go
func Open(dir string, opts ...Option) (*Trip, error)
```

Opens or creates a codetrip engine instance. `dir` is the data storage directory, created automatically on first use.

**Configuration Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `WithCacheSize(size int64)` | Storage engine cache size | 256 MB |
| `WithPhase(phase pipeline.Phase)` | Register custom Pipeline Phase | none |
| `WithNodeCacheSize(size int)` | Node cache entry count | 10000 |
| `WithTraversalLimit(limit int)` | Graph traversal node limit | 100000 |
| `WithScalePreset(preset string)` | Scale preset: `small`/`medium`/`large` | none |
| `WithQuantization(enable bool)` | Enable int8 vector quantization | false |
| `WithTwoStageSearch(enable bool)` | Enable two-stage search (int8 coarse + float32 refine) | false |
| `WithBM25ChunkSize(size int)` | BM25 index chunk size | 65536 |
| `WithCypherTimeout(d time.Duration)` | Cypher query timeout | 30s |
| `WithAutoMigrate(enable bool)` | Auto Schema version migration | false |

**Example:**

```go
trip, err := codetrip.Open("./data",
    codetrip.WithCacheSize(512 << 20),     // 512MB cache
    codetrip.WithScalePreset("large"),      // large-scale preset
    codetrip.WithQuantization(true),        // int8 quantization
    codetrip.WithTwoStageSearch(true),      // two-stage search
    codetrip.WithCypherTimeout(60*time.Second), // 60s Cypher timeout
)
```

### Close — Close Engine

```go
func (trip *Trip) Close() error
```

Closes the engine, releasing all BM25 indexes and Pebble storage resources. Must be called before program exit (typically with `defer`).

### Ping — Health Check

```go
func (trip *Trip) Ping() error
```

Checks if the engine is working properly. Returns `nil` on success.

### Backup — Data Backup

```go
func (trip *Trip) Backup(backupDir string) error
```

Creates an engine snapshot backup using Pebble Checkpoint to the specified directory.

**Example:**

```go
err := trip.Backup("/backups/codetrip-20240101")
```

### GraphStore — Get Graph Store

```go
func (trip *Trip) GraphStore(repo string) *graph.GraphStore
```

Returns the underlying graph store instance for the specified repository, for advanced graph operations.

---

## Index Management

### IndexRepo — Index Repository

```go
func (trip *Trip) IndexRepo(ctx context.Context, repoPath string, opts ...IndexOption) (*IndexResult, error)
```

Parses a local repository into a knowledge graph. Internally runs a 14-stage DAG pipeline: Scan → Structure → Markdown → COBOL → Parse → Routes → Tools → ORM → CrossFile → ScopeResolution → PruneLocal → MRO → Communities → Index.

> **Note:** Vector embedding has been separated from the indexing pipeline into the `EmbedRepo` API. Indexing no longer includes an embedding stage.

**Index Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `WithRepoName(name)` | Repository name (default: last directory in path) | path base name |
| `WithMaxWorkers(n)` | Max concurrent workers | 0 (GOMAXPROCS) |
| `WithByteBudget(budget)` | Byte budget per chunk | 20 MB |
| `WithCFG(enable)` | Enable Control Flow Graph (CFG) construction | false |
| `WithPDG(enable)` | Enable Program Dependence Graph (PDG) construction | false |
| `WithIndexTimeout(d)` | Index timeout | 30 min |

**`IndexResult` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Repo` | string | Repository name |
| `Files` | int | Scanned file count |
| `Nodes` | int | Created node count |
| `Edges` | int | Created edge count |
| `Duration` | float64 | Duration in seconds |

**Example:**

```go
result, err := trip.IndexRepo(ctx, "/path/to/repo",
    codetrip.WithRepoName("my-service"),
    codetrip.WithCFG(true),
    codetrip.WithMaxWorkers(4),
)
```

### ReIndex — Incremental Re-index

```go
func (trip *Trip) ReIndex(ctx context.Context, repoPath string, opts ...IndexOption) (*ReIndexResult, error)
```

Detects repository changes (added/modified/deleted files) and incrementally updates the code graph, BM25, and vector indexes. The repository must have been indexed with `IndexRepo` first.

**`ReIndexResult` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Added` | int | Added file count |
| `Modified` | int | Modified file count |
| `Deleted` | int | Deleted file count |
| `Unchanged` | int | Unchanged file count |

**Example:**

```go
result, err := trip.ReIndex(ctx, "/path/to/my-project",
    codetrip.WithRepoName("my-project"),
)
fmt.Printf("Added: %d, Modified: %d, Deleted: %d\n", result.Added, result.Modified, result.Deleted)
```

### EmbedRepo — Vector Embedding

```go
func (trip *Trip) EmbedRepo(ctx context.Context, repo string, opts ...EmbedOption) (*EmbedResult, error)
```

Generates dual-modal vector embeddings (Description + Code) for an already indexed repository, enabling semantic hybrid search. Requires prior `IndexRepo`.

**Dual-Modal Embedding:**
- **Description modality**: Symbol signature + relationship summary
- **Code modality**: Source code snippet chunking

**Embed Options:**

| Option | Description | Default |
|--------|-------------|---------|
| `WithEmbedEndpoint(url)` | Embedding service HTTP endpoint (required) | none |
| `WithEmbedModel(name)` | Model name | auto-detect |
| `WithEmbedAPIKey(key)` | API key | none |
| `WithEmbedDimensions(d)` | Vector dimensions | auto-detect |
| `WithEmbedBatchSize(n)` | Batch size for embedding requests | 16 |
| `WithEmbedIncremental(enable)` | Skip nodes with unchanged content hash | false |
| `WithEmbedQuantInt8(enable)` | Enable int8 vector quantization | false |
| `WithEmbedTwoStageSearch(enable)` | Enable two-stage search | false |
| `WithEmbedTimeout(d)` | Embedding timeout | 30 min |

**`EmbedResult` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `NodesEmbedded` | int | Number of embedded nodes |
| `DescChunks` | int | Description chunk count |
| `CodeChunks` | int | Code chunk count |
| `Skipped` | int | Skipped (unchanged) count |
| `Errors` | int | Error count |
| `Duration` | float64 | Duration in seconds |

**Example:**

```go
result, err := trip.EmbedRepo(ctx, "my-project",
    codetrip.WithEmbedEndpoint("http://localhost:11434/v1/embeddings"),
    codetrip.WithEmbedIncremental(true),
    codetrip.WithEmbedQuantInt8(true),
    codetrip.WithEmbedTwoStageSearch(true),
)
```

### Verify — Data Consistency Verification

```go
func (trip *Trip) Verify(ctx context.Context) ([]VerifyIssue, error)
```

Checks engine consistency by verifying adjacency lists, type/name/file indexes, and embedding hash integrity.

**`VerifyIssue` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Type` | string | Issue type |
| `Repo` | string | Repository |
| `Detail` | string | Issue description |

**Example:**

```go
issues, err := trip.Verify(ctx)
for _, issue := range issues {
    fmt.Printf("[%s] %s: %s\n", issue.Type, issue.Repo, issue.Detail)
}
```

### DropIndex — Delete Index

```go
func (trip *Trip) DropIndex(repoName string) error
```

Deletes all index data for the specified repository, including nodes, edges, adjacency lists, type indexes, name indexes, file indexes, full-text indexes, embedding vectors, and scope data.

### ListRepos — List Repositories

```go
func (trip *Trip) ListRepos() ([]RepoInfo, error)
```

Returns a list of all currently indexed repositories.

### Stats — Index Statistics

```go
func (trip *Trip) Stats(repo string) (*IndexStats, error)
```

Returns index statistics for the specified repository:

| Field | Description |
|-------|-------------|
| `NameCount` | Name index entry count |
| `LabelCount` | Label index entry count |
| `FileCount` | File index entry count |
| `UIDCount` | UID index entry count |

### RepoStatus — Repository Status

```go
func (trip *Trip) RepoStatus(repoName string) (*RepoStatusInfo, error)
```

Returns repository status overview:

| Field | Description |
|-------|-------------|
| `Name` | Repository name |
| `NodeCount` | Node count |
| `EdgeCount` | Edge count |

---

## Graph Queries

### Cypher — Execute Cypher Query

```go
func (trip *Trip) Cypher(ctx context.Context, query string, params ...Param) (*CypherResult, error)
```

Queries the knowledge graph using the Cypher query language. Specify the target repository via `Param{Key: "repo", Value: "repo-name"}`. Built-in Volcano iterator model execution engine with default 30s timeout protection (adjustable via `WithCypherTimeout`).

**`CypherResult` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Columns` | []string | Column names |
| `Rows` | []map[string]any | Result rows |
| `Stats` | map[string]int | Query statistics (e.g., `rows` count) |

**Example:**

```go
// Find all function nodes
result, _ := trip.Cypher(ctx,
    "MATCH (n:Function) RETURN n.name, n.filePath LIMIT 10",
    codetrip.Param{Key: "repo", Value: "my-project"},
)

// Find call relationships
result, _ = trip.Cypher(ctx,
    "MATCH (a)-[:CALLS]->(b) WHERE a.name = $name RETURN b.name",
    codetrip.Param{Key: "repo", Value: "my-project"},
    codetrip.Param{Key: "name", Value: "handleRequest"},
)
```

### Query — Graph Query (Shorthand)

```go
func (trip *Trip) Query(ctx context.Context, stmt string, params ...Param) (*QueryResult, error)
```

Same functionality as `Cypher`, returns `QueryResult` (without Stats field).

---

## Search

### Search — Code Search

```go
func (trip *Trip) Search(ctx context.Context, req *SearchRequest) (*SearchResult, error)
```

Supports two search modes:

| Mode | `Semantic` Value | Description |
|------|--------|-------------|
| BM25 text search | `false` | Keyword-based full-text retrieval |
| Hybrid search | `true` | BM25 + dual-modal semantic vector RRF fusion ranking |

**BM25 Fallback:** When `Semantic=true` but `EmbedRepo` has not been run for the repository, the search automatically falls back to BM25. `SearchResult.Fallback` returns `"bm25"`.

**`SearchRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Query` | string | Search query string |
| `Limit` | int | Max result count (default 20) |
| `Repo` | string | Target repository (empty for default) |
| `Semantic` | bool | Enable semantic search |

**`SearchResult` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Results` | []SearchItem | Search result list |
| `Fallback` | string | Fallback indicator: `"bm25"` means semantic search fell back to BM25 |

**`SearchItem` Fields:**

| Field | Description |
|-------|-------------|
| `NodeID` | Graph node ID |
| `Name` | Symbol name |
| `Kind` | Symbol type |
| `FilePath` | File path |
| `Score` | Relevance score |
| `StartLine` | Start line number |
| `EndLine` | End line number |

**Example:**

```go
// BM25 search
result, _ := trip.Search(ctx, &codetrip.SearchRequest{
    Query: "authenticate",
    Limit: 10,
    Repo:  "my-project",
})

// Semantic hybrid search
result, _ = trip.Search(ctx, &codetrip.SearchRequest{
    Query:    "user login validation",
    Semantic: true,
    Limit:    10,
    Repo:     "my-project",
})

// Check for fallback
if result.Fallback == "bm25" {
    fmt.Println("Semantic search unavailable,     fmt.Println("Semantic search unavailable, fell back to BM25 search")
}
```

---

## Analysis Tools

### Impact — Impact Analysis

```go
func (trip *Trip) Impact(ctx context.Context, req *ImpactRequest) (*ImpactResult, error)
```

Evaluates the downstream/upstream impact scope and risk level of a symbol change. Based on BFS graph traversal, displaying affected symbols grouped by depth.

**`ImpactRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Target` | string | Target symbol name |
| `TargetUID` | string | Target symbol UID (overrides name) |
| `Direction` | string | `"downstream"` or `"upstream"` |
| `FilePath` | string | File path (auxiliary positioning) |
| `Kind` | string | Symbol type filter |
| `MaxDepth` | int | Traversal depth limit (default 3, max 32) |
| `RelationTypes` | []string | Only track specified edge types |
| `MinConfidence` | float64 | Minimum confidence filter |
| `IncludeTests` | bool | Include test code |
| `SummaryOnly` | bool | Return summary only |
| `Limit` | int | Result count limit |
| `Offset` | int | Result offset |
| `TimeoutMs` | int | Timeout in milliseconds |

**`ImpactResult` Fields:**

| Field | Description |
|-------|-------------|
| `Risk` | Risk level: `LOW` / `MEDIUM` / `HIGH` / `CRITICAL` |
| `AffectedProcesses` | Affected process list |
| `AffectedModules` | Affected module list |
| `ByDepth` | Affected symbols grouped by depth |
| `ByDepthCounts` | Affected symbol counts per depth |

**Risk Level Rules:** Affected symbols ≥10 → CRITICAL, ≥6 → HIGH, ≥3 → MEDIUM, otherwise → LOW.

**Example:**

```go
result, _ := trip.Impact(ctx, &codetrip.ImpactRequest{
    Target:        "handleRequest",
    Direction:     "downstream",
    MaxDepth:      3,
    MinConfidence: 0.8,
})
fmt.Printf("Risk: %s\n", result.Risk)
for _, dg := range result.ByDepth {
    fmt.Printf("  Depth %d: %d symbols\n", dg.Depth, len(dg.Symbols))
}
```

### Context — 360-Degree Symbol View

```go
func (trip *Trip) Context(ctx context.Context, req *ContextRequest) (*ContextResult, error)
```

Gets a complete context view for a symbol: including definition info, all incoming (who references me) and outgoing (what I reference) references, and same-name disambiguation candidates.

**`ContextRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Name` | string | Symbol name |
| `UID` | string | Symbol UID (overrides name) |
| `FilePath` | string | File path (for disambiguation) |
| `Kind` | string | Symbol type filter |
| `IncludeContent` | bool | Include source code content |

**`ContextResult` Fields:**

| Field | Description |
|-------|-------------|
| `Symbol` | Symbol basic info (NodeID, Name, Kind, FilePath) |
| `Incoming` | Incoming reference groups (by edge type) |
| `Outgoing` | Outgoing reference groups (by edge type) |
| `Processes` | Process list the symbol belongs to |
| `Disambiguation` | Same-name disambiguation candidates |

**Example:**

```go
result, _ := trip.Context(ctx, &codetrip.ContextRequest{
    Name:     "User",
    FilePath: "pkg/models/user.go",
})
fmt.Printf("Symbol: %s (%s) @ %s\n", result.Symbol.Name, result.Symbol.Kind, result.Symbol.FilePath)
for _, group := range result.Incoming {
    fmt.Printf("  Incoming [%s]: %d refs\n", group.Type, len(group.Refs))
}
```

### DetectChanges — Change Detection

```go
func (trip *Trip) DetectChanges(ctx context.Context, req *DetectChangesRequest) (*DetectChangesResult, error)
```

Incremental index based on SHA1 content hash, detecting file changes and assessing affected processes and risks.

**`DetectChangesRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Scope` | string | Detection scope (directory or file path) |
| `BaseRef` | string | Base reference (reserved) |
| `Repo` | string | Repository name |

**`DetectChangesResult` Fields:**

| Field | Description |
|-------|-------------|
| `ChangedSymbols` | Changed symbol list |
| `AffectedProcesses` | Affected process list |
| `RiskSummary` | Risk summary (Level, TotalChanges, HighRisk) |

**Example:**

```go
result, _ := trip.DetectChanges(ctx, &codetrip.DetectChangesRequest{
    Repo:  "my-project",
    Scope: "src/handlers/",
})
fmt.Printf("Risk: %s, changes: %d\n", result.RiskSummary.Level, result.RiskSummary.TotalChanges)
```

### Rename — Multi-File Coordinated Renaming

```go
func (trip *Trip) Rename(ctx context.Context, req *RenameRequest) (*RenameResult, error)
```

Locates all reference points of a symbol across files, returning renaming edit suggestions. Uses a two-tier strategy:
1. **High confidence**: Precise references based on graph edges (CALLS / ACCESSES / IMPORTS)
2. **Low confidence**: Fuzzy matches based on BM25 text search

**`RenameRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `SymbolName` | string | Original symbol name |
| `SymbolUID` | string | Symbol UID (overrides name) |
| `NewName` | string | New name |
| `FilePath` | string | File path (auxiliary positioning) |
| `DryRun` | bool | Dry-run mode |

**`RenameResult` contains `RenameEdit` list:**

| Field | Description |
|-------|-------------|
| `FilePath` | File path |
| `OldText` | Original text |
| `NewText` | Replacement text |
| `Confidence` | Confidence: `high` or `low` |

**Example:**

```go
result, _ := trip.Rename(ctx, &codetrip.RenameRequest{
    SymbolName: "oldFunc",
    NewName:    "newFunc",
    DryRun:     true,
})
for _, edit := range result.Edits {
    fmt.Printf("[%s] %s: %s → %s\n", edit.Confidence, edit.FilePath, edit.OldText, edit.NewText)
}
```

### RouteMap — API Route Mapping

```go
func (trip *Trip) RouteMap(ctx context.Context, req *RouteMapRequest) (*RouteMapResult, error)
```

Queries API route nodes with their handlers, consumers, and middleware.

**`RouteMapRequest`:** `Route string`, `Repo string`

**`RouteInfo` Fields:**

| Field | Description |
|-------|-------------|
| `Path` | Route path |
| `Method` | HTTP method |
| `HandlerID` | Handler function node ID |
| `Middleware` | Middleware list |
| `Consumers` | Consumer node ID list |

### ToolMap — MCP/RPC Tool Mapping

```go
func (trip *Trip) ToolMap(ctx context.Context, req *ToolMapRequest) (*ToolMapResult, error)
```

Queries MCP/RPC tool definition nodes with their handlers.

**`ToolMapRequest`:** `Tool string`, `Repo string`

**`ToolInfo` Fields:**

| Field | Description |
|-------|-------------|
| `Name` | Tool name |
| `Description` | Tool description |
| `HandlerID` | Handler function node ID |

### ShapeCheck — Response Shape Checking

```go
func (trip *Trip) ShapeCheck(ctx context.Context, req *ShapeCheckRequest) (*ShapeCheckResult, error)
```

Detects mismatches between Route producer response fields and consumer expected fields.

**`ShapeCheckRequest`:** `Route string`, `Repo string`

**`ShapeMismatch` Fields:**

| Field | Description |
|-------|-------------|
| `Route` | Route name |
| `Field` | Mismatched field |
| `Producer` | Producer's field definition |
| `Consumer` | Consumer's expected field definition |

### ApiImpact — API Impact Analysis

```go
func (trip *Trip) ApiImpact(ctx context.Context, req *ApiImpactRequest) (*ApiImpactResult, error)
```

Combined RouteMap + Impact + ShapeCheck comprehensive API change impact analysis.

**`ApiImpactRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Route` | string | Target route name |
| `Repo` | string | Repository name |

**`ApiImpactResult` Fields:**

| Field | Description |
|-------|-------------|
| `Risk` | Risk level |
| `Consumers` | Consumer info list |
| `Mismatches` | Shape mismatch list |
| `Middleware` | Involved middleware |
| `Processes` | Affected process list |

**Risk Level Rules (when Impact doesn't provide risk):** Consumers ≥10 → CRITICAL, ≥5 → HIGH, ≥2 → MEDIUM, otherwise → LOW.

### Check — Structural Check

```go
func (trip *Trip) Check(ctx context.Context, req *CheckRequest) (*CheckResult, error)
```

Performs structural checks, such as circular dependency detection.

**`CheckRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Repo` | string | Repository name |
| `Cycles` | bool | Detect circular dependencies |

**Example:**

```go
result, _ := trip.Check(ctx, &codetrip.CheckRequest{
    Repo:   "my-project",
    Cycles: true,
})
fmt.Printf("Detected %d circular dependencies\n", len(result.Cycles))
```

### Explain — Taint Tracking

```go
func (trip *Trip) Explain(ctx context.Context, req *ExplainRequest) (*ExplainResult, error)
```

Traces taint propagation paths in data flow. Finds TAINTED incoming edges on the target symbol, tracing complete hop paths (including SANITIZES sanitization nodes and TAINT_PATH propagation nodes).

**`ExplainRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `Target` | string | Target symbol name |
| `Repo` | string | Repository name |
| `Limit` | int | Max findings (default 100) |

**`ExplainResult` Fields:**

| Field | Description |
|-------|-------------|
| `Findings` | Taint finding list |
| `TotalFindings` | Total finding count |
| `Truncated` | Whether truncated due to Limit |

**`TaintFinding` Fields:**

| Field | Description |
|-------|-------------|
| `Category` | Taint category |
| `SourceLine` | Source line number |
| `SinkLine` | Sink line number |
| `HopPath` | Hop path (NodeID + Line) |

---

## Cross-Repository Grouping

### GroupList — List Groups

```go
func (trip *Trip) GroupList() ([]GroupInfo, error)
```

Lists all cross-repository group information.

**`GroupInfo` Fields:**

| Field | Description |
|-------|-------------|
| `Name` | Group name |
| `Description` | Group description |
| `Repos` | Included repository name list |

### GroupSync — Sync Group

```go
func (trip *Trip) GroupSync(ctx context.Context, req *GroupSyncRequest) (*GroupSyncResult, error)
```

Syncs cross-repository groups, auto-detecting cross-repo contracts and bridge links.

**`GroupSyncRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `GroupName` | string | Group name |
| `RepoPaths` | map[string]string | Repository path mapping (repoPath → repoName) |

**`GroupSyncResult` Fields:**

| Field | Description |
|-------|-------------|
| `Group` | Group name |
| `Contracts` | Detected contract count |
| `BridgeLinks` | Detected bridge link count |
| `Duration` | Duration in seconds |

**Example:**

```go
result, _ := trip.GroupSync(ctx, &codetrip.GroupSyncRequest{
    GroupName: "microservices",
    RepoPaths: map[string]string{
        "/path/to/user-service":    "user-service",
        "/path/to/order-service":   "order-service",
        "/path/to/payment-service": "payment-service",
    },
})
fmt.Printf("Contracts: %d, Bridges: %d\n", result.Contracts, result.BridgeLinks)
```

### GroupImpact — Cross-Repo Impact Analysis

```go
func (trip *Trip) GroupImpact(ctx context.Context, req *GroupImpactRequest) (*GroupImpactResult, error)
```

Cross-repository impact analysis, tracing cross-repo references (name matching + contract links).

**`GroupImpactRequest` Fields:**

| Field | Type | Description |
|-------|------|-------------|
| `GroupName` | string | Group name |
| `Target` | string | Target symbol name |
| `Direction` | string | Analysis direction |

**`GroupImpactResult` Fields:**

| Field | Description |
|-------|-------------|
| `Risk` | Risk level |
| `LocalImpact` | Local repo impact result |
| `CrossRepoRefs` | Cross-repo reference list |

**`CrossRepoRef` Fields:**

| Field | Description |
|-------|-------------|
| `SourceRepo` | Source repository |
| `SourceSymbol` | Source symbol |
| `TargetRepo` | Target repository |
| `TargetSymbol` | Target symbol |
| `MatchType` | Match type: `name` or `contract` |
| `Confidence` | Confidence score |

---

## Extension Registration

codetrip provides rich plugin extension points via the `Register*` series of methods.

### RegisterLanguageProvider — Register Language Parser

```go
func (trip *Trip) RegisterLanguageProvider(lang graph.Label, provider LanguageProvider)
```

Registers a parser for the specified language, controlling code capture, call extraction, class extraction, field extraction, import resolution, etc.

**`LanguageProvider` Interface:**

| Method | Description |
|--------|-------------|
| `Language()` | Returns language label |
| `TreeSitterLanguage()` | Returns Tree-sitter language pointer |
| `QuerySet()` | Returns query set (scope/declaration/import/type binding/reference) |
| `InterpretScope(captures)` | Interprets scope capture results |
| `InterpretDeclaration(captures)` | Interprets declaration capture results |
| `InterpretImport(captures)` | Interprets import capture results |
| `InterpretTypeBinding(captures)` | Interprets type binding capture results |
| `InterpretReference(captures)` | Interprets reference capture results |
| `Captures()` | **Legacy** Returns capture configuration |
| `CallExtractConfig()` | **Legacy** Returns call extraction configuration |
| `ClassExtractConfig()` | **Legacy** Returns class extraction configuration |
| `FieldExtractConfig()` | **Legacy** Returns field extraction configuration |
| `ImportResolveConfig()` | **Legacy** Returns import resolution configuration |
| `Interpret(captures)` | **Legacy** Interprets capture results |
| `ImportSemantics()` | Import semantics strategy |

> **Note:** `QuerySet()` + `InterpretXxx()` series are the new interface, recommended for use. `Captures()` / `CallExtractConfig()` etc. are Legacy compatibility interfaces.

**Built-in language providers:** Go, TypeScript, JavaScript, Python, Java, C++, C, C#, Rust, Markdown

### RegisterScopeResolver — Register Scope Resolver

```go
func (trip *Trip) RegisterScopeResolver(lang graph.Label, resolver ScopeResolver)
```

Registers a scope resolver for the specified language. `ScopeResolver` has been split into composite interfaces:

| Sub-interface | Description |
|---------------|-------------|
| `CoreResolver` | Core scope resolution (MRO construction, scope tree maintenance) |
| `BindingResolver` | Binding resolution (import target resolution, binding merging) |
| `HookResolver` | Hook resolution (call compatibility, method override) |
| `EmitResolver` | Emit resolution (symbol emission, scope boundary handling) |

### RegisterPhase — Register Custom Pipeline Phase

```go
func (trip *Trip) RegisterPhase(phase pipeline.Phase)
```

Appends a custom processing phase after the built-in stages of the indexing pipeline.

### RegisterTool — Register Custom Analysis Tool

```go
func (trip *Trip) RegisterTool(name string, tool Tool)
```

Registers a custom analysis tool, supporting two interfaces:

**Basic `Tool` Interface:**

```go
type Tool interface {
    Name() string
    Run(ctx context.Context, trip *Trip, req interface{}) (interface{}, error)
}
```

**Generic `GenericTool[T, R]` Interface (Recommended):**

```go
type GenericTool[T any, R any] interface {
    Name() string
    Run(ctx context.Context, trip *Trip, req T) (R, error)
}
```

Also provides `ToolAdapter` to adapt `GenericTool` to the basic `Tool` interface.

**Example:**

```go
type myTool struct{}

func (t *myTool) Name() string { return "my_tool" }
func (t *myTool) Run(ctx context.Context, trip *codetrip.Trip, req interface{}) (interface{}, error) {
    // Custom analysis logic
    return map[string]any{"status": "ok"}, nil
}

trip.RegisterTool("my_tool", &myTool{})
```

### RegisterEmbedder — Register Vector Embedding Model

```go
func (trip *Trip) RegisterEmbedder(embedder Embedder)
```

Registers a vector embedding model for semantic search. Defaults to `NoopEmbedder` (no embedding generation).

**`Embedder` Interface:**

| Method | Description |
|--------|-------------|
| `Dimensions()` | Embedding dimensions |
| `Embed(texts)` | Batch text embedding |
| `EmbedBatch(nodes, config)` | Batch node embedding |

### RegisterContractExtractor — Register Contract Extractor

```go
func (trip *Trip) RegisterContractExtractor(contractType ContractType, extractor ContractExtractor)
```

Registers a contract extractor for the specified type, used for detecting API contracts during cross-repository grouping.

**Supported Contract Types:**

| Type | Constant |
|------|----------|
| HTTP | `ContractHTTP` |
| gRPC | `ContractGRPC` |
| Thrift | `ContractThrift` |
| Topic | `ContractTopic` |
| Lib | `ContractLib` |
| Custom | `ContractCustom` |
| Include | `ContractInclude` |

---

## Command-Line Tool

codetrip provides a command-line tool `codetrip` supporting the following commands. All commands share the global flag `--trip-dir` for specifying the engine data directory (default: `~/.codetrip`).

---

### index — Index Repository

Scans and indexes a code repository, building the code graph and search index.

**Syntax:** `codetrip index <repo-path>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | Last directory in path | Repository name |
| `--workers` | int | 0 (GOMAXPROCS) | Max concurrent workers |
| `--byte-budget` | int64 | 20 MB | Byte budget per chunk |
| `--cfg` | bool | false | Enable CFG construction |
| `--pdg` | bool | false | Enable PDG construction |

**Example:**

```bash
# Basic indexing
codetrip index /path/to/my-project

# Specify repo name, enable CFG
codetrip index /path/to/my-project --name my-service --cfg

# Limit workers
codetrip index /path/to/my-project --workers 4 --byte-budget 41943040

# Custom trip directory
codetrip index /path/to/my-project --trip-dir /custom/trip
```

---

### reindex — Incremental Re-index

Detects repository changes and incrementally updates the code graph and search indexes. The repository must have been indexed with `codetrip index` first.

**Syntax:** `codetrip reindex <repo-path>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--name` | string | Last directory in path | Repository name |

**Example:**

```bash
# Incremental re-index
codetrip reindex /path/to/my-project

# Specify repo name
codetrip reindex /path/to/my-project --name my-service
```

---

### embed — Generate Vector Embeddings

Generates dual-modal vector embeddings for an already indexed repository, enabling semantic hybrid search. Requires prior `codetrip index`.

**Syntax:** `codetrip embed`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--endpoint` | string | **Required** | Embedding service endpoint (e.g., `http://localhost:11434/v1/embeddings`) |
| `--model` | string | Auto-detect | Model name |
| `--api-key` | string | None | API key |
| `--dimensions` | int | Auto-detect | Vector dimensions |
| `--batch-size` | int | 16 | Batch embedding size |
| `--incremental` | bool | false | Skip unchanged nodes |
| `--quant-int8` | bool | false | Enable int8 quantization |
| `--two-stage` | bool | false | Enable two-stage search |
| `--timeout` | duration | 30m | Embedding timeout |

**Example:**

```bash
# Generate embeddings
codetrip embed --repo my-project --endpoint http://localhost:11434/v1/embeddings

# Incremental embedding + int8 quantization + two-stage search
codetrip embed --repo my-project --endpoint http://localhost:11434/v1/embeddings \
  --incremental --quant-int8 --two-stage
```

---

### query — Execute Cypher Query

Execute a Cypher query against the knowledge graph.

**Syntax:** `codetrip query <cypher-query>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |

**Example:**

```bash
# Find all function nodes
codetrip query "MATCH (n:Function) RETURN n.name LIMIT 10" --repo my-project

# Find call relationships
codetrip query "MATCH (a)-[:CALLS]->(b) WHERE a.name = 'handleRequest' RETURN b.name" --repo my-project
```

---

### search — Code Search

Search for code symbols using BM25 or hybrid (semantic) search.

**Syntax:** `codetrip search <query>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--limit` | int | 20 | Max result count |
| `--semantic` | bool | false | Enable semantic hybrid search |

**Example:**

```bash
# BM25 text search
codetrip search "authenticate" --repo my-project

# Semantic hybrid search (requires prior embed)
codetrip search "user login validation" --repo my-project --semantic --limit 5
```

---

### impact — Impact Analysis

Evaluate the downstream/upstream impact scope and risk level of modifying a symbol.

**Syntax:** `codetrip impact <target>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--direction` | string | downstream | Traversal direction: `downstream` or `upstream` |
| `--max-depth` | int | 3 | Max traversal depth |

**Example:**

```bash
# Downstream impact analysis
codetrip impact handleRequest --repo my-project

# Upstream impact analysis with deeper traversal
codetrip impact User --repo my-project --direction upstream --max-depth 5
```

---

### context — 360-Degree Symbol View

Get a complete context view for a symbol: definition info, incoming/outgoing references, disambiguation candidates.

**Syntax:** `codetrip context <symbol-name>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--file` | string | "" | File path (for disambiguation) |

**Example:**

```bash
codetrip context handleRequest --repo my-project
codetrip context User --repo my-project --file pkg/models/user.go
```

---

### check — Structural Check

Perform structural checks, such as circular dependency detection.

**Syntax:** `codetrip check`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--cycles` | bool | true | Detect circular dependencies |

**Example:**

```bash
codetrip check --repo my-project
```

---

### explain — Taint Tracking

Trace taint propagation paths in data flow, marking complete paths from source to sink.

**Syntax:** `codetrip explain <target>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--limit` | int | 100 | Max finding count |

**Example:**

```bash
codetrip explain handleRequest --repo my-project
codetrip explain handleRequest --repo my-project --limit 20
```

---

### rename — Multi-File Coordinated Renaming

Locate all reference points of a symbol across files, returning renaming edit suggestions.

**Syntax:** `codetrip rename <symbol-name>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--new-name` | string | **Required** | New symbol name |
| `--dry-run` | bool | true | Dry-run mode (no file modifications) |

**Example:**

```bash
# Dry-run mode to preview renaming scope
codetrip rename oldFunc --repo my-project --new-name newFunc

# Execute renaming
codetrip rename oldFunc --repo my-project --new-name newFunc --dry-run=false
```

---

### detect-changes — Change Detection

Detect file changes and affected symbols using incremental indexing.

**Syntax:** `codetrip detect-changes <scope>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |

**Example:**

```bash
codetrip detect-changes /path/to/my-project --repo my-project
```

---

### route-map — API Route Mapping

List API routes with their handlers and consumers.

**Syntax:** `codetrip route-map`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--route` | string | "" | Filter by route name |

**Example:**

```bash
codetrip route-map --repo my-project
codetrip route-map --repo my-project --route /api/users
```

---

### tool-map — MCP/RPC Tool Mapping

List MCP/RPC tool definitions with their handlers.

**Syntax:** `codetrip tool-map`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--tool` | string | "" | Filter by tool name |

**Example:**

```bash
codetrip tool-map --repo my-project
```

---

### shape-check — Response Shape Checking

Detect mismatches between Route producer response fields and consumer expected fields.

**Syntax:** `codetrip shape-check`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--route` | string | "" | Filter by route name |

**Example:**

```bash
codetrip shape-check --repo my-project
```

---

### api-impact — API Comprehensive Impact Analysis

Combined RouteMap + Impact + ShapeCheck comprehensive API change impact analysis.

**Syntax:** `codetrip api-impact`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |
| `--route` | string | "" | Analyze specific API route |

**Example:**

```bash
codetrip api-impact --repo my-project
codetrip api-impact --repo my-project --route /api/users
```

---

### group — Cross-Repo Group Management

Cross-repository group listing, syncing, and impact analysis. Contains three subcommands.

#### group list — List Cross-Repo Groups

**Syntax:** `codetrip group list`

**Example:**

```bash
codetrip group list
```

#### group sync — Sync Cross-Repo Group

**Syntax:** `codetrip group sync <group-name>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string[] | None | Repository paths (repeatable, format: `path=name`) |

**Example:**

```bash
codetrip group sync microservices \
  --repo /path/to/user-service=user-service \
  --repo /path/to/order-service=order-service
```

#### group impact — Cross-Repo Impact Analysis

**Syntax:** `codetrip group impact <group-name> <target>`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--direction` | string | downstream | Analysis direction |

**Example:**

```bash
codetrip group impact microservices handleRequest
codetrip group impact microservices User --direction upstream
```

---

### info — View Index Statistics

Display index statistics (name, label, file, UID index entry counts).

**Syntax:** `codetrip info`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |

**Example:**

```bash
codetrip info --repo my-project
```

---

### version — View Version

**Syntax:** `codetrip version`

**Example:**

```bash
codetrip version
```

---

### list-repos — List Indexed Repositories

**Syntax:** `codetrip list-repos`

**Example:**

```bash
codetrip list-repos
```

---

### repo-status — Repository Status Overview

**Syntax:** `codetrip repo-status`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |

**Example:**

```bash
codetrip repo-status --repo my-project
```

---

### drop — Delete Repository Index

Delete all index data for the specified repository.

**Syntax:** `codetrip drop`

**Flags:**

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--repo` | string | **Required** | Repository name |

**Example:**

```bash
codetrip drop --repo old-project
```

---

### mcp — Start MCP Server

Start a Model Context Protocol (MCP) server that exposes codetrip's code analysis capabilities as tools for AI coding agents (such as Claude Desktop, Cursor, etc.) via stdio.

**Syntax:** `codetrip mcp`

**Example:**

```bash
# Start via stdio (for Claude Desktop, Cursor, etc.)
codetrip mcp

# Specify trip directory
codetrip mcp --trip-dir /custom/trip
```

**MCP Tool List (18 tools):**

| Tool Name | Description |
|-----------|-------------|
| `index_repo` | Index code repository |
| `reindex` | Incremental re-index |
| `embed_repo` | Generate dual-modal vector embeddings |
| `cypher_query` | Execute Cypher query |
| `search_symbols` | Search code symbols |
| `impact_analysis` | Impact analysis |
| `symbol_context` | 360-degree symbol view |
| `detect_changes` | Change detection |
| `structural_check` | Structural check |
| `explain_taint` | Taint tracking |
| `route_map` | API route mapping |
| `tool_map` | MCP/RPC tool mapping |
| `shape_check` | Response shape checking |
| `api_impact` | API comprehensive impact analysis |
| `rename_symbol` | Multi-file coordinated renaming |
| `index_stats` | Index statistics |
| `list_repos` | List indexed repositories |
| `repo_status` | Repository status overview |
| `drop_index` | Delete repository index |
| `group_list` | Cross-repo group list |
| `group_sync` | Cross-repo group sync |
| `group_impact` | Cross-repo impact analysis |

---

## Built-in Tool Index

| Tool Name | API Method | Description |
|-----------|------------|-------------|
| `check` | `Check()` | Structural checks (circular dependencies, etc.) |
| `list_repos` | `ListRepos()` | List indexed repositories |
| `repo_status` | `RepoStatus()` | Repository status overview |
| `impact` | `Impact()` | Impact analysis |
| `context` | `Context()` | 360-degree symbol view |
| `detect_changes` | `DetectChanges()` | Change detection |
| `rename` | `Rename()` | Multi-file coordinated renaming |
| `route_map` | `RouteMap()` | API route mapping |
| `tool_map` | `ToolMap()` | MCP/RPC tool mapping |
| `shape_check` | `ShapeCheck()` | Response shape checking |
| `api_impact` | `ApiImpact()` | API comprehensive impact analysis |
| `explain` | `Explain()` | Taint tracking |
| `search` | `Search()` | Code search |
| `group_list` | `GroupList()` | Cross-repo group list |
| `group_sync` | `GroupSync()` | Cross-repo group sync |
| `group_impact` | `GroupImpact()` | Cross-repo impact analysis |
| `embed_repo` | `EmbedRepo()` | Dual-modal vector embedding |

---

## Typical Workflows

### Workflow 1: Single Repository Code Understanding

```
Open → IndexRepo → Cypher/Query explore structure → Context view symbol details → Search find related code
```

### Workflow 2: Change Impact Assessment

```
Open → IndexRepo → DetectChanges identify changes → Impact analyze effects → Check detect circular dependencies
```

### Workflow 3: Security Audit

```
Open → IndexRepo(WithCFG+PDG) → Explain trace taint paths → ShapeCheck check interface consistency
```

### Workflow 4: Microservice Architecture Analysis

```
Open → IndexRepo(multiple repos) → GroupSync build cross-repo group → GroupImpact cross-repo impact analysis → ApiImpact API change assessment
```

### Workflow 5: Large-Scale Refactoring

```
Open → IndexRepo → Rename(dry-run) assess scope → Rename(execute) perform renaming
```

### Workflow 6: Enabling Semantic Search

```
Open → IndexRepo → EmbedRepo(generate embeddings) → Search(Semantic=true) semantic hybrid search
```

### Workflow 7: Incremental Update & Data Verification

```
Open → ReIndex(incremental re-index) → EmbedRepo(WithEmbedIncremental) incremental embedding → Verify data consistency verification
```")
}
```