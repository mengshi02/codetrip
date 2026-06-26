package taint

import (
	"fmt"
	"sync"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/ingestion/cfg"
)

// DefaultTaintAnalyzer is the default taint analyzer
// Performs in-process source→sink data flow analysis
type DefaultTaintAnalyzer struct{}

// NewDefaultTaintAnalyzer creates a default taint analyzer
func NewDefaultTaintAnalyzer() *DefaultTaintAnalyzer {
	return &DefaultTaintAnalyzer{}
}

// legacyTaintState represents taint state, uses sync.Pool for reuse
type legacyTaintState struct {
	// Taint state for each variable: variable → tainted (whether tainted)
	taintedVars map[string]bool
	// Taint sources: variable → source information
	taintSources map[string]sourceInfo
	// Propagation paths: variable → list of hops
	taintPaths map[string][]HopInfo
	// Sanitizers on the path
	sanitizedVars map[string]bool
}

type sourceInfo struct {
	name     string
	line     int
	category string
}

var legacyTaintStatePool = sync.Pool{
	New: func() any {
		return &legacyTaintState{
			taintedVars:   make(map[string]bool),
			taintSources:  make(map[string]sourceInfo),
			taintPaths:    make(map[string][]HopInfo),
			sanitizedVars: make(map[string]bool),
		}
	},
}

func getTaintState() *legacyTaintState {
	return legacyTaintStatePool.Get().(*legacyTaintState)
}

func putTaintState(s *legacyTaintState) {
	for k := range s.taintedVars {
		delete(s.taintedVars, k)
	}
	for k := range s.taintSources {
		delete(s.taintSources, k)
	}
	for k := range s.taintPaths {
		delete(s.taintPaths, k)
	}
	for k := range s.sanitizedVars {
		delete(s.sanitizedVars, k)
	}
	legacyTaintStatePool.Put(s)
}

