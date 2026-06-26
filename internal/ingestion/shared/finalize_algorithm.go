// Package shared — finalize algorithm for cross-file import resolution.
// Ported from gitnexus-shared scope-resolution/finalize-algorithm.ts.
//
// Pure logic that takes per-file parse output (ParsedImport[] + SymbolDefinition[])
// and returns:
//   - Linked ImportEdge[] per module scope, with targetModuleScope and targetDefId
//     filled where resolvable; edges that could not be resolved within the hard
//     fixpoint cap are marked linkStatus: 'unresolved'.
//   - Materialized bindings per module scope — local defs merged with imported /
//     wildcard-expanded / re-exported names via the provider's mergeBindings.
//   - The SCC condensation of the import graph, exposed so disjoint SCCs can be
//     processed in parallel.
//
// The algorithm is SCC-aware: Tarjan SCC over the file-level import graph,
// reverse-topological order (leaves first), bounded fixpoint inside each SCC.
// No language-specific logic — all via caller-supplied FinalizeHooks.
package shared

import "sort"

// ─── Public contracts ────────────────────────────────────────────────

// FinalizeFile is per-file input for the finalize pass.
type FinalizeFile struct {
	FilePath     string
	ModuleScope  ScopeID
	ParsedImports []ParsedImport
	// Defs exported from this file — the "what other files can import by name" surface.
	LocalDefs []SymbolDefinition
}

// FinalizeInput is the full input to the finalize function.
type FinalizeInput struct {
	Files           []FinalizeFile
	WorkspaceIndex  WorkspaceIndex
}

// WorkspaceIndex — opaque workspace context forwarded to provider hooks.
// Defined as an interface so callers can inject any implementation.
type WorkspaceIndex interface{}

// FinalizeHooks — provider-supplied hooks. Mirror LanguageProvider scope-resolution hooks;
// finalize calls them purely and expects pure answers.
type FinalizeHooks interface {
	// ResolveImportTarget resolves a raw import target to the concrete file path
	// that owns it. Returns nil when no target file is resolvable.
	// May return multiple file paths (e.g. Go package-scoped imports fan out).
	ResolveImportTarget(targetRaw string, fromFile string, workspace WorkspaceIndex) []string

	// ExpandsWildcardTo returns the names visible in the exporting module scope
	// for a wildcard import * from M.
	ExpandsWildcardTo(targetModuleScope ScopeID, workspace WorkspaceIndex) []string

	// MergeBindings merges incoming bindings into existing for a given name.
	// Return value replaces the bucket entirely — no implicit append.
	MergeBindings(existing []BindingRef, incoming []BindingRef, scope ScopeID) []BindingRef
}

// FinalizedSccExt — one SCC in the file-level import graph (extended version).
type FinalizedSccExt struct {
	Files    []string
	IsCycle  bool // true if SCC has ≥2 files OR a single file that self-imports
}

// FinalizeStatsExt — counters reported by finalize (extended version).
type FinalizeStatsExt struct {
	TotalFiles      int
	TotalEdges      int // total ImportEdgeDraft records generated (≥ ParsedImport count)
	LinkedEdges     int // edges whose finalized edge does NOT carry linkStatus='unresolved'
	UnresolvedEdges int // edges whose finalized edge carries linkStatus='unresolved'
	SccCount        int
	LargestSccSize  int
}

// FinalizeOutput — the result of the finalize pass.
type FinalizeOutput struct {
	Imports  map[ScopeID][]ImportEdge                    // linked ImportEdge[] per module scope
	Bindings map[ScopeID]map[string][]BindingRef         // materialized bindings per module scope
	Sccs     []FinalizedSccExt                           // SCCs in reverse-topological order (leaves first)
	Stats    FinalizeStatsExt
}

// ─── Entry point ─────────────────────────────────────────────────────

