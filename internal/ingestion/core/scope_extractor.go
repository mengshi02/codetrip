// Scope Extractor — core scope extraction logic shared across languages.
//
// Mirrors TS scope-extractor.ts, skeleton for codetrip.
// Provides the base scope extraction algorithm that language-specific
// providers can override. Handles scope nesting, visibility (exported vs
// private), and scope ID generation.
// Deferred to Phase 6 when tree-sitter integration is complete.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// ScopeExtractionConfig controls how scopes are extracted.
type ScopeExtractionConfig struct {
	MaxNestingDepth int    // max depth to extract (default: unlimited)
	IncludeBlocks   bool   // include block scopes (if/for/while bodies)
	IncludeClosures bool   // include anonymous function/closure scopes
	Language        shared.SupportedLanguage
}

// DefaultScopeExtractionConfig provides sensible defaults.
var DefaultScopeExtractionConfig = ScopeExtractionConfig{
	MaxNestingDepth: 0, // 0 = unlimited
	IncludeBlocks:   false,
	IncludeClosures: true,
}

// ExtractScopesFromTree walks the tree-sitter AST and extracts scope boundaries.
//
// Current status: skeleton — full implementation deferred to Phase 6.
func ExtractScopesFromTree(ast interface{}, filePath string, config ScopeExtractionConfig) ([]ScopeInfo, error) {
	// TODO(Phase 6): walk tree-sitter AST, extract function/class/module scopes
	return []ScopeInfo{}, nil
}

// GenerateScopeID creates a deterministic scope ID from file path + name + kind.
func GenerateScopeID(filePath string, name string, kind string) string {
	return filePath + "::" + kind + ":" + name
}

// BuildScopeTree organizes flat ScopeInfo list into parent-child relationships.
// Returns a map of scopeID → children scopeIDs.
func BuildScopeTree(scopes []ScopeInfo) map[string][]string {
	tree := map[string][]string{}
	for _, s := range scopes {
		if s.ParentID != "" {
			tree[s.ParentID] = append(tree[s.ParentID], s.ScopeID)
		}
	}
	return tree
}