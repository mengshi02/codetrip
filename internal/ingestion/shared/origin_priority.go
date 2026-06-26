// Package shared — Origin priority for tie-breaking in resolution.
// Ported from gitnexus-shared scope-resolution/origin-priority.ts (31 lines).
package shared

// OriginForTieBreak is the binding origin used in tie-break key construction.
// Lower values indicate higher priority (local bindings beat imported ones).
type OriginForTieBreak int

const (
	OriginPriorityLocal     OriginForTieBreak = 0 // local def — highest priority
	OriginPriorityImport    OriginForTieBreak = 1 // named import
	OriginPriorityNamespace OriginForTieBreak = 2 // namespace import (import * as)
	OriginPriorityWildcard  OriginForTieBreak = 3 // wildcard-expanded import
	OriginPriorityReexport  OriginForTieBreak = 4 // re-export
	OriginPriorityGlobal    OriginForTieBreak = 5 // global-name lookup (no scope chain)
	OriginPriorityGlobalQN  OriginForTieBreak = 6 // global-qualified lookup
)

// ORIGIN_PRIORITY maps BindingOrigin to its tie-break priority.
// Lower = higher priority.
var ORIGIN_PRIORITY = map[BindingOrigin]OriginForTieBreak{
	OriginLocal:     OriginPriorityLocal,
	OriginImport:    OriginPriorityImport,
	OriginNamespace: OriginPriorityNamespace,
	OriginWildcard:  OriginPriorityWildcard,
	OriginReexport:  OriginPriorityReexport,
}

// GlobalNameOrigin is the synthetic origin for global-name evidence.
const GlobalNameOrigin BindingOrigin = "global-name"

// GlobalQualifiedOrigin is the synthetic origin for global-qualified evidence.
const GlobalQualifiedOrigin BindingOrigin = "global-qualified"

func init() {
	ORIGIN_PRIORITY[GlobalNameOrigin] = OriginPriorityGlobal
	ORIGIN_PRIORITY[GlobalQualifiedOrigin] = OriginPriorityGlobalQN
}