// Finalize runs the cross-file finalize algorithm.
func Finalize(input FinalizeInput, hooks FinalizeHooks) FinalizeOutput {
	byFilePath := make(map[string]*FinalizeFile)
	for i := range input.Files {
		byFilePath[input.Files[i].FilePath] = &input.Files[i]
	}

	// ── Phase 0: pre-resolve raw import targets.
	edgeIndex := make(map[string][]importEdgeDraft)
	var totalEdges int

	for i := range input.Files {
		file := &input.Files[i]
		var drafts []importEdgeDraft
		for j := range file.ParsedImports {
			parsed := &file.ParsedImports[j]
			draftArr := makeEdgeDrafts(parsed, file, hooks, input.WorkspaceIndex)
			drafts = append(drafts, draftArr...)
			totalEdges += len(draftArr)
		}
		edgeIndex[file.FilePath] = drafts
	}

	// ── Phase 1: build file-level import graph.
	graph := make(map[string]map[string]bool)
	for i := range input.Files {
		graph[input.Files[i].FilePath] = make(map[string]bool)
	}
	for fromFile, drafts := range edgeIndex {
		edges := graph[fromFile]
		for _, d := range drafts {
			if d.targetFile != "" && byFilePath[d.targetFile] != nil {
				edges[d.targetFile] = true
			}
		}
	}

	// ── Phase 2: Tarjan SCC → reverse-topological list.
	sccs := tarjanSccs(graph)

	// ── Phase 2.5: precompute per-file re-export closure.
	reexportClosures := buildReexportClosures(input.Files, byFilePath, edgeIndex)

	// ── Phase 3: process SCCs in reverse-topological order.
	linkedByScope := make(map[ScopeID][]ImportEdge)
	var linkedEdges int

	for _, scc := range sccs {
		sccFiles := make(map[string]bool)
		for _, f := range scc.Files {
			sccFiles[f] = true
		}
		capacity := countEdgesWithin(edgeIndex, sccFiles)

		progressed := true
		iterations := 0
		for progressed && iterations < capacity {
			progressed = false
			iterations++
			for _, filePath := range scc.Files {
				drafts := edgeIndex[filePath]
				for i := range drafts {
					draft := &drafts[i]
					if draft.finalized != nil {
						continue
					}
					finalized := tryFinalize(draft, byFilePath, reexportClosures)
					if finalized != nil {
						draft.finalized = finalized
						progressed = true
					}
				}
				// Write back modified drafts (pointer fields changed).
				edgeIndex[filePath] = drafts
			}
		}

		// Any drafts still not finalized → mark unresolved.
		for _, filePath := range scc.Files {
			drafts := edgeIndex[filePath]
			for i := range drafts {
				if drafts[i].finalized == nil {
					unresolved := "unresolved"
					drafts[i].finalized = &drafts[i].base
					drafts[i].finalized.LinkStatus = &unresolved
				}
			}
			edgeIndex[filePath] = drafts
		}
	}

	// ── Phase 4: collect finalized ImportEdge[] per module scope.
	for i := range input.Files {
		file := &input.Files[i]
		drafts := edgeIndex[file.FilePath]
		var finalized []ImportEdge
		for _, d := range drafts {
			edge := d.finalized
			if edge == nil {
				panic("Invariant violated: import edge was not finalized for " + file.FilePath)
			}
			if d.source.Kind == ParsedImportWildcard && *edge.LinkStatus != "unresolved" {
				expanded := expandWildcard(edge, byFilePath, hooks, input.WorkspaceIndex)
				finalized = append(finalized, expanded...)
			} else {
				finalized = append(finalized, *edge)
			}
			if edge.LinkStatus == nil || *edge.LinkStatus != "unresolved" {
				linkedEdges++
			}
		}
		linkedByScope[file.ModuleScope] = finalized
	}

	// ── Phase 5: materialize bindings.
	bindingsByScope := materializeBindings(input.Files, linkedByScope, hooks)

	// ── Stats.
	var largestSccSize int
	for _, scc := range sccs {
		if len(scc.Files) > largestSccSize {
			largestSccSize = len(scc.Files)
		}
	}

	stats := FinalizeStatsExt{
		TotalFiles:      len(input.Files),
		TotalEdges:      totalEdges,
		LinkedEdges:     linkedEdges,
		UnresolvedEdges: totalEdges - linkedEdges,
		SccCount:        len(sccs),
		LargestSccSize:  largestSccSize,
	}

	return FinalizeOutput{
		Imports:  linkedByScope,
		Bindings: bindingsByScope,
		Sccs:     sccs,
		Stats:    stats,
	}
}

// ─── Internal: edge drafting (phase 0) ───────────────────────────────

type importEdgeDraft struct {
	source    ParsedImport
	fromFile  string
	fromScope ScopeID
	targetFile string // "" means nil (no target)
	base      ImportEdge
	finalized *ImportEdge // nil until finalized
}

