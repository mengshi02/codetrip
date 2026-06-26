// Package shared defines cross-module types shared across the ingestion pipeline.
// This is the Go equivalent of gitnexus-shared.
package shared

// SupportedLanguage represents a supported programming language.
// This mirrors core.SupportedLanguage but lives in the shared package
// for cross-module access without circular dependencies.
type SupportedLanguage string

const (
	SupportedLanguageTypeScript  SupportedLanguage = "typescript"
	SupportedLanguageJavaScript  SupportedLanguage = "javascript"
	SupportedLanguagePython      SupportedLanguage = "python"
	SupportedLanguageJava        SupportedLanguage = "java"
	SupportedLanguageC           SupportedLanguage = "c"
	SupportedLanguageCpp         SupportedLanguage = "cpp"
	SupportedLanguageCSharp     SupportedLanguage = "csharp"
	SupportedLanguageGo         SupportedLanguage = "go"
	SupportedLanguageRust       SupportedLanguage = "rust"
)

// CVQualifier represents a C++ cv-qualifier on a parameter type.
type CVQualifier string

const (
	CVNone          CVQualifier = "none"
	CVConst         CVQualifier = "const"
	CVVolatile      CVQualifier = "volatile"
	CVConstVolatile CVQualifier = "const volatile"
	CVUnknown       CVQualifier = "unknown"
)

// IndirectionKind represents the coarse value/reference/pointer shape of a parameter.
type IndirectionKind string

const (
	IndirectionValue     IndirectionKind = "value"
	IndirectionLValueRef IndirectionKind = "lvalue-ref"
	IndirectionRValueRef IndirectionKind = "rvalue-ref"
	IndirectionPointer   IndirectionKind = "pointer"
	IndirectionUnknown   IndirectionKind = "unknown"
)

// ParameterTypeClass describes the C++ sidecar shape of a parameter type.
// Normalized base type, cv-qualification, and indirection information
// are preserved from the original C++ parameter spelling.
type ParameterTypeClass struct {
	// Base is the normalized base type, matching the coarse parameterTypes vocabulary when known.
	Base string
	// CV is the top-level cv signal preserved from the original C++ parameter spelling.
	CV CVQualifier
	// Indirection is the coarse value/reference/pointer shape.
	Indirection IndirectionKind
	// PointerDepth is the number of pointer markers when Indirection is "pointer"; otherwise 0.
	PointerDepth int
}

// Range represents a source code range with 1-based line numbers.
type Range struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
}

// NodeLabel represents the type/label of a graph node.
type NodeLabel string

// NodeLabel constants — the complete set of graph node labels.
const (
	LabelClass       NodeLabel = "Class"
	LabelStruct      NodeLabel = "Struct"
	LabelInterface   NodeLabel = "Interface"
	LabelEnum        NodeLabel = "Enum"
	LabelRecord      NodeLabel = "Record"
	LabelTrait       NodeLabel = "Trait"
	LabelMethod      NodeLabel = "Method"
	LabelConstructor NodeLabel = "Constructor"
	LabelProperty    NodeLabel = "Property"
	LabelImpl        NodeLabel = "Impl"
	LabelFunction    NodeLabel = "Function"
	LabelMacro       NodeLabel = "Macro"
	LabelDelegate    NodeLabel = "Delegate"
	LabelProject     NodeLabel = "Project"
	LabelPackage     NodeLabel = "Package"
	LabelModule      NodeLabel = "Module"
	LabelFolder      NodeLabel = "Folder"
	LabelFile        NodeLabel = "File"
	LabelVariable    NodeLabel = "Variable"
	LabelDecorator   NodeLabel = "Decorator"
	LabelImport      NodeLabel = "Import"
	LabelType        NodeLabel = "Type"
	LabelCodeElement NodeLabel = "CodeElement"
	LabelCommunity   NodeLabel = "Community"
	LabelProcess     NodeLabel = "Process"
	LabelTypedef     NodeLabel = "Typedef"
	LabelUnion       NodeLabel = "Union"
	LabelNamespace   NodeLabel = "Namespace"
	LabelTypeAlias   NodeLabel = "TypeAlias"
	LabelConst       NodeLabel = "Const"
	LabelStatic      NodeLabel = "Static"
	LabelAnnotation  NodeLabel = "Annotation"
	LabelTemplate    NodeLabel = "Template"
	LabelSection     NodeLabel = "Section"
	LabelRoute       NodeLabel = "Route"
	LabelTool        NodeLabel = "Tool"
	LabelBasicBlock  NodeLabel = "BasicBlock"
)

