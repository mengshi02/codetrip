// Package csharp — C# import decomposer: split a C# using directive
// into ParsedImport records.
// C# has three main using variants:
//
//	using System;             → namespace import (all types visible)
//	using System.IO;          → namespace import (nested namespace)
//	using static System.Math; → using static (member import)
//	using MyAlias = System;   → alias import (type/namespace alias)
//
// Ported from TS languages/csharp/import-decomposer.ts.
package csharp

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// SplitCsharpImportStatement takes a raw C# using capture and decomposes it
// into one or more ParsedImport records based on the using directive type.
//
// Mirrors TS splitCsharpImportStatement(captures).
// TODO: full implementation — currently returns empty slice.
func SplitCsharpImportStatement(captures shared.CaptureMatch) []shared.ParsedImport {
	// TODO: parse @using-namespace, @using-static, @using-alias captures,
	// produce ParsedImport records with appropriate Kind (namespace, named, alias).
	return nil
}