func makeEdgeDrafts(
	parsed *ParsedImport,
	file *FinalizeFile,
	hooks FinalizeHooks,
	workspace WorkspaceIndex,
) []importEdgeDraft {
	// Dynamic-unresolved passes through.
	if parsed.Kind == ParsedImportDynamicUnresolved {
		base := ImportEdge{
			LocalName:          parsed.LocalName,
			TargetFile:         nil,
			TargetExportedName: "",
			Kind:               ImportEdgeDynamicUnresolved,
		}
		return []importEdgeDraft{
			{
				source:     *parsed,
				fromFile:   file.FilePath,
				fromScope:  file.ModuleScope,
				targetFile: "",
				base:       base,
				finalized:  &base, // already fully finalized
			},
		}
	}

	targetRaw := ""
	if parsed.TargetRaw != nil {
		targetRaw = *parsed.TargetRaw
	}
	targetFiles := hooks.ResolveImportTarget(targetRaw, file.FilePath, workspace)

	// Edge is unresolvable at the file level.
	if len(targetFiles) == 0 {
		unresolved := "unresolved"
		base := ImportEdge{
			LocalName:          extractLocalName(parsed),
			TargetFile:         nil,
			TargetExportedName: extractExportedName(parsed),
			Kind:               edgeKindFor(parsed),
			LinkStatus:         &unresolved,
		}
		return []importEdgeDraft{
			{
				source:     *parsed,
				fromFile:   file.FilePath,
				fromScope:  file.ModuleScope,
				targetFile: "",
				base:       base,
				finalized:  &base,
			},
		}
	}

	// Resolvable at the file level.
	isFileLevelTerminal := parsed.Kind == ParsedImportSideEffect || parsed.Kind == ParsedImportDynamicResolved
	var drafts []importEdgeDraft
	for _, tf := range targetFiles {
		localName := extractLocalName(parsed)
		exportedName := extractExportedName(parsed)
		targetFilePtr := tf
		base := ImportEdge{
			LocalName:          localName,
			TargetFile:         &targetFilePtr,
			TargetExportedName: exportedName,
			Kind:               edgeKindFor(parsed),
		}
		var fin *ImportEdge
		if isFileLevelTerminal {
			fin = &base
		}
		drafts = append(drafts, importEdgeDraft{
			source:     *parsed,
			fromFile:   file.FilePath,
			fromScope:  file.ModuleScope,
			targetFile: tf,
			base:       base,
			finalized:  fin,
		})
	}
	return drafts
}

func edgeKindFor(parsed *ParsedImport) ImportEdgeKind {
	if parsed.Kind == ParsedImportWildcard {
		return ImportEdgeWildcardExpanded
	}
	return ImportEdgeKind(parsed.Kind)
}

func extractLocalName(parsed *ParsedImport) string {
	switch parsed.Kind {
	case ParsedImportWildcard, ParsedImportSideEffect, ParsedImportDynamicResolved:
		return ""
	default:
		return parsed.LocalName
	}
}

func extractExportedName(parsed *ParsedImport) string {
	switch parsed.Kind {
	case ParsedImportNamed, ParsedImportAlias, ParsedImportNamespace, ParsedImportReexport:
		return parsed.ImportedName
	case ParsedImportWildcard, ParsedImportDynamicUnresolved, ParsedImportDynamicResolved, ParsedImportSideEffect:
		return ""
	}
	return ""
}

// ─── Internal: per-edge finalization (phase 3) ──────────────────────

func tryFinalize(
	draft *importEdgeDraft,
	byFilePath map[string]*FinalizeFile,
	reexportClosures map[string]map[string]reexportClosureEntry,
) *ImportEdge {
	if draft.targetFile == "" {
		return &draft.base // already terminal
	}
	targetModule := byFilePath[draft.targetFile]
	if targetModule == nil {
		return &draft.base // external target — leave as-is
	}

	// Wildcards finalize at the file level.
	if draft.source.Kind == ParsedImportWildcard {
		edge := draft.base
		edge.TargetModuleScope = &targetModule.ModuleScope
		return &edge
	}

	// Namespace imports alias the target module.
	if draft.source.Kind == ParsedImportNamespace {
		importedName := extractExportedName(&draft.source)
		moduleDef := findExportByName(targetModule.LocalDefs, importedName)
		edge := draft.base
		edge.TargetModuleScope = &targetModule.ModuleScope
		if moduleDef != nil {
			edge.TargetDefID = &moduleDef.NodeID
		}
		return &edge
	}

	// named / alias / reexport: look up imported name.
	importedName := extractExportedName(&draft.source)
	exported := findExportByName(targetModule.LocalDefs, importedName)

	if exported != nil {
		edge := draft.base
		edge.TargetModuleScope = &targetModule.ModuleScope
		edge.TargetDefID = &exported.NodeID
		if draft.source.Kind == ParsedImportReexport {
			edge.TransitiveVia = []string{draft.targetFile}
		}
		return &edge
	}

	// Multi-hop re-export follow via precomputed closure.
	followed := lookupReexportedName(reexportClosures, draft.targetFile, importedName)
	if followed == nil {
		return nil // keep trying in later iteration
	}

	viaFiles := []string{draft.targetFile}
	viaFiles = append(viaFiles, followed.via...)

	var transitiveVia []string
	if draft.source.Kind == ParsedImportReexport || len(viaFiles) > 1 {
		transitiveVia = viaFiles
	}

	edge := draft.base
	edge.TargetModuleScope = &targetModule.ModuleScope
	edge.TargetDefID = &followed.def.NodeID
	if transitiveVia != nil {
		edge.TransitiveVia = transitiveVia
	}
	return &edge
}

