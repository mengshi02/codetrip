package model

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// RegistrationHook -- registration hook type
// ---------------------------------------------------------------------------

// RegistrationHook -- a pure side-effect closure that writes to a specific registry.
type RegistrationHook func(name string, def *shared.SymbolDefinition)

// ---------------------------------------------------------------------------
// RegistrationTableDeps -- factory dependencies
// ---------------------------------------------------------------------------

// RegistrationTableDeps -- dependency injection for CreateRegistrationTable.
type RegistrationTableDeps struct {
	Types   MutableTypeRegistry
	Methods MutableMethodRegistry
	Fields  MutableFieldRegistry
}

// ---------------------------------------------------------------------------
// LabelBehavior -- NodeLabel behavior classification
// ---------------------------------------------------------------------------

// LabelBehavior -- behavior category of a NodeLabel in the ingestion pipeline.
//   dispatch     -> owner-scoped registry write
//   CallableOnly -> callableByName index (no owner registry)
//   Inert        -> fileIndex only (metadata/structural nodes)
type LabelBehavior string

const (
	BehaviorDispatch     LabelBehavior = "dispatch"
	BehaviorCallableOnly LabelBehavior = "callable-only"
	BehaviorInert        LabelBehavior = "inert"
)

// ---------------------------------------------------------------------------
// LABEL_BEHAVIOR -- single source of truth: NodeLabel -> behavior classification
// ---------------------------------------------------------------------------

// labelBehaviorMap -- Go equivalent of TS LABEL_BEHAVIOR.
// TS uses `as const satisfies Record<NodeLabel, LabelBehavior>` for compile-time
// exhaustive check. Go has no such capability; coverage must be ensured by code review.
var labelBehaviorMap = map[shared.NodeLabel]LabelBehavior{
	// dispatch -- owner-scoped registry writes
	shared.LabelClass:      BehaviorDispatch,
	shared.LabelStruct:     BehaviorDispatch,
	shared.LabelInterface:  BehaviorDispatch,
	shared.LabelEnum:       BehaviorDispatch,
	shared.LabelRecord:     BehaviorDispatch,
	shared.LabelTrait:      BehaviorDispatch,
	shared.LabelMethod:     BehaviorDispatch,
	shared.LabelConstructor: BehaviorDispatch,
	shared.LabelProperty:   BehaviorDispatch,
	shared.LabelImpl:       BehaviorDispatch,

	// callable-only -- file index + callableByName, no owner scope
	shared.LabelFunction:  BehaviorCallableOnly,
	shared.LabelMacro:     BehaviorCallableOnly,
	shared.LabelDelegate:  BehaviorCallableOnly,

	// inert -- file index only
	shared.LabelProject:    BehaviorInert,
	shared.LabelPackage:    BehaviorInert,
	shared.LabelModule:     BehaviorInert,
	shared.LabelFolder:     BehaviorInert,
	shared.LabelFile:       BehaviorInert,
	shared.LabelVariable:   BehaviorInert,
	shared.LabelDecorator:  BehaviorInert,
	shared.LabelImport:     BehaviorInert,
	shared.LabelType:       BehaviorInert,
	shared.LabelCodeElement: BehaviorInert,
	shared.LabelCommunity:  BehaviorInert,
	shared.LabelProcess:    BehaviorInert,
	shared.LabelTypedef:    BehaviorInert,
	shared.LabelUnion:      BehaviorInert,
	shared.LabelNamespace:  BehaviorInert,
	shared.LabelTypeAlias:  BehaviorInert,
	shared.LabelConst:      BehaviorInert,
	shared.LabelStatic:     BehaviorInert,
	shared.LabelAnnotation: BehaviorInert,
	shared.LabelTemplate:   BehaviorInert,
	shared.LabelSection:    BehaviorInert,
	shared.LabelRoute:      BehaviorInert,
	shared.LabelTool:       BehaviorInert,
	shared.LabelBasicBlock: BehaviorInert,
}

