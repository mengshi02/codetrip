package scope_resolution

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ScopeResolutionOutput holds the result of the scopeResolution pipeline phase.
// Mirrors TS scope-resolution/pipeline/phase.ts ScopeResolutionOutput.
type ScopeResolutionOutput struct {
	// Ran is true when at least one language ran.
	Ran bool
	// FilesProcessed is the count of files seen across all languages.
	FilesProcessed int
	// ImportsEmitted is the count of IMPORTS edges emitted.
	ImportsEmitted int
	// ReferenceEdgesEmitted is the count of CALLS/ACCESSES/INHERITS/USES edges.
	ReferenceEdgesEmitted int
	// ResolutionOutcomes is an additive stream of resolver diagnostics.
	ResolutionOutcomes []ResolutionOutcome
	// PerLanguage is the per-language breakdown for telemetry.
	PerLanguage map[shared.SupportedLanguage]*ScopeResolutionLangStats
}

// ScopeResolutionLangStats holds per-language resolution statistics.
type ScopeResolutionLangStats struct {
	FilesProcessed        int
	ImportsEmitted        int
	ReferenceEdgesEmitted int
}

// NoopScopeResolutionOutput is the zero-value output when no language runs.
var NoopScopeResolutionOutput = &ScopeResolutionOutput{
	Ran:                   false,
	FilesProcessed:        0,
	ImportsEmitted:        0,
	ReferenceEdgesEmitted: 0,
	ResolutionOutcomes:    nil,
	PerLanguage:           map[shared.SupportedLanguage]*ScopeResolutionLangStats{},
}

// ScopeResolutionSubPhase identifies a sub-phase within the scope-resolution
// pipeline for logging and diagnostic purposes.
type ScopeResolutionSubPhase string

const (
	SubPhaseExtract           ScopeResolutionSubPhase = "extract"
	SubPhaseFinalize          ScopeResolutionSubPhase = "finalize"
	SubPhasePopulateOwners    ScopeResolutionSubPhase = "populateOwners"
	SubPhaseEmitHeritage      ScopeResolutionSubPhase = "emitHeritage"
	SubPhaseEmitImplicit      ScopeResolutionSubPhase = "emitImplicit"
	SubPhaseBuildMro          ScopeResolutionSubPhase = "buildMro"
	SubPhaseEmitReceiverBound ScopeResolutionSubPhase = "emitReceiverBound"
	SubPhaseEmitFreeCall      ScopeResolutionSubPhase = "emitFreeCall"
	SubPhaseEmitReferences    ScopeResolutionSubPhase = "emitReferences"
	SubPhaseEmitImports       ScopeResolutionSubPhase = "emitImports"
	SubPhaseNamespaceSiblings ScopeResolutionSubPhase = "namespaceSiblings"
	SubPhaseReconcile         ScopeResolutionSubPhase = "reconcile"
	SubPhaseValidate          ScopeResolutionSubPhase = "validate"
)

// RunScopeResolutionPhaseInput holds all inputs to the scopeResolution phase.
// Mirrors the TS phase.ts execute() context extraction.
type RunScopeResolutionPhaseInput struct {
	// Graph is the writable knowledge graph (shared.KnowledgeGraph).
	Graph shared.KnowledgeGraph
	// Registry holds all registered ScopeResolver instances.
	Registry *ScopeResolverRegistry
	// FilePaths is the full set of scanned file paths in the workspace.
	FilePaths []string
	// AllPathSet is the set version of FilePathPaths for O(1) lookup.
	AllPathSet map[string]bool
	// RepoPath is the absolute path to the repository root.
	RepoPath string
	// SemanticModel is the mutable semantic model built during the parse phase.
	// If nil, a fresh model will be created per language (less optimal).
	SemanticModel model.MutableSemanticModel
	// OnProgress is an optional progress callback.
	OnProgress func(phase string, percent int, message string)
	// OnWarn is an optional warning callback.
	OnWarn func(msg string)
}

