// Package csharp — C# import target resolution.
// Resolves a C# using directive (e.g. "System.Collections.Generic")
// to repo-relative file paths, using .csproj RootNamespace and workspace file set.
// Ported from TS languages/csharp/import-target.ts.
package csharp

// CsharpResolveContext holds context for C# import target resolution.
type CsharpResolveContext struct {
	RootNamespace string            // from .csproj <RootNamespace>
	ProjectDir    string            // directory containing .csproj
	AllFilePaths  map[string]bool   // workspace file set
}

// ResolveCsharpImportTarget resolves a C# using namespace path to one or
// more repo-relative file paths. Returns nil for unresolvable/external
// namespaces (e.g. System.*, Microsoft.*), a single entry for resolved,
// or multiple for ambiguous targets.
//
// Mirrors TS resolveCsharpImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig).
// TODO: full implementation — currently returns nil.
func ResolveCsharpImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	// TODO: reject stdlib namespaces (System.*, Microsoft.*),
	// strip RootNamespace prefix, match remaining path against allFilePaths.
	return nil
}

// ResolveCsharpTarget resolves a C# import target using a pre-built
// CsharpResolveContext. This is the internal resolution function that
// the scope_resolver.go ResolveImportTarget delegates to after
// extracting context from resolutionConfig.
//
// TODO: full implementation — currently returns nil.
func ResolveCsharpTarget(ctx *CsharpResolveContext, targetRaw string, fromFile string) []string {
	// TODO: implement C#-specific resolution logic using context.
	return nil
}