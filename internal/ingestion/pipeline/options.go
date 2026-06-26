// Pipeline options — per-run configuration.
//
// Ported from gitnexus pipeline.ts PipelineOptions (189 lines).
// Only options relevant to the 9-language codetrip subset are retained;
// worker-pool, parse-cache, and env-var options are simplified for Go.
package pipeline

// PipelineOptions controls pipeline behaviour for a single run.
type PipelineOptions struct {
	// SkipGraphPhases skips MRO, community detection, and process extraction
	// for faster test runs. The pruneLocalSymbols phase still runs — it is
	// graph construction (it cleans up inert local symbols), not graph analysis.
	SkipGraphPhases bool

	// PDG enables control-flow-graph / PDG substrate construction.
	// Off by default: scope-resolution emits no BasicBlock nodes or CFG edges.
	PDG bool

	// PDGMaxFunctionLines is the per-function source-line cap for CFG construction.
	// 0 means no cap (unlimited). Over-cap functions are skipped.
	PDGMaxFunctionLines int

	// PDGMaxEdgesPerFunction is the per-function CFG edge cap.
	// 0 means no cap (unlimited).
	PDGMaxEdgesPerFunction int

	// PDGMaxReachingDefEdgesPerFunction is the per-function REACHING_DEF edge cap.
	// 0 means no cap (unlimited).
	PDGMaxReachingDefEdgesPerFunction int

	// PDGMaxTaintFindingsPerFunction is the per-function taint findings cap.
	// 0 means no cap (unlimited).
	PDGMaxTaintFindingsPerFunction int

	// PDGMaxTaintHops is the per-finding taint hop cap.
	// 0 means no cap (unlimited).
	PDGMaxTaintHops int

	// KeepLocalValueSymbols keeps inert block-local value symbols (Const/Variable/Static)
	// that the pruneLocalSymbols phase would otherwise drop.
	KeepLocalValueSymbols bool

	// FetchWrappers lists extra fetch-wrapper function names to treat as HTTP consumers.
	// The routes phase unions these with the auto-detected fetch() wrappers.
	FetchWrappers []string

	// Languages restricts the pipeline to a subset of the 9 supported languages.
	// Empty/nil means all languages are processed.
	Languages []string
}