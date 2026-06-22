# Changelog

All notable changes to codetrip will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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