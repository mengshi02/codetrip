// Phase: markdown
//
// Processes markdown files for documentation extraction.
// Headings become NamedSection symbols and [[wiki-links]] become
// IMPORTS relationships.
//
// @deps    structure
// @reads   allPaths (from structure)
// @writes  graph (MarkdownSection nodes, wiki-link IMPORTS edges)
// @output  MarkdownOutput
//
// Ported from gitnexus pipeline-phases/markdown.ts (47 lines).
package pipeline

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ── Output type ──────────────────────────────────────────────────────────

// MarkdownOutput is the result of the markdown phase.
type MarkdownOutput struct {
	// Markdown files found.
	MarkdownFiles []string
	// Number of sections extracted across all markdown files.
	TotalSections int
}

// ── Phase implementation ─────────────────────────────────────────────────

// markdownPhaseImpl implements the markdown phase.
type markdownPhaseImpl struct{ basePhase }

func (p *markdownPhaseImpl) Execute(ctx *PipelineContext, deps map[string]*PhaseResult) (interface{}, error) {
	structureOut, err := GetPhaseOutputTyped[*StructureOutput](deps, "structure")
	if err != nil {
		return nil, err
	}

	if ctx.OnProgress != nil {
		ctx.OnProgress("extracting", 15, "Processing markdown files...")
	}

	// Filter markdown files from allPaths
	var mdFiles []string
	for _, p := range structureOut.AllPaths {
		if strings.HasSuffix(strings.ToLower(p), ".md") ||
			strings.HasSuffix(strings.ToLower(p), ".markdown") ||
			strings.HasSuffix(strings.ToLower(p), ".mdx") {
			mdFiles = append(mdFiles, p)
		}
	}

	// Process each markdown file
	totalSections := 0
	for _, mdPath := range mdFiles {
		content := core.ReadFileContents(ctx.RepoPath, []string{mdPath})
		if text, ok := content[mdPath]; ok {
			result, _ := core.ProcessMarkdownFile(text, mdPath)
			if result != nil {
				totalSections += len(result.Sections)
				_ = result // TODO: add markdown sections to graph
			}
		}
	}

	_ = shared.GenerateID // ensure shared import is used (will be needed for graph writes)

	return &MarkdownOutput{
		MarkdownFiles: mdFiles,
		TotalSections: totalSections,
	}, nil
}

// ── Phase instance ───────────────────────────────────────────────────────

var markdownPhase = &markdownPhaseImpl{basePhase{name: "markdown", deps: []string{"structure"}}}