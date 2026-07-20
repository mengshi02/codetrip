# Codetrip User Guide

Codetrip is a hybrid-graph code intelligence engine with three supported integration surfaces: Go library, CLI, and MCP. All three use the same persisted repository snapshots and public `Trip` operations.

## Data and update model

The default data directory is `~/.codetrip`; override it with the global `--trip-dir` flag.

```text
db/       authoritative graph, adjacency, metadata, and active pointers
index/    versioned symbol indexes
content/  versioned source indexes
vectors/  optional semantic data
```

Each logical repository points to one immutable physical snapshot. `index --replace` builds a complete new graph and all derived indexes, publishes the active pointer only after success, and retires the previous snapshot. Codetrip does not modify an active snapshot incrementally.

## CLI

All business commands are single words:

| Command | Purpose |
|---|---|
| `index` | Parse and persist a repository |
| `search` | Search symbols and metadata |
| `source` | Search file names and source contents |
| `embed` | Build semantic data for a repository |
| `hybrid` | Fuse symbol and semantic retrieval |
| `traverse` | Run bounded BFS from a node |
| `path` | Find a shortest directed path |
| `export` | Export the persisted graph as CSV |
| `list` | List active logical repositories |
| `mcp` | Start the stdio MCP server |
| `version` | Print the build version |

### Index and replace

```bash
codetrip index /src/project --repo project
codetrip index /src/project --repo project --replace
```

Index flags:

| Flag | Meaning |
|---|---|
| `--repo` | Logical repository name; defaults to the source directory name |
| `--replace` | Atomically publish a complete replacement snapshot |
| `--export` | Write parser-validation CSV files to a directory |
| `--export-strict` | Fail indexing if validation CSV generation fails |

### Search

```bash
codetrip search "ParseConfig" --repo project --limit 20
codetrip source 'lang:go file:config ParseConfig' --repo project --context 2
```

`search` targets indexed symbols and metadata. `source` targets file names and source contents and supports literal, regular-expression, file, and language filters.

### Semantic and hybrid retrieval

```bash
codetrip embed --repo project \
  --endpoint http://localhost:11434/v1/embeddings \
  --model nomic-embed-text \
  --dimensions 768

codetrip hybrid "configuration loading" --repo project \
  --endpoint http://localhost:11434/v1/embeddings \
  --model nomic-embed-text \
  --dimensions 768
```

Use `--quantize-int8` on `embed` when a compact semantic representation is required. The API key may be supplied with `--api-key` or `CODETRIP_EMBEDDING_API_KEY`.

### Graph traversal

```bash
codetrip traverse NODE_ID --repo project --direction both --depth 3
codetrip traverse NODE_ID --repo project --relations CALLS,IMPORTS
codetrip path SOURCE_ID TARGET_ID --repo project
```

Traversal directions are `out`, `in`, and `both`. The engine enforces a configurable visit limit.

### CSV

Validation CSV captures the ingest result before persistence and is intended for language tuning:

```bash
codetrip index /src/project --repo project \
  --export ./validation-output/project --export-strict
```

Full export reflects the active persisted graph:

```bash
codetrip export --repo project --output ./exports/project
```

It writes `nodes.csv`, `edges.csv`, and `manifest.json` with row counts and SHA-256 checksums.

## Go library

### Open and configure

```go
engine, err := codetrip.Open("./.codetrip",
    codetrip.WithCacheSize(512<<20),
    codetrip.WithMaxConcurrentIndex(2),
    codetrip.WithNodeCacheSize(500_000),
    codetrip.WithTraversalLimit(100_000),
    codetrip.WithScalePreset(codetrip.ScaleMedium),
)
if err != nil { /* handle */ }
defer engine.Close()
```

### Index and inspect repositories

```go
result, err := engine.IndexRepo(ctx, "/src/project",
    codetrip.WithRepoName("project"),
    codetrip.WithReplaceExisting(true),
    codetrip.WithIndexTimeout(30*time.Minute),
    codetrip.WithCSVExport("./validation-output/project"),
    codetrip.WithCSVExportStrict(true),
)

repositories, err := engine.ListRepos()
metrics := engine.GetMetrics()
```

### Search and graph queries

```go
symbols, err := engine.Search(ctx, &codetrip.SearchRequest{
    Repo: "project", Query: "ParseConfig", Limit: 20,
})

source, err := engine.SearchSource(ctx, &codetrip.SourceSearchRequest{
    Repo: "project", Query: "lang:go ParseConfig", Limit: 20, ContextLines: 2,
})

nodes, err := engine.Traverse(ctx, &codetrip.TraverseRequest{
    Repo: "project", StartNodeID: nodeID,
    Direction: codetrip.TraverseAny, MaxDepth: 3,
    RelationTypes: []string{"CALLS"},
})

path, err := engine.ShortestPath(ctx, &codetrip.PathRequest{
    Repo: "project", SourceNodeID: sourceID, TargetNodeID: targetID,
})
```

### Semantic API

Implement `codetrip.Embedder` or use the HTTP implementation:

```go
embedder := codetrip.NewHTTPEmbedder(endpoint, model, apiKey, 768)

embedded, err := engine.EmbedRepo(ctx, "project", embedder, &codetrip.EmbedOptions{
    QuantizeInt8: true,
})

if err := engine.AttachEmbedder("project", embedder); err != nil { /* handle */ }
hybrid, err := engine.HybridSearch(ctx, &codetrip.HybridSearchRequest{
    Repo: "project", Query: "configuration loading", Limit: 20,
})
```

### Persisted graph export

```go
manifest, err := engine.ExportFullCSV("project", "./exports/project")
```

The Go API exposes domain request/result types. Internal storage and index implementations are intentionally not public.

## MCP

```bash
codetrip mcp --trip-dir ~/.codetrip
```

The stdio server exposes `list_repositories`, `search_symbols`, `search_source`, `traverse_graph`, and `shortest_path`. MCP lives in the CLI adapter and invokes only public library methods.

## Validation

Codetrip uses one production analysis path. Independent expectations and fixtures live under `validation/`.

```bash
go build -o ./codetrip ./cmd/codetrip
python3 validation/tools/check_quality.py
python3 -m unittest discover -s validation/tools -p 'test_*.py'
```
