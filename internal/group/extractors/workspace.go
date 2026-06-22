package extractors

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// WorkspaceExtractor is a workspace dependency extractor that dispatches to language-specific extraction implementations
type WorkspaceExtractor struct{}

// langDeps describes detected dependency information for a single language workspace
type langDeps struct {
	Language   string
	Manifest   string // package manager file name (e.g., go.mod)
	Imports    []importInfo
}

// importInfo describes a single dependency item
type importInfo struct {
	Path    string // dependency path/name
	Version string // version (if available)
}

// ExtractLibContracts extracts workspace dependencies (main entry point)
// Finds cross-repository package imports, internally dispatched by WorkspaceExtractor per language
func ExtractLibContracts(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error) {
	we := &WorkspaceExtractor{}
	return we.Extract(ctx, repo, gs)
}

// Extract executes workspace dependency extraction, dispatched by language
func (we *WorkspaceExtractor) Extract(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error) {
	// 1. Extract Import contracts based on graph nodes (preserve original logic)
	baseContracts, err := we.extractImportContracts(ctx, repo, gs)
	if err != nil {
		return nil, err
	}

	// 2. Detect package manager files by language, extract language-specific dependencies
	langContracts := we.extractByLanguage(ctx, repo, gs)

	// Merge results
	return append(baseContracts, langContracts...), nil
}

// extractImportContracts extracts contracts based on graph Import nodes (original logic)
func (we *WorkspaceExtractor) extractImportContracts(ctx context.Context, repo string, gs *graph.GraphStore) ([]Contract, error) {
	var contracts []Contract

	// Find Import nodes
	importNodes, err := gs.GetNodesByLabel(gs.Repo(), string(graph.LabelImport))
	if err != nil {
		return contracts, nil
	}

	for _, node := range importNodes {
		importPath := node.GetPropString("path")
		if importPath == "" {
			importPath = node.Name
		}

		// Provider: the imported package
		contracts = append(contracts, Contract{
			ID:         util.GenerateID(repo, "lib-provider", node.ID),
			ContractID: fmt.Sprintf("lib:%s:%s", repo, importPath),
			Type:       "lib",
			Role:       "provider",
			Repo:       repo,
			SymbolUID:  node.UID(),
			Confidence: 0.85,
			Meta: map[string]any{
				"package": importPath,
				"name":    node.Name,
			},
		})

		// Consumer: the importer
		inEdges, _ := gs.GetAllInEdges(node.ID)
		for _, edge := range inEdges {
			if edge.Type == graph.RelImports {
				src, e := gs.GetNode(edge.Source)
				if e != nil {
					continue
				}
				contracts = append(contracts, Contract{
					ID:         util.GenerateID(repo, "lib-consumer", src.ID),
					ContractID: fmt.Sprintf("lib:%s:%s", repo, importPath),
					Type:       "lib",
					Role:       "consumer",
					Repo:       repo,
					SymbolUID:  src.UID(),
					Confidence: 0.8,
					Meta: map[string]any{
						"package": importPath,
						"name":    src.Name,
					},
				})
			}
		}
	}

	return contracts, nil
}

// extractByLanguage dispatches workspace dependency extraction by language
func (we *WorkspaceExtractor) extractByLanguage(ctx context.Context, repo string, gs *graph.GraphStore) []Contract {
	var contracts []Contract

	// Collect all file paths in the repository to detect languages
	allNodes := gs.GetAllNodes(gs.Repo(), 0)
	filePaths := we.collectFilePaths(allNodes)

	// Detect and extract by language
	if we.hasManifest(filePaths, "go.mod") {
		contracts = append(contracts, we.extractGoWorkspace(ctx, repo, gs, allNodes)...)
	}
	if we.hasManifest(filePaths, "package.json") {
		contracts = append(contracts, we.extractNodeWorkspace(ctx, repo, gs, allNodes)...)
	}
	if we.hasManifest(filePaths, "requirements.txt") || we.hasManifest(filePaths, "setup.py") || we.hasManifest(filePaths, "pyproject.toml") {
		contracts = append(contracts, we.extractPythonWorkspace(ctx, repo, gs, allNodes)...)
	}
	if we.hasManifest(filePaths, "pom.xml") || we.hasManifest(filePaths, "build.gradle") || we.hasManifest(filePaths, "build.gradle.kts") {
		contracts = append(contracts, we.extractJavaWorkspace(ctx, repo, gs, allNodes)...)
	}
	if we.hasManifest(filePaths, "Cargo.toml") {
		contracts = append(contracts, we.extractRustWorkspace(ctx, repo, gs, allNodes)...)
	}
	if we.hasManifest(filePaths, "mix.exs") {
		contracts = append(contracts, we.extractElixirWorkspace(ctx, repo, gs, allNodes)...)
	}

	return contracts
}

