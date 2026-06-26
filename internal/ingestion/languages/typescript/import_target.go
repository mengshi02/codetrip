// Package typescript — TypeScript import target resolution.
// Resolves a TypeScript/TSX import path (e.g. "./utils", "@/services/user")
// to a repo-relative file path, using workspace file set and optional
// tsconfig path aliases.
//
// TypeScript import resolution strategies:
//  1. tsconfig path aliases (@/ → src/, ~/ → src/)
//  2. Relative imports (./foo → foo.ts)
//  3. Index file resolution (./utils → utils/index.ts)
//  4. Extension variants (.ts, .tsx, .d.ts, .js)
//  5. Suffix-based package matching
//
// Ported from TS languages/typescript/import-target.ts.
package typescript

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// TsResolveContext holds the workspace context for TypeScript import
// resolution. Includes file sets, suffix indexes, resolution cache,
// and tsconfig path aliases.
type TsResolveContext struct {
	FromFile          string
	AllFilePaths      map[string]bool
	AllFileList       []string
	NormalizedFileList []string
	ResolveCache      map[string]*string // targetRaw → resolved path or nil
	Index             interface{}        // SuffixIndex — typed concretely later
	TsconfigPaths     *core.TsconfigPaths
	Language          shared.SupportedLanguage // TypeScript or JavaScript
}

// ResolveTsImportTarget resolves a ParsedImport to a workspace file path.
// Returns nil for dynamic-unresolved imports without a string literal,
// empty targetRaw, or unresolvable targets.
//
// Mirrors TS resolveTsImportTarget(parsedImport, workspaceIndex): string | null.
// TODO: full implementation — currently returns nil.
func ResolveTsImportTarget(parsedImport shared.ParsedImport, workspaceIndex shared.WorkspaceIndex) *string {
	// TODO: extract targetRaw from parsedImport, narrow workspace context,
	// resolve using standard-strategy resolver.
	return nil
}

// ResolveTsTarget resolves a raw module-path string to a workspace file
// path using the standard-strategy resolver. Operates directly on the
// source string without requiring a ParsedImport.
//
// Mirrors TS resolveTsTarget(targetRaw, ctx): string | null.
// TODO: full implementation — currently returns nil.
func ResolveTsTarget(targetRaw string, ctx TsResolveContext) *string {
	// TODO: try tsconfig alias resolution, then relative path resolution,
	// then suffix-based package matching.
	return nil
}