package ingest

import (
	"fmt"
	"log"
	"strings"

	graph "github.com/mengshi02/codetrip/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// Call processor — resolves call sites to their target definitions.
//
// Resolution strategy (3-tier with callable filtering + receiver disambiguation):
//   A. collectTieredCandidates — narrow candidates by scope tier (same-file > import-scoped > unique-global)
//   B. filterCallableCandidates — filter to callable symbol kinds (constructor-aware)
//   C. Apply arity filtering when parameter metadata is available
//   D. Apply receiver-type filtering for member calls with typed receivers
//   Rule: multiple candidates after all filtering → reject (wrong edge worse than no edge)

// ─────────────────────────────────────────────────────────────────────────────
// CallResolveResult — result of resolving a call site.
// ─────────────────────────────────────────────────────────────────────────────

// CallResolveResult holds the resolved call target with confidence.
type CallResolveResult struct {
	NodeID     string
	Confidence float64
	Reason     string // "same-file", "import-resolved", "unique-global", "laravel-route"
}

// ─────────────────────────────────────────────────────────────────────────────
// TieredCandidates — candidates grouped by resolution tier.
// ─────────────────────────────────────────────────────────────────────────────

// ResolutionTier represents the scope tier for candidate resolution.
type ResolutionTier int

const (
	TierSameFile     ResolutionTier = iota // "same-file"
	TierImportScoped                       // "import-scoped"
	TierUniqueGlobal                       // "unique-global"
)

// TieredCandidates holds candidates with their resolution tier.
type TieredCandidates struct {
	Candidates []*SymbolDefinition
	Tier       ResolutionTier
}

func tierToString(t ResolutionTier) string {
	switch t {
	case TierSameFile:
		return "same-file"
	case TierImportScoped:
		return "import-resolved"
	case TierUniqueGlobal:
		return "unique-global"
	default:
		return "unknown"
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// collectTieredCandidates — collect candidates grouped by scope tier.
//
// Key difference from ResolveSymbol: Tier 1 uses lookupFuzzy + filePath filter
// (may return multiple same-file candidates) instead of LookupExactFull (single match).
// This allows filterCallableCandidates to narrow multiple same-file defs to one.
// ─────────────────────────────────────────────────────────────────────────────

func collectTieredCandidates(
	calledName string,
	currentFile string,
	ctx *ResolveContext,
) *TieredCandidates {
	allDefs := ctx.SymbolTable.LookupFuzzy(calledName)
	if ctx.AssignableOwnerIDs != nil {
		currentLanguage := GetLanguageFromFilename(currentFile)
		compatible := allDefs[:0]
		for _, def := range allDefs {
			candidateLanguage := GetLanguageFromFilename(def.FilePath)
			if callLanguagesCompatible(currentLanguage, candidateLanguage) {
				compatible = append(compatible, def)
			}
		}
		allDefs = compatible
	}

	// Tier 1: Same-file — highest priority, prevents imports from shadowing local defs
	localDefs := make([]*SymbolDefinition, 0)
	for _, def := range allDefs {
		if def.FilePath == currentFile {
			localDefs = append(localDefs, def)
		}
	}
	if len(localDefs) > 0 {
		return &TieredCandidates{Candidates: localDefs, Tier: TierSameFile}
	}

	// Tier 2a-named: Check named bindings with re-export chain following
	if ctx.NamedImportMap != nil {
		if binding, ok := ctx.NamedImportMap[currentFile][calledName]; ok && strings.HasPrefix(binding.SourcePath, "@unresolved:") {
			importPath := strings.TrimPrefix(binding.SourcePath, "@unresolved:")
			packagePath := importPath
			if idx := strings.LastIndex(packagePath, "."); idx >= 0 {
				packagePath = packagePath[:idx]
			}
			packageDir := strings.ReplaceAll(packagePath, ".", "/")
			lookupName := binding.ExportedName
			if lookupName == "" {
				lookupName = calledName
			}
			var packageDefs []*SymbolDefinition
			for _, def := range ctx.SymbolTable.LookupFuzzy(lookupName) {
				if strings.Contains(def.FilePath, packageDir+"/") {
					packageDefs = append(packageDefs, def)
				}
			}
			if len(packageDefs) > 0 {
				return &TieredCandidates{Candidates: packageDefs, Tier: TierImportScoped}
			}
			return nil
		}
		namedDefs := WalkBindingChain(calledName, currentFile, ctx.SymbolTable, ctx.NamedImportMap, allDefs)
		if len(namedDefs) == 1 {
			return &TieredCandidates{Candidates: namedDefs, Tier: TierImportScoped}
		}
		// Multiple named binding candidates → don't return; fall through
	}

	if len(allDefs) == 0 {
		return nil
	}

	// Tier 2b: Import-scoped — filter by imported files
	importedFiles := ctx.ImportMap[currentFile]
	if len(importedFiles) > 0 {
		var importedDefs []*SymbolDefinition
		for _, def := range allDefs {
			if importedFiles[def.FilePath] {
				importedDefs = append(importedDefs, def)
			}
		}
		if len(importedDefs) > 0 {
			return &TieredCandidates{Candidates: importedDefs, Tier: TierImportScoped}
		}
	}

	// Tier 2c: Package-scoped — filter by imported package dirs
	importedPackages := ctx.PackageMap[currentFile]
	if len(importedPackages) > 0 {
		var packageDefs []*SymbolDefinition
		for _, def := range allDefs {
			for ds := range importedPackages {
				if isFileInPackageDir(def.FilePath, ds) {
					packageDefs = append(packageDefs, def)
					break
				}
			}
		}
		if len(packageDefs) > 0 {
			return &TieredCandidates{Candidates: packageDefs, Tier: TierImportScoped}
		}
	}

	// Tier 3: Global — pass all candidates through; filterCallableCandidates will narrow
	return &TieredCandidates{Candidates: allDefs, Tier: TierUniqueGlobal}
}

func callLanguagesCompatible(source, target string) bool {
	if source == target {
		return true
	}
	// C and C++ intentionally interoperate through headers and C APIs.
	return (source == "c" || source == "cpp") && (target == "c" || target == "cpp")
}

// ─────────────────────────────────────────────────────────────────────────────
// filterCallableCandidates — filter candidates to callable symbol kinds.
// ─────────────────────────────────────────────────────────────────────────────

var callableSymbolTypes = map[string]bool{
	"Function":    true,
	"Method":      true,
	"Constructor": true,
	"Macro":       true,
	"Delegate":    true,
}

var constructorTargetTypes = map[string]bool{
	"Constructor": true,
	"Class":       true,
	"Struct":      true,
	"Record":      true,
}

func filterCallableCandidates(
	candidates []*SymbolDefinition,
	argCount int, // -1 if unknown
	callForm CallForm,
) []*SymbolDefinition {
	var kindFiltered []*SymbolDefinition

	if callForm == CallFormConstructor {
		// For constructor calls, prefer Constructor > Class/Struct/Record > callable fallback
		var constructors []*SymbolDefinition
		for _, c := range candidates {
			if c.Type == "Constructor" {
				constructors = append(constructors, c)
			}
		}
		if len(constructors) > 0 {
			kindFiltered = constructors
		} else {
			var types []*SymbolDefinition
			for _, c := range candidates {
				if constructorTargetTypes[c.Type] {
					types = append(types, c)
				}
			}
			if len(types) > 0 {
				kindFiltered = types
			} else {
				for _, c := range candidates {
					if callableSymbolTypes[c.Type] {
						kindFiltered = append(kindFiltered, c)
					}
				}
			}
		}
	} else {
		for _, c := range candidates {
			if callableSymbolTypes[c.Type] {
				kindFiltered = append(kindFiltered, c)
			}
		}
	}

	if len(kindFiltered) == 0 {
		return nil
	}
	if argCount < 0 {
		return kindFiltered
	}

	// Arity filtering — only if parameter metadata is available
	hasParameterMetadata := false
	for _, c := range kindFiltered {
		if c.ParameterCount != nil {
			hasParameterMetadata = true
			break
		}
	}
	if !hasParameterMetadata {
		return kindFiltered
	}

	var arityFiltered []*SymbolDefinition
	for _, c := range kindFiltered {
		if c.ParameterCount == nil || *c.ParameterCount == argCount {
			arityFiltered = append(arityFiltered, c)
		}
	}
	return arityFiltered
}

// collapsePhysicalTargetDuplicates removes candidates that serialize to the
// same graph node. The current CSV schema does not encode C++ signatures in a
// node ID, so overload declarations such as f(int) and f(double) may be
// distinct symbol-table candidates but the same physical Method node. Keeping
// both would reject an otherwise unambiguous graph edge even though either
// candidate produces exactly the same target.
func collapsePhysicalTargetDuplicates(candidates []*SymbolDefinition) []*SymbolDefinition {
	if len(candidates) < 2 {
		return candidates
	}
	seen := make(map[string]bool, len(candidates))
	result := make([]*SymbolDefinition, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || seen[candidate.NodeID] {
			continue
		}
		seen[candidate.NodeID] = true
		result = append(result, candidate)
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// resolveCallTarget — core resolution logic for a single call site.
//
// Uses collectTieredCandidates + filterCallableCandidates + receiver-type
// disambiguation — NOT the shared ResolveSymbol (which skips callable filtering).
// ─────────────────────────────────────────────────────────────────────────────

func resolveCallTarget(
	calledName string,
	filePath string,
	receiverTypeName string,
	ctx *ResolveContext,
	callForm CallForm,
	argCount int, // -1 if unknown
) *CallResolveResult {
	if callForm == CallFormConstructor && ctx.AssignableOwnerIDs != nil {
		visibleOwners := make(map[string]bool)
		for _, definition := range visibleTypeDefinitions(calledName, filePath, ctx) {
			if definition.Type == "Class" || definition.Type == "Struct" || definition.Type == "Record" {
				visibleOwners[definition.NodeID] = true
			}
		}
		var constructors []*SymbolDefinition
		for _, candidate := range ctx.SymbolTable.LookupFuzzy(calledName) {
			if candidate.Type == "Constructor" && visibleOwners[candidate.OwnerID] {
				constructors = append(constructors, candidate)
			}
		}
		constructors = filterCallableCandidates(constructors, argCount, CallFormConstructor)
		constructors = collapsePhysicalTargetDuplicates(constructors)
		if len(constructors) == 1 {
			return &CallResolveResult{NodeID: constructors[0].NodeID, Confidence: 0.95, Reason: "visible-constructor-exact"}
		}
		// A C++ field_initializer is also used for data-member initializers
		// such as env(Env::Default()). Without a visible same-named type this
		// is not a constructor target and must not fall back to a free function.
		if len(visibleOwners) == 0 {
			return nil
		}
	}
	// Enhanced typed receivers outrank lexical same-file candidates. Otherwise
	// an unrelated local class can shadow a method inherited from an imported
	// base class merely because it is defined in the caller's file.
	if receiverTypeName != "" && ctx.AssignableOwnerIDs != nil {
		exactOwners := make(map[string]bool)
		allowedOwners := make(map[string]bool)
		hasConcreteReceiverType := false
		for _, typeDef := range visibleTypeDefinitions(receiverTypeName, filePath, ctx) {
			exactOwners[typeDef.NodeID] = true
			allowedOwners[typeDef.NodeID] = true
			if typeDef.Type == "Class" || typeDef.Type == "Struct" || typeDef.Type == "Record" || typeDef.Type == "Interface" {
				hasConcreteReceiverType = true
			}
			for ownerID := range ctx.AssignableOwnerIDs[typeDef.NodeID] {
				allowedOwners[ownerID] = true
			}
		}
		if len(allowedOwners) == 0 && callForm == CallFormMember {
			return nil
		}
		var exactCandidates, typedCandidates []*SymbolDefinition
		for _, candidate := range ctx.SymbolTable.LookupFuzzy(calledName) {
			ownerTypeName := candidate.OwnerID
			if idx := strings.LastIndex(ownerTypeName, ":"); idx >= 0 {
				ownerTypeName = ownerTypeName[idx+1:]
			}
			ownerNameMatches := len(exactOwners) == 0 && ownerTypeName == receiverTypeName
			if allowedOwners[candidate.OwnerID] || ownerNameMatches {
				typedCandidates = append(typedCandidates, candidate)
			}
			if exactOwners[candidate.OwnerID] || ownerNameMatches {
				exactCandidates = append(exactCandidates, candidate)
			}
		}
		exactCandidates = filterCallableCandidates(exactCandidates, argCount, CallFormMember)
		exactCandidates = collapsePhysicalTargetDuplicates(exactCandidates)
		if len(exactCandidates) == 1 {
			return &CallResolveResult{NodeID: exactCandidates[0].NodeID, Confidence: 0.95, Reason: "receiver-type-exact"}
		}
		// ParameterCount is the declared maximum, not the required minimum.
		// Permit omitted default arguments only when the exact receiver has one
		// physical same-name member; overloaded members remain unresolved.
		if argCount >= 0 && len(exactCandidates) == 0 {
			var defaultArgCandidates []*SymbolDefinition
			for _, candidate := range filterCallableCandidates(ctx.SymbolTable.LookupFuzzy(calledName), -1, CallFormMember) {
				ownerTypeName := candidate.OwnerID
				if idx := strings.LastIndex(ownerTypeName, ":"); idx >= 0 {
					ownerTypeName = ownerTypeName[idx+1:]
				}
				if (exactOwners[candidate.OwnerID] || (len(exactOwners) == 0 && ownerTypeName == receiverTypeName)) &&
					candidate.ParameterCount != nil && argCount <= *candidate.ParameterCount {
					defaultArgCandidates = append(defaultArgCandidates, candidate)
				}
			}
			defaultArgCandidates = collapsePhysicalTargetDuplicates(defaultArgCandidates)
			if len(defaultArgCandidates) == 1 {
				return &CallResolveResult{NodeID: defaultArgCandidates[0].NodeID, Confidence: 0.95, Reason: "receiver-type-default-args"}
			}
		}
		typedCandidates = filterCallableCandidates(typedCandidates, argCount, CallFormMember)
		typedCandidates = collapsePhysicalTargetDuplicates(typedCandidates)
		if len(typedCandidates) == 1 {
			confidence := 0.9
			reason := "receiver-type"
			if typedCandidates[0].FilePath == filePath {
				confidence = 0.95
			}
			return &CallResolveResult{NodeID: typedCandidates[0].NodeID, Confidence: confidence, Reason: reason}
		}
		if hasConcreteReceiverType {
			var ownerless []*SymbolDefinition
			for _, candidate := range ctx.SymbolTable.LookupFuzzy(calledName) {
				if candidate.OwnerID == "" {
					ownerless = append(ownerless, candidate)
				}
			}
			ownerless = filterCallableCandidates(ownerless, argCount, CallFormMember)
			ownerless = collapsePhysicalTargetDuplicates(ownerless)
			if len(ownerless) == 1 {
				return &CallResolveResult{NodeID: ownerless[0].NodeID, Confidence: 0.9, Reason: "receiver-type-ownerless-definition"}
			}
		}
		// An explicit receiver type is stronger negative evidence than a global
		// name match. If the type (or its ancestors) does not provide exactly one
		// callable target, keep the call unresolved instead of guessing.
		return nil
	}

	// A. Collect tiered candidates
	tiered := collectTieredCandidates(calledName, filePath, ctx)
	if tiered == nil {
		return nil
	}

	// B. Filter to callable symbol kinds (constructor-aware)
	filteredCandidates := filterCallableCandidates(tiered.Candidates, argCount, callForm)
	filteredCandidates = collapsePhysicalTargetDuplicates(filteredCandidates)

	// D. Receiver-type filtering: for member calls with a known receiver type,
	// filter candidates by ownerId matching the resolved type's nodeId
	if callForm == CallFormMember && receiverTypeName != "" && len(filteredCandidates) > 1 {
		typeDefs := visibleTypeDefinitions(receiverTypeName, filePath, ctx)
		if len(typeDefs) > 0 {
			typeNodeIDs := make(map[string]bool)
			for _, d := range typeDefs {
				typeNodeIDs[d.NodeID] = true
				for ownerID := range ctx.AssignableOwnerIDs[d.NodeID] {
					typeNodeIDs[ownerID] = true
				}
			}
			var ownerFiltered []*SymbolDefinition
			for _, c := range filteredCandidates {
				if c.OwnerID != "" && typeNodeIDs[c.OwnerID] {
					ownerFiltered = append(ownerFiltered, c)
				}
			}
			if len(ownerFiltered) == 1 {
				def := ownerFiltered[0]
				return &CallResolveResult{
					NodeID:     def.NodeID,
					Confidence: tierToConfidence(tiered.Tier),
					Reason:     tierToString(tiered.Tier),
				}
			}
			// If receiver filtering narrows to 0, fall through to name-only resolution
			// If still 2+, refuse (don't guess)
			if len(ownerFiltered) > 1 {
				return nil
			}
		}
	}

	if len(filteredCandidates) != 1 {
		return nil
	}

	def := filteredCandidates[0]
	return &CallResolveResult{
		NodeID:     def.NodeID,
		Confidence: tierToConfidence(tiered.Tier),
		Reason:     tierToString(tiered.Tier),
	}
}

// visibleTypeDefinitions prevents a simple receiver type such as Iterator
// from matching unrelated same-name nested classes elsewhere in a repository.
// C/C++ headers are followed transitively because a type may arrive through an
// included public header rather than a direct include in the call-site file.
func visibleTypeDefinitions(typeName, currentFile string, ctx *ResolveContext) []*SymbolDefinition {
	if ctx.ImportMap == nil {
		return ctx.SymbolTable.LookupFuzzy(typeName)
	}
	visibleFiles := map[string]bool{currentFile: true}
	queue := []string{currentFile}
	for len(queue) > 0 {
		file := queue[0]
		queue = queue[1:]
		for imported := range ctx.ImportMap[file] {
			if !visibleFiles[imported] {
				visibleFiles[imported] = true
				queue = append(queue, imported)
			}
		}
	}
	names := map[string]bool{typeName: true}
	queueNames := []string{typeName}
	for len(queueNames) > 0 {
		name := queueNames[0]
		queueNames = queueNames[1:]
		for _, alias := range ctx.SymbolTable.LookupTypeAliases(name) {
			if visibleFiles[alias.FilePath] && !names[alias.TargetName] {
				names[alias.TargetName] = true
				queueNames = append(queueNames, alias.TargetName)
			}
		}
	}
	var result []*SymbolDefinition
	for name := range names {
		for _, def := range ctx.SymbolTable.LookupFuzzy(name) {
			if visibleFiles[def.FilePath] {
				result = append(result, def)
			}
		}
	}
	return result
}

// BuildAssignableOwnerIDs resolves extracted inheritance before call
// resolution. It supplies the minimum type relation needed to resolve a method
// inherited by a typed receiver without requiring a full compiler type system.
func BuildAssignableOwnerIDs(
	heritage []ExtractedHeritage,
	symbolTable *SymbolTable,
	im ImportMap,
	pm PackageMap,
	nim NamedImportMap,
	order ImportOrderMap,
) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	ctx := &ResolveContext{
		SymbolTable: symbolTable, NamedImportMap: nim,
		ImportMap: im, PackageMap: pm, ImportOrderMap: order,
	}
	for _, item := range heritage {
		childDefs := symbolTable.LookupFuzzy(item.ChildID)
		parent := resolveHeritageDefinition(item.ParentName, item.FilePath, ctx)
		if parent == nil {
			continue
		}
		for _, child := range childDefs {
			if child.FilePath != item.FilePath {
				continue
			}
			if result[child.NodeID] == nil {
				result[child.NodeID] = make(map[string]bool)
			}
			if !isHeritageTypeDefinition(child) {
				continue
			}
			result[child.NodeID][parent.NodeID] = true
		}
	}

	// Small fixed-point closure supports multi-level inheritance.
	changed := true
	for changed {
		changed = false
		for child, parents := range result {
			for parent := range parents {
				for ancestor := range result[parent] {
					if !result[child][ancestor] {
						result[child][ancestor] = true
						changed = true
					}
				}
			}
		}
	}
	return result
}

func tierToConfidence(tier ResolutionTier) float64 {
	switch tier {
	case TierSameFile:
		return 0.95
	case TierImportScoped:
		return 0.9
	case TierUniqueGlobal:
		return 0.5
	default:
		return 0.0
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ProcessCalls — full call processing with AST parsing.
// ─────────────────────────────────────────────────────────────────────────────

func ProcessCalls(
	g *graph.KnowledgeGraph,
	reg *LanguageRegistry,
	files []string,
	langMap map[string]string,
	astCache map[string]*sitter.Tree,
	srcCache map[string][]byte,
	parser *sitter.Parser,
	symbolTable *SymbolTable,
	im ImportMap,
	pm PackageMap,
	nim NamedImportMap,
) {
	ctx := &ResolveContext{
		SymbolTable:    symbolTable,
		NamedImportMap: nim,
		ImportMap:      im,
		PackageMap:     pm,
		ImportOrderMap: nil,
	}

	for _, fp := range files {
		lang, ok := langMap[fp]
		if !ok {
			continue
		}
		tree, ok := astCache[fp]
		if !ok {
			continue
		}
		src, ok := srcCache[fp]
		if !ok {
			continue
		}
		l, err := reg.GetLanguage(lang)
		if err != nil {
			continue
		}
		qs := LanguageQueries(lang)
		if qs == "" {
			continue
		}
		q, queryErr := sitter.NewQuery(l, qs)
		if queryErr != nil {
			continue
		}
		captureNames := q.CaptureNames()
		qc := sitter.NewQueryCursor()
		matches := qc.Matches(q, tree.RootNode(), src)

		// Build per-file TypeEnv for receiver resolution
		typeEnv := BuildTypeEnv(tree.RootNode(), lang, src)

		for {
			m := matches.Next()
			if m == nil {
				break
			}
			captureMap := buildCaptureMap(m, captureNames)

			// Only process @call captures
			callNode, ok := captureMap["call"]
			if !ok || callNode == nil {
				continue
			}
			nameNode, ok := captureMap["call.name"]
			if !ok || nameNode == nil {
				continue
			}
			calledName := nameNode.Utf8Text(src)
			if calledName == "" || IsBuiltInOrNoiseForLanguage(calledName, lang) {
				continue
			}

			callForm := InferCallForm(callNode, nameNode, src)
			receiverName := ""
			receiverTypeName := ""
			if callForm == CallFormMember {
				receiverName = ExtractReceiverName(nameNode, src)
				if receiverName != "" {
					receiverTypeName = LookupTypeEnv(typeEnv, receiverName, callNode, src)
				}
			}

			// Resolve the call target (with callForm and argCount for callable filtering)
			argCount := CountCallArguments(callNode)
			resolved := resolveCallTarget(calledName, fp, receiverTypeName, ctx, callForm, argCount)
			if resolved == nil {
				continue
			}

			// Find enclosing function (caller)
			sourceID := findEnclosingFunctionID(callNode, fp, src)
			if sourceID == "" {
				sourceID = "File:" + fp
			}

			relID := makeCallRelID(sourceID, calledName, resolved.NodeID)
			g.AddRelationship(&graph.GraphRelationship{
				ID:         relID,
				SourceID:   sourceID,
				TargetID:   resolved.NodeID,
				Type:       graph.RelCALLS,
				Confidence: resolved.Confidence,
				Reason:     resolved.Reason,
			})
		}
		qc.Close()
		q.Close()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// ProcessCallsFromExtracted — fast path using pre-extracted call sites.
// ─────────────────────────────────────────────────────────────────────────────

func ProcessCallsFromExtracted(
	g *graph.KnowledgeGraph,
	extractedCalls []ExtractedCall,
	symbolTable *SymbolTable,
	im ImportMap,
	pm PackageMap,
	nim NamedImportMap,
	order ImportOrderMap,
	assignableOwners map[string]map[string]bool,
) {
	ctx := &ResolveContext{
		SymbolTable:        symbolTable,
		NamedImportMap:     nim,
		ImportMap:          im,
		PackageMap:         pm,
		ImportOrderMap:     order,
		AssignableOwnerIDs: assignableOwners,
	}

	fc, gc := symbolTable.Stats()
	log.Printf("[call-processor] ProcessCallsFromExtracted: %d extracted calls, symbolTable: %d files, %d global symbols", len(extractedCalls), fc, gc)
	log.Printf("[call-processor] importMap files=%d, packageMap files=%d", len(im), len(pm))
	// Debug: print sample keys from importMap and packageMap
	debugIdx := 0
	for k, v := range im {
		if debugIdx >= 3 {
			break
		}
		log.Printf("[call-processor] importMap key=%q values=%d", k, len(v))
		debugIdx++
	}
	debugIdx = 0
	for k, v := range pm {
		if debugIdx >= 3 {
			break
		}
		log.Printf("[call-processor] packageMap key=%q values=%d", k, len(v))
		debugIdx++
	}
	resolved := 0
	builtin := 0
	failed := 0

	sampleIdx := 0
	for _, call := range extractedCalls {
		calledName := call.CallName
		if calledName == "" || IsBuiltInOrNoiseForLanguage(calledName, call.Language) {
			builtin++
			if sampleIdx < 30 {
				log.Printf("[call-processor] builtin/noise sample[%d]: name=%q file=%s form=%v", sampleIdx, calledName, call.FilePath, call.CallForm)
				sampleIdx++
			}
			continue
		}

		receiverTypeName := call.ReceiverTypeName
		callForm := call.CallForm
		if ctx.AssignableOwnerIDs != nil && call.Language == "cpp" && receiverTypeName != "" && len(call.ReceiverChain) > 0 {
			currentType := receiverTypeName
			validChain := true
			for index, methodName := range call.ReceiverChain {
				argCount := -1
				if index < len(call.ReceiverChainArgCounts) {
					argCount = call.ReceiverChainArgCounts[index]
				}
				step := resolveCallTarget(methodName, call.FilePath, currentType, ctx, CallFormMember, argCount)
				if step == nil {
					validChain = false
					break
				}
				definition := ctx.SymbolTable.LookupNodeIDWithArity(step.NodeID, argCount)
				if definition == nil || definition.ReturnType == "" {
					validChain = false
					break
				}
				currentType = normalizeCppReturnType(definition.ReturnType)
				if currentType == "" {
					validChain = false
					break
				}
			}
			if validChain {
				receiverTypeName = currentType
			} else {
				receiverTypeName = ""
			}
		}
		if ctx.AssignableOwnerIDs != nil && call.Language == "cpp" && callForm == CallFormMember && call.ReceiverName != "" {
			for _, qualifier := range visibleTypeDefinitions(call.ReceiverName, call.FilePath, ctx) {
				if qualifier.Type == "Namespace" {
					callForm = CallFormFree
					receiverTypeName = ""
					break
				}
			}
		}
		inferredKotlinConstructor := false
		if ctx.AssignableOwnerIDs != nil && call.Language == "kotlin" && callForm == CallFormFree && calledName[0] >= 'A' && calledName[0] <= 'Z' {
			for _, candidate := range symbolTable.LookupFuzzy(calledName) {
				if candidate.Type == "Class" || candidate.Type == "Struct" || candidate.Type == "Record" {
					callForm = CallFormConstructor
					inferredKotlinConstructor = true
					break
				}
			}
		}
		if ctx.AssignableOwnerIDs != nil && call.Language == "cpp" && callForm == CallFormFree {
			for _, definition := range visibleTypeDefinitions(calledName, call.FilePath, ctx) {
				if definition.Type == "Class" || definition.Type == "Struct" || definition.Type == "Record" {
					callForm = CallFormConstructor
					break
				}
			}
		}
		implicitReceiverFallback := false
		if receiverTypeName == "" && callForm == CallFormFree && ctx.AssignableOwnerIDs != nil {
			if enclosing := symbolTable.FindEnclosingFunction(call.FilePath, call.StartByte); enclosing != nil && enclosing.OwnerID != "" {
				receiverTypeName = enclosing.OwnerID
				if idx := strings.LastIndex(receiverTypeName, ":"); idx >= 0 {
					receiverTypeName = receiverTypeName[idx+1:]
				}
				callForm = CallFormMember
				implicitReceiverFallback = true
			}
		}
		if call.CallForm == CallFormMember && receiverTypeName == "" && call.ReceiverName != "" {
			// No type from extraction — leave empty, resolution falls back to name-only
		}
		if ctx.AssignableOwnerIDs != nil && callForm == CallFormMember && receiverTypeName == "" {
			// A complex or value-like untyped receiver is not evidence for a
			// repository-wide same-name target. Preserve it as unresolved instead
			// of guessing. Uppercase receivers may be class/object names (Kotlin
			// companion APIs and static-style calls) and retain scoped fallback.
			likelyTypeReceiver := call.ReceiverName != "" && call.ReceiverName[0] >= 'A' && call.ReceiverName[0] <= 'Z'
			if !likelyTypeReceiver {
				failed++
				continue
			}
		}

		res := resolveCallTarget(calledName, call.FilePath, receiverTypeName, ctx, callForm, call.ArgCount)
		if inferredKotlinConstructor && res != nil && res.Reason == "unique-global" {
			res = nil
		}
		if res == nil && implicitReceiverFallback {
			// A bare call inside a class may target either an implicit member or a
			// top-level/imported helper. Only accept lexical free-call evidence;
			// never fall through to repository-wide unique-name guessing.
			lexical := resolveCallTarget(calledName, call.FilePath, "", ctx, CallFormFree, call.ArgCount)
			if lexical != nil && lexical.Reason != "unique-global" {
				res = lexical
			}
		}
		if res == nil {
			failed++
			if failed <= 10 {
				fuzzys := ctx.SymbolTable.LookupFuzzy(calledName)
				log.Printf("[call-processor] resolve failed[%d]: name=%q file=%s form=%v fuzzyCount=%d", failed, calledName, call.FilePath, call.CallForm, len(fuzzys))
			}
			continue
		}
		resolved++

		sourceID := call.SourceID
		if sourceID == "" {
			if ctx.AssignableOwnerIDs != nil {
				if enclosing := symbolTable.FindEnclosingFunction(call.FilePath, call.StartByte); enclosing != nil {
					sourceID = enclosing.NodeID
				}
			} else {
				sourceID = findEnclosingFunctionIDFromByte(call.FilePath, call.StartByte, symbolTable)
			}
		}
		if sourceID == "" && ctx.AssignableOwnerIDs != nil {
			sourceID = symbolTable.FindEnclosingOwnerID(call.FilePath, call.StartByte)
		}
		if sourceID == "" {
			sourceID = "File:" + call.FilePath
		}

		relID := makeCallRelID(sourceID, calledName, res.NodeID)
		g.AddRelationship(&graph.GraphRelationship{
			ID:         relID,
			SourceID:   sourceID,
			TargetID:   res.NodeID,
			Type:       graph.RelCALLS,
			Confidence: res.Confidence,
			Reason:     res.Reason,
		})
	}
	log.Printf("[call-processor] Result: %d resolved, %d builtin/noise, %d failed", resolved, builtin, failed)
}

func normalizeCppReturnType(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "const ")
	value = strings.TrimSpace(strings.TrimRight(value, "*& "))
	if idx := strings.LastIndex(value, "::"); idx >= 0 {
		value = value[idx+2:]
	}
	return value
}

// ─────────────────────────────────────────────────────────────────────────────
// ProcessRoutesFromExtracted — resolve Laravel routes to CALLS edges.
// ─────────────────────────────────────────────────────────────────────────────

func ProcessRoutesFromExtracted(
	g *graph.KnowledgeGraph,
	extractedRoutes []ExtractedRoute,
	symbolTable *SymbolTable,
	im ImportMap,
	pm PackageMap,
) {
	ctx := &ResolveContext{
		SymbolTable:    symbolTable,
		NamedImportMap: make(NamedImportMap),
		ImportMap:      im,
		PackageMap:     pm,
	}

	for _, route := range extractedRoutes {
		if route.Controller == "" || route.Method == "" {
			continue
		}

		// Resolve controller class
		resolution := ResolveSymbol(route.Controller, route.FilePath, ctx)
		if resolution == nil {
			continue
		}

		confidence := resolution.Confidence

		// Find the method on the controller
		methodID := symbolTable.LookupExact(resolution.Definition.FilePath, route.Method)
		sourceID := "File:" + route.FilePath

		if methodID == "" {
			// Construct method ID manually
			guessedID := fmt.Sprintf("Method::%s::%s::0", resolution.Definition.FilePath, route.Method)
			relID := makeCallRelID(sourceID, "route", guessedID)
			g.AddRelationship(&graph.GraphRelationship{
				ID:         relID,
				SourceID:   sourceID,
				TargetID:   guessedID,
				Type:       graph.RelCALLS,
				Confidence: confidence * 0.8,
				Reason:     "laravel-route",
			})
			continue
		}

		relID := makeCallRelID(sourceID, "route", methodID)
		g.AddRelationship(&graph.GraphRelationship{
			ID:         relID,
			SourceID:   sourceID,
			TargetID:   methodID,
			Type:       graph.RelCALLS,
			Confidence: confidence,
			Reason:     "laravel-route",
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func makeCallRelID(src, calledName, tgt string) string {
	return "CALLS:" + src + ":" + calledName + "->" + tgt
}

// buildCaptureMap builds a name→Node map from a query match.
func buildCaptureMap(m *sitter.QueryMatch, captureNames []string) map[string]*sitter.Node {
	cm := make(map[string]*sitter.Node)
	for _, c := range m.Captures {
		idx := int(c.Index)
		name := ""
		if idx < len(captureNames) {
			name = captureNames[idx]
		}
		node := c.Node
		cm[name] = &node
	}
	return cm
}

// findEnclosingFunctionID walks up the AST to find the enclosing function's node ID.
// Mirrors v4.1.0 parse-worker findEnclosingFunctionId: does NOT look up symbolTable;
// instead uses extractFunctionName's label to generate the ID directly.
// This ensures constructor_declaration gets "Method:" prefix (not "Constructor:"),
// matching baseline behavior.
func findEnclosingFunctionID(callNode *sitter.Node, filePath string, source []byte) string {
	current := callNode.Parent()
	for current != nil {
		if FunctionNodeTypes[current.Kind()] {
			funcName, label := ExtractFunctionName(current, source)
			if funcName != "" {
				// TODO: apply language-specific labelOverride for Kotlin/C++ when needed
				return graph.GenerateID(label, fmt.Sprintf("%s:%s", filePath, funcName))
			}
		}
		current = current.Parent()
	}
	return ""
}

// findEnclosingFunctionIDFromByte finds the enclosing function for a byte offset.
func findEnclosingFunctionIDFromByte(filePath string, startByte uint, symbolTable *SymbolTable) string {
	result := symbolTable.FindEnclosingFunctionID(filePath, startByte)
	// Debug: log if the result refers to a constructor (by checking the symbolTable entry)
	if result != "" && !strings.HasPrefix(result, "Method:") && !strings.HasPrefix(result, "File:") {
		// Check if the enclosing symbol is a Constructor type
		parts := strings.SplitN(result, ":", 3)
		if len(parts) >= 3 {
			label := parts[0]
			if label != "Method" && label != "File" && label != "Function" {
				log.Printf("[call-processor] findEnclosingFunctionIDFromByte: file=%s offset=%d label=%s result=%s", filePath, startByte, label, result)
			}
		}
	}
	return result
}
