// Package java — Java import target resolution.
// Resolves a Java import path (e.g. "com.example.MyClass") to a
// repo-relative file path, using package declarations and workspace file set.
// Ported from TS languages/java/import-target.ts.
package java

// JavaResolveContext holds resolution context for a Java import target,
// including the import path, the source file requesting it, and the
// workspace file set.
type JavaResolveContext struct {
	TargetRaw      string         // raw import path string (e.g. "com.example.Foo")
	FromFile       string         // file containing the import statement
	AllFilePaths   map[string]bool // workspace's file set
	PackageDeclMap map[string]string // map of filePath → package declaration
}

// ResolveJavaImportTarget resolves a Java import path to one or more
// repo-relative file paths. Returns nil for unresolvable/external classes,
// a single entry for resolved, or multiple entries for ambiguous targets.
//
// Java imports are absolute — "com.foo.Bar" maps to com/foo/Bar.java
// in the source tree. No tsconfig-like resolution config is needed.
//
// Mirrors TS resolveJavaImportTarget(targetRaw, fromFile, allFilePaths, resolutionConfig).
// TODO: full implementation — currently returns nil.
func ResolveJavaImportTarget(
	targetRaw string,
	fromFile string,
	allFilePaths map[string]bool,
	resolutionConfig interface{},
) []string {
	// TODO: convert dot-separated import path to file path
	// (com.foo.Bar → com/foo/Bar.java), match against allFilePaths.
	return nil
}

// ResolveJavaTarget resolves a Java type reference within a resolution context,
// taking into account the current file's package and any import-on-demand
// declarations.
//
// Mirrors TS resolveJavaTarget(ctx).
// TODO: full implementation — currently returns nil.
func ResolveJavaTarget(ctx JavaResolveContext) []string {
	// TODO: use JavaResolveContext to resolve type names
	// considering same-package visibility and import-on-demand.
	return nil
}