# codetrip

**Hybrid Graph-Augmented Code Intelligence Engine**

codetrip builds a knowledge graph from your codebase and augments it with hybrid retrieval (BM25 + semantic vectors) to power deep code intelligence — impact analysis, taint tracking, structural checks, cross-repo reasoning, and more.

## How It Works

```
  Source Code                    codetrip Engine                        Intelligence
──────────────     ──────────────────────────────────────     ──────────────────────────────
                                       │
  .go .ts .py  ──►  Parse (tree-sitter)│  ──►  Code Graph         ──►  Impact Analysis
  .rs .java …       Scope Resolution   │       (38 node types,    ──►  Taint Explanation
                    Embedding Pipeline │        29 edge types)    ──►  Structural Checks
                                       │                          ──►  Symbol Context
                          ┌────────────┼────────────┐             ──►  Route / API Analysis
                          │            │            │             ──►  Cross-Repo Reasoning
                        BM25      HNSW Vectors   Cypher           ──►  Rename Refactoring
                          │            │            │             ──►  Change Detection
                          └──────── RRF Fusion ─────┘
                                  Hybrid Search
```

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

### Index

```bash
codetrip index /path/to/my-project
```

### Query

```bash
# Cypher graph query
codetrip query "MATCH (n:Function) RETURN n.name LIMIT 10" --repo my-project

# Hybrid search (BM25 + semantic)
codetrip search "user authentication" --repo my-project --semantic
```

### Analyze

```bash
# Impact analysis
codetrip impact User --repo my-project --direction downstream

# Taint tracking
codetrip explain --repo my-project --target handleRequest

# API route impact
codetrip api-impact --repo my-project --route /api/users

# Structural checks (cycles, etc.)
codetrip check --repo my-project --cycles
```

## Core Capabilities

| Capability | Description |
|---|---|
| **Graph Indexing** | Multi-language AST → knowledge graph (Go, TS, JS, Python, Java, Rust, C, C++, C#, Markdown) |
| **Cypher Query** | Declarative graph traversal with Volcano iterator model |
| **Hybrid Search** | BM25 + HNSW semantic vectors, RRF fusion |
| **Impact Analysis** | Upstream/downstream dependency tracing |
| **Taint Explanation** | CFG-based data flow tracking with source-to-sink path explanation |
| **Symbol Context** | 360° symbol view with incoming/outgoing references |
| **Rename Refactoring** | Graph-backed multi-file coordinated renaming |
| **Change Detection** | SHA1 hash-driven incremental re-indexing |
| **Route / API Analysis** | Framework-aware route extraction + impact + shape checking |
| **Cross-Repo Groups** | Multi-repo contract matching and cross-repo impact analysis |
| **Wiki Generation** | LLM-augmented project documentation from the knowledge graph |

## Architecture

- **Storage**: Pebble (LSM-tree key-value store) with sharded LRU node cache
- **Graph**: 38 node labels, typed edges, adjacency indexes
- **Search**: Bluge BM25 + custom HNSW with int8 quantization and two-stage retrieval
- **Parsing**: Tree-sitter with extensible language providers
- **Embedding**: ONNX runtime / HTTP embedding pipeline with mmap quantized vectors
- **Query Engine**: Cypher AST → Volcano iterator plan with timeout protection
- **Scalability**: Designed for 1M+ node repos (batched indexing, chunked BM25, GC pools)

## MCP Integration

codetrip ships an MCP server for AI coding agent integration:

```bash
codetrip mcp
```

Exposes all intelligence capabilities as MCP tools for Claude, Cursor, and other MCP-compatible agents.

## Go API

```go
trip, _ := codetrip.Open("~/.codetrip")

// Index
result, _ := trip.IndexRepo(ctx, "/path/to/repo", codetrip.WithRepoName("my-app"))

// Query
rows, _ := trip.Cypher(ctx, "MATCH (n:Function)-[:CALLS]->(m) RETURN n.name, m.name")

// Impact
impact, _ := trip.Impact(ctx, &codetrip.ImpactRequest{Target: "User", Direction: "downstream"})

// Search
results, _ := trip.Search(ctx, &codetrip.SearchRequest{Query: "auth", Semantic: true})

trip.Close()
```

## License

MIT