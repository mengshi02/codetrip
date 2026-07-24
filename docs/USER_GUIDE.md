# Codetrip User Guide

Codetrip is a Hybrid Graph-Augmented Code Intelligence Engine with three supported integration surfaces: Go library, CLI, and MCP. All three use the same persisted repository snapshots and public `Engine` operations.

## Data and update model

The default data directory is `~/.codetrip`; override it with the global `--dir` flag.

```text
repos/<id>/
  manifest.json  logical repository metadata
  graph/db/      authoritative graph, adjacency, and active pointer
  index/         versioned symbol indexes
  content/       versioned source indexes
  vectors/       optional semantic data
trash/           interrupted deletion cleanup
```

Each logical repository owns an independent database and points to one immutable physical snapshot. Repository databases open lazily, so accessing one project does not lock unrelated projects. `index --replace` builds a complete new graph and all derived indexes, publishes the active pointer only after success, and retires the previous snapshot. Codetrip does not modify an active snapshot incrementally. Data created by the earlier shared-database layout must be reindexed.

## CLI

All business commands are single words:

| Command | Purpose |
|---|---|
| `index` | Parse and persist a repository |
| `delete` | Delete a repository and all persisted data |
| `search` | Search symbols and metadata |
| `source` | Search code, engineering configuration, or documentation contents |
| `embed` | Build semantic data for a repository |
| `hybrid` | Fuse symbol and semantic retrieval |
| `context` | Explain a symbol and its direct semantic neighborhood |
| `impact` | Analyze symbols affected by a change |
| `check` | Check graph integrity and repository structure |
| `diff` | Map Git changes to symbols and affected code |
| `rename` | Plan a symbol rename without modifying source |
| `traverse` | Run bounded BFS from a node |
| `path` | Find a shortest directed path |
| `export` | Export the persisted graph as CSV |
| `list` | List active logical repositories |
| `mcp` | Start the stdio MCP server or configure supported agent clients |
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
| `--export` | Write deterministic parser-inspection CSV files to a directory |
| `--export-strict` | Fail indexing if parser CSV generation fails |

Delete a logical repository, including every graph, source, symbol, and vector snapshot:

```bash
codetrip delete project
```

The library exposes the same operation as `Engine.DeleteRepo`. Destructive repository management is intentionally not exposed through MCP.

### Search

```bash
codetrip search "ParseConfig" --repo project --limit 20
codetrip source 'lang:go file:config ParseConfig' --repo project --context 2
codetrip source 'deployment architecture' --repo project --scope docs
codetrip source 'NewHTTPServer' --repo project --scope all
```

`search` targets indexed symbols and metadata. `source` searches repository text and supports literal, regular-expression, file, and language filters. Its `--scope` is `code` by default: `code` includes programming languages and engineering configuration, `docs` searches documentation only, and `all` searches both. Binary files, dependencies, build output, caches, and unsupported text remain excluded.

Linux and macOS use the native high-throughput source backend. Windows uses a
portable full-text backend followed by exact line matching. The public API and
query features are the same, but Windows source search is slower on large
repositories. Rebuild a repository snapshot after moving its data directory
between Windows and Linux/macOS because their source-index formats differ. Reindex existing repositories after upgrading to the scoped source-index format.

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
codetrip context NODE_ID --repo project
codetrip context NODE_ID --repo project --relations CALLS,IMPLEMENTS
codetrip impact NODE_ID --repo project --depth 3 --limit 100
codetrip check --repo project
codetrip check --repo project --checks confidence --confidence 0.7
codetrip traverse NODE_ID --repo project --direction both --depth 3
codetrip traverse NODE_ID --repo project --relations CALLS,IMPORTS
codetrip path SOURCE_ID TARGET_ID --repo project
```

`context` returns the symbol, its source excerpt when the original checkout is
available, and direct typed relationships. It excludes structural graph noise
by default. `impact` follows incoming semantic dependencies and reports
affected actionable symbols with depth, relationship, and confidence. It
defaults to depth 3 and supports `--relations` for narrower analysis.

`check` runs `integrity` and `cycles` by default. Integrity reports missing
edge endpoints and invalid self-dependencies. Cycle analysis distinguishes
inheritance cycles (errors) from import cycles (warnings). The optional
`confidence` check reports semantic relationships below `--confidence`; it is
not enabled by default because it is intended for focused review rather than
as a correctness failure.

### Change analysis

```bash
# Compare HEAD with tracked working-tree changes.
codetrip diff --repo project

