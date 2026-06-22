package phases

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ParsePhase tree-sitter parsing → symbol nodes
type ParsePhase struct{}

func NewParsePhase() *ParsePhase { return &ParsePhase{} }

func (p *ParsePhase) Name() string           { return "parse" }
func (p *ParsePhase) Dependencies() []string { return []string{"structure"} }

func (p *ParsePhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	repoRoot := input.Config.RepoPath

	// Pre-compile queries for each provider (one-time cost per language)
	type providerWithQueries struct {
		provider langProvider
		cq       *compiledQueries
	}
	compiledCache := make(map[string]*providerWithQueries, len(input.Providers))
	for langName, prov := range input.Providers {
		lp, ok := prov.(langProvider)
		if !ok {
			continue // skip providers without TreeSitterLanguage
		}
		tsLang := lp.TreeSitterLanguage()
		cq, err := compileQueries(lp, tsLang)
		if err != nil {
			return nil, fmt.Errorf("compile queries for %s: %w", langName, err)
		}
		compiledCache[langName] = &providerWithQueries{provider: lp, cq: cq}
	}

	// Phase 1 (parallel): Read files and run tree-sitter parsing.
	// This is CPU-bound and benefits from parallelism.
	// No graph writes happen here — only populating f.Symbols, f.Imports, etc.
	err := pipeline.ProcessSlice(ctx, input.Files, input.Config.MaxWorkers,
		func(ctx context.Context, f *pipeline.ParsedFile) error {
			fullPath := filepath.Join(repoRoot, f.Path)

			// Read file content
			content, err := os.ReadFile(fullPath)
			if err != nil {
				return nil // skip files that cannot be read
			}
			f.Content = content

			// Provider-driven parsing (pure computation, no graph writes)
			if pq, ok := compiledCache[f.Language]; ok {
				parseWithProvider(f, pq.provider, pq.cq)
			}

			return nil
		},
	)
	if err != nil {
		return nil, err
	}

	// Phase 2 (serial): Create graph nodes and edges from parsed results.
	// Serial execution ensures deterministic results — no race conditions
	// on counters, BufferNode/BufferEdge, or GetNodesByName lookups.
	nodesAdded := 0
	edgesAdded := 0

	totalSymbols := 0
	for _, f := range input.Files {
		totalSymbols += len(f.Symbols)
	}
	slog.Info("parse phase: creating graph nodes", "repo", input.Repo, "files", len(input.Files), "total_symbols", totalSymbols)

	for _, f := range input.Files {
		// Create graph node for each symbol
		for _, sym := range f.Symbols {
			symNode := createSymbolNode(input.Repo, sym)
			if err := input.Graph.BufferNode(symNode); err == nil {
				sym.NodeID = symNode.ID
				nodesAdded++

				// Create DEFINES edge: File → Symbol
				if len(f.NodeIDs) > 0 {
					edge := graph.NewEdge(graph.RelDefines, f.NodeIDs[0], symNode.ID)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
				}
			} else {
				slog.Warn("parse: BufferNode failed", "label", sym.Label, "name", sym.Name, "error", err)
			}
		}

		// Create nodes for imports
		for _, imp := range f.Imports {
			impNode := graph.NewNode(input.Repo, graph.LabelImport, imp.Path).
				WithFile(imp.SourceFile).
				WithProp("line", imp.Line).
				WithProp("isWildcard", imp.IsWildcard).
				WithProp("alias", imp.Alias)
			if err := input.Graph.BufferNode(impNode); err == nil {
				edgesAdded++
				if len(f.NodeIDs) > 0 {
					edge := graph.NewEdge(graph.RelContains, f.NodeIDs[0], impNode.ID)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
				}
			}
		}

		// Create nodes for call sites (backward compatible)
		for _, cs := range f.CallSites {
			csNode := graph.NewNode(input.Repo, graph.LabelCallSite, cs.Name).
				WithFile(cs.FilePath).
				WithProp("line", cs.Line).
				WithProp("receiver", cs.Receiver)
			if err := input.Graph.BufferNode(csNode); err == nil {
				nodesAdded++
			}
		}
	}

	// Flush all symbol/import/call-site nodes before resolving EXTENDS/IMPLEMENTS
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	// Create EXTENDS/IMPLEMENTS/INHERITS edges (requires GetNodesByName — safe now,
	// all nodes are flushed and we are in a serial pass)
	// INHERITS is a unified edge covering both EXTENDS and IMPLEMENTS,
	// simplifying MRO traversal and inheritance queries.
	for _, f := range input.Files {
		for _, ci := range f.ClassInfos {
			if ci.NodeID == "" {
				continue
			}
			for _, parent := range ci.Parents {
				parentNodes, err := input.Graph.GetNodesByName(input.Repo, parent)
				if err == nil && len(parentNodes) > 0 {
					edge := graph.NewEdge(graph.RelExtends, ci.NodeID, parentNodes[0].ID).
						WithProp("confidence", 0.95)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
					// Also create unified INHERITS edge
					inheritEdge := graph.NewEdge(graph.RelInherits, ci.NodeID, parentNodes[0].ID).
						WithProp("confidence", 0.95).
						WithProp("kind", "extends")
					if err := input.Graph.BufferEdge(inheritEdge); err == nil {
						edgesAdded++
					}
				}
			}
			for _, iface := range ci.Implements {
				ifaceNodes, err := input.Graph.GetNodesByName(input.Repo, iface)
				if err == nil && len(ifaceNodes) > 0 {
					edge := graph.NewEdge(graph.RelImplements, ci.NodeID, ifaceNodes[0].ID).
						WithProp("confidence", 0.9)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
					// Also create unified INHERITS edge
					inheritEdge := graph.NewEdge(graph.RelInherits, ci.NodeID, ifaceNodes[0].ID).
						WithProp("confidence", 0.9).
						WithProp("kind", "implements")
					if err := input.Graph.BufferEdge(inheritEdge); err == nil {
						edgesAdded++
					}
				}
			}
		}
	}

	// Create HAS_METHOD and HAS_PROPERTY edges (Class/Struct/Interface → member)
	// After all symbol nodes are flushed, we can resolve class-to-member relationships.
	// A Method with a "receiver" prop matching a class name gets HAS_METHOD.
	// A Property node within a class's line range gets HAS_PROPERTY.
	for _, f := range input.Files {
		// Collect class-like nodes in this file (Class, Struct, Interface, Trait)
		type classInfo struct {
			nodeID    string
			name      string
			startLine int
			endLine   int
		}
		var classNodes []classInfo
		for _, sym := range f.Symbols {
			if sym.NodeID == "" {
				continue
			}
			if sym.Label == graph.LabelClass || sym.Label == graph.LabelStruct ||
				sym.Label == graph.LabelInterface || sym.Label == graph.LabelTrait {
				classNodes = append(classNodes, classInfo{
					nodeID:    sym.NodeID,
					name:      sym.Name,
					startLine: sym.StartLine,
					endLine:   sym.EndLine,
				})
			}
		}

		if len(classNodes) == 0 {
			continue
		}

		// For each Method with a receiver prop, create HAS_METHOD edge
		for _, sym := range f.Symbols {
			if sym.NodeID == "" || sym.Label != graph.LabelMethod {
				continue
			}
			if recv, ok := sym.Props["receiver"]; ok {
				recvName := fmt.Sprintf("%v", recv)
				for _, cls := range classNodes {
					if cls.name == recvName {
						edge := graph.NewEdge(graph.RelHasMethod, cls.nodeID, sym.NodeID).
							WithProp("confidence", 0.95)
						if err := input.Graph.BufferEdge(edge); err == nil {
							edgesAdded++
						}
						break
					}
				}
			}
		}

		// For each Property, create HAS_PROPERTY edge from enclosing class
		for _, sym := range f.Symbols {
			if sym.NodeID == "" || sym.Label != graph.LabelProperty {
				continue
			}
			for _, cls := range classNodes {
				// Property belongs to class if in same file and within class line range
				if sym.StartLine > 0 && cls.startLine > 0 && cls.endLine > 0 &&
					sym.StartLine >= cls.startLine && sym.StartLine <= cls.endLine {
					edge := graph.NewEdge(graph.RelHasProperty, cls.nodeID, sym.NodeID).
						WithProp("confidence", 0.95)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
					break
				}
			}
		}
	}

	// Flush heritage edges
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush heritage edges: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded:   nodesAdded,
		EdgesAdded:   edgesAdded,
		Files:        input.Files,
		FilesUpdated: true,
	}, nil
}