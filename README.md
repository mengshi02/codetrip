# codetrip

Codetrip is an embeddable hybrid-graph code intelligence engine. It turns a source repository into a typed code graph and exposes the result through a Go library, a CLI, and an MCP server.

The production pipeline has one analysis path. Language accuracy is tuned with the fixtures and gold expectations under `validation/`; optional CSV output makes parser results inspectable without changing runtime behavior.

## Capabilities

- Multi-language parsing with tree-sitter and language-aware symbol resolution.
- Typed nodes and relationships for files, symbols, calls, imports, inheritance, overrides, communities, and execution processes.
- Repository-isolated persistence, adjacency indexes, caches, BFS, and shortest path traversal.
- Symbol search, source substring/regular-expression search, semantic retrieval, and hybrid rank fusion.
- Atomic full-snapshot replacement: a new graph and all derived indexes are built before publication.
- Deterministic validation CSV and complete persisted-graph CSV export.
- CLI, MCP, and Go library integration surfaces.

Supported parsers include Go, TypeScript/TSX, JavaScript/JSX, Python, Java, C, C++, C#, Rust, Ruby, PHP, Swift, and Kotlin.

## Architecture

```text
source repository
       |
       v
validated ingest pipeline
       |
       v
typed in-memory graph
       |
       +----> durable graph + adjacency + cache
       +----> symbol index
       +----> source index
       +----> optional semantic index
       |
       v
atomic active-snapshot publication
       |
       +----> Go API
       +----> CLI
       +----> MCP
```

The durable graph is authoritative. Search structures are repository-scoped derived data and use the same physical snapshot identity. CSV is an offline validation, inspection, and interchange artifact.

## Install

```bash
go install github.com/mengshi02/codetrip/cmd/codetrip@latest
```

Or build locally:

```bash
make build
```

Codetrip uses CGO for tree-sitter. Release binaries are built on native Linux,
macOS, and Windows runners and require no third-party runtime installation.
`make build-all` dispatches that GitHub Actions matrix; use `gh run watch` and
`gh run download` to follow it and retrieve the archives.

## CLI quick start

```bash
# Build a repository snapshot.
codetrip index /path/to/project --repo project

# Replace an existing repository with a complete new snapshot.
codetrip index /path/to/project --repo project --replace

# Search symbols and source text.
codetrip search "ParseConfig" --repo project
codetrip source 'lang:go ParseConfig' --repo project

# Traverse the graph.
codetrip traverse NODE_ID --repo project --direction both --depth 3
codetrip path SOURCE_ID TARGET_ID --repo project

# Inspect persisted data.
codetrip list
codetrip export --repo project --output ./exports/project
```

Semantic and hybrid retrieval use an OpenAI-compatible embedding endpoint:

```bash
codetrip embed --repo project \
  --endpoint http://localhost:11434/v1/embeddings \
  --model nomic-embed-text

codetrip hybrid "configuration loading" --repo project \
  --endpoint http://localhost:11434/v1/embeddings \
  --model nomic-embed-text
```

Run `codetrip <command> --help` for the complete flag set.

## Go library

```go
package main

import (
    "context"
    "log"

    "github.com/mengshi02/codetrip"
)

func main() {
    engine, err := codetrip.Open("./.codetrip")
    if err != nil {
        log.Fatal(err)
    }
    defer engine.Close()

    ctx := context.Background()
    _, err = engine.IndexRepo(ctx, "/path/to/project",
        codetrip.WithRepoName("project"),
        codetrip.WithReplaceExisting(true),
    )
    if err != nil {
        log.Fatal(err)
    }

    symbols, err := engine.Search(ctx, &codetrip.SearchRequest{
        Repo: "project", Query: "ParseConfig", Limit: 20,
    })
    if err != nil {
        log.Fatal(err)
    }
    _ = symbols
}
```

The public library also provides source search, embedding, hybrid search, traversal, shortest path, repository listing, CSV export, metrics, and configuration options. Internal storage and indexing types are not part of the public contract.

## MCP

```bash
codetrip mcp --trip-dir ~/.codetrip
```

The stdio server exposes:

- `list_repositories`
- `search_symbols`
- `search_source`
- `traverse_graph`
- `shortest_path`

MCP is implemented in the CLI adapter and calls only the public Go library API.

## Validation

Export parser-validation CSV while indexing:

```bash
codetrip index /path/to/project --repo project \
  --export ./validation-output/project \
  --export-strict
```

Run the independent fixture gate:

```bash
python3 validation/tools/check_quality.py
```

See [English user guide](docs/USER_GUIDE.md), [中文用户手册](docs/USER_GUIDE_ZH.md), and [quality standard](validation/QUALITY_STANDARD.md).

## Development

```bash
go test ./...
go vet ./...
make release-build
```

## License

MIT
