// Phase: scan
//
// Walks the repository filesystem and collects file paths + sizes.
// Does NOT read file contents — that happens in downstream phases.
//
// @deps    (none — this is the pipeline root)
// @reads   repoPath (filesystem)
// @writes  graph (nothing yet — just returns scanned paths)
// @output  ScannedFile[], allPaths[], totalFiles
//
// Ported from gitnexus pipeline-phases/scan.ts (60 lines).
package pipeline

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ── Output type ──────────────────────────────────────────────────────────

// ScanOutput is the result of the scan phase.
type ScanOutput struct {
	ScannedFiles []core.ScannedFile
	AllPaths     []string
	TotalFiles   int
}

// ScannedFile is an alias for core.ScannedFile.
type ScannedFile = core.ScannedFile

// ── Phase implementation ─────────────────────────────────────────────────

// scanPhaseImpl implements the scan phase.
type scanPhaseImpl struct{ basePhase }

func (p *scanPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 0, "Scanning repository...")
	}

	// Walk the repository filesystem — stat files, get paths + sizes
	scanned, err := core.WalkRepositoryPaths(
		ctx.RepoPath,
		core.DefaultMaxFileSizeBytes,
		func(current, total int, filePath string) {
			if ctx.OnProgress != nil {
				pct := 0
				if total > 0 {
					pct = current * 100 / total
				}
				ctx.OnProgress("extracting", pct, "Scanning repository...")
			}
		},
	)
	if err != nil {
		return nil, err
	}

	// Build allPaths
	allPaths := make([]string, len(scanned))
	for i, f := range scanned {
		allPaths[i] = f.Path
	}

	return &ScanOutput{
		ScannedFiles: scanned,
		AllPaths:     allPaths,
		TotalFiles:   len(scanned),
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var scanPhase = &scanPhaseImpl{basePhase{name: "scan", deps: nil}}