// Analyze performs taint analysis
// 1. Find all source and sink points from FunctionCFG's Sites
// 2. Perform forward data flow propagation along CFG edges
// 3. Check if any source taint propagates to sink
// 4. Check if there are sanitizers blocking the path
// 5. Output list of TaintResult
func (a *DefaultTaintAnalyzer) Analyze(fcfg *cfg.FunctionCFG, registry *TaintModelRegistry) ([]TaintResult, error) {
	if fcfg == nil {
		return nil, fmt.Errorf("functioncfg is nil")
	}
	if registry == nil {
		return nil, fmt.Errorf("taint model registry is nil")
	}

	// Build predecessor/successor mapping
	succMap := make(map[string][]cfgEdge, len(fcfg.Blocks))
	for _, edge := range fcfg.Edges {
		succMap[edge.From] = append(succMap[edge.From], cfgEdge{
			to:       edge.To,
			edgeType: edge.EdgeType,
		})
	}

	// Build block ID to block mapping
	blockMap := make(map[string]*cfg.BasicBlock, len(fcfg.Blocks))
	for i := range fcfg.Blocks {
		blockMap[fcfg.Blocks[i].ID] = &fcfg.Blocks[i]
	}

	// Build line number to Statements mapping
	lineStatements := make(map[int]*cfg.StatementFacts)
	for i := range fcfg.Statements {
		lineStatements[fcfg.Statements[i].Line] = &fcfg.Statements[i]
	}

	// Identify source and sink points
	type sourcePoint struct {
		site     cfg.SiteRecord
		spec     SourceSpec
		varName  string // tainted variable name
	}
	type sinkPoint struct {
		site    cfg.SiteRecord
		spec    SinkSpec
		varName string // used tainted variable
	}

	var sources []sourcePoint
	var sinks []sinkPoint
	sanitizerLines := make(map[int]SanitizerSpec) // line → sanitizer

	for _, site := range fcfg.Sites {
		switch site.SiteType {
		case "call", "member-read":
			// Check if it's a source
			if spec, ok := registry.IsSource(site.Symbol); ok {
				// Find the variable defined on this line as the tainted variable
				varName := findDefinedVar(lineStatements, site.Line)
				if varName == "" {
					varName = "_taint_" + site.Symbol
				}
				sources = append(sources, sourcePoint{
					site:    site,
					spec:    spec,
					varName: varName,
				})
			}

			// Check if it's a sink
			if spec, ok := registry.IsSink(site.Symbol); ok {
				// Find the variable used on this line
				varName := findUsedVar(lineStatements, site.Line)
				sinks = append(sinks, sinkPoint{
					site:    site,
					spec:    spec,
					varName: varName,
				})
			}

			// Check if it's a sanitizer
			if spec, ok := registry.IsSanitizer(site.Symbol); ok {
				sanitizerLines[site.Line] = spec
			}

		case "construct":
			// Constructor calls can also be sinks
			if spec, ok := registry.IsSink(site.Symbol); ok {
				varName := findUsedVar(lineStatements, site.Line)
				sinks = append(sinks, sinkPoint{
					site:    site,
					spec:    spec,
					varName: varName,
				})
			}
		}
	}

	// If no source or sink, return empty
	if len(sources) == 0 || len(sinks) == 0 {
		return nil, nil
	}

	// Forward data flow propagation for taints
	state := getTaintState()
	defer putTaintState(state)

	// Initialization: mark source variables as tainted
	for _, src := range sources {
		state.taintedVars[src.varName] = true
		state.taintSources[src.varName] = sourceInfo{
			name:     src.spec.Name,
			line:     src.site.Line,
			category: src.spec.Category,
		}
		state.taintPaths[src.varName] = []HopInfo{
			{NodeID: src.site.NodeID, Line: src.site.Line},
		}
	}

	// Perform forward propagation along CFG edges
	// Use worklist algorithm
	type workItem struct {
		blockID string
	}

	visited := make(map[string]bool)
	workList := make([]workItem, 0, len(fcfg.Blocks))

	// Start from entry block
	if len(fcfg.Blocks) > 0 {
		workList = append(workList, workItem{blockID: fcfg.Blocks[0].ID})
	}

	for len(workList) > 0 {
		item := workList[0]
		workList = workList[1:]

		if visited[item.blockID] {
			continue
		}
		visited[item.blockID] = true

		block := blockMap[item.blockID]
		if block == nil {
			continue
		}

		// Process each line in the block
		for line := block.StartLine; line <= block.EndLine; line++ {
			// Check sanitizer
			if spec, ok := sanitizerLines[line]; ok {
				// Sanitize all tainted variables
				for v := range state.taintedVars {
					state.sanitizedVars[v] = true
					// Record sanitization path
					state.taintPaths[v] = append(state.taintPaths[v], HopInfo{
						NodeID: fmt.Sprintf("sanitizer:%d:%s", line, spec.Name),
						Line:   line,
					})
				}
			}

			// Process statements: variable definition and propagation
			if stmt, ok := lineStatements[line]; ok {
				// Check if any tainted variable is used
				usesTainted := false
				for _, usedVar := range stmt.Uses {
					if state.taintedVars[usedVar] && !state.sanitizedVars[usedVar] {
						usesTainted = true
						break
					}
				}

				// If a tainted variable is used, propagate taint to defined variables
				if usesTainted {
					for _, defVar := range stmt.Defines {
						// Find the used tainted variable
						for _, usedVar := range stmt.Uses {
							if state.taintedVars[usedVar] {
								state.taintedVars[defVar] = true
								// Inherit taint source
								if src, ok := state.taintSources[usedVar]; ok {
									state.taintSources[defVar] = src
								}
								// Inherit propagation path
								if path, ok := state.taintPaths[usedVar]; ok {
									newPath := make([]HopInfo, len(path))
									copy(newPath, path)
									newPath = append(newPath, HopInfo{
										NodeID: stmt.StatementID,
										Line:   line,
									})
									state.taintPaths[defVar] = newPath
								}
								// Inherit sanitization state
								if state.sanitizedVars[usedVar] {
									state.sanitizedVars[defVar] = true
								}
							}
						}
					}
				}
			}
		}

		// Add successor blocks to worklist
		for _, edge := range succMap[item.blockID] {
			if !visited[edge.to] {
				workList = append(workList, workItem{blockID: edge.to})
			}
		}
	}

	// Check if sink points are reached by taint
	var results []TaintResult
	for _, sink := range sinks {
		for _, usedVar := range []string{sink.varName} {
			if state.taintedVars[usedVar] {
				src := state.taintSources[usedVar]
				path := state.taintPaths[usedVar]
				sanitized := state.sanitizedVars[usedVar]

				// Add sink to path
				hopPath := make([]HopInfo, len(path))
				copy(hopPath, path)
				hopPath = append(hopPath, HopInfo{
					NodeID: sink.site.NodeID,
					Line:   sink.site.Line,
				})

				// Calculate confidence
				confidence := 1.0
				if sanitized {
					confidence = 0.3 // Lower confidence after sanitizer processing
				}
				if len(hopPath) > 5 {
					confidence *= 0.8 // Lower confidence for longer paths
				}

				// Determine vulnerability category
				category := determineCategory(src.category, sink.spec.Category)

				results = append(results, TaintResult{
					Category:    category,
					SourceName:  src.name,
					SourceLine:  src.line,
					SinkName:    sink.spec.Name,
					SinkLine:    sink.site.Line,
					HopPath:     hopPath,
					Sanitized:   sanitized,
					Confidence:  confidence,
				})
			}
		}
	}

	return results, nil
}

