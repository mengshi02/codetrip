// Package javascript — Import-target resolver for JavaScript.
// Delegates to the TypeScript ResolveTsTarget standard-strategy resolver
// with language = SupportedLanguageJavaScript so the resolver tries
// .js / .jsx extensions in addition to .ts / .tsx.
// No tsconfig.json path-alias support (JavaScript projects don't use
// tsconfig.json compilerOptions.paths in general).
//
// Ported from TS languages/javascript/import-target.ts.
package javascript

import (
	typescript "github.com/mengshi02/codetrip/internal/ingestion/languages/typescript"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JsResolveContext is an alias for TsResolveContext — JS shares the same
// resolution context shape, but with Language = JavaScript and TsconfigPaths = nil.
type JsResolveContext = typescript.TsResolveContext

// ResolveJsImportTarget resolves a JS import using the TypeScript resolver
// with JavaScript language flag and no tsconfig paths.
func ResolveJsImportTarget(parsedImport shared.ParsedImport, workspaceIndex shared.WorkspaceIndex) *string {
	return typescript.ResolveTsImportTarget(parsedImport, workspaceIndex)
}

// ResolveJsTarget resolves a JS import target using the TypeScript resolver
// with JavaScript language flag and no tsconfig paths.
func ResolveJsTarget(targetRaw string, ctx JsResolveContext) *string {
	ctx.Language = shared.SupportedLanguageJavaScript
	ctx.TsconfigPaths = nil
	return typescript.ResolveTsTarget(targetRaw, ctx)
}