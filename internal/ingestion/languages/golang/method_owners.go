// Package go implements the Go language provider for the codetrip ingestion pipeline.
//
// method_owners.go — Populate ownerId on Go Method defs by matching receiver types
// against struct definitions in the module scope.
//
// Go method declarations are top-level (func (r *T) M()), not nested inside a struct body.
// The generic populateClassOwnedMembers requires the method's parent scope to be a Class scope,
// which never matches Go. This pass bridges the gap by reading the self typeBinding that
// synthesizeGoReceiverBinding creates, locating the matching struct def, and stamping ownerId
// onto the Method def.
//
// Ported from TS languages/go/method-owners.ts.
package golang

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PopulateGoOwners stamps ownerId on Go Method defs by matching receiver types
// extracted from @type-binding.self captures against struct defs in the module scope.
//
// Mirrors TS populateGoOwners(parsed).
func PopulateGoOwners(parsed *shared.ParsedFile) {
	// 1. Standard nested-class pass — stamps ownerId on Property/Method defs
	// inside Class scopes. With Class scopes now created for Go struct/interface
	// declarations, this handles struct field ownership.
	PopulateClassOwnedMembers(parsed)

	PopulateGoOwnersInPackage([]*shared.ParsedFile{parsed})
}

// PopulateGoWorkspaceOwners stamps ownerId across all Go files grouped by package.
// It first infers package names from file contents, groups files by package directory,
// then calls PopulateGoOwnersInPackage for each group.
//
// Mirrors TS populateGoWorkspaceOwners(parsedFiles, ctx).
func PopulateGoWorkspaceOwners(
	parsedFiles []*shared.ParsedFile,
	fileContents map[string]string,
) {
	filesByPackage := map[string][]*shared.ParsedFile{}
	for _, parsed := range parsedFiles {
		pkgName := InferPackageName(fileContents[parsed.FilePath])
		if pkgName == "" {
			continue
		}
		key := PackageDir(parsed.FilePath) + "\x00" + pkgName
		filesByPackage[key] = append(filesByPackage[key], parsed)
	}

	for _, bucket := range filesByPackage {
		PopulateGoOwnersInPackage(bucket)
	}
}

// ownerEntry holds the struct def info used for method-owner matching.
type ownerEntry struct {
	nodeID        string
	qualifiedName string
}

// PopulateGoOwnersInPackage builds a struct name → def map from ALL scopes' ownedDefs
// and matches Go Method defs' receiver types against it.
//
// Mirrors TS populateGoOwnersInPackage(parsedFiles).
func PopulateGoOwnersInPackage(parsedFiles []*shared.ParsedFile) {
	// Build struct name → def map from ALL scopes' ownedDefs (struct defs
	// live in Class scopes now, not Module scope).
	structByQualifiedName := map[string]*ownerEntry{}
	for _, parsed := range parsedFiles {
		for _, scope := range parsed.Scopes {
			for i := range scope.OwnedDefs {
				def := &scope.OwnedDefs[i]
				if IsClassLike(def.Type) && def.QualifiedName != nil {
					structByQualifiedName[*def.QualifiedName] = &ownerEntry{
						nodeID:        def.NodeID,
						qualifiedName: *def.QualifiedName,
					}
				}
			}
		}
	}

	// 2. Go-specific method owner: each Method def lives in a Function scope whose
	// typeBindings carry the self entry (kept there by goBindingScopeFor).
	if len(structByQualifiedName) == 0 {
		return
	}
	for _, parsed := range parsedFiles {
		for _, scope := range parsed.Scopes {
			if scope.Kind != shared.ScopeKindFunction {
				continue
			}
			methodDefs := filterUnownedMethods(scope.OwnedDefs)
			if len(methodDefs) == 0 {
				continue
			}

			// Find the self typeBinding in this Function scope.
			receiverType, _ := findReceiverType(scope)
			if receiverType == "" {
				continue
			}
			receiverType = strings.TrimLeft(receiverType, "*")
			receiverType = strings.TrimSpace(receiverType)

			owner := structByQualifiedName[receiverType]
			if owner == nil {
				// Try suffix match: qname.EndsWith("." + receiverType)
				for qname, candidate := range structByQualifiedName {
					if strings.HasSuffix(qname, "."+receiverType) {
						owner = candidate
						break
					}
				}
			}
			if owner != nil {
				for _, defIdx := range methodDefs {
					scope.OwnedDefs[defIdx].OwnerID = &owner.nodeID
					// TODO: set goReceiverKind when SymbolDefinition gains that field
					if scope.OwnedDefs[defIdx].QualifiedName != nil &&
						!strings.Contains(*scope.OwnedDefs[defIdx].QualifiedName, ".") {
						simpleName := *scope.OwnedDefs[defIdx].QualifiedName
						if idx := strings.LastIndex(simpleName, "."); idx >= 0 {
							simpleName = simpleName[idx+1:]
						}
						qualified := owner.qualifiedName + "." + simpleName
						scope.OwnedDefs[defIdx].QualifiedName = &qualified
					}
				}
			}
		}
	}
}

