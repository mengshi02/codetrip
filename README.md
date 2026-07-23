# codetrip [![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT) [![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white)](https://go.dev/) [![Version](https://img.shields.io/badge/version-0.2.0-blue.svg)](https://github.com/mengshi02/codetrip) [![Build](https://github.com/mengshi02/codetrip/actions/workflows/go.yml/badge.svg)](https://github.com/mengshi02/codetrip/actions/workflows/go.yml)

**Hybrid Graph-Augmented Code Intelligence Engine**

Codetrip turns a source repository into a typed code graph and combines graph traversal, lexical source search, and semantic retrieval. Use it as a Go library, a CLI, or an MCP server.

## How It Works

```text
+--------------------+     +------------------------------+     +------------------------+
| Source Code        |     | Codetrip Engine              |     | Intelligence           |
|                    |     |                              |     |                        |
| .go  .ts  .py      | --> | Parse + Language Resolution  | --> | Symbol Search          |
| .rs  .java  ...    |     |             |                |     | Source Search          |
|                    |     |             v                |     | Graph Context          |
| Repository         |     |      Typed Code Graph        |     | Impact Analysis        |
|                    |     |             |                |     | Structural Checks      |
|                    |     |   +---------+---------+      |     | Change Analysis        |
|                    |     |   |         |         |      |     | Rename Planning        |
|                    |     | Symbol    Source   Semantic  |     | Graph Traversal        |
|                    |     | Index     Index    Vectors   |     | Shortest Path          |
|                    |     |   +---------+---------+      |     | Hybrid Retrieval       |
|                    |     |             |                |     |                        |
|                    |     |     Atomic Snapshot          |     | Agent Context          |
|                    |     |             |                |     |                        |
|                    |     |      Go LIB / CLI / MCP      |     |                        |
+--------------------+     +------------------------------+     +------------------------+
```

Each repository owns an independent storage directory and is published as an atomic snapshot. The durable graph is authoritative; search indexes and vectors are repository-scoped derived data. Repository databases open lazily, so operations on one project do not lock unrelated projects.

## Quick Start

### Install

```bash
go install github.com/mengshi02/codetrip/cmd/codetrip@latest
```

Or build from source:

```bash
git clone https://github.com/mengshi02/codetrip.git
cd codetrip && make build
```

### Index and Search

```bash
codetrip index /path/to/project --repo project

codetrip search "ParseConfig" --repo project
codetrip source 'lang:go ParseConfig' --repo project
codetrip source 'architecture' --repo project --scope docs

# Remove the repository and every persisted snapshot.
codetrip delete project
```

### Explore the Graph

```bash
codetrip context NODE_ID --repo project
codetrip impact NODE_ID --repo project --depth 3
codetrip check --repo project
codetrip diff HEAD~1 --target HEAD --repo project
codetrip rename NODE_ID NewName --repo project
codetrip traverse NODE_ID --repo project --direction both --depth 3
codetrip path SOURCE_ID TARGET_ID --repo project
```

### Semantic Search

```bash
codetrip embed --repo project \
  --endpoint http://localhost:11434/v1/embeddings \
  --model nomic-embed-text

codetrip hybrid "configuration loading" --repo project \
  --endpoint http://localhost:11434/v1/embeddings \
  --model nomic-embed-text
```

## Core Capabilities

| Capability | Description |
|---|---|
| **Code Graph** | Typed files, symbols, calls, imports, inheritance, overrides, communities, and processes |
| **Multi-language Parsing** | Go, TypeScript/TSX, JavaScript/JSX, Python, Java, C, C++, C#, Rust, PHP, Swift, and Kotlin |
| **Graph Navigation** | Bounded BFS and shortest directed paths over typed relationships |
| **Agent Intelligence** | Noise-filtered symbol context and bounded reverse impact analysis |
| **Structural Checks** | Graph integrity, invalid self-dependencies, inheritance cycles, import cycles, and optional confidence review |
| **Change Intelligence** | Git changed-line mapping to persisted symbols with aggregated reverse impact |
| **Rename Planning** | Non-mutating conflict detection, semantic references, and exact source candidates |
| **Symbol Search** | Repository-scoped lexical search over symbols and metadata |
| **Source Search** | File-name and source-content search with literal, regex, file, and language filters |
| **Semantic Retrieval** | HTTP embeddings, persisted vectors, optional int8 quantization, and hybrid rank fusion |
| **Atomic Snapshots** | Complete replacement builds before publication; active data is never partially updated |
| **CSV Export** | Deterministic parser-inspection CSV and complete persisted-graph export |

## Go Library

```go
engine, err := codetrip.Open("./.codetrip")
if err != nil {
    log.Fatal(err)
}
defer engine.Close()

_, err = engine.IndexRepo(ctx, "/path/to/project",
    codetrip.WithRepoName("project"),
    codetrip.WithReplaceExisting(true),
)

result, err := engine.Search(ctx, &codetrip.SearchRequest{
    Repo: "project", Query: "ParseConfig", Limit: 20,
})

context, err := engine.Context(ctx, &codetrip.ContextRequest{
    Repo: "project", NodeID: nodeID,
})

impact, err := engine.Impact(ctx, &codetrip.ImpactRequest{
    Repo: "project", NodeID: nodeID, MaxDepth: 3,
})

checks, err := engine.Check(ctx, &codetrip.CheckRequest{
    Repo: "project",
})

changes, err := engine.Diff(ctx, &codetrip.DiffRequest{
    Repo: "project", BaseRef: "HEAD~1", TargetRef: "HEAD",
})

rename, err := engine.Rename(ctx, &codetrip.RenameRequest{
    Repo: "project", NodeID: nodeID, NewName: "LoadConfig",
})
```

The public API also provides source search, embedding, hybrid search, context, impact analysis, traversal, shortest paths, repository listing, CSV export, metrics, and configuration options.

## MCP

```bash
codetrip mcp --dir ~/.codetrip
```

The stdio server exposes the same core names as the CLI:

```text
list  search  source  context  impact  check  diff  rename  traverse  path
```

## CSV Inspection

```bash
codetrip index /path/to/project --repo project \
  --export ./local-review/project --export-strict

codetrip export --repo project --output ./exports/project
```

Language fixtures and review tools are maintainer-local assets; users only need the CSV export capability.

## Platforms

Release binaries are available for Linux and macOS on amd64/arm64, and Windows on amd64. They are distributed as single executables with no third-party runtime installation. Windows uses a portable source-search backend and may be slower on large repositories.

## Documentation

- [English user guide](docs/USER_GUIDE.md)
- [中文用户手册](docs/USER_GUIDE_ZH.md)

## Development

```bash
go test ./...
go vet ./...
```

## License

MIT
