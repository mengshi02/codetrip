# codetrip Makefile — Hybrid Graph-Augmented Code Intelligence Engine
# Build codetrip for multiple platforms and architectures

BINARY_NAME := codetrip
VERSION := $(shell cat VERSION)
GO_MODULE := github.com/mengshi02/codetrip
MAIN_PKG := ./cmd/codetrip

LDFLAGS := -s -w -X github.com/mengshi02/codetrip.Version=$(VERSION)

# Target platforms
PLATFORMS := \
	windows/amd64 \
	windows/arm64 \
	linux/amd64 \
	linux/arm64 \
	darwin/amd64 \
	darwin/arm64

# Output directory
BIN_DIR := bin

# Default target
.PHONY: all
all: build

# Build for current platform only
.PHONY: build
build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PKG)

# Build all platforms
.PHONY: build-all
build-all: $(PLATFORMS)

# Define per-platform build targets
.PHONY: $(PLATFORMS)
$(PLATFORMS):
	@mkdir -p $(BIN_DIR)
	$(eval OS := $(word 1,$(subst /, ,$@)))
	$(eval ARCH := $(word 2,$(subst /, ,$@)))
	$(eval EXT := $(if $(filter windows,$(OS)),.exe,))
	@echo "Building $(BINARY_NAME)-$(OS)-$(ARCH)$(EXT) ..."
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(LDFLAGS)" -o $(BIN_DIR)/$(BINARY_NAME)-$(OS)-$(ARCH)$(EXT) $(MAIN_PKG)

# Clean build artifacts
.PHONY: clean
clean:
	rm -rf $(BIN_DIR)

# Run tests
.PHONY: test
test:
	go test ./...

# Show version info
.PHONY: version
version:
	@echo "$(BINARY_NAME) v$(VERSION)"

# Install locally
.PHONY: install
install:
	go install -ldflags "$(LDFLAGS)" $(MAIN_PKG)

# Help
.PHONY: help
help:
	@echo "Available targets:"
	@echo "  build       - Build for current platform"
	@echo "  build-all   - Build for all platforms (windows/linux/darwin, amd64/arm64)"
	@echo "  clean       - Remove build artifacts"
	@echo "  test        - Run tests"
	@echo "  version     - Show version"
	@echo "  install     - Install binary locally"
	@echo "  help        - Show this help"