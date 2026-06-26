// Phase: parse
//
// Chunked parse + resolve loop: reads source in byte-budget chunks,
// dispatches to GoroutinePool for concurrent parsing via gotreesitter
// (pure Go, goroutine-safe), resolves imports/calls/heritage, and
// synthesizes wildcard import bindings.
//
// This phase encapsulates the entire runChunkedParseAndResolve function.
// The chunk loop is a memory optimization internal to this phase,
// not a phase boundary.
//
// @deps    structure, markdown
// @reads   scannedFiles, allPaths, totalFiles (from structure)
// @writes  graph (Symbol nodes, IMPORTS/CALLS/EXTENDS/IMPLEMENTS/ACCESSES edges)
// @output  exportedTypeMap, allFetchCalls, allExtractedRoutes, allDecoratorRoutes,
//
//	allToolDefs, allORMQueries, bindingAccumulator, parsedFiles
//
// Ported from gitnexus pipeline-phases/parse.ts (100 lines).
package pipeline

// ── Output type ──────────────────────────────────────────────────────────

// ParseOutput is the result of the parse phase.
type ParseOutput struct {
	ExportedTypeMap     map[string]map[string]string
	AllFetchCalls       []interface{} // ExtractedFetchCall — typed later
	AllFetchWrapperDefs []interface{}
	AllExtractedRoutes  []interface{} // ExtractedRoute
	AllDecoratorRoutes  []interface{} // ExtractedDecoratorRoute
	AllToolDefs         []interface{} // ExtractedToolDef
	AllORMQueries       []interface{} // ExtractedORMQuery
	BindingAccumulator  interface{}   // *core.BindingAccumulator
	Model               interface{}   // MutableSemanticModel
	AllPaths            []string
	AllPathSet          map[string]bool
	TotalFiles          int
	UsedWorkerPool      bool
	ParsedFiles         []interface{} // []shared.ParsedFile
}

// ── Phase implementation ─────────────────────────────────────────────────

// parsePhaseImpl implements the parse phase.
type parsePhaseImpl struct{ basePhase }

func (p *parsePhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	structureOut, err := GetPhaseOutputTyped[*StructureOutput](deps, "structure")
	if err != nil {
		return nil, err
	}

	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 20, "Parsing source files...")
	}

	// Determine chunk byte budget
	budget := ResolveChunkByteBudget(0) // use default

	// Run the chunked parse + resolve loop
	implResult, err := RunChunkedParseAndResolve(
		ctx.Graph,
		structureOut.ScannedFiles,
		structureOut.AllPaths,
		structureOut.TotalFiles,
		ctx.RepoPath,
		budget,
	)
	if err != nil {
		return nil, err
	}

	// Convert ParseImplResult → ParseOutput
	return &ParseOutput{
		ExportedTypeMap:     implResult.ExportedTypeMap,
		AllFetchCalls:       implResult.AllFetchCalls,
		AllFetchWrapperDefs: implResult.AllFetchWrapperDefs,
		AllExtractedRoutes:  implResult.AllExtractedRoutes,
		AllDecoratorRoutes:  implResult.AllDecoratorRoutes,
		AllToolDefs:         implResult.AllToolDefs,
		AllORMQueries:       implResult.AllORMQueries,
		BindingAccumulator:  implResult.BindingAccumulator,
		AllPaths:            implResult.AllPaths,
		AllPathSet:          structureOut.AllPathSet,
		TotalFiles:          implResult.TotalFiles,
		UsedWorkerPool:      implResult.UsedWorkerPool,
		ParsedFiles:         nil, // TODO: convert []shared.ParsedFile → []interface{}
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var parsePhase = &parsePhaseImpl{basePhase{name: "parse", deps: []string{"structure", "markdown"}}}