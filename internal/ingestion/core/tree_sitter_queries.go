// Tree-sitter Queries — language-specific query definitions for symbol extraction.
//
// Mirrors TS tree-sitter-queries.ts, skeleton for codetrip.
// Each language has a set of tree-sitter S-expression queries that extract
// symbols (functions, classes, methods, imports, etc.) from parsed source.
// The actual queries are loaded from .scm files at runtime.
// Deferred to Phase 6 when language providers are fully implemented.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// QueryCategory classifies what a tree-sitter query extracts.
type QueryCategory string

const (
	QueryCategoryDefinitions  QueryCategory = "definitions"
	QueryCategoryReferences   QueryCategory = "references"
	QueryCategoryImports      QueryCategory = "imports"
	QueryCategoryClassMembers QueryCategory = "class_members"
	QueryCategoryDocComments  QueryCategory = "doc_comments"
)

// TSQuerySet holds all tree-sitter queries for a given language.
type TSQuerySet struct {
	Language shared.SupportedLanguage
	Queries  map[QueryCategory]string // raw S-expression strings
}

// QueryResult is a single capture from a tree-sitter query.
type QueryResult struct {
	CaptureName string    // e.g., "name.definition.function"
	NodeText    string    // the text of the captured node
	StartRow    int
	StartCol    int
	EndRow      int
	EndCol      int
	Category    QueryCategory
}

// LoadQueriesForLanguage loads tree-sitter queries from .scm files for a language.
//
// Current status: skeleton — full implementation deferred to Phase 6.
func LoadQueriesForLanguage(lang shared.SupportedLanguage) (*TSQuerySet, error) {
	// TODO(Phase 6): read .scm files from lang/queries/ directory
	return &TSQuerySet{
		Language: lang,
		Queries:  map[QueryCategory]string{},
	}, nil
}

// RunQuery executes a single tree-sitter query on parsed source code.
//
// Current status: skeleton — full implementation deferred to Phase 6.
func RunQuery(query string, source []byte) ([]QueryResult, error) {
	// TODO(Phase 6): use tree-sitter Go bindings to execute query
	return []QueryResult{}, nil
}

// MergeQueryResults combines multiple query results by category.
func MergeQueryResults(results ...[]QueryResult) map[QueryCategory][]QueryResult {
	merged := map[QueryCategory][]QueryResult{}
	for _, batch := range results {
		for _, r := range batch {
			merged[r.Category] = append(merged[r.Category], r)
		}
	}
	return merged
}