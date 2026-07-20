# codetrip build and release entry points.

BINARY_NAME := codetrip
VERSION := $(shell tr -d '\n' < VERSION)
MAIN_PKG := ./cmd/codetrip
BIN_DIR := bin

GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
EXT := $(if $(filter windows,$(GOOS)),.exe,)
OUTPUT ?= $(BIN_DIR)/$(BINARY_NAME)-$(GOOS)-$(GOARCH)$(EXT)

LDFLAGS := -s -w -X github.com/mengshi02/codetrip.Version=$(VERSION)
RELEASE_LDFLAGS := $(LDFLAGS) -linkmode external
RELEASE_TAGS := netgo osusergo

ifeq ($(GOOS),linux)
RELEASE_LDFLAGS += -extldflags=-static
endif
ifeq ($(GOOS),windows)
RELEASE_LDFLAGS += -extldflags=-static
endif

.PHONY: all build release-build build-all clean test version install help

all: build

# Build for the current platform with its native C toolchain.
build:
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=1 go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY_NAME) $(MAIN_PKG)

# Build one native release artifact. CI invokes this target on the matching OS/CPU runner.
release-build:
	@mkdir -p $(dir $(OUTPUT))
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-tags '$(RELEASE_TAGS)' \
		-ldflags '$(RELEASE_LDFLAGS)' \
		-o $(OUTPUT) $(MAIN_PKG)

# CGO releases need native runners. Dispatch the GitHub Actions build matrix.
build-all:
	@command -v gh >/dev/null 2>&1 || { echo "error: GitHub CLI (gh) is required"; exit 1; }
	@gh auth status >/dev/null 2>&1 || { \
	  echo "error: GitHub CLI is not authenticated"; \
	  echo "run 'gh auth login' once, or export GH_TOKEN with Actions workflow permission"; \
	  exit 2; \
	}
	@git ls-files --error-unmatch .github/workflows/release.yml >/dev/null 2>&1 || { \
	  echo "error: .github/workflows/release.yml has not been committed"; \
	  echo "commit and push the release workflow before running build-all"; \
	  exit 2; \
	}
	@REF=$$(git branch --show-current); \
	  test -n "$$REF" || { echo "error: build-all must be run from a branch"; exit 1; }; \
	  echo "Dispatching release build matrix for $$REF (v$(VERSION)) ..."; \
	  gh workflow run release.yml --ref "$$REF"
	@echo "Build submitted. Use 'gh run watch' to follow it and 'gh run download' to fetch artifacts."

clean:
	rm -rf $(BIN_DIR)

test:
	go test ./...

version:
	@echo "$(BINARY_NAME) v$(VERSION)"

install:
	CGO_ENABLED=1 go install -ldflags '$(LDFLAGS)' $(MAIN_PKG)

help:
	@echo "Available targets:"
	@echo "  build          - Build for the current platform with CGO enabled"
	@echo "  release-build  - Build one native release artifact (used by CI)"
	@echo "  build-all      - Dispatch the GitHub Actions multi-platform build"
	@echo "  clean          - Remove build artifacts"
	@echo "  test           - Run tests"
	@echo "  version        - Show version"
	@echo "  install        - Install for the current platform"
	@echo "  help           - Show this help"