# Compare two commits.
codetrip diff HEAD~1 --target HEAD --repo project

# Map changed symbols without reverse impact expansion.
codetrip diff HEAD~1 --target HEAD --repo project --no-impact
```

`diff` parses zero-context Git hunks, maps changed lines to persisted actionable
symbols, and aggregates `impact` results with the changed symbols recorded as
causes. It uses the source directory stored in the repository manifest and
restricts Git output to that directory, including when the indexed repository
is a subdirectory of a larger worktree. Untracked files are not included until
they are added to Git and indexed.

### Rename planning

```bash
codetrip rename NODE_ID LoadConfig --repo project
```

`rename` is analysis-only and never edits files. It validates the requested
identifier, reports same-file conflicts as errors and repository-wide
collisions as warnings, follows incoming typed relationships to identify
semantic references, and searches the code index for exact identifier
occurrences. Declaration and graph-backed reference lines are marked as
high-confidence edits; comments, strings, reflective access, and other textual
candidates remain explicitly marked `requiresReview`.

Canonical traversal directions are `out`, `in`, and `both`. Agent-friendly aliases are accepted: `forward`, `down`, `downstream`, and `call` map to `out`; `reverse`, `backward`, `up`, and `upstream` map to `in`; `any`, `all`, and `bidirectional` map to `both`. Direction controls edge orientation, while `--relations CALLS` restricts the relationship type. The engine enforces a configurable visit limit.

### CSV

Validation CSV captures the ingest result before persistence and is intended for language tuning:

```bash
codetrip index /src/project --repo project \
  --export ./local-review/project --export-strict
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
    codetrip.WithCSVExport("./local-review/project"),
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

symbolContext, err := engine.Context(ctx, &codetrip.ContextRequest{
    Repo: "project", NodeID: nodeID,
})

impact, err := engine.Impact(ctx, &codetrip.ImpactRequest{
    Repo: "project", NodeID: nodeID, MaxDepth: 3, Limit: 100,
})

checks, err := engine.Check(ctx, &codetrip.CheckRequest{
    Repo: "project",
})

changes, err := engine.Diff(ctx, &codetrip.DiffRequest{
    Repo: "project", BaseRef: "HEAD~1", TargetRef: "HEAD",
    MaxDepth: 3, Limit: 100,
})

rename, err := engine.Rename(ctx, &codetrip.RenameRequest{
    Repo: "project", NodeID: nodeID, NewName: "LoadConfig",
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

Configure every supported client detected on the current machine:

```bash
codetrip mcp setup
```

Supported targets are `codex`, `claude`, `cursor`, `vscode`, and `copilot`.
Pass one or more targets explicitly, use `--dry-run` to inspect the planned
changes, and use `--force` only when an existing Codetrip entry should be
replaced:

```bash
codetrip mcp setup codex cursor
codetrip mcp setup --dry-run
codetrip mcp setup claude --force
```

The setup command preserves unrelated MCP servers. Cursor's JSON configuration
is merged atomically and backed up before modification. The other clients are
configured through their official command-line interfaces.

To start the stdio server directly:

```bash
codetrip mcp --dir ~/.codetrip
```

The stdio server exposes `list`, `search`, `source`, `context`, `impact`, `check`, `diff`, `rename`, `traverse`, and `path`, matching the corresponding CLI command names. MCP lives in the CLI adapter and invokes only public `Engine` methods.

The MCP process opens the engine for one tool request and releases it when the
request finishes, so an idle server does not hold the database lock or block CLI
indexing. MCP requests are serialized; a request made while a CLI index command
owns the database may return a temporary store-busy error and can be retried.

## Local language tuning

Codetrip uses one production analysis path. Maintainers can inspect the
deterministic CSV produced by `--export` using their local fixtures, gold
expectations, and review tools. Those tuning assets are intentionally not part
of the public source repository and are not required by CLI, MCP, or Go library
users.

```bash
codetrip index /src/project --repo project \
  --export /path/to/local-review/project --export-strict
```
