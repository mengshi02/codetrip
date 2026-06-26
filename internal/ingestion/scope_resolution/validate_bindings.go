package scope_resolution

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/ingestion/model"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// ValidateBindingsImmutability — dev-mode runtime validator for
// Contract Invariant I8.
//
// Go has no Object.isFrozen. Instead we take a deep snapshot of the
// bindings channel at finalize time and compare element-by-element
// against the current state. Any mutation is a violation.
//
// The six channels validated:
//   1. indexes.bindings          — MUST be frozen (snapshot matches)
//   2. indexes.bindingAugmentations — inner arrays must NOT be frozen
//      (Go: must be non-nil mutable slices; we only warn if a bucket
//      was explicitly snapshotted as empty and is still empty but
//      that's not a defect — instead we just warn on zero-length
//      immutable-looking buckets. In practice Go slices are always
//      mutable so this check is a no-op; we keep it for structural
//      parity with TS.)
//   3. indexes.workspaceFqnBindings — mutable (post-finalize push)
//   4. indexes.workspaceTypeBindings — mutable (post-finalize set)
//   5. indexes.namespaceFqnBindings  — mutable inner buckets
//   6. indexes.namespaceTypeBindings — mutable inner maps
//
// Returns the number of violations found.
// ---------------------------------------------------------------------------

// bindingBucketSnapshot records the length of a single BindingRef bucket
// at finalize time, plus the defIDs of every element (for deep compare).
type bindingBucketSnapshot struct {
	length int
	defIDs []string // BindingRef.Def.ID() for each element
}

// bindingsSnapshot captures the entire bindings map at finalize time.
type bindingsSnapshot struct {
	buckets map[shared.ScopeID]map[string]bindingBucketSnapshot
}

// snapshotBindings captures a deep snapshot of indexes.bindings at
// finalize time. Call this immediately after finalizeScopeModel returns.
func snapshotBindings(indexes *model.ScopeResolutionIndexes) *bindingsSnapshot {
	snap := &bindingsSnapshot{
		buckets: make(map[shared.ScopeID]map[string]bindingBucketSnapshot),
	}
	for scopeID, bucketMap := range indexes.Bindings() {
		inner := make(map[string]bindingBucketSnapshot, len(bucketMap))
		for name, bucket := range bucketMap {
			ids := make([]string, len(bucket))
			for i, ref := range bucket {
				ids[i] = ref.Def.NodeID
			}
			inner[name] = bindingBucketSnapshot{
				length: len(bucket),
				defIDs: ids,
			}
		}
		snap.buckets[scopeID] = inner
	}
	return snap
}

// ValidateBindingsImmutability checks that no post-finalize hook has
// mutated the frozen bindings channel (indexes.bindings). Any drift
// from the snapshot is reported via the onWarn callback.
//
// Mirrors TS scope-resolution/pipeline/validate-bindings-immutability.ts.
//
// Contract invariant I8: indexes.bindings is the finalize-output channel
// and MUST be treated as immutable from the moment finalizeScopeModel
// returns. Post-finalize hooks MUST write to bindingAugmentations instead.
//
// If snap is nil (development mode disabled), returns 0 immediately.
func ValidateBindingsImmutability(
	indexes *model.ScopeResolutionIndexes,
	snap *bindingsSnapshot,
	onWarn func(string),
) int {
	if snap == nil {
		return 0
	}

	violations := 0

	// ── Channel 1: bindings (MUST be frozen — snapshot must match) ──
	for scopeID, snapBucketMap := range snap.buckets {
		currentBucketMap, ok := indexes.Bindings()[scopeID]
		if !ok {
			// Entire scope was removed — violation
			onWarn(fmt.Sprintf(
				"binding-immutability: indexes.bindings[%s] was removed — "+
					"finalize produced this scope bucket but a post-finalize hook deleted it. "+
					"See ScopeResolver Invariant I8.", scopeID))
			violations++
			continue
		}
		for name, snapBucket := range snapBucketMap {
			currentBucket, ok := currentBucketMap[name]
			if !ok {
				onWarn(fmt.Sprintf(
					"binding-immutability: indexes.bindings[%s][%s] was removed — "+
						"finalize produced this bucket but a post-finalize hook deleted it. "+
						"See ScopeResolver Invariant I8.", scopeID, name))
				violations++
				continue
			}
			if len(currentBucket) != snapBucket.length {
				onWarn(fmt.Sprintf(
					"binding-immutability: indexes.bindings[%s][%s] length changed from %d to %d — "+
						"a post-finalize hook mutated a frozen bucket. Hooks must write to "+
						"indexes.bindingAugmentations instead. See ScopeResolver Invariant I8.",
					scopeID, name, snapBucket.length, len(currentBucket)))
				violations++
				continue
			}
			// Deep compare: check each element's Def.NodeID
			for i, ref := range currentBucket {
				if i < len(snapBucket.defIDs) && ref.Def.NodeID != snapBucket.defIDs[i] {
					onWarn(fmt.Sprintf(
						"binding-immutability: indexes.bindings[%s][%s][%d] Def.NodeID changed from %s to %s — "+
							"a post-finalize hook mutated a frozen bucket element. "+
							"See ScopeResolver Invariant I8.",
						scopeID, name, i, snapBucket.defIDs[i], ref.Def.NodeID))
					violations++
				}
			}
		}
	}

	// Check for buckets that were ADDED (not in snapshot = mutation)
	for scopeID, currentBucketMap := range indexes.Bindings() {
		snapBucketMap, ok := snap.buckets[scopeID]
		if !ok {
			onWarn(fmt.Sprintf(
				"binding-immutability: indexes.bindings[%s] was ADDED after finalize — "+
					"hooks must write to indexes.bindingAugmentations instead. "+
					"See ScopeResolver Invariant I8.", scopeID))
			violations++
			continue
		}
		for name := range currentBucketMap {
			if _, ok := snapBucketMap[name]; !ok {
				onWarn(fmt.Sprintf(
					"binding-immutability: indexes.bindings[%s][%s] was ADDED after finalize — "+
						"hooks must write to indexes.bindingAugmentations instead. "+
						"See ScopeResolver Invariant I8.", scopeID, name))
				violations++
			}
		}
	}

	// ── Channels 2-6: mutable channels — structural sanity checks ──
	// In Go, slices and maps are always mutable, so we can't check
	// Object.isFrozen. Instead we do minimal structural validation
	// for parity with the TS validator.

	// Channel 2: bindingAugmentations — inner buckets should be mutable.
	// Go slices are always mutable; this check is a no-op but kept for
	// structural parity. We could check for nil-vs-empty confusion but
	// that's not a freeze issue.

	// Channel 3: workspaceFqnBindings — mutable (hooks push() directly)
	for name, bucket := range indexes.WorkspaceFqnBindings() {
		if bucket == nil {
			onWarn(fmt.Sprintf(
				"binding-immutability: indexes.workspaceFqnBindings[%s] is nil — "+
					"the workspace channel is mutable by contract; nil buckets should be "+
					"initialized as empty slices. See ScopeResolver Invariant I8.", name))
			violations++
		}
	}

	// Channel 4: workspaceTypeBindings — map must be non-nil and mutable
	if indexes.WorkspaceTypeBindings() == nil {
		onWarn("binding-immutability: indexes.workspaceTypeBindings is nil — " +
			"the workspace type channel is populated post-finalize and must be initialized. " +
			"See ScopeResolver Invariant I8.")
		violations++
	}

	// Channel 5: namespaceFqnBindings — per-namespace mutable buckets
	for ns, inner := range indexes.NamespaceFqnBindings() {
		if inner == nil {
			onWarn(fmt.Sprintf(
				"binding-immutability: indexes.namespaceFqnBindings[%s] is nil — "+
					"per-namespace buckets are mutable by contract. "+
					"See ScopeResolver Invariant I8.", ns))
			violations++
		}
		for name, bucket := range inner {
			if bucket == nil {
				onWarn(fmt.Sprintf(
					"binding-immutability: indexes.namespaceFqnBindings[%s][%s] is nil — "+
						"per-namespace buckets are mutable by contract. "+
						"See ScopeResolver Invariant I8.", ns, name))
				violations++
			}
		}
	}

	// Channel 6: namespaceTypeBindings — per-namespace type maps
	for ns, inner := range indexes.NamespaceTypeBindings() {
		if inner == nil {
			onWarn(fmt.Sprintf(
				"binding-immutability: indexes.namespaceTypeBindings[%s] is nil — "+
					"per-namespace type maps are populated post-finalize and must stay mutable. "+
					"See ScopeResolver Invariant I8.", ns))
			violations++
		}
	}

	return violations
}