// collectFilePaths collects all file paths from nodes
func (we *WorkspaceExtractor) collectFilePaths(nodes []*graph.Node) []string {
	seen := make(map[string]bool)
	var paths []string
	for _, node := range nodes {
		if node.FilePath != "" && !seen[node.FilePath] {
			seen[node.FilePath] = true
			paths = append(paths, node.FilePath)
		}
	}
	return paths
}

// hasManifest checks if the file path list contains the specified package manager file
func (we *WorkspaceExtractor) hasManifest(filePaths []string, manifest string) bool {
	for _, p := range filePaths {
		if filepath.Base(p) == manifest {
			return true
		}
	}
	return false
}

// ============== Per-Language Workspace Extractors ==============

// extractGoWorkspace extracts Go workspace dependencies (go.mod)
func (we *WorkspaceExtractor) extractGoWorkspace(ctx context.Context, repo string, gs *graph.GraphStore, allNodes []*graph.Node) []Contract {
	var contracts []Contract

	for _, node := range allNodes {
		if filepath.Base(node.FilePath) != "go.mod" {
			continue
		}

		// 从节点属性提取 module 路径和依赖
		modulePath := node.GetPropString("module")
		if modulePath == "" {
			modulePath = node.Name
		}

		// Provider: 当前模块
		contracts = append(contracts, Contract{
			ID:         util.GenerateID(repo, "lib-provider", node.ID),
			ContractID: fmt.Sprintf("lib:%s:%s", repo, modulePath),
			Type:       "lib",
			Role:       "provider",
			Repo:       repo,
			SymbolUID:  node.UID(),
			Confidence: 0.95,
			Meta: map[string]any{
				"package":  modulePath,
				"language": "go",
				"manifest": "go.mod",
				"name":     node.Name,
			},
		})

		// Consumer: each require dependency declared in go.mod
		// Get requires list from node properties (filled by parser)
		reqs := we.extractStringListProp(node, "requires")
		for _, req := range reqs {
			contracts = append(contracts, Contract{
				ID:         util.GenerateID(repo, "lib-consumer", node.ID+"-"+req),
				ContractID: fmt.Sprintf("lib:%s:%s", repo, req),
				Type:       "lib",
				Role:       "consumer",
				Repo:       repo,
				SymbolUID:  node.UID(),
				Confidence: 0.9,
				Meta: map[string]any{
					"package":  req,
					"language": "go",
					"manifest": "go.mod",
				},
			})
		}
	}

	return contracts
}

// extractNodeWorkspace extracts Node.js workspace dependencies (package.json)
func (we *WorkspaceExtractor) extractNodeWorkspace(ctx context.Context, repo string, gs *graph.GraphStore, allNodes []*graph.Node) []Contract {
	var contracts []Contract

	for _, node := range allNodes {
		if filepath.Base(node.FilePath) != "package.json" {
			continue
		}

		pkgName := node.GetPropString("name")
		if pkgName == "" {
			pkgName = node.Name
		}

		// Provider: 当前包
		contracts = append(contracts, Contract{
			ID:         util.GenerateID(repo, "lib-provider", node.ID),
			ContractID: fmt.Sprintf("lib:%s:%s", repo, pkgName),
			Type:       "lib",
			Role:       "provider",
			Repo:       repo,
			SymbolUID:  node.UID(),
			Confidence: 0.95,
			Meta: map[string]any{
				"package":  pkgName,
				"language": "node",
				"manifest": "package.json",
				"name":     node.Name,
			},
		})

		// Consumer: dependencies + devDependencies
		deps := we.extractStringListProp(node, "dependencies")
		deps = append(deps, we.extractStringListProp(node, "devDependencies")...)
		for _, dep := range deps {
			contracts = append(contracts, Contract{
				ID:         util.GenerateID(repo, "lib-consumer", node.ID+"-"+dep),
				ContractID: fmt.Sprintf("lib:%s:%s", repo, dep),
				Type:       "lib",
				Role:       "consumer",
				Repo:       repo,
				SymbolUID:  node.UID(),
				Confidence: 0.85,
				Meta: map[string]any{
					"package":  dep,
					"language": "node",
					"manifest": "package.json",
				},
			})
		}
	}

	return contracts
}

