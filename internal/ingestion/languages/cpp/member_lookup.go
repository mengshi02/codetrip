package cpp

// C++ Member Lookup — class member access and dependent base resolution.
//
// C++ member lookup is complex due to:
//   - Multiple inheritance with ambiguous base members
//   - Virtual inheritance and diamond hierarchies
//   - Dependent base classes in templates (cannot look up members until instantiation)
//   - Access control (public/protected/private)
//   - Hidden friends (found only via ADL)
//
// This module provides the state and lookup primitives for C++ member resolution.
// Ported from TS languages/cpp/member-lookup.ts.

import (
	"sync"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// memberLookupMutex guards member lookup state.
var memberLookupMutex sync.RWMutex

// dependentBases maps class def NodeIDs to their dependent base class names.
// Dependent bases are template base classes whose members cannot be resolved
// until the template is instantiated.
// Key: class def NodeID → Value: []baseClassName
var dependentBases map[string][]string

// memberLookupCache caches member lookup results to avoid repeated MRO walks.
// Key: "classID::memberName" → Value: []candidateDefNodeID
var memberLookupCache map[string][]string

func init() {
	dependentBases = make(map[string][]string)
	memberLookupCache = make(map[string][]string)
}

// ClearCppMemberLookupState resets member lookup state.
func ClearCppMemberLookupState() {
	memberLookupMutex.Lock()
	defer memberLookupMutex.Unlock()
	dependentBases = make(map[string][]string)
	memberLookupCache = make(map[string][]string)
}

// ClearCppDependentBases resets dependent base tracking.
func ClearCppDependentBases() {
	memberLookupMutex.Lock()
	defer memberLookupMutex.Unlock()
	dependentBases = make(map[string][]string)
}

// RegisterCppDependentBase registers a dependent base class.
func RegisterCppDependentBase(classDefNodeID string, baseClassName string) {
	memberLookupMutex.Lock()
	defer memberLookupMutex.Unlock()
	dependentBases[classDefNodeID] = append(dependentBases[classDefNodeID], baseClassName)
}

// LookupCppMember performs class member lookup through the MRO chain.
// Returns candidate definition NodeIDs.
// TODO: full implementation — walk MRO chain with access control.
func LookupCppMember(classDef shared.SymbolDefinition, memberName string) []string {
	memberLookupMutex.RLock()
	defer memberLookupMutex.RUnlock()
	// Check cache first
	cacheKey := classDef.NodeID + "::" + memberName
	if results, ok := memberLookupCache[cacheKey]; ok {
		return results
	}
	// TODO: walk MRO chain
	return nil
}