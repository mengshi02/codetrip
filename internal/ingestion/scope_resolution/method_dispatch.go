package scope_resolution

import (
	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// MethodDispatch resolves method calls through the MRO (Method Resolution Order).
// Mirrors TS scope-resolution/graph-bridge/method-dispatch.ts.

// DispatchMethodCall resolves a method call through MRO.
// Returns the target DefID if found, empty string otherwise.
//
// Walks the MRO chain for receiverDefID, checking each ancestor class
// for an owned member matching methodName. Returns the first match
// (per MRO priority).
func DispatchMethodCall(
	receiverDefID string,
	methodName string,
	mro map[string][]string,
	indexes *model.ScopeResolutionIndexes,
) string {
	// Check the receiver class itself first
	if found := FindOwnedMember(receiverDefID, methodName, indexes); found != "" {
		return found
	}

	// Walk MRO chain
	chain := MroFor(receiverDefID, mro)
	for _, ancestorDefID := range chain {
		if found := FindOwnedMember(ancestorDefID, methodName, indexes); found != "" {
			return found
		}
	}

	return ""
}

// FindOwnedMember searches a class's owned defs for a method/field with the
// given name. Returns the NodeID (DefID) if found, empty string otherwise.
//
// Mirrors TS findOwnedMember: walks all scopes in the scope tree,
// finds the class scope containing classDefID as an owned def,
// then searches that scope's other owned defs for a matching member name.
func FindOwnedMember(
	classDefID string,
	memberName string,
	indexes *model.ScopeResolutionIndexes,
) string {
	defs := indexes.Defs()
	if defs == nil {
		return ""
	}

	// Look up the class definition
	classDef := defs.Get(shared.DefID(classDefID))
	if classDef == nil {
		return ""
	}

	scopeTree := indexes.ScopeTree()
	if scopeTree == nil {
		return ""
	}

	// Walk all scopes to find the one that owns classDefID
	for scopeID, scope := range scopeTree.ByID() {
		_ = scopeID
		// Check if this scope owns the class def
		foundClassScope := false
		for _, ownedDef := range scope.OwnedDefs {
			if ownedDef.NodeID == classDefID {
				foundClassScope = true
				break
			}
		}
		if !foundClassScope {
			continue
		}

		// Found the class scope — now search its owned defs for the member
		for _, ownedDef := range scope.OwnedDefs {
			if ownedDef.NodeID == classDefID {
				continue // skip the class itself
			}
			if matchMemberName(&ownedDef, memberName) {
				return ownedDef.NodeID
			}
		}
		// Only search the first scope that owns the class def
		return ""
	}

	return ""
}

// matchMemberName checks if a SymbolDefinition matches the given member name.
// Tries unqualified tail of QualifiedName first, then falls back to NodeID simple name.
func matchMemberName(def *shared.SymbolDefinition, memberName string) bool {
	if def.QualifiedName != nil {
		qn := *def.QualifiedName
		dotIdx := lastDot(qn)
		unqualified := qn
		if dotIdx >= 0 {
			unqualified = qn[dotIdx+1:]
		}
		if unqualified == memberName {
			return true
		}
	}
	return false
}

// lastDot returns the index of the last '.' in s, or -1 if not found.
func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

// MroFor returns the MRO chain for a given DefID from the pre-computed map.
// Falls back to an empty slice if the DefID is not in the map.
func MroFor(defID string, mro map[string][]string) []string {
	if chain, ok := mro[defID]; ok {
		return chain
	}
	return nil
}