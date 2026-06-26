// Phase: structure
//
// Classifies scanned files by language, builds path sets, and computes
// structural metadata (directory tree, file type breakdown).
//
// @deps    scan
// @reads   scannedFiles, allPaths, totalFiles (from scan)
// @writes  graph (File/Folder nodes for structure)
// @output  StructureOutput
//
// Ported from gitnexus pipeline-phases/structure.ts (51 lines).
package pipeline

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ── Output type ──────────────────────────────────────────────────────────

// StructureOutput is the result of the structure phase.
type StructureOutput struct {
	ScannedFiles []core.ScannedFile
	AllPaths     []string
	AllPathSet   map[string]bool
	TotalFiles   int
}

// ── Phase implementation ─────────────────────────────────────────────────

// structurePhaseImpl implements the structure phase.
type structurePhaseImpl struct{ basePhase }

func (p *structurePhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	scanOut, err := GetPhaseOutputTyped[*ScanOutput](deps, "scan")
	if err != nil {
		return nil, err
	}

	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 10, "Building file structure...")
	}

	// Build the File/Folder graph structure
	core.ProcessStructure(ctx.Graph, scanOut.AllPaths)

	// Build allPathSet for fast lookup
	allPathSet := make(map[string]bool, len(scanOut.AllPaths))
	for _, p := range scanOut.AllPaths {
		allPathSet[p] = true
	}

	return &StructureOutput{
		ScannedFiles: scanOut.ScannedFiles,
		AllPaths:     scanOut.AllPaths,
		AllPathSet:   allPathSet,
		TotalFiles:   scanOut.TotalFiles,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var structurePhase = &structurePhaseImpl{basePhase{name: "structure", deps: []string{"scan"}}}