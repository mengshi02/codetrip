// Package golang — Go interface implementation detection.
// In Go, interface satisfaction is structural (duck-typed): a struct
// implements an interface if it has all the interface's methods, with
// no explicit "implements" keyword. This function detects such implicit
// relationships by comparing method sets.
// Ported from TS languages/go/interface-impls.ts.
package golang

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// InterfaceImplEntry records one struct → interface implementation pair.
type InterfaceImplEntry struct {
	StructDefID    string
	InterfaceDefID string
}

// goDefInfo holds definition info for interface-impl detection.
type goDefInfo struct {
	defID         string
	qualifiedName string
	methodSet     map[string]bool
}

// DetectGoInterfaceImplementations examines all Go struct and interface
// definitions across parsedFiles and returns a map of interface DefID →
// list of struct DefIDs that structurally satisfy the interface.
//
// Mirrors TS detectGoInterfaceImplementations(parsedFiles, nodeLookup).
func DetectGoInterfaceImplementations(
	parsedFiles []*shared.ParsedFile,
	nodeLookup *scope_resolution.GraphNodeLookup,
) map[string][]InterfaceImplEntry {
	var structs []goDefInfo
	var interfaces []goDefInfo

	for _, pf := range parsedFiles {
		for _, scope := range pf.Scopes {
			for i := range scope.OwnedDefs {
				def := &scope.OwnedDefs[i]
				if def.Type == shared.LabelStruct || def.Type == shared.LabelClass {
					qname := ""
					if def.QualifiedName != nil {
						qname = *def.QualifiedName
					}
					structs = append(structs, goDefInfo{
						defID:       def.NodeID,
						qualifiedName: qname,
						methodSet:   nil, // populated below
					})
				} else if def.Type == shared.LabelInterface {
					qname := ""
					if def.QualifiedName != nil {
						qname = *def.QualifiedName
					}
					interfaces = append(interfaces, goDefInfo{
						defID:       def.NodeID,
						qualifiedName: qname,
						methodSet:   nil,
					})
				}
			}
		}
	}

	// Collect method sets for structs and interfaces.
	// Methods are defs with OwnerID pointing to the struct/interface def,
	// or Function scopes with a self type binding pointing to the type.
	structMethodSets := collectMethodSets(parsedFiles, structs)
	ifaceMethodSets := collectMethodSets(parsedFiles, interfaces)

	// For each interface, check which structs satisfy it.
	result := map[string][]InterfaceImplEntry{}
	for _, iface := range interfaces {
		ifaceMethods := ifaceMethodSets[iface.defID]
		if len(ifaceMethods) == 0 {
			continue
		}
		for _, st := range structs {
			structMethods := structMethodSets[st.defID]
			if satisfiesInterface(structMethods, ifaceMethods) {
				result[iface.defID] = append(result[iface.defID], InterfaceImplEntry{
					StructDefID:    st.defID,
					InterfaceDefID: iface.defID,
				})
			}
		}
	}

	return result
}

// collectMethodSets builds a map of defID → method simple-name set.
// It looks for Method defs whose OwnerID matches the given struct/interface defIDs.
func collectMethodSets(parsedFiles []*shared.ParsedFile, defs []goDefInfo) map[string]map[string]bool {
	// Build a lookup from defID to index.
	defByID := map[string]int{}
	for i, d := range defs {
		defByID[d.defID] = i
	}

	result := map[string]map[string]bool{}
	// Walk all scopes looking for Method defs with an OwnerID.
	for _, pf := range parsedFiles {
		for _, scope := range pf.Scopes {
			for _, def := range scope.OwnedDefs {
				if def.Type != shared.LabelMethod || def.OwnerID == nil {
					continue
				}
				if idx, ok := defByID[*def.OwnerID]; ok {
					defID := defs[idx].defID
					if result[defID] == nil {
						result[defID] = map[string]bool{}
					}
					// Extract simple method name from qualified name.
					name := ""
					if def.QualifiedName != nil {
						name = *def.QualifiedName
						if dot := strings.LastIndex(name, "."); dot >= 0 {
							name = name[dot+1:]
						}
					}
					if name != "" {
						result[defID][name] = true
					}
				}
			}
		}
	}
	return result
}

// satisfiesInterface checks whether structMethods contains all ifaceMethods.
func satisfiesInterface(structMethods, ifaceMethods map[string]bool) bool {
	for m := range ifaceMethods {
		if !structMethods[m] {
			return false
		}
	}
	return true
}