// DefID is the unique identifier for a definition node in the graph.
type DefID = string

// ScopeID is the unique identifier for a scope node in the graph.
type ScopeID = string

// SymbolDefinition — the core data record emitted by extractors and stored
// in SymbolTable + owner-scoped registries.
// Optional fields use *string/*int pointers (nil = undefined).
type SymbolDefinition struct {
	NodeID                string
	FilePath              string
	Type                  NodeLabel
	QualifiedName         *string
	ParameterCount        *int
	RequiredParameterCount *int
	ParameterTypes        []string
	ParameterTypeClasses  []ParameterTypeClass
	ReturnType            *string
	DeclaredType          *string
	TemplateArguments     []string
	OwnerID               *string
	NamespacePrefix       *string // sidecar namespace prefix (tagNamespacePrefixes), nil = none
	IsDeleted             bool
}

// Capture represents a named capture from a tree-sitter query match.
type Capture struct {
	Name  string
	Node  interface{} // *gotreesitter.Node — using interface{} to avoid import cycle
	Range Range
	Text  string
}

// GenerateID builds a deterministic ID string for a graph node.
func GenerateID(label, name string) string {
	return label + ":" + name
}

// ---------------------------------------------------------------------------
// Scope Resolution types — ported from gitnexus-shared scope-resolution/types.ts
// ---------------------------------------------------------------------------

// ScopeKind represents the kind of lexical scope.
type ScopeKind string

const (
	ScopeKindModule     ScopeKind = "Module"     // file root
	ScopeKindNamespace  ScopeKind = "Namespace"  // C++ namespace, C# namespace, Rust mod
	ScopeKindClass      ScopeKind = "Class"      // class/struct/trait/interface body
	ScopeKindFunction   ScopeKind = "Function"   // function/method/closure/lambda body
	ScopeKindBlock      ScopeKind = "Block"      // { ... }, if-body, for-body, match arms
	ScopeKindExpression ScopeKind = "Expression" // comprehensions, for-init, pattern bindings
)

// CaptureMatch groups captures from a single query match, keyed by capture name.
type CaptureMatch map[string]Capture

// ParsedImportKind discriminates the 8 import variants.
type ParsedImportKind string

const (
	ParsedImportNamed            ParsedImportKind = "named"
	ParsedImportAlias            ParsedImportKind = "alias"
	ParsedImportNamespace        ParsedImportKind = "namespace"
	ParsedImportReexport         ParsedImportKind = "reexport"
	ParsedImportWildcard         ParsedImportKind = "wildcard"
	ParsedImportDynamicUnresolved ParsedImportKind = "dynamic-unresolved"
	ParsedImportDynamicResolved  ParsedImportKind = "dynamic-resolved"
	ParsedImportSideEffect       ParsedImportKind = "side-effect"
)

// ParsedImport represents a provider-interpreted raw import.
// Discriminated union: each variant carries only the fields relevant to its kind.
type ParsedImport struct {
	Kind         ParsedImportKind
	LocalName    string  // scope-visible name (named/alias/namespace/reexport/dynamic-unresolved)
	ImportedName string  // name in the source module (named/alias/namespace/reexport)
	Alias        string  // rename (alias/reexport only)
	TargetRaw    *string // raw target path; nil for dynamic-unresolved with no string form
}