// cfgEdge is a simplified CFG edge
type cfgEdge struct {
	to       string
	edgeType string
}

// findDefinedVar finds the variable defined on the specified line
func findDefinedVar(lineStatements map[int]*cfg.StatementFacts, line int) string {
	if stmt, ok := lineStatements[line]; ok && len(stmt.Defines) > 0 {
		return stmt.Defines[0]
	}
	return ""
}

// findUsedVar finds the variable used on the specified line
func findUsedVar(lineStatements map[int]*cfg.StatementFacts, line int) string {
	if stmt, ok := lineStatements[line]; ok && len(stmt.Uses) > 0 {
		return stmt.Uses[0]
	}
	return ""
}

// determineCategory determines vulnerability category based on source and sink categories
func determineCategory(sourceCategory, sinkCategory string) string {
	switch {
	case sinkCategory == "code-exec":
		return "command-injection"
	case sinkCategory == "xss":
		return "xss"
	case sinkCategory == "sql-injection":
		return "sql-injection"
	case sinkCategory == "file-write":
		return "path-traversal"
	case sinkCategory == "deserialization":
		return "unsafe-deserialization"
	default:
		return sourceCategory + "-to-" + sinkCategory
	}
}

// EmitTaintResults emits taint analysis results to GraphStore
// Creates TAINTED and TAINT_PATH edges
func EmitTaintResults(gs *graph.GraphStore, results []TaintResult, fcfg *cfg.FunctionCFG) error {
	if gs == nil {
		return fmt.Errorf("graphstore is nil")
	}

	return gs.Batch(func(b *graph.Batch) error {
		for _, result := range results {
			// Create TAINTED edge: source → sink
			sourceID := fmt.Sprintf("%s:taint:source:%d", fcfg.FuncID, result.SourceLine)
			sinkID := fmt.Sprintf("%s:taint:sink:%d", fcfg.FuncID, result.SinkLine)

			taintedEdge := graph.NewEdge(graph.RelTainted, sourceID, sinkID)
			taintedEdge.WithProp("category", result.Category)
			taintedEdge.WithProp("sourceName", result.SourceName)
			taintedEdge.WithProp("sinkName", result.SinkName)
			taintedEdge.WithProp("sanitized", result.Sanitized)
			taintedEdge.WithProp("confidence", result.Confidence)
			if err := b.AddEdge(taintedEdge); err != nil {
				return fmt.Errorf("add tainted edge: %w", err)
			}

			// Create SANITIZES edge (if sanitizer exists)
			if result.Sanitized {
				sanitizesEdge := graph.NewEdge(graph.RelSanitizes, sourceID, sinkID)
				sanitizesEdge.WithProp("category", result.Category)
				if err := b.AddEdge(sanitizesEdge); err != nil {
					return fmt.Errorf("add sanitizes edge: %w", err)
				}
			}

			// Create TAINT_PATH edges connecting path nodes
			for i := 0; i < len(result.HopPath)-1; i++ {
				pathEdge := graph.NewEdge(graph.RelTaintPath, result.HopPath[i].NodeID, result.HopPath[i+1].NodeID)
				pathEdge.WithProp("hopIndex", i)
				pathEdge.WithProp("category", result.Category)
				if err := b.AddEdge(pathEdge); err != nil {
					return fmt.Errorf("add taint path edge: %w", err)
				}
			}
		}
		return nil
	})
}