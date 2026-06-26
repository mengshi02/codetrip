// Pipeline orchestrator — buildPhaseList + RunPipelineFromRepo.
//
// The pipeline is composed of named phases with explicit dependencies.
// Phase dependency graph (codetrip 9-language subset — cobol removed):
//
//	scan → structure → markdown → parse → [routes, tools, orm]
//	  → crossFile → scopeResolution → pruneLocalSymbols
//	  → mro → communities → processes
//
// Ported from gitnexus pipeline.ts (296 lines).
package pipeline

import (
	"time"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ── Base phase ───────────────────────────────────────────────────────────

// basePhase provides common fields for all phase implementations.
type basePhase struct {
	name string
	deps []string
}

func (p *basePhase) Name() string   { return p.name }
func (p *basePhase) Deps() []string { return p.deps }

// ── Phase list builder ───────────────────────────────────────────────────

// BuildPhaseList builds the ordered phase list for the given options.
// Graph phases (mro, communities, processes) are excluded when
// SkipGraphPhases is true.
func BuildPhaseList(options *PipelineOptions) []PipelinePhase {
	return NewPhaseRegistry().
		Register(scanPhase).
		Register(structurePhase).
		Register(markdownPhase).
		Register(parsePhase).
		Register(routesPhase).
		Register(toolsPhase).
		Register(ormPhase).
		Register(crossFilePhase).
		Register(scopeResolutionPhase).
		Register(pruneLocalSymbolsPhase).
		Register(mroPhase, func(o *PipelineOptions) bool {
			if o == nil {
				return true
			}
			return !o.SkipGraphPhases
		}).
		Register(communitiesPhase, func(o *PipelineOptions) bool {
			if o == nil {
				return true
			}
			return !o.SkipGraphPhases
		}).
		Register(processesPhase, func(o *PipelineOptions) bool {
			if o == nil {
				return true
			}
			return !o.SkipGraphPhases
		}).
		Build(options)
}

// ── Pipeline result ──────────────────────────────────────────────────────

// PipelineResult is the final output of the pipeline.
type PipelineResult struct {
	Graph              shared.KnowledgeGraph
	RepoPath           string
	TotalFileCount     int
	CommunityResult    interface{} // typed later
	ProcessResult      interface{} // typed later
	ResolutionOutcomes interface{} // typed later
	UsedWorkerPool     bool
}

// ── Pipeline orchestrator ────────────────────────────────────────────────

// RunPipelineFromRepo runs the full ingestion pipeline on a repository.
// It builds the phase list and executes phases in dependency order.
// The caller must provide a KnowledgeGraph implementation for the pipeline
// to populate — the pipeline itself is purely additive.
func RunPipelineFromRepo(
	repoPath string,
	graph shared.KnowledgeGraph,
	onProgress func(phase string, percent int, message string),
	options *PipelineOptions,
) (*PipelineResult, error) {
	pipelineStart := time.Now().UnixMilli()

	phases := BuildPhaseList(options)

	ctx := &PipelineContext{
		RepoPath:      repoPath,
		Graph:         graph,
		OnProgress:    onProgress,
		Options:       options,
		PipelineStart: pipelineStart,
	}

	results, err := RunPipeline(phases, ctx)
	if err != nil {
		return nil, err
	}

	// Extract final results for the PipelineResult contract
	parseOut, _ := GetPhaseOutputTyped[*ParseOutput](results, "parse")
	totalFiles := 0
	usedWorkerPool := false
	if parseOut != nil {
		totalFiles = parseOut.TotalFiles
		usedWorkerPool = parseOut.UsedWorkerPool
	}

	var communityResult interface{}
	var processResult interface{}
	var resolutionOutcomes interface{}

	scopeOut, _ := GetPhaseOutputTyped[*ScopeResolutionOutput](results, "scopeResolution")
	if scopeOut != nil {
		resolutionOutcomes = scopeOut.ResolutionOutcomes
	}

	if options == nil || !options.SkipGraphPhases {
		commOut, _ := GetPhaseOutputTyped[*CommunitiesOutput](results, "communities")
		if commOut != nil {
			communityResult = commOut.CommunityResult
		}
		procOut, _ := GetPhaseOutputTyped[*ProcessesOutput](results, "processes")
		if procOut != nil {
			processResult = procOut.ProcessResult
		}
	}

	if onProgress != nil {
		msg := "Graph complete! (graph phases skipped)"
		if communityResult != nil && processResult != nil {
			msg = "Graph complete!"
		}
		onProgress("complete", 100, msg)
	}

	return &PipelineResult{
		Graph:              graph,
		RepoPath:           repoPath,
		TotalFileCount:     totalFiles,
		CommunityResult:    communityResult,
		ProcessResult:      processResult,
		ResolutionOutcomes: resolutionOutcomes,
		UsedWorkerPool:     usedWorkerPool,
	}, nil
}