// ParsedTypeBinding is a provider-interpreted type binding.
type ParsedTypeBinding struct {
	BoundName    string
	RawTypeName  string
	Source       TypeRefSource
}

// TypeRefSource discriminates the 7 sources of a type reference.
type TypeRefSource string

const (
	TypeRefSourceAnnotation          TypeRefSource = "annotation"
	TypeRefSourceParameterAnnotation TypeRefSource = "parameter-annotation"
	TypeRefSourceReturnAnnotation    TypeRefSource = "return-annotation"
	TypeRefSourceSelf                TypeRefSource = "self"
	TypeRefSourceAssignmentInferred  TypeRefSource = "assignment-inferred"
	TypeRefSourceConstructorInferred TypeRefSource = "constructor-inferred"
	TypeRefSourceReceiverPropagated  TypeRefSource = "receiver-propagated"
)

// ImportEdgeKind discriminates the 8 import edge variants.
type ImportEdgeKind string

const (
	ImportEdgeNamed            ImportEdgeKind = "named"
	ImportEdgeAlias            ImportEdgeKind = "alias"
	ImportEdgeNamespace        ImportEdgeKind = "namespace"
	ImportEdgeWildcardExpanded ImportEdgeKind = "wildcard-expanded"
	ImportEdgeReexport         ImportEdgeKind = "reexport"
	ImportEdgeDynamicUnresolved ImportEdgeKind = "dynamic-unresolved"
	ImportEdgeDynamicResolved  ImportEdgeKind = "dynamic-resolved"
	ImportEdgeSideEffect       ImportEdgeKind = "side-effect"
)

// ImportEdge — a cross-file import edge attached to a module/namespace scope.
type ImportEdge struct {
	LocalName         string
	TargetFile        *string    // nil only when Kind == dynamic-unresolved
	TargetExportedName string
	TargetModuleScope *ScopeID   // filled at finalize
	TargetDefID       *DefID     // filled at finalize
	Kind              ImportEdgeKind
	TransitiveVia     []string   // re-export chain provenance
	LinkStatus        *string    // "unresolved" when SCC fixpoint failed
}

// BindingOrigin discriminates the 5 binding provenance origins.
type BindingOrigin string

const (
	OriginLocal     BindingOrigin = "local"
	OriginImport    BindingOrigin = "import"
	OriginNamespace BindingOrigin = "namespace"
	OriginWildcard  BindingOrigin = "wildcard"
	OriginReexport  BindingOrigin = "reexport"
)

// BindingRef — a name binding visible at a scope, with provenance.
type BindingRef struct {
	Def    SymbolDefinition
	Origin BindingOrigin
	Via    *ImportEdge // non-nil for non-local origins
}

// TypeRef — a reference to a named type, anchored at its declaration site.
type TypeRef struct {
	RawName         string
	DeclaredAtScope ScopeID
	Source          TypeRefSource
	TypeArgs        []TypeRef // reserved for V2+: generic type arguments
}

// Callsite describes a call site for arity compatibility checks.
type Callsite struct {
	Arity         *int
	ArgumentTypes []string // empty string = type not inferred
}

// Scope — the canonical lexical-scope node. Forms the spine of the SemanticModel.
type Scope struct {
	ID          ScopeID
	Parent      *ScopeID
	Kind        ScopeKind
	Range       Range
	FilePath    string
	Bindings    map[string][]BindingRef    // names visible from this scope
	OwnedDefs   []SymbolDefinition         // defs structurally owned by this scope
	Imports     []ImportEdge               // import edges attached to this scope
	TypeBindings map[string]TypeRef        // local type facts
}

// ScopeLookup — minimal scope-lookup contract: map a ScopeID back to its Scope.
type ScopeLookup interface {
	GetScope(id ScopeID) *Scope
}

// ResolutionEvidenceKind discriminates the 10 evidence kinds.
type ResolutionEvidenceKind string