// InferPackageName extracts the Go package name from source text.
// Returns "" if no package declaration is found.
func InferPackageName(sourceText string) string {
	// Match: ^\s*package\s+([A-Za-z_][A-Za-z0-9_]*)
	for _, line := range strings.Split(sourceText, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			name := strings.TrimSpace(line[len("package "):])
			if name != "" && isGoIdentifier(name) {
				return name
			}
		}
	}
	return ""
}

// PackageDir returns the directory part of a file path.
func PackageDir(filePath string) string {
	normalized := strings.ReplaceAll(filePath, "\\", "/")
	idx := strings.LastIndex(normalized, "/")
	if idx == -1 {
		return ""
	}
	return normalized[:idx]
}

// --- internal helpers --

func isGoIdentifier(s string) bool {
	if len(s) == 0 {
		return false
	}
	if !((s[0] >= 'a' && s[0] <= 'z') || (s[0] >= 'A' && s[0] <= 'Z') || s[0] == '_') {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_') {
			return false
		}
	}
	return true
}

// filterUnownedMethods returns indices of Method defs with nil OwnerID.
func filterUnownedMethods(defs []shared.SymbolDefinition) []int {
	var result []int
	for i := range defs {
		if defs[i].Type == shared.LabelMethod && defs[i].OwnerID == nil {
			result = append(result, i)
		}
	}
	return result
}

// findReceiverType finds the receiver typeBinding with source=="self" in a Function scope.
func findReceiverType(scope *shared.Scope) (typeName string, kind string) {
	for _, tb := range scope.TypeBindings {
		if tb.Source == "self" {
			raw := tb.RawName
			if raw != "" {
				k := "value"
				if strings.HasPrefix(strings.TrimSpace(raw), "*") {
					k = "pointer"
				}
				return raw, k
			}
		}
	}
	return "", ""
}

// IsClassLike checks whether a NodeLabel represents a class-like definition.
// TODO: move to shared or scope_resolution package once walkers.go is fully implemented.
func IsClassLike(label shared.NodeLabel) bool {
	return label == shared.LabelClass || label == shared.LabelStruct || label == shared.LabelInterface
}

// PopulateClassOwnedMembers stamps ownerId on defs structurally owned by Class scopes.
// TODO: full implementation — currently delegates to scope_resolution.PopulateClassOwnedMembers
// when available, or uses a local simplified version.
func PopulateClassOwnedMembers(parsed *shared.ParsedFile) {
	// Simplified: walk scopes, stamp ownerId for defs inside Class scopes
	for _, scope := range parsed.Scopes {
		if scope.Kind == shared.ScopeKindClass {
			for i := range scope.OwnedDefs {
				scope.OwnedDefs[i].OwnerID = &scope.ID
			}
		}
	}
}
