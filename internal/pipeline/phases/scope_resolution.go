package phases

import (
	"context"
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ScopeResolutionPhase is the scope resolution phase
// Strictly implemented per ARCHITECTURE.md §11
// Flow: finalizeScopeModel → resolveReferenceSites → emitReceiverBoundCalls → emitFreeCallFallback → emitReferencesViaLookup → emitImportEdges
type ScopeResolutionPhase struct{}

func NewScopeResolutionPhase() *ScopeResolutionPhase { return &ScopeResolutionPhase{} }

func (p *ScopeResolutionPhase) Name() string           { return "scopeResolution" }
func (p *ScopeResolutionPhase) Dependencies() []string { return []string{"crossFile"} }

func (p *ScopeResolutionPhase) Run(ctx context.Context, input *pipeline.PhaseInput) (*pipeline.PhaseOutput, error) {
	nodesAdded := 0
	edgesAdded := 0

	// Step 1: finalizeScopeModel — build scope indexes
	indexes := buildScopeResolutionIndexes(input)

	// Step 2: resolveReferenceSites — resolve reference sites
	refIndex := resolveReferenceSites(input, indexes)

	// Step 3-6: emit edges in order per contract I1
	handledSites := make(map[string]bool) // track processed call sites

	// Step 3: emitReceiverBoundCalls (FIRST — Contract Invariant I1)
	edgesAdded += emitReceiverBoundCalls(input, refIndex, handledSites)

	// Step 4: emitFreeCallFallback (THEN)
	edgesAdded += emitFreeCallFallback(input, refIndex, handledSites)

	// Step 5: emitReferencesViaLookup (LAST — consume handledSites)
	edgesAdded += emitReferencesViaLookup(input, refIndex, handledSites)

	// Step 6: emitImportEdges
	edgesAdded += emitImportEdges(input, indexes)

	// Flush any remaining buffered writes
	if err := input.Graph.FlushBuffer(); err != nil {
		return nil, fmt.Errorf("flush buffer: %w", err)
	}

	return &pipeline.PhaseOutput{
		NodesAdded: nodesAdded,
		EdgesAdded: edgesAdded,
	}, nil
}

// ============ Scope Resolution Indexes ============

// ScopeResolutionIndexes represents scope resolution indexes
type ScopeResolutionIndexes struct {
	// nameToNodes: name → []*Node (find nodes by name)
	NameToNodes map[string][]*graph.Node
	// fileToSymbols: filePath → []*Node (find symbols by file)
	FileToSymbols map[string][]*graph.Node
	// importByFile: filePath → []*ImportInfo (find imports by file)
	ImportByFile map[string][]*pipeline.ImportInfo
	// callSitesByFile: filePath → []*CallSite (find call sites by file)
	CallSitesByFile map[string][]*pipeline.CallSite
	// classByFile: filePath → []*ClassInfo
	ClassByFile map[string][]*pipeline.ClassInfo
}

// buildScopeResolutionIndexes builds scope resolution indexes
func buildScopeResolutionIndexes(input *pipeline.PhaseInput) *ScopeResolutionIndexes {
	indexes := &ScopeResolutionIndexes{
		NameToNodes:     make(map[string][]*graph.Node),
		FileToSymbols:   make(map[string][]*graph.Node),
		ImportByFile:    make(map[string][]*pipeline.ImportInfo),
		CallSitesByFile: make(map[string][]*pipeline.CallSite),
		ClassByFile:     make(map[string][]*pipeline.ClassInfo),
	}

	// Build name indexes from GraphStore
	iter := input.Graph.IterNodes(input.Repo)
	defer iter.Close()
	for iter.Next() {
		node := iter.Node()
		if node.Label.IsSymbol() || node.Label == graph.LabelImport {
			indexes.NameToNodes[node.Name] = append(indexes.NameToNodes[node.Name], node)
			if node.FilePath != "" {
				indexes.FileToSymbols[node.FilePath] = append(indexes.FileToSymbols[node.FilePath], node)
			}
		}
	}

	// Build call site and class indexes from ParsedFile
	for _, f := range input.Files {
		indexes.CallSitesByFile[f.Path] = f.CallSites
		indexes.ClassByFile[f.Path] = f.ClassInfos
		for _, imp := range f.Imports {
			indexes.ImportByFile[f.Path] = append(indexes.ImportByFile[f.Path], imp)
		}
	}

	return indexes
}

// ============ Reference Resolution ============

// ReferenceIndex represents reference index
type ReferenceIndex struct {
	// receiverBoundCalls: receiver → method → []*CallSite (method calls with receiver)
	receiverBoundCalls map[string]map[string][]*pipeline.CallSite
	// freeCalls: name → []*CallSite (free calls without receiver)
	freeCalls map[string][]*pipeline.CallSite
	// importPaths: path → []*Node (import path → nodes)
	importPaths map[string][]*graph.Node
}

// resolveReferenceSites resolves reference sites
func resolveReferenceSites(input *pipeline.PhaseInput, indexes *ScopeResolutionIndexes) *ReferenceIndex {
	ref := &ReferenceIndex{
		receiverBoundCalls: make(map[string]map[string][]*pipeline.CallSite),
		freeCalls:          make(map[string][]*pipeline.CallSite),
		importPaths:        make(map[string][]*graph.Node),
	}

	// Build receiver-bound call indexes
	for _, f := range input.Files {
		for _, cs := range f.CallSites {
			if cs.Receiver != "" {
				if ref.receiverBoundCalls[cs.Receiver] == nil {
					ref.receiverBoundCalls[cs.Receiver] = make(map[string][]*pipeline.CallSite)
				}
				ref.receiverBoundCalls[cs.Receiver][cs.Name] = append(
					ref.receiverBoundCalls[cs.Receiver][cs.Name], cs)
			} else {
				ref.freeCalls[cs.Name] = append(ref.freeCalls[cs.Name], cs)
			}
		}
	}

	// Build import path indexes
	for _, nodes := range indexes.NameToNodes {
		for _, n := range nodes {
			if n.Label == graph.LabelImport {
				// Import node name is the path
				ref.importPaths[n.Name] = append(ref.importPaths[n.Name], n)
			}
		}
	}

	return ref
}

// ============ Emit Phases (strictly in I1 contract order) ============

// emitReceiverBoundCalls emits receiver-bound call edges (FIRST)
// For each method call with explicit receiver, find the corresponding method in receiver type, create CALLS edge
func emitReceiverBoundCalls(input *pipeline.PhaseInput, ref *ReferenceIndex, handledSites map[string]bool) int {
	edgesAdded := 0

	for receiver, methods := range ref.receiverBoundCalls {
		// Find receiver type node
		receiverNodes := findNodesByName(input, receiver)

		for _, recvNode := range receiverNodes {
			// Receiver must be class/struct/interface type
			if !isClassLikeLabel(recvNode.Label) {
				continue
			}

			for methodName, calls := range methods {
				// Find method in receiver type
				methodNodes := findMethodOfClass(input, methodName, recvNode)

				for _, call := range calls {
					// Mark as processed
					siteKey := callSiteKey(call)
					handledSites[siteKey] = true

					// Create CALLS edge for call site
					for _, methodNode := range methodNodes {
						edge := graph.NewEdge(graph.RelCalls, recvNode.ID, methodNode.ID).
							WithProp("confidence", 0.95).
							WithProp("line", call.Line).
							WithProp("file", call.FilePath)
						if err := input.Graph.BufferEdge(edge); err == nil {
							edgesAdded++
						}
					}
				}
			}
		}
	}

	return edgesAdded
}

// emitFreeCallFallback emits free call fallback edges (THEN)
// For unprocessed free calls (without receiver), try to find target in current file scope
func emitFreeCallFallback(input *pipeline.PhaseInput, ref *ReferenceIndex, handledSites map[string]bool) int {
	edgesAdded := 0

	for name, calls := range ref.freeCalls {
		// Skip processed ones
		for _, call := range calls {
			siteKey := callSiteKey(call)
			if handledSites[siteKey] {
				continue
			}

			// Find target function by name globally
			targetNodes := findNodesByName(input, name)
			for _, targetNode := range targetNodes {
				// Only match function/method
				if targetNode.Label != graph.LabelFunction && targetNode.Label != graph.LabelMethod {
					continue
				}

				// Same file priority (Tier1 confidence 0.95)
				confidence := 0.5 // Tier3 global default
				if targetNode.FilePath == call.FilePath {
					confidence = 0.95
				} else if isSamePackage(input, call.FilePath, targetNode.FilePath) {
					confidence = 0.9 // Tier2 same package
				}

				// Find caller node (enclosing function/method)
				callerNodes := findEnclosingFunction(input, call)
				for _, callerNode := range callerNodes {
					edge := graph.NewEdge(graph.RelCalls, callerNode.ID, targetNode.ID).
						WithProp("confidence", confidence).
						WithProp("line", call.Line)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
				}

				handledSites[siteKey] = true
			}
		}
	}

	return edgesAdded
}

// emitReferencesViaLookup emits generic reference edges (LAST — consume handledSites)
// For still unprocessed references, create ACCESSES edges via name lookup
func emitReferencesViaLookup(input *pipeline.PhaseInput, ref *ReferenceIndex, handledSites map[string]bool) int {
	edgesAdded := 0

	// Process remaining free calls
	for name, calls := range ref.freeCalls {
		for _, call := range calls {
			siteKey := callSiteKey(call)
			if handledSites[siteKey] {
				continue
			}

			// Try fuzzy matching — find any symbol node with same name
			targetNodes := findNodesByName(input, name)
			for _, targetNode := range targetNodes {
				if !targetNode.Label.IsSymbol() {
					continue
				}

				callerNodes := findEnclosingFunction(input, call)
				for _, callerNode := range callerNodes {
					edge := graph.NewEdge(graph.RelAccesses, callerNode.ID, targetNode.ID).
						WithProp("confidence", 0.5).
						WithProp("line", call.Line)
					if err := input.Graph.BufferEdge(edge); err == nil {
						edgesAdded++
					}
				}

				handledSites[siteKey] = true
			}
		}
	}

	return edgesAdded
}

// emitImportEdges emits import edges
// Create IMPORTS edges between importing file and imported symbols
func emitImportEdges(input *pipeline.PhaseInput, indexes *ScopeResolutionIndexes) int {
	edgesAdded := 0

	for _, f := range input.Files {
		for _, imp := range f.Imports {
			// Find nodes in import source file
			targetNodes := resolveImportToNodes(input, imp, indexes)
			if len(targetNodes) == 0 {
				continue
			}

			// Find current file node
			fileNodes, err := input.Graph.GetNodesByFile(input.Repo, f.Path)
			if err != nil || len(fileNodes) == 0 {
				continue
			}

			var fileNode *graph.Node
			for _, n := range fileNodes {
				if n.Label == graph.LabelFile {
					fileNode = n
					break
				}
			}
			if fileNode == nil {
				continue
			}

			// Create IMPORTS edge for each imported symbol
			for _, target := range targetNodes {
				edge := graph.NewEdge(graph.RelImports, fileNode.ID, target.ID).
					WithProp("confidence", 0.9).
					WithProp("importPath", imp.Path)
				if err := input.Graph.BufferEdge(edge); err == nil {
					edgesAdded++
				}
			}
		}
	}

	return edgesAdded
}

// ============ Helper Functions ============

// callSiteKey generates unique key for call site
func callSiteKey(cs *pipeline.CallSite) string {
	return fmt.Sprintf("%s:%s:%d", cs.FilePath, cs.Name, cs.Line)
}

// findNodesByName finds nodes by name
func findNodesByName(input *pipeline.PhaseInput, name string) []*graph.Node {
	nodes, err := input.Graph.GetNodesByName(input.Repo, name)
	if err != nil {
		return nil
	}
	return nodes
}

// findMethodOfClass finds method of class/struct
func findMethodOfClass(input *pipeline.PhaseInput, methodName string, classNode *graph.Node) []*graph.Node {
	var result []*graph.Node

	// Find all outgoing edges of this class (HAS_METHOD)
	outEdges, err := input.Graph.GetAllOutEdges(classNode.ID)
	if err != nil {
		return nil
	}

	for _, edge := range outEdges {
		if edge.Type == graph.RelHasMethod || edge.Type == graph.RelContains || edge.Type == graph.RelDefines {
			target, err := input.Graph.GetNode(edge.Target)
			if err != nil {
				continue
			}
			if target.Name == methodName && (target.Label == graph.LabelMethod || target.Label == graph.LabelFunction) {
				result = append(result, target)
			}
		}
	}

	// If not found via edges, fall back to global name lookup
	if len(result) == 0 {
		allMethods := findNodesByName(input, methodName)
		for _, m := range allMethods {
			if m.Label == graph.LabelMethod {
				// Check if belongs to this class (via receiver in Props)
				if recv, ok := m.Props.GetProp("receiver"); ok {
					if fmt.Sprintf("%v", recv) == classNode.Name {
						result = append(result, m)
					}
				}
			}
		}
	}

	return result
}

// findEnclosingFunction finds the function/method containing the call site
func findEnclosingFunction(input *pipeline.PhaseInput, call *pipeline.CallSite) []*graph.Node {
	var result []*graph.Node

	// Find function/method containing this line in same file's symbols
	symbols, err := input.Graph.GetNodesByFile(input.Repo, call.FilePath)
	if err != nil {
		return nil
	}

	for _, sym := range symbols {
		if sym.Label != graph.LabelFunction && sym.Label != graph.LabelMethod {
			continue
		}

		startLine := 0
		endLine := 0
		if v, ok := sym.Props.GetProp("startLine"); ok {
			startLine = toInt(v)
		}
		if v, ok := sym.Props.GetProp("endLine"); ok {
			endLine = toInt(v)
		}

		if startLine > 0 && endLine > 0 && call.Line >= startLine && call.Line <= endLine {
			result = append(result, sym)
		}
	}

	return result
}

// resolveImportToNodes resolves import info to target nodes
func resolveImportToNodes(input *pipeline.PhaseInput, imp *pipeline.ImportInfo, indexes *ScopeResolutionIndexes) []*graph.Node {
	var result []*graph.Node

	// Go wildcard import: import path corresponds to package directory
	// Try to find symbols in files matching this path
	for filePath, symbols := range indexes.FileToSymbols {
		// Check if file path contains import path
		if strings.Contains(filePath, imp.Path) {
			for _, sym := range symbols {
				if sym.Label.IsSymbol() {
					result = append(result, sym)
				}
			}
		}
	}

	// Find by name directly (handle named imports)
	if !imp.IsWildcard && len(imp.Symbols) > 0 {
		for _, symName := range imp.Symbols {
			nodes := findNodesByName(input, symName)
			result = append(result, nodes...)
		}
	}

	return result
}

// isClassLikeLabel checks if label is class/struct/interface
func isClassLikeLabel(label graph.Label) bool {
	switch label {
	case graph.LabelClass, graph.LabelStruct, graph.LabelInterface, graph.LabelTrait:
		return true
	}
	return false
}

// isSamePackage checks if two files are in same package
func isSamePackage(input *pipeline.PhaseInput, file1, file2 string) bool {
	// Simplified implementation: compare directory paths
	dir1 := file1
	dir2 := file2
	if idx := strings.LastIndex(file1, "/"); idx >= 0 {
		dir1 = file1[:idx]
	}
	if idx := strings.LastIndex(file2, "/"); idx >= 0 {
		dir2 = file2[:idx]
	}
	return dir1 == dir2
}

// toInt converts any to int
func toInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	default:
		return 0
	}
}