const (
	EvidenceLocal                ResolutionEvidenceKind = "local"
	EvidenceScopeChain           ResolutionEvidenceKind = "scope-chain"
	EvidenceImport               ResolutionEvidenceKind = "import"
	EvidenceTypeBinding          ResolutionEvidenceKind = "type-binding"
	EvidenceOwnerMatch           ResolutionEvidenceKind = "owner-match"
	EvidenceKindMatch            ResolutionEvidenceKind = "kind-match"
	EvidenceArityMatch           ResolutionEvidenceKind = "arity-match"
	EvidenceGlobalName           ResolutionEvidenceKind = "global-name"
	EvidenceGlobalQualified      ResolutionEvidenceKind = "global-qualified"
	EvidenceDynamicImportUnresolved ResolutionEvidenceKind = "dynamic-import-unresolved"
)

// ResolutionEvidence — one piece of evidence for a Resolution.
type ResolutionEvidence struct {
	Kind   ResolutionEvidenceKind
	Weight float64
	Note   string // optional debug annotation
}

// Resolution — a ranked resolution candidate returned by registry lookups.
type Resolution struct {
	Def        SymbolDefinition
	Confidence float64
	Evidence   []ResolutionEvidence
	Path       []ScopeID // optional debug trace: scopes walked
}

// ReferenceKind discriminates the 7 reference kinds.
type ReferenceKind string

const (
	ReferenceCall           ReferenceKind = "call"
	ReferenceRead           ReferenceKind = "read"
	ReferenceWrite          ReferenceKind = "write"
	ReferenceTypeReference  ReferenceKind = "type-reference"
	ReferenceInherits       ReferenceKind = "inherits"
	ReferenceImportUse      ReferenceKind = "import-use"
	ReferenceMacro          ReferenceKind = "macro"
)

// Reference — a post-resolution usage fact.
type Reference struct {
	FromScope  ScopeID
	ToDef      DefID
	AtRange    Range
	Kind       ReferenceKind
	Confidence float64
	Evidence   []ResolutionEvidence
}

// ReferenceIndex — two-way index over Reference records.
type ReferenceIndex struct {
	BySourceScope map[ScopeID][]Reference
	ByTargetDef   map[DefID][]Reference
}

// LookupParams — parameters accepted by Registry.lookup.
type LookupParams struct {
	AcceptedKinds          []NodeLabel
	UseReceiverTypeBinding bool
	OwnerScopedContributor interface{} // typed concretely in registries package
	ArityHint              *int
	ExplicitReceiver       *string // receiver name, e.g. "user" in user.save()
}

// RegistryContributor — opaque placeholder for per-kind registry contributor.
type RegistryContributor = interface{}

// ReferenceSite — a pre-resolved use-site reference, enriched from source.
// Full version ported from gitnexus-shared reference-site.ts.
type ReferenceSite struct {
	DefID               DefID
	FilePath            string
	Range               Range
	SymbolName          string
	RawQualifiedName    *string         // dotted name as written, e.g. "app.models.User"
	Kind                ReferenceKind   // call/read/write/type-reference/inherits/import-use/macro
	CallForm            CallForm        // how the reference was made
	ExplicitReceiver    *string         // e.g. "user" in user.save()
	InScope             ScopeID         // scope where this reference occurs
	Arity               *int            // number of arguments at call site
	ArgumentTypes       []string        // inferred argument types
	ArgumentTypeClasses []ParameterTypeClass
}

// CallForm discriminates the 4 call-site forms.
type CallForm string

const (
	CallFormFree        CallForm = "free"        // bare function call: foo()
	CallFormMethod      CallForm = "method"      // receiver call: obj.method()
	CallFormStatic      CallForm = "static"      // Class.staticMethod()
	CallFormConstructor CallForm = "constructor" // new ClassName()
)

// FinalizedScc — a strongly-connected component in the file-level import graph.
type FinalizedScc struct {
	FilePaths []string
}

// FinalizeStats — coarse-grained statistics from the finalize pass.
type FinalizeStats struct {
	ScopeCount      int
	DefCount        int
	ImportEdgeCount int
	BindingCount    int
}
