package core

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/odvcencio/gotreesitter"
)

// MethodVisibility reuses FieldVisibility — method and field modifiers share the same set.
type MethodVisibility = FieldVisibility

// ParameterTypeClass classifies parameter passing semantics (value/ref/out/ptr).
type ParameterTypeClass string

const (
	ParamTypeClassValue ParameterTypeClass = "value"
	ParamTypeClassRef   ParameterTypeClass = "ref"
	ParamTypeClassOut   ParameterTypeClass = "out"
	ParamTypeClassPtr   ParameterTypeClass = "ptr"
)

// ParameterInfo represents a single method/constructor parameter.
type ParameterInfo struct {
	Name       string
	Type       *string             // nil = untyped
	RawType    *string             // nil = absent — full type text including generics (e.g. vector<int>)
	TypeClass  *ParameterTypeClass // nil = absent — resolved parameter type class
	IsOptional bool
	IsVariadic bool // e.g. Java ...params, Go ...interface{}, TS ...rest
}

// MethodInfo represents a method or constructor extracted from a class/struct/interface.
type MethodInfo struct {
	Name          string
	ReceiverType  *string // nil = absent (Go receiver, Kotlin extension)
	ReturnType    *string // nil = untyped
	Visibility    MethodVisibility
	IsStatic      bool
	IsConstructor bool
	IsAbstract    bool
	IsFinal       bool
	IsOverride    bool // C# virtual/override, Java @Override
	IsAsync       bool // TS async, C# async
	IsSealed      bool // C# sealed
	IsVirtual     bool // C# virtual
	IsNew         bool // C# new (hiding inherited member)
	IsConst       bool // C++ const method
	IsPartial     bool // C# partial method
	IsDeleted     bool // C++ = delete
	Parameters    []ParameterInfo
	Annotations   []string // nil = absent
	SourceFile    string
	Line          int
}

// MethodExtractorContext holds context passed during method extraction.
type MethodExtractorContext struct {
	FilePath string
	Language SupportedLanguage
}

// ExtractedMethods is the result of method extraction from a type declaration.
type ExtractedMethods struct {
	OwnerName string
	Methods   []MethodInfo
}

// MethodExtractor extracts method declarations from AST nodes.
type MethodExtractor interface {
	Language() SupportedLanguage
	Extract(node *gotreesitter.Node, ctx *MethodExtractorContext, source []byte, lang *gotreesitter.Language) *ExtractedMethods
	IsTypeDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool
	ExtractFromNode(node *gotreesitter.Node, ctx *MethodExtractorContext, source []byte, lang *gotreesitter.Language) *MethodInfo
	ExtractFunctionName(node *gotreesitter.Node, filePath *string, lang *gotreesitter.Language) *FunctionNameResult
}

// FunctionNameResult is returned by ExtractFunctionName.
// Provides both the function name and its node label for languages
// with non-standard AST structures (e.g. C/C++ declarator unwrapping).
type FunctionNameResult struct {
	FuncName *string
	Label    shared.NodeLabel
}

// MethodExtractionConfig defines per-language method extraction configuration.
// Each field mirrors the TS property; nil func fields mean "not set".
type MethodExtractionConfig struct {
	Language             SupportedLanguage
	TypeDeclarationNodes []string
	MethodNodeTypes      []string
	BodyNodeTypes        []string        // optional: nil = not set
	ConstructorNodeTypes []string        // optional: nil = not set
	VisibilityModifiers  []string        // optional: nil = not set
	StaticOwnerTypes     map[string]bool // nil = not set (triggers invariant check)

	// Required hooks
	ExtractName       func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string
	ExtractReturnType func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string
	ExtractParameters func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []ParameterInfo
	ExtractVisibility func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) MethodVisibility
	IsStatic          func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool
	IsAbstract        func(node *gotreesitter.Node, ownerNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool
	IsFinal           func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool

	// Optional hooks (nil = not set)
	ExtractAnnotations        func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string
	ExtractReceiverType       func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string
	IsVirtual                 func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool
	IsOverride                func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool
	IsAsync                   func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool
	IsPartial                 func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool
	IsConst                   func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool
	IsDeleted                 func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool
	ExtractOwnerName          func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string
	ExtractPrimaryConstructor func(ownerNode *gotreesitter.Node, ctx *MethodExtractorContext, source []byte, lang *gotreesitter.Language) *MethodInfo
	ExtractFunctionName       func(node *gotreesitter.Node, filePath *string, lang *gotreesitter.Language) *FunctionNameResult
}