// ─── Internal: re-export closure (phase 2.5) ────────────────────────

type reexportClosureEntry struct {
	def SymbolDefinition
	via []string
}

func buildReexportClosures(
	files []FinalizeFile,
	byFilePath map[string]*FinalizeFile,
	edgeIndex map[string][]importEdgeDraft,
) map[string]map[string]reexportClosureEntry {
	closures := make(map[string]map[string]reexportClosureEntry)
	for i := range files {
		closures[files[i].FilePath] = make(map[string]reexportClosureEntry)
	}

	// Step 1: build re-export sub-graph.
	subGraph := make(map[string]map[string]bool)
	for i := range files {
		targets := make(map[string]bool)
		drafts := edgeIndex[files[i].FilePath]
		for _, d := range drafts {
			if d.source.Kind != ParsedImportReexport && d.source.Kind != ParsedImportWildcard {
				continue
			}
			if d.targetFile == "" || byFilePath[d.targetFile] == nil {
				continue
			}
			targets[d.targetFile] = true
		}
		subGraph[files[i].FilePath] = targets
	}

	// Step 2: SCC over sub-graph.
	subSccs := tarjanSccs(subGraph)

	// Step 3: process SCCs.
	for _, scc := range subSccs {
		if !scc.IsCycle {
			filePath := scc.Files[0]
			populateFileClosure(filePath, byFilePath, edgeIndex, closures)
			continue
		}
		cap := len(scc.Files) + 1
		progressed := true
		iter := 0
		for progressed && iter < cap {
			progressed = false
			iter++
			for _, filePath := range scc.Files {
				if populateFileClosure(filePath, byFilePath, edgeIndex, closures) {
					progressed = true
				}
			}
		}
	}

	return closures
}

func populateFileClosure(
	filePath string,
	byFilePath map[string]*FinalizeFile,
	edgeIndex map[string][]importEdgeDraft,
	closures map[string]map[string]reexportClosureEntry,
) bool {
	myClosure := closures[filePath]
	if myClosure == nil {
		return false
	}
	before := len(myClosure)
	drafts := edgeIndex[filePath]
	if drafts == nil {
		return false
	}

	// Named re-exports — precedence over wildcards.
	for _, draft := range drafts {
		if draft.source.Kind != ParsedImportReexport {
			continue
		}
		targetFile := draft.targetFile
		if targetFile == "" {
			continue
		}
		targetModule := byFilePath[targetFile]
		if targetModule == nil {
			continue
		}

		localName := draft.source.LocalName
		if _, exists := myClosure[localName]; exists {
			continue
		}

		importedName := draft.source.ImportedName
		direct := findExportByName(targetModule.LocalDefs, importedName)
		if direct != nil {
			myClosure[localName] = reexportClosureEntry{def: *direct, via: []string{targetFile}}
			continue
		}
		targetClosure := closures[targetFile]
		if targetClosure != nil {
			inherited, ok := targetClosure[importedName]
			if ok {
				via := []string{targetFile}
				via = append(via, inherited.via...)
				myClosure[localName] = reexportClosureEntry{def: inherited.def, via: via}
			}
		}
	}

	// Wildcard re-exports.
	for _, draft := range drafts {
		if draft.source.Kind != ParsedImportWildcard {
			continue
		}
		targetFile := draft.targetFile
		if targetFile == "" {
			continue
		}
		targetModule := byFilePath[targetFile]
		if targetModule == nil {
			continue
		}

		for _, def := range targetModule.LocalDefs {
			name := deriveSimpleName(def)
			if name == "" {
				continue
			}
			if _, exists := myClosure[name]; exists {
				continue
			}
			myClosure[name] = reexportClosureEntry{def: def, via: []string{targetFile}}
		}
		targetClosure := closures[targetFile]
		if targetClosure != nil {
			for name, entry := range targetClosure {
				if _, exists := myClosure[name]; exists {
					continue
				}
				via := []string{targetFile}
				via = append(via, entry.via...)
				myClosure[name] = reexportClosureEntry{def: entry.def, via: via}
			}
		}
	}

	return len(myClosure) > before
}