// extractPythonWorkspace extracts Python workspace dependencies (requirements.txt / setup.py / pyproject.toml)
func (we *WorkspaceExtractor) extractPythonWorkspace(ctx context.Context, repo string, gs *graph.GraphStore, allNodes []*graph.Node) []Contract {
	var contracts []Contract

	// Detect Python project name (from setup.py / pyproject.toml)
	projectName := ""
	for _, node := range allNodes {
		base := filepath.Base(node.FilePath)
		if base == "setup.py" || base == "pyproject.toml" {
			projectName = node.GetPropString("name")
			if projectName == "" {
				projectName = node.Name
			}

			// Provider
			contracts = append(contracts, Contract{
				ID:         util.GenerateID(repo, "lib-provider", node.ID),
				ContractID: fmt.Sprintf("lib:%s:%s", repo, projectName),
				Type:       "lib",
				Role:       "provider",
				Repo:       repo,
				SymbolUID:  node.UID(),
				Confidence: 0.9,
				Meta: map[string]any{
					"package":  projectName,
					"language": "python",
					"manifest": base,
					"name":     node.Name,
				},
			})
		}
	}

	// Consumer: dependencies declared in requirements.txt
	for _, node := range allNodes {
		if filepath.Base(node.FilePath) != "requirements.txt" {
			continue
		}

		reqs := we.extractStringListProp(node, "requires")
		for _, req := range reqs {
			// Remove version specifiers
			pkg := we.stripPythonVersionSpecifier(req)
			contracts = append(contracts, Contract{
				ID:         util.GenerateID(repo, "lib-consumer", node.ID+"-"+pkg),
				ContractID: fmt.Sprintf("lib:%s:%s", repo, pkg),
				Type:       "lib",
				Role:       "consumer",
				Repo:       repo,
				SymbolUID:  node.UID(),
				Confidence: 0.85,
				Meta: map[string]any{
					"package":  pkg,
					"language": "python",
					"manifest": "requirements.txt",
					"raw":      req,
				},
			})
		}
	}

	_ = projectName // avoid unused warning
	return contracts
}

// extractJavaWorkspace extracts Java workspace dependencies (pom.xml / build.gradle)
func (we *WorkspaceExtractor) extractJavaWorkspace(ctx context.Context, repo string, gs *graph.GraphStore, allNodes []*graph.Node) []Contract {
	var contracts []Contract

	for _, node := range allNodes {
		base := filepath.Base(node.FilePath)
		if base != "pom.xml" && base != "build.gradle" && base != "build.gradle.kts" {
			continue
		}

		groupID := node.GetPropString("groupId")
		artifactID := node.GetPropString("artifactId")
		if artifactID == "" {
			artifactID = node.Name
		}

		// Build Maven coordinate
		coord := artifactID
		if groupID != "" {
			coord = groupID + ":" + artifactID
		}

		// Provider: current project
		contracts = append(contracts, Contract{
			ID:         util.GenerateID(repo, "lib-provider", node.ID),
			ContractID: fmt.Sprintf("lib:%s:%s", repo, coord),
			Type:       "lib",
			Role:       "provider",
			Repo:       repo,
			SymbolUID:  node.UID(),
			Confidence: 0.9,
			Meta: map[string]any{
				"package":    coord,
				"groupId":    groupID,
				"artifactId": artifactID,
				"language":   "java",
				"manifest":   base,
				"name":       node.Name,
			},
		})

		// Consumer: declared dependencies
		deps := we.extractStringListProp(node, "dependencies")
		for _, dep := range deps {
			contracts = append(contracts, Contract{
				ID:         util.GenerateID(repo, "lib-consumer", node.ID+"-"+dep),
				ContractID: fmt.Sprintf("lib:%s:%s", repo, dep),
				Type:       "lib",
				Role:       "consumer",
				Repo:       repo,
				SymbolUID:  node.UID(),
				Confidence: 0.85,
				Meta: map[string]any{
					"package":  dep,
					"language": "java",
					"manifest": base,
				},
			})
		}
	}

	return contracts
}