// RunScopeResolutionPhase executes the full scope-resolution pipeline across
// all registered languages. This is the top-level entry point called by the
// pipeline's scopeResolution phase.
//
// Mirrors the TS scopeResolutionPhase execute() logic:
//   - For each language in SCOPE_RESOLVERS:
//   - Filter scanned files by language extension
//   - Read file contents
//   - Build graph node lookup (shared across languages)
//   - Drive scope-based pipeline end-to-end via RunScopeResolution
//   - Emit IMPORTS / CALLS / ACCESSES / INHERITS / USES edges
//   - Aggregate per-language stats into combined output
func RunScopeResolutionPhase(input *RunScopeResolutionPhaseInput) *ScopeResolutionOutput {
	if input.Registry == nil || len(input.Registry.All()) == 0 {
		return NoopScopeResolutionOutput
	}

	// ── Partition files by language (O(F) single pass) ──────────────────
	// Mirrors TS phase.ts filesByLang bucketing.
	allResolvers := input.Registry.All()
	filesByLang := make(map[shared.SupportedLanguage][]string)
	for _, fp := range input.FilePaths {
		lang := shared.GetLanguageFromFilename(fp)
		if lang == "" {
			continue
		}
		if _, hasResolver := allResolvers[lang]; !hasResolver {
			continue
		}
		filesByLang[lang] = append(filesByLang[lang], fp)
	}

	// ── Pre-count for progress reporting ────────────────────────────────
	totalScopeFiles := 0
	totalScopeLangs := 0
	for lang := range allResolvers {
		if count := len(filesByLang[lang]); count > 0 {
			totalScopeLangs++
			totalScopeFiles += count
		}
	}

	if totalScopeFiles == 0 {
		return NoopScopeResolutionOutput
	}

	const scopePctStart = 90
	const scopePctRange = 8
	processedScopeFiles := 0

	if input.OnProgress != nil {
		input.OnProgress("scopeResolution", scopePctStart, "Resolving types")
	}

	// ── Build graph node lookup ONCE (shared across all languages) ──────
	// Mirrors TS phase.ts: "buildGraphNodeLookup(ctx.graph)" built once
	// and shared — the previous per-language rebuild burned O(N×L) CPU+heap.
	sharedNodeLookup := BuildGraphNodeLookup(input.Graph)

	// ── Aggregate accumulators ──────────────────────────────────────────
	var totalFiles int
	var totalImports int
	var totalRefs int
	var anyRan bool
	var allOutcomes []ResolutionOutcome
	perLanguage := make(map[shared.SupportedLanguage]*ScopeResolutionLangStats)

	// ── Per-language resolution loop ────────────────────────────────────
	// Mirrors TS phase.ts: "for (const [lang, provider] of SCOPE_RESOLVERS)"
	for lang, provider := range allResolvers {
		langFiles := filesByLang[lang]
		if len(langFiles) == 0 {
			continue
		}

		// ── Step 1: Load per-language resolution config ──────────────
		// One I/O round trip per workspace pass — cached implicitly by
		// the result handed to every resolveImportTarget call.
		var resolutionConfig interface{}
		if provider.LoadResolutionConfig() != nil {
			resolutionConfig = provider.LoadResolutionConfig()(input.RepoPath)
		}

		// ── Step 2: Read file contents ───────────────────────────────
		// Mirrors TS: "await readFileContents(ctx.repoPath, primaryFilePaths)"
		contents := core.ReadFileContents(input.RepoPath, langFiles)

		// ── Step 3: Build allFilePaths set ───────────────────────────
		// The full workspace file set for import resolution.
		allFilePaths := input.AllPathSet
		if allFilePaths == nil {
			allFilePaths = make(map[string]bool, len(input.FilePaths))
			for _, p := range input.FilePaths {
				allFilePaths[p] = true
			}
		}

		// ── Step 4: Parse files via scope extractor ──────────────────
		// Mirrors TS: extractParsedFile loop in runScopeResolution.
		// The Go architecture currently does not have a disk-backed
		// ParsedFile store or preExtractedParsedFiles, so we extract
		// each file on-the-fly using the provider's LanguageProvider.
		lp := provider.LanguageProvider()
		var parsedFiles []*shared.ParsedFile
		for _, fp := range langFiles {
			content, ok := contents[fp]
			if !ok {
				continue
			}
			pf := extractParsedFileFromProvider(lp, fp, []byte(content))
			if pf != nil {
				parsedFiles = append(parsedFiles, pf)
			}
		}

		if len(parsedFiles) == 0 {
			continue
		}

		// ── Step 5: Build/create semantic model ──────────────────────
		// The semantic model is built during the parse phase and shared
		// across all languages. If not provided, create one per language.
		semModel := input.SemanticModel
		if semModel == nil {
			semModel = model.CreateSemanticModel()
		}

		// ── Step 6: Progress reporting ───────────────────────────────
		langFileCount := len(langFiles)
		langLabel := string(lang)
		if len(langLabel) > 0 {
			langLabel = string(lang[0]) + langLabel[1:]
		}
		currentLangIdx := len(perLanguage) + 1
		langTag := langLabel
		if totalScopeLangs > 1 {
			langTag = fmt.Sprintf("%s [%d/%d]", langLabel, currentLangIdx, totalScopeLangs)
		}

		if input.OnProgress != nil {
			pct := scopePctStart + (processedScopeFiles*scopePctRange)/totalScopeFiles
			input.OnProgress("scopeResolution", pct,
				fmt.Sprintf("Resolving types — %s, %d files", langTag, langFileCount))
		}

		// ── Step 7: Run scope resolution ─────────────────────────────
		// Mirrors TS: "runScopeResolution(input, provider)"
		result, err := RunScopeResolution(&RunScopeResolutionInput{
			Graph:                   input.Graph,
			Provider:                provider,
			ParsedFiles:             parsedFiles,
			NodeLookup:              sharedNodeLookup,
			RepoPath:                input.RepoPath,
			ResolutionConfig:        resolutionConfig,
			AllFilePaths:            allFilePaths,
			SemanticModel:           semModel,
			OnWarn:                  input.OnWarn,
			RecordResolutionOutcome: func(outcome ResolutionOutcome) { allOutcomes = append(allOutcomes, outcome) },
		})

		if err != nil {
			if input.OnWarn != nil {
				input.OnWarn(fmt.Sprintf("[scope-resolution:%s] %v", lang, err))
			}
			// Record the language as having run but with zero stats
			// so telemetry shows it was attempted.
			perLanguage[lang] = &ScopeResolutionLangStats{}
			anyRan = true
			processedScopeFiles += langFileCount
			continue
		}

		// ── Step 8: Accumulate stats ─────────────────────────────────
		processedScopeFiles += langFileCount
		anyRan = true
		totalFiles += result.FilesProcessed
		totalImports += result.ImportsEmitted
		totalRefs += result.ReferenceEdgesEmitted
		perLanguage[lang] = &ScopeResolutionLangStats{
			FilesProcessed:        result.FilesProcessed,
			ImportsEmitted:        result.ImportsEmitted,
			ReferenceEdgesEmitted: result.ReferenceEdgesEmitted,
		}

		// Log in dev mode
		fmt.Printf("[scope-resolution:%s] %d files → %d IMPORTS + %d reference edges\n",
			lang, result.FilesProcessed, result.ImportsEmitted, result.ReferenceEdgesEmitted)
	}

	// ── Final progress report ───────────────────────────────────────────
	if anyRan && input.OnProgress != nil {
		input.OnProgress("scopeResolution", scopePctStart+scopePctRange, "Resolving types — complete")
	}

	if !anyRan {
		return NoopScopeResolutionOutput
	}

	return &ScopeResolutionOutput{
		Ran:                   true,
		FilesProcessed:        totalFiles,
		ImportsEmitted:        totalImports,
		ReferenceEdgesEmitted: totalRefs,
		ResolutionOutcomes:    allOutcomes,
		PerLanguage:           perLanguage,
	}
}