// ---------------------------------------------------------------------------
// Derived runtime sets from labelBehaviorMap
// ---------------------------------------------------------------------------

// ALL_NODE_LABELS -- complete NodeLabel list derived from labelBehaviorMap keys.
var ALL_NODE_LABELS []shared.NodeLabel

func init() {
	ALL_NODE_LABELS = make([]shared.NodeLabel, 0, len(labelBehaviorMap))
	for label := range labelBehaviorMap {
		ALL_NODE_LABELS = append(ALL_NODE_LABELS, label)
	}
}

// CALLABLE_ONLY_LABELS -- alias for FREE_CALLABLE_TYPES.
var CALLABLE_ONLY_LABELS = FREE_CALLABLE_TYPES

// INERT_LABELS -- set of NodeLabels that only go into fileIndex.
var INERT_LABELS map[string]bool

// DISPATCH_LABELS -- set of NodeLabels that have dispatch hooks.
var DISPATCH_LABELS map[string]bool

func init() {
	INERT_LABELS = labelsWithBehavior(BehaviorInert)
	DISPATCH_LABELS = labelsWithBehavior(BehaviorDispatch)
}

func labelsWithBehavior(behavior LabelBehavior) map[string]bool {
	m := make(map[string]bool)
	for label, b := range labelBehaviorMap {
		if b == behavior {
			m[string(label)] = true
		}
	}
	return m
}

// ---------------------------------------------------------------------------
// Factory: CreateRegistrationTable
// ---------------------------------------------------------------------------

// CreateRegistrationTable -- build the dispatch table.
// Must be called in each CreateSemanticModel; closures capture the current instance's registries.
// Cannot reuse a module-level singleton, or hooks would write to the wrong SemanticModel.
func CreateRegistrationTable(deps RegistrationTableDeps) map[shared.NodeLabel]RegistrationHook {
	types := deps.Types
	methods := deps.Methods
	fields := deps.Fields

	// Hook 1: class-like -- Class/Struct/Interface/Enum/Record/Trait
	// Six labels share the same closure.
	classLikeHook := func(name string, def *shared.SymbolDefinition) {
		qualifiedKey := name
		if def.QualifiedName != nil {
			qualifiedKey = *def.QualifiedName
		}
		types.RegisterClass(name, qualifiedKey, def)
	}

	// Hook 2: method-like -- Method/Constructor.
	// Silently skip when ownerId is missing (extractor contract violation).
	methodHook := func(name string, def *shared.SymbolDefinition) {
		if def.OwnerID != nil {
			methods.Register(*def.OwnerID, name, def)
		}
	}

	// Hook 3: property -- Property.
	// Property is not in FREE_CALLABLE_TYPES; SymbolTable.add() already excludes it
	// from callableByName. Common names like id/name/type don't pollute the callable index.
	propertyHook := func(name string, def *shared.SymbolDefinition) {
		if def.OwnerID != nil {
			fields.Register(*def.OwnerID, name, def)
		}
	}

	// Hook 4: impl-block -- Rust impl blocks.
	// Separated from classLikeHook because heritage resolution doesn't treat Impl as a parent type.
	implHook := func(name string, def *shared.SymbolDefinition) {
		types.RegisterImpl(name, def)
	}

	// dispatchByLabel -- the single source of truth for label -> hook mapping.
	dispatchByLabel := map[shared.NodeLabel]RegistrationHook{
		// class-like -- six labels share classLikeHook
		shared.LabelClass:      classLikeHook,
		shared.LabelStruct:     classLikeHook,
		shared.LabelInterface:  classLikeHook,
		shared.LabelEnum:       classLikeHook,
		shared.LabelRecord:     classLikeHook,
		shared.LabelTrait:      classLikeHook,
		// method-like -- Method + Constructor
		shared.LabelMethod:     methodHook,
		shared.LabelConstructor: methodHook,
		// property
		shared.LabelProperty:   propertyHook,
		// impl-block
		shared.LabelImpl:       implHook,
	}

	return dispatchByLabel
}
