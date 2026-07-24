# Changelog

All notable changes to codetrip will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2026-07-24

### Added

- Added `codetrip mcp setup` for one-command integration with Codex, Claude Code, Cursor, VS Code/Copilot, and GitHub Copilot CLI.
- Added automatic client detection, `--dry-run`, targeted client setup, guarded replacement, and Cursor configuration backup.

### Changed

- Improved README onboarding around coding-agent workflows and MCP setup.


## [0.2.0] - 2026-07-23

### Added

- Production semantic analysis for Go, Java, Kotlin, C#, Python, Rust, Swift,
  TypeScript/TSX, JavaScript/JSX, PHP, C, and C++.
- Typed `CALLS`, `EXTENDS`, `IMPLEMENTS`, `OVERRIDES`,
  `METHOD_IMPLEMENTS`, and `DISPATCHES_TO` relationships with cross-file
  resolution.
- Repository-scoped symbol search, source search, semantic vectors, hybrid
  retrieval, bounded graph traversal, and shortest paths.
- `code`, `docs`, and `all` source-search scopes.
- `context` symbol intelligence with source excerpts and typed direct
  relationships.
- `impact` reverse semantic dependency analysis with depth, confidence, and
  relationship details.
- `check` graph integrity, inheritance-cycle, import-cycle, and optional
  low-confidence relationship analysis.
- `diff` Git changed-line mapping to persisted symbols with aggregated impact
  and change causes.
- Non-mutating `rename` planning with identifier validation, conflict
  detection, semantic references, and exact textual candidates.
- Deterministic parser-inspection CSV and complete persisted-graph CSV export.
- Public Go `Engine` APIs for all supported library capabilities.
- Single-word CLI commands and matching stdio MCP tools.
- One-command MCP setup for Codex, Claude Code, Cursor, VS Code/Copilot, and
  GitHub Copilot CLI, with client detection, dry-run, and guarded replacement.

### Changed

- Rebuilt ingestion around one production analysis path shared by the Go
  library, CLI, and MCP.
- Isolated graph, symbol, source, and vector data per repository.
- Repository replacement now builds and atomically publishes a complete
  snapshot before retiring the previous snapshot.
- MCP is hosted by the CLI adapter and invokes only public `Engine` methods.
- Traversal results now include edges, relationship types, directions,
  confidence, and resolution reasons.
- Source search defaults to code and engineering configuration instead of
  unrestricted repository text.
- Updated README architecture diagram and English and Chinese user guides for
  the current engine capabilities.

### Fixed

- Improved cross-file calls, interface implementations, inheritance, and
  method override resolution across all supported languages.
- Restored PHP Composer namespace, inheritance, implementation, override, and
  call resolution on large framework repositories.
- Fixed Go interface method sets and cross-file call chains.
- Fixed C# generic inheritance and explicit override handling.
- Fixed Python named imports and default-argument calls, Rust workspace
  imports, Swift package visibility and generic constraints, and C/C++ symbol
  resolution.
- Removed dangling community memberships, self imports, invalid self semantic
  relationships, and relationships with missing endpoints.
- Prevented one repository from locking unrelated repository data.
- Added a portable Windows source-search backend while preserving the public
  search behavior.

### Removed

- Removed the obsolete community, ingestion, route, wiki, group, collection,
  and incremental-index implementations replaced by the production engine.
- Removed analysis-mode branching; language tuning now uses the single
  production path and optional local CSV validation.


## [0.1.0] - 2026-06-20

### Added

- **Core Engine**: Hybrid Graph-Augmented Code Intelligence Engine with embedded Pebble storage
- **Multi-language Parsing**: Tree-sitter based parsing with providers for Go, TypeScript, JavaScript, Python, Java, Rust, C, C++, C#, and Markdown
- **Knowledge Graph**: 38 node label types with typed edges, adjacency indexes, and scope resolution
- **Cypher Query Engine**: Declarative graph traversal with Volcano iterator model and timeout protection
- **Hybrid Search**: BM25 full-text search (Bluge) + HNSW semantic vector search with RRF fusion
- **Impact Analysis**: Upstream/downstream dependency tracing with risk assessment
- **Taint Explanation**: CFG-based data flow tracking with source-to-sink path explanation
- **Symbol Context**: 360° symbol view with incoming/outgoing references and disambiguation
- **Rename Refactoring**: Graph-backed multi-file coordinated renaming with confidence scoring
- **Change Detection**: SHA1 hash-driven incremental re-indexing
- **Route / API Analysis**: Framework-aware route extraction (Next.js, Laravel, Express, etc.), API impact analysis, and response shape checking
- **Cross-Repo Groups**: Multi-repo contract matching, bridge building, and cross-repo impact analysis
- **Wiki Generation**: LLM-augmented project documentation from the knowledge graph
- **MCP Server**: Model Context Protocol server for AI coding agent integration
- **Embedding Pipeline**: HTTP embedding with int8 quantization, mmap vector files, and two-stage search
- **Scalability**: Designed for 1M+ node repos — batched indexing, chunked BM25, sharded LRU cache, GC pools
- **CLI**: Full command-line interface with 20+ subcommands
- **Go API**: Embedded library API with extensible provider/resolver/tool registration

### Changed

- Project definition established as **Hybrid Graph-Augmented Code Intelligence Engine**