// extractParsedFileFromProvider drives the LanguageProvider's extraction
// pipeline to produce a ParsedFile from source content.
// Mirrors TS extractParsedFile() which calls provider hooks to build
// scope tree, imports, definitions, and reference sites.
func extractParsedFileFromProvider(
	lp core.LanguageProvider,
	filePath string,
	source []byte,
) *shared.ParsedFile {
	// Extract symbols and scopes using the LanguageProvider.
	symbols, err := lp.ExtractSymbols(source, filePath, nil)
	if err != nil {
		return nil
	}

	scopes, err := lp.ExtractScopes(source, filePath)
	if err != nil {
		return nil
	}

	importEdges, err := lp.ExtractImports(source, filePath, nil)
	if err != nil {
		return nil
	}

	// Build ParsedFile from extracted data.
	// Construct the module scope and child scopes.
	moduleScope := &shared.Scope{
		ID:   shared.ScopeID(filePath + ":module"),
		Kind: shared.ScopeKindModule,
	}

	var childScopes []*shared.Scope
	for i := range scopes {
		si := &scopes[i]
		childScopes = append(childScopes, &shared.Scope{
			ID:   shared.ScopeID(si.ScopeID),
			Kind: scopeKindFromString(si.Kind),
		})
	}

	allScopes := make([]*shared.Scope, 0, 1+len(childScopes))
	allScopes = append(allScopes, moduleScope)
	allScopes = append(allScopes, childScopes...)

	// Convert LanguageProvider extraction results to shared.SymbolDefinition.
	// The LanguageProvider returns []shared.SymbolDefinition directly.
	var defs []shared.SymbolDefinition
	for i := range symbols {
		defs = append(defs, symbols[i])
	}

	// Convert ImportEdges to ParsedImports.
	var parsedImports []shared.ParsedImport
	for _, ie := range importEdges {
		targetRaw := ie.ImportPath
		parsedImports = append(parsedImports, shared.ParsedImport{
			Kind:      shared.ParsedImportNamed,
			LocalName: ie.ImportPath,
			TargetRaw: &targetRaw,
		})
	}

	return &shared.ParsedFile{
		FilePath:      filePath,
		ModuleScope:   moduleScope,
		Scopes:        allScopes,
		ParsedImports: parsedImports,
		LocalDefs:     defs,
	}
}

// scopeKindFromString maps a string kind to a ScopeKind constant.
func scopeKindFromString(kind string) shared.ScopeKind {
	switch kind {
	case "module":
		return shared.ScopeKindModule
	case "class":
		return shared.ScopeKindClass
	case "function":
		return shared.ScopeKindFunction
	case "block":
		return shared.ScopeKindBlock
	case "namespace":
		return shared.ScopeKindNamespace
	default:
		return shared.ScopeKindBlock
	}
}