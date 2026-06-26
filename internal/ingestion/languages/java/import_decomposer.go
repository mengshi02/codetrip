// Package java — Java import decomposer: split a Java import declaration
// into ParsedImport records (one per imported name or wildcard).
//
// Java import statements:
//
//	import com.foo.Bar;      → single named import
//	import com.foo.*;        → wildcard (import-on-demand)
//	import static com.foo.Bar.method; → static single import
//	import static com.foo.Bar.*;      → static wildcard import
//
// Ported from TS languages/java/import-decomposer.ts.
package java

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// SplitJavaImportStatement takes a raw Java import capture and decomposes it
// into one ParsedImport record. Java imports are always single (no grouped imports
// like Go), but static imports need special handling.
//
// Mirrors TS splitJavaImportStatement(captures).
// TODO: full implementation — currently returns empty slice.
func SplitJavaImportStatement(captures shared.CaptureMatch) []shared.ParsedImport {
	// TODO: extract @import-path capture, determine Kind
	// (named / wildcard / static-named / static-wildcard),
	// fill ImportPath, ImportedName, and IsStatic fields.
	return nil
}