# Contributing to codetrip

Thank you for your interest in contributing to codetrip — Hybrid Graph-Augmented Code Intelligence Engine.

## Getting Started

### Prerequisites

- Go 1.26+
- Git

### Build

```bash
git clone https://github.com/mengshi02/codetrip.git
cd codetrip
make build
```

### Test

```bash
go test ./...
```

## How to Contribute

### Bug Reports

- Search existing issues before opening a new one
- Include steps to reproduce, expected behavior, and actual behavior
- Specify the codetrip version and Go version

### Feature Requests

- Describe the use case and expected benefit
- Check if it aligns with codetrip's scope as a code intelligence engine

### Pull Requests

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/my-feature`)
3. Make your changes with clear, concise commits
4. Add tests for new functionality
5. Ensure all tests pass (`go test ./...`)
6. Submit a pull request with a clear description

## Code Style

- Follow standard Go conventions (`gofmt`, `golint`)
- Use English for all code comments
- Keep functions focused and readable
- Add doc comments to exported types and functions

## Adding a Language Provider

codetrip uses tree-sitter based language providers. To add support for a new language:

1. Implement the `LanguageProvider` interface in `internal/pipeline/lang/`
2. Add accuracy tests following the existing `*_provider_accuracy_test.go` pattern
3. Register the provider in `trip.go:registerBuiltinProviders()`
4. Add the file label type in `internal/graph/node.go`

## Adding a Pipeline Phase

1. Implement the `pipeline.Phase` interface
2. Register it in `trip.go:registerBuiltinPhases()` or via `WithPhase()` option

## License

By contributing, you agree that your contributions will be licensed under the MIT License.