// extractRustWorkspace extracts Rust workspace dependencies (Cargo.toml)
func (we *WorkspaceExtractor) extractRustWorkspace(ctx context.Context, repo string, gs *graph.GraphStore, allNodes []*graph.Node) []Contract {
	var contracts []Contract

	for _, node := range allNodes {
		if filepath.Base(node.FilePath) != "Cargo.toml" {
			continue
		}

		crateName := node.GetPropString("name")
		if crateName == "" {
			crateName = node.Name
		}

		// Provider: 当前 crate
		contracts = append(contracts, Contract{
			ID:         util.GenerateID(repo, "lib-provider", node.ID),
			ContractID: fmt.Sprintf("lib:%s:%s", repo, crateName),
			Type:       "lib",
			Role:       "provider",
			Repo:       repo,
			SymbolUID:  node.UID(),
			Confidence: 0.95,
			Meta: map[string]any{
				"package":  crateName,
				"language": "rust",
				"manifest": "Cargo.toml",
				"name":     node.Name,
			},
		})

		// Consumer: dependencies declared in [dependencies]
		deps := we.extractStringListProp(node, "dependencies")
		for _, dep := range deps {
			contracts = append(contracts, Contract{
				ID:         util.GenerateID(repo, "lib-consumer", node.ID+"-"+dep),
				ContractID: fmt.Sprintf("lib:%s:%s", repo, dep),
				Type:       "lib",
				Role:       "consumer",
				Repo:       repo,
				SymbolUID:  node.UID(),
				Confidence: 0.9,
				Meta: map[string]any{
					"package":  dep,
					"language": "rust",
					"manifest": "Cargo.toml",
				},
			})
		}
	}

	return contracts
}

// extractElixirWorkspace extracts Elixir workspace dependencies (mix.exs)
func (we *WorkspaceExtractor) extractElixirWorkspace(ctx context.Context, repo string, gs *graph.GraphStore, allNodes []*graph.Node) []Contract {
	var contracts []Contract

	for _, node := range allNodes {
		if filepath.Base(node.FilePath) != "mix.exs" {
			continue
		}

		appName := node.GetPropString("app")
		if appName == "" {
			appName = node.Name
		}

		// Provider: 当前 OTP app
		contracts = append(contracts, Contract{
			ID:         util.GenerateID(repo, "lib-provider", node.ID),
			ContractID: fmt.Sprintf("lib:%s:%s", repo, appName),
			Type:       "lib",
			Role:       "provider",
			Repo:       repo,
			SymbolUID:  node.UID(),
			Confidence: 0.95,
			Meta: map[string]any{
				"package":  appName,
				"language": "elixir",
				"manifest": "mix.exs",
				"name":     node.Name,
			},
		})

		// Consumer: dependencies declared in deps
		deps := we.extractStringListProp(node, "deps")
		for _, dep := range deps {
			contracts = append(contracts, Contract{
				ID:         util.GenerateID(repo, "lib-consumer", node.ID+"-"+dep),
				ContractID: fmt.Sprintf("lib:%s:%s", repo, dep),
				Type:       "lib",
				Role:       "consumer",
				Repo:       repo,
				SymbolUID:  node.UID(),
				Confidence: 0.9,
				Meta: map[string]any{
					"package":  dep,
					"language": "elixir",
					"manifest": "mix.exs",
				},
			})
		}
	}

	return contracts
}

// ============== Helper methods ==============

// extractStringListProp extracts string list from node property
// Supports property values in []string or []any (JSON array) format
func (we *WorkspaceExtractor) extractStringListProp(node *graph.Node, prop string) []string {
	val := node.GetProp(prop, nil)
	if val == nil {
		return nil
	}

	switch v := val.(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		// Single string, split by newline
		if v == "" {
			return nil
		}
		return strings.Split(v, "\n")
	}
	return nil
}

// stripPythonVersionSpecifier removes Python dependency version specifiers
// Example: "requests>=2.0" → "requests", "flask==2.1.0" → "flask"
func (we *WorkspaceExtractor) stripPythonVersionSpecifier(dep string) string {
	for _, sep := range []string{">=", "<=", "==", "!=", "~=", ";", ">", "<", "["} {
		if idx := strings.Index(dep, sep); idx >= 0 {
			return strings.TrimSpace(dep[:idx])
		}
	}
	return strings.TrimSpace(dep)
}