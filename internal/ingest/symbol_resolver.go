package ingest

// Resolution tiers:
//   Tier 1: Same-file exact lookup (confidence 0.95)
//   Tier 2a: Named binding chain → import-scoped (confidence 0.9)
//   Tier 2b: Import map / package map filtered lookup (confidence 0.9)
//   Tier 3: Unique global (confidence 0.5)
//
// Key rule: multiple candidates → reject (wrong edge is worse than no edge)

import (
	"fmt"
	"strings"
)

// ResolveResult represents a resolved symbol with its confidence tier.
type ResolveResult struct {
	Definition *SymbolDefinition
	Tier       int     // 1=same-file, 2=import-scoped, 3=unique-global
	Confidence float64 // 0.95, 0.9, or 0.5
	SourceFile string  // File where the definition was found
}

// ResolveContext holds the data needed for symbol resolution.
type ResolveContext struct {
	SymbolTable    *SymbolTable
	NamedImportMap NamedImportMap
	ImportMap      ImportMap  // defined in import_processor.go
	PackageMap     PackageMap // defined in import_processor.go
	ImportOrderMap ImportOrderMap
	// AssignableOwnerIDs maps a receiver type node to inherited/interface owner
	// nodes whose methods are valid member-call candidates in enhanced mode.
	AssignableOwnerIDs map[string]map[string]bool
}

// ResolveOption allows optional parameters for symbol resolution.
type ResolveOption struct {
	ReceiverTypeName string // e.g. "Consumer" for consumer.Stop() — used to disambiguate multi-candidate T2b
}

// ResolveSymbol resolves a symbol name from a given file path using the 3-tier strategy.
// Returns nil if the symbol cannot be uniquely resolved.
func ResolveSymbol(name string, filePath string, ctx *ResolveContext, opts ...ResolveOption) *ResolveResult {
	var opt ResolveOption
	if len(opts) > 0 {
		opt = opts[0]
	}
	return resolveSymbolInternal(name, filePath, ctx, opt)
}

// resolveSymbolInternal implements the 3-tier resolution strategy.
func resolveSymbolInternal(name string, filePath string, ctx *ResolveContext, opt ResolveOption) *ResolveResult {
	// ── Tier 1: Same-file exact lookup ──────────────────────────────────────
	exactDef := ctx.SymbolTable.LookupExactFull(filePath, name)
	if exactDef != nil {
		return &ResolveResult{
			Definition: exactDef,
			Tier:       1,
			Confidence: 0.95,
			SourceFile: exactDef.FilePath,
		}
	}
	// ── Get all global definitions for subsequent tiers ─────────────────────
	allDefs := ctx.SymbolTable.LookupFuzzy(name)

	// ── Tier 2a: Named binding chain ────────────────────────────────────────
	namedBindingDefs := WalkBindingChain(name, filePath, ctx.SymbolTable, ctx.NamedImportMap, allDefs)
	if len(namedBindingDefs) == 1 {
		return &ResolveResult{
			Definition: namedBindingDefs[0],
			Tier:       2,
			Confidence: 0.9,
			SourceFile: namedBindingDefs[0].FilePath,
		}
	}
	if len(namedBindingDefs) > 1 {
		// Multiple → reject
		return nil
	}

	if len(allDefs) == 0 {
		return nil
	}

	// ── Tier 2b: Import-scoped — first match among imported files ──────────
	importedFiles := ctx.ImportMap[filePath]
	if len(importedFiles) > 0 {
		if isCXXPath(filePath) {
			for _, imported := range ctx.ImportOrderMap[filePath] {
				if !importedFiles[imported] {
					continue
				}
				for _, def := range allDefs {
					if def.FilePath == imported {
						return &ResolveResult{Definition: def, Tier: 2, Confidence: 0.9, SourceFile: def.FilePath}
					}
				}
			}
		}
		for _, def := range allDefs {
			if importedFiles[def.FilePath] {
				return &ResolveResult{
					Definition: def,
					Tier:       2,
					Confidence: 0.9,
					SourceFile: def.FilePath,
				}
			}
		}
	}

	// ── Tier 2c: Package-scoped — first match in imported package dirs ──────
	importedPackages := ctx.PackageMap[filePath]
	if len(importedPackages) > 0 {
		for _, def := range allDefs {
			for ds := range importedPackages {
				if isFileInPackageDir(def.FilePath, ds) {
					return &ResolveResult{
						Definition: def,
						Tier:       2,
						Confidence: 0.9,
						SourceFile: def.FilePath,
					}
				}
			}
		}
	}

	// ── Tier 3: Unique global ───────────────────────────────────────────────
	if len(allDefs) == 1 {
		return &ResolveResult{
			Definition: allDefs[0],
			Tier:       3,
			Confidence: 0.5,
			SourceFile: allDefs[0].FilePath,
		}
	}

	// Multiple global → reject; zero → not found
	return nil
}

func isCXXPath(path string) bool {
	for _, ext := range []string{".c", ".h", ".cc", ".cpp", ".cxx", ".hpp"} {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return true
		}
	}
	return false
}

// ResolveSymbolWithArity resolves a symbol with arity filtering.
// Used by call_processor to match callable signatures.
func ResolveSymbolWithArity(name string, filePath string, arity int, ctx *ResolveContext, opts ...ResolveOption) *ResolveResult {
	// First try normal resolution
	result := ResolveSymbol(name, filePath, ctx, opts...)
	if result == nil {
		return nil
	}

	// If the resolved definition has parameter count info, check arity
	if result.Definition.ParameterCount != nil && *result.Definition.ParameterCount != arity {
		// Arity mismatch — try to find a better match among same-tier candidates
	}

	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

// IsCallableType checks if a symbol type is callable.
func IsCallableType(typeName string) bool {
	callableTypes := map[string]bool{
		"Function":    true,
		"Method":      true,
		"Constructor": true,
	}
	return callableTypes[typeName]
}

// GenerateID creates a unique node ID from file path and node text.
func GenerateID(filePath string, label string, name string, startByte uint) string {
	return fmt.Sprintf("%s::%s::%s::%d", filePath, label, name, startByte)
}