func lookupReexportedName(
	closures map[string]map[string]reexportClosureEntry,
	filePath string,
	name string,
) *reexportClosureEntry {
	closure := closures[filePath]
	if closure == nil {
		return nil
	}
	entry, ok := closure[name]
	if !ok {
		return nil
	}
	return &entry
}

func deriveSimpleName(def SymbolDefinition) string {
	q := def.QualifiedName
	if q == nil || *q == "" {
		return ""
	}
	dot := lastIndex(*q, '.')
	if dot == -1 {
		return *q
	}
	return (*q)[dot+1:]
}

func lastIndex(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}
	return -1
}

func findExportByName(defs []SymbolDefinition, name string) *SymbolDefinition {
	var fallback *SymbolDefinition
	for i := range defs {
		d := &defs[i]
		if deriveSimpleName(*d) != name {
			continue
		}
		if isCallableOrTypeLike(d.Type) {
			return d
		}
		if fallback == nil {
			fallback = d
		}
	}
	return fallback
}

var callableOrTypeLikeLabels = map[NodeLabel]bool{
	LabelFunction:    true,
	LabelMethod:      true,
	LabelConstructor: true,
	LabelClass:       true,
	LabelInterface:   true,
	LabelEnum:        true,
	LabelStruct:      true,
	LabelRecord:      true,
	LabelTrait:       true,
	LabelNamespace:   true,
	LabelModule:      true,
	LabelTypeAlias:   true,
	LabelType:        true,
	LabelTypedef:     true,
}

func isCallableOrTypeLike(t NodeLabel) bool {
	return callableOrTypeLikeLabels[t]
}

func countEdgesWithin(edgeIndex map[string][]importEdgeDraft, files map[string]bool) int {
	n := 0
	for filePath := range files {
		drafts := edgeIndex[filePath]
		for _, d := range drafts {
			if d.targetFile != "" && files[d.targetFile] {
				n++
			}
		}
	}
	if n < 1 {
		n = 1 // guarantee at least one pass
	}
	return n
}

// ─── Internal: wildcard expansion (phase 4) ─────────────────────────

func expandWildcard(
	edge *ImportEdge,
	byFilePath map[string]*FinalizeFile,
	hooks FinalizeHooks,
	workspace WorkspaceIndex,
) []ImportEdge {
	if edge.TargetModuleScope == nil || edge.TargetFile == nil {
		return []ImportEdge{*edge}
	}
	target := byFilePath[*edge.TargetFile]
	if target == nil {
		return []ImportEdge{*edge}
	}

	names := hooks.ExpandsWildcardTo(*edge.TargetModuleScope, workspace)
	if len(names) == 0 {
		return []ImportEdge{*edge}
	}

	var expanded []ImportEdge
	for _, name := range names {
		def := findExportByName(target.LocalDefs, name)
		if def == nil {
			continue
		}
		expanded = append(expanded, ImportEdge{
			LocalName:          name,
			TargetFile:         edge.TargetFile,
			TargetExportedName: name,
			Kind:               ImportEdgeWildcardExpanded,
			TargetModuleScope:  edge.TargetModuleScope,
			TargetDefID:        &def.NodeID,
		})
	}
	return expanded
}

// ─── Internal: bindings materialization (phase 5) ───────────────────

func materializeBindings(
	files []FinalizeFile,
	linkedByScope map[ScopeID][]ImportEdge,
	hooks FinalizeHooks,
) map[ScopeID]map[string][]BindingRef {
	out := make(map[ScopeID]map[string][]BindingRef)

	// Build nodeId → SymbolDefinition index.
	defById := make(map[string]*SymbolDefinition)
	for i := range files {
		for j := range files[i].LocalDefs {
			d := &files[i].LocalDefs[j]
			defById[d.NodeID] = d
		}
	}

	for i := range files {
		file := &files[i]
		scopeBindings := make(map[string][]BindingRef)

		// Start with local defs.
		for j := range file.LocalDefs {
			def := &file.LocalDefs[j]
			name := deriveSimpleName(*def)
			if name == "" {
				continue
			}
			incoming := []BindingRef{{Def: *def, Origin: OriginLocal}}
			existing := scopeBindings[name]
			scopeBindings[name] = hooks.MergeBindings(existing, incoming, file.ModuleScope)
		}

		// Layer in finalized imports.
		imports := linkedByScope[file.ModuleScope]
		for _, edge := range imports {
			if edge.TargetDefID == nil || (edge.LinkStatus != nil && *edge.LinkStatus == "unresolved") {
				continue
			}
			def := defById[*edge.TargetDefID]
			if def == nil {
				continue
			}

			origin := OriginImport
			switch edge.Kind {
			case ImportEdgeNamespace:
				origin = OriginNamespace
			case ImportEdgeWildcardExpanded:
				origin = OriginWildcard
			case ImportEdgeReexport:
				origin = OriginReexport
			}

			fallback := deriveSimpleName(*def)
			name := edge.LocalName
			if name == "" {
				name = fallback
			}
			if name == "" {
				continue
			}
			incoming := []BindingRef{{Def: *def, Origin: origin, Via: &edge}}
			existing := scopeBindings[name]
			scopeBindings[name] = hooks.MergeBindings(existing, incoming, file.ModuleScope)
		}

		out[file.ModuleScope] = scopeBindings
	}

	return out
}

// ─── Internal: Tarjan SCC ───────────────────────────────────────────

// Iterative Tarjan SCC. Returns SCCs in reverse-topological order (leaves first).
func tarjanSccs(graph map[string]map[string]bool) []FinalizedSccExt {
	indexMap := make(map[string]int)
	lowlinkMap := make(map[string]int)
	onStack := make(map[string]bool)
	var stack []string
	var sccs []FinalizedSccExt
	idx := 0

	// Deterministic order: sort node keys.
	var allNodes []string
	for node := range graph {
		allNodes = append(allNodes, node)
	}
	sort.Strings(allNodes)

	// Iterative DFS.
	type frame struct {
		node    string
		children []string
		childIdx int
		entered  bool
	}
	var iterStack []frame

	for _, root := range allNodes {
		if _, visited := indexMap[root]; visited {
			continue
		}

		// Collect children of root in sorted order.
		childSet := graph[root]
		var children []string
		for c := range childSet {
			children = append(children, c)
		}
		sort.Strings(children)

		iterStack = append(iterStack, frame{node: root, children: children, entered: false})

		for len(iterStack) > 0 {
			fr := &iterStack[len(iterStack)-1]

			if !fr.entered {
				fr.entered = true
				indexMap[fr.node] = idx
				lowlinkMap[fr.node] = idx
				idx++
				stack = append(stack, fr.node)
				onStack[fr.node] = true
			}

			// Process next child.
			if fr.childIdx < len(fr.children) {
				child := fr.children[fr.childIdx]
				fr.childIdx++

				if _, hasIdx := indexMap[child]; !hasIdx {
					childSet := graph[child]
					var childChildren []string
					for c := range childSet {
						childChildren = append(childChildren, c)
					}
					sort.Strings(childChildren)

					iterStack = append(iterStack, frame{node: child, children: childChildren, entered: false})
				} else if onStack[child] {
					lowlinkMap[fr.node] = minInt(lowlinkMap[fr.node], indexMap[child])
				}
				continue
			}

			// Post-visit: compute SCC if fr.node is a root.
			if lowlinkMap[fr.node] == indexMap[fr.node] {
				var sccFiles []string
				var selfInCycle bool
				for {
					w := stack[len(stack)-1]
					stack = stack[:len(stack)-1]
					onStack[w] = false
					sccFiles = append(sccFiles, w)
					if w == fr.node {
						selfInCycle = graph[w] != nil && graph[w][w]
						break
					}
				}
				isCycle := len(sccFiles) > 1 || selfInCycle
				sccs = append(sccs, FinalizedSccExt{Files: sccFiles, IsCycle: isCycle})
			}

			iterStack = iterStack[:len(iterStack)-1]
			// Propagate lowlink to parent.
			if len(iterStack) > 0 {
				parent := &iterStack[len(iterStack)-1]
				lowlinkMap[parent.node] = minInt(lowlinkMap[parent.node], lowlinkMap[fr.node])
			}
		}
	}

	return sccs
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}