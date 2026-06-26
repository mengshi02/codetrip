package c

import (
	"strings"
	"sync"
)

// staticNames tracks per-file sets of function names declared with `static` storage class.
// Populated during emitCScopeCaptures and consumed by ExpandCWildcardNames
// to exclude file-local symbols from cross-file wildcard import visibility.
var (
	staticNamesMu sync.RWMutex
	staticNames   = make(map[string]map[string]bool)
)

// MarkStaticName records a symbol name as `static` (file-local linkage) for the given file.
func MarkStaticName(filePath, name string) {
	staticNamesMu.Lock()
	defer staticNamesMu.Unlock()
	if staticNames[filePath] == nil {
		staticNames[filePath] = make(map[string]bool)
	}
	staticNames[filePath][name] = true
}

// IsStaticName checks whether a symbol name has `static` linkage in the given file.
func IsStaticName(filePath, name string) bool {
	staticNamesMu.RLock()
	defer staticNamesMu.RUnlock()
	if names, ok := staticNames[filePath]; ok {
		return names[name]
	}
	return false
}

// GetStaticNamesForFile returns the static names recorded for the given file.
// Used to snapshot the per-file data for side-channel serialization.
func GetStaticNamesForFile(filePath string) []string {
	staticNamesMu.RLock()
	defer staticNamesMu.RUnlock()
	names := staticNames[filePath]
	if names == nil {
		return nil
	}
	result := make([]string, 0, len(names))
	for name := range names {
		result = append(result, name)
	}
	return result
}

// ClearStaticNames clears tracked static names (for testing / per-pass reset).
func ClearStaticNames() {
	staticNamesMu.Lock()
	defer staticNamesMu.Unlock()
	staticNames = make(map[string]map[string]bool)
}

// SymbolDef represents a simplified symbol definition for wildcard expansion.
type SymbolDef struct {
	QualifiedName string
	FilePath      string
}

// ExpandCWildcardNames returns the names visible through a C wildcard import (#include).
// All module-scope defs from the target file are visible EXCEPT those
// declared with `static` storage class (file-local linkage in C).
// Ported from GitNexus c/static-linkage.ts.
func ExpandCWildcardNames(targetFilePath string, targetDefs []SymbolDef) []string {
	seen := make(map[string]bool)
	var names []string

	for _, def := range targetDefs {
		name := simpleName(def.QualifiedName)
		if name == "" {
			continue
		}
		if IsStaticName(targetFilePath, name) {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	return names
}

func simpleName(qualifiedName string) string {
	if qualifiedName == "" {
		return ""
	}
	if idx := strings.LastIndex(qualifiedName, "."); idx >= 0 {
		return qualifiedName[idx+1:]
	}
	return qualifiedName
}