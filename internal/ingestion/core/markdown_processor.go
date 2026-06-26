// Markdown Processor — extracts wiki links and headings from .md files.
//
// Mirrors TS markdown-processor.ts, skeleton for codetrip.
// Markdown files are treated as lightweight nodes: headings become
// NamedSection symbols and [[wiki-links]] become IMPORTS relationships.
// Deferred to Phase 3.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// MarkdownSection represents a heading + its content block.
type MarkdownSection struct {
	Title     string
	Level     int // 1-6 for # through ######
	LineStart int
	LineEnd   int
}

// MarkdownLink represents a [[wiki-link]] or [ref](url) found in markdown.
type MarkdownLink struct {
	Target    string
	Display   string
	IsWikiLink bool
	Line      int
}

// ProcessMarkdownResult holds what we extracted from a single .md file.
type ProcessMarkdownResult struct {
	Sections []MarkdownSection
	Links    []MarkdownLink
	Title    string // first H1
}

// ProcessMarkdownFile extracts sections and links from markdown content.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func ProcessMarkdownFile(content string, filePath string) (*ProcessMarkdownResult, error) {
	// TODO(Phase 3): parse headings, wiki-links, inline links
	result := &ProcessMarkdownResult{
		Sections: []MarkdownSection{},
		Links:    []MarkdownLink{},
	}
	return result, nil
}

// AddMarkdownToGraph writes ProcessMarkdownResult nodes/edges into the KnowledgeGraph.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func AddMarkdownToGraph(graph shared.KnowledgeGraph, filePath string, result *ProcessMarkdownResult) error {
	// TODO(Phase 3): create NamedSection nodes + IMPORTS edges for links
	return nil
}