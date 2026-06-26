// Package shared — MethodDispatchIndex for MRO + implements materialized view.
// Ported from gitnexus-shared scope-resolution/method-dispatch-index.ts (194 lines).
package shared

// MethodDispatchIndex is a materialized view of method resolution order (MRO)
// and interface implementations. It supports:
//   - Step 2 of the 7-step lookup: type-binding MRO walk for receiver-dispatched calls
//   - Step 3 of the 7-step lookup: owner-scoped contributor (implsByInterfaceDefId)
type MethodDispatchIndex struct {
	// mroByOwnerDefId maps a class/struct/enum DefID → its MRO (linearized ancestor DefIDs).
	// The first element is the class itself, then parents in MRO order.
	mroByOwnerDefId map[DefID][]DefID

	// implsByInterfaceDefId maps an interface DefID → DefIDs of types that implement it.
	// Used by owner-scoped contributor (Step 3) to find concrete impls.
	implsByInterfaceDefId map[DefID][]DefID

	// extendsOnlyMroByOwnerDefId (optional) maps a class DefID → MRO excluding
	// interface-default-method ancestors. Used by languages with trait mixins
	// (e.g., PHP traits, Rust trait-default-methods) where the primary MRO
	// includes trait defaults but a secondary walk is needed for class-only chain.
	extendsOnlyMroByOwnerDefId map[DefID][]DefID
}

// NewMethodDispatchIndex creates an empty MethodDispatchIndex.
func NewMethodDispatchIndex() *MethodDispatchIndex {
	return &MethodDispatchIndex{
		mroByOwnerDefId:          make(map[DefID][]DefID),
		implsByInterfaceDefId:    make(map[DefID][]DefID),
		extendsOnlyMroByOwnerDefId: nil, // allocated lazily
	}
}

// MROByOwner returns the MRO for the given owner DefID, or nil.
func (m *MethodDispatchIndex) MROByOwner(defID DefID) []DefID {
	return m.mroByOwnerDefId[defID]
}

// ImplsByInterface returns the implementing types for the given interface DefID.
func (m *MethodDispatchIndex) ImplsByInterface(defID DefID) []DefID {
	return m.implsByInterfaceDefId[defID]
}

// ExtendsOnlyMRO returns the extends-only MRO for the given owner DefID, or nil.
func (m *MethodDispatchIndex) ExtendsOnlyMRO(defID DefID) []DefID {
	if m.extendsOnlyMroByOwnerDefId == nil {
		return nil
	}
	return m.extendsOnlyMroByOwnerDefId[defID]
}

// MethodDispatchInput provides the callbacks needed to build the index.
// computeMro and computeImplsOf are language-specific strategies provided
// by the language provider.
type MethodDispatchInput struct {
	// AllClassDefIds is every class/struct/enum/interface DefID in the workspace.
	AllClassDefIDs []DefID
	// ComputeMro computes the MRO for a given DefID. Returns ancestor DefIDs
	// in linearization order (class itself first, then parents).
	ComputeMro func(defID DefID) []DefID
	// ComputeImplsOf returns DefIDs of types that implement the given interface.
	ComputeImplsOf func(interfaceDefID DefID) []DefID
	// WithExtendsOnlyMRO enables the extendsOnlyMroByOwnerDefId secondary index.
	// When true, ComputeMro is also called with a "extends-only" flag (passed
	// via a separate callback if needed, or the same ComputeMro with a flag).
	WithExtendsOnlyMRO bool
}

// BuildMethodDispatchIndex constructs a MethodDispatchIndex using the provided
// callbacks. This is the Go equivalent of the TS builder that takes
// computeMro + implementsOf callbacks.
func BuildMethodDispatchIndex(input *MethodDispatchInput) *MethodDispatchIndex {
	idx := &MethodDispatchIndex{
		mroByOwnerDefId:       make(map[DefID][]DefID, len(input.AllClassDefIDs)),
		implsByInterfaceDefId: make(map[DefID][]DefID),
	}

	// Build MRO for each class
	for _, defID := range input.AllClassDefIDs {
		mro := input.ComputeMro(defID)
		if len(mro) > 0 {
			idx.mroByOwnerDefId[defID] = mro
		}
	}

	// Build implsByInterface by iterating all class defs and checking
	// if they implement any interface. The input.ComputeImplsOf callback
	// provides this mapping.
	for _, defID := range input.AllClassDefIDs {
		impls := input.ComputeImplsOf(defID)
		if len(impls) > 0 {
			idx.implsByInterfaceDefId[defID] = impls
		}
	}

	// Optional: build extends-only MRO
	if input.WithExtendsOnlyMRO {
		idx.extendsOnlyMroByOwnerDefId = make(map[DefID][]DefID)
		// The ComputeMro callback with extends-only semantics is expected
		// to be handled by the caller providing an appropriate callback;
		// here we just allocate the map. The actual population happens
		// via SetExtendsOnlyMRO or a secondary build pass.
	}

	return idx
}

// SetMRO sets the MRO for a given owner DefID. Used by post-build augmentation.
func (m *MethodDispatchIndex) SetMRO(defID DefID, mro []DefID) {
	m.mroByOwnerDefId[defID] = mro
}

// SetImplsOf sets the implementing types for an interface DefID.
func (m *MethodDispatchIndex) SetImplsOf(interfaceDefID DefID, impls []DefID) {
	m.implsByInterfaceDefId[interfaceDefID] = impls
}

// SetExtendsOnlyMRO sets the extends-only MRO for a given owner DefID.
func (m *MethodDispatchIndex) SetExtendsOnlyMRO(defID DefID, mro []DefID) {
	if m.extendsOnlyMroByOwnerDefId == nil {
		m.extendsOnlyMroByOwnerDefId = make(map[DefID][]DefID)
	}
	m.extendsOnlyMroByOwnerDefId[defID] = mro
}