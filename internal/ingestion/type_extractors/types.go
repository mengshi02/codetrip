// Package typeextractors provides per-language type extraction configuration
// and shared utilities for building type environments from AST nodes.
//
// Ported from TS type-extractors/types.ts.
package typeextractors

import (
	"github.com/odvcencio/gotreesitter"
)

// ---------------------------------------------------------------------------
// TypeArgPosition — which generic argument represents the element type
// ---------------------------------------------------------------------------

// TypeArgPosition indicates which template/generic argument position holds
// the element type for container iteration.
type TypeArgPosition string

const (
	// TypeArgFirst means the first type argument is the element type (e.g., Map<User, Order> → User).
	TypeArgFirst TypeArgPosition = "first"
	// TypeArgLast means the last type argument is the element type (e.g., Map<User, Order> → Order).
	TypeArgLast TypeArgPosition = "last"
)

// ---------------------------------------------------------------------------
// PendingAssignment — discriminated union for Tier-2 propagation
// ---------------------------------------------------------------------------

// PendingAssignmentKind enumerates the four pending-assignment variants.
type PendingAssignmentKind string

const (
	PAKindCopy             PendingAssignmentKind = "copy"
	PAKindCallResult       PendingAssignmentKind = "callResult"
	PAKindFieldAccess      PendingAssignmentKind = "fieldAccess"
	PAKindMethodCallResult PendingAssignmentKind = "methodCallResult"
)

// PendingAssignment represents a deferred type-propagation item.
// The Kind field determines which other fields are populated:
//   - copy:             Lhs, Rhs
//   - callResult:       Lhs, Callee, (optional CalleeFqn, Line)
//   - fieldAccess:      Lhs, Receiver, Field
//   - methodCallResult: Lhs, Receiver, Method
type PendingAssignment struct {
	Kind    PendingAssignmentKind
	Lhs     string  // always set
	Rhs     string  // copy: the identifier on the RHS
	Callee  string  // callResult: the callee name
	CalleeFqn string // callResult: optional fully-qualified callee
	Line    *int    // callResult: optional source line
	Receiver string // fieldAccess / methodCallResult: receiver object name
	Field   string // fieldAccess: field name
	Method  string // methodCallResult: method name
}

// ---------------------------------------------------------------------------
// PatternBindingResult
// ---------------------------------------------------------------------------

// NarrowingRange describes the AST byte-offset range where a type narrowing
// override should apply (e.g. the body of an if-statement for null-checks).
type NarrowingRange struct {
	StartIndex uint32
	EndIndex   uint32
}

// PatternBindingResult holds the result of extracting a typed variable binding
// from a pattern-matching construct.
type PatternBindingResult struct {
	VarName       string
	TypeName      string
	NarrowingRange *NarrowingRange // optional
}

// ---------------------------------------------------------------------------
// ConstructorBindingResult
// ---------------------------------------------------------------------------

// ConstructorBindingResult is returned by ConstructorBindingScanner when it
// finds an untyped `var = callee()` pattern for return-type inference.
type ConstructorBindingResult struct {
	VarName           string
	CalleeName        string
	ReceiverClassName string // optional hint for method calls on known receivers
}

// ---------------------------------------------------------------------------
// Interface types used by extractors
// ---------------------------------------------------------------------------

// ClassNameLookup checks whether a name is a known class/struct.
// Only the Has method is required.
type ClassNameLookup interface {
	Has(name string) bool
}

// ReturnTypeLookup resolves a callee name to its return type name.
// Backed by SymbolTable.lookupCallableByName.
type ReturnTypeLookup interface {
	// LookupProcessedType returns the processed type name after stripping wrappers
	// (e.g., 'User' from 'Promise<User>'). Used for call-result variable bindings.
	LookupProcessedType(callee string) *string
	// LookupRawReturnType returns the raw return type as declared in the symbol
	// (e.g., '[]User', 'List<User>'). Used for iterable-element extraction.
	LookupRawReturnType(callee string) *string
}

// ---------------------------------------------------------------------------
// Function type aliases
// ---------------------------------------------------------------------------

// TypeBindingExtractor extracts type bindings from a declaration node into the env map.
type TypeBindingExtractor func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string)

// ParameterExtractor extracts type bindings from a parameter node into the env map.
type ParameterExtractor func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string)

// DeclarationTypeNodeLocator optionally locates the type-annotation AST node for a declaration node.
// Returns nil when no type node can be found.
type DeclarationTypeNodeLocator func(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node

// InitializerExtractor extracts type bindings from a constructor-call initializer.
type InitializerExtractor func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string, classNames ClassNameLookup)

// ConstructorBindingScanner scans an AST node for untyped `var = callee()` patterns.
// Returns nil if the node does not match.
type ConstructorBindingScanner func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *ConstructorBindingResult

// ForLoopExtractor extracts loop variable type binding from a for-each statement.
type ForLoopExtractor func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *ForLoopExtractorContext)

// ForLoopExtractorContext groups the parameters passed to ForLoopExtractor.
type ForLoopExtractorContext struct {
	ScopeEnv             map[string]string                // mutable type-env for the current scope
	DeclarationTypeNodes map[string]*gotreesitter.Node    // maps "scope\0varName" to type annotation node
	Scope                string                           // current scope key, e.g. "process@42"
	ReturnTypeLookup     ReturnTypeLookup                 // resolves callee → return type
}

// PendingAssignmentExtractor extracts pending assignments for Tier 2 propagation.
// May return one or more items (for destructuring patterns) or nil.
type PendingAssignmentExtractor func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []PendingAssignment

// PatternBindingExtractor extracts a typed variable binding from a pattern-matching construct.
type PatternBindingExtractor func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string, declarationTypeNodes map[string]*gotreesitter.Node, scope string) *PatternBindingResult

// LiteralTypeInferrer infers the type name of a literal AST node for overload disambiguation.
type LiteralTypeInferrer func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string

// ConstructorTypeDetector detects constructor-style call expressions that don't use `new`.
type ConstructorTypeDetector func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, classNames ClassNameLookup) *string

// DeclaredTypeUnwrapper unwraps a declared type name to its inner type.
// E.g., C++ shared_ptr<Animal> → Animal. Returns nil if no unwrapping applies.
type DeclaredTypeUnwrapper func(declaredType string, typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string

// ---------------------------------------------------------------------------
// LanguageTypeConfig — per-language type extraction configuration
// ---------------------------------------------------------------------------

// LanguageTypeConfig defines per-language type extraction configuration.
// Ported from TS type-extractors/types.ts LanguageTypeConfig.
type LanguageTypeConfig struct {
	// AllowPatternBindingOverwrite enables pattern binding to overwrite existing
	// scopeEnv entries. WARNING: enables function-scope type pollution.
	// Only for languages with smart-cast semantics (e.g. Kotlin when/is).
	AllowPatternBindingOverwrite bool

	// DeclarationNodeTypes lists AST node types that represent typed declarations.
	DeclarationNodeTypes []string

	// GetDeclarationTypeNode optionally locates the type-annotation node for a declaration.
	GetDeclarationTypeNode DeclarationTypeNodeLocator

	// ForLoopNodeTypes lists AST node types for for-each/for-in statements.
	ForLoopNodeTypes []string

	// PatternBindingNodeTypes lists AST node types on which extractPatternBinding should run.
	PatternBindingNodeTypes []string

	// ExtractDeclaration extracts a (varName → typeName) binding from a declaration node.
	ExtractDeclaration TypeBindingExtractor

	// ExtractParameter extracts a (varName → typeName) binding from a parameter node.
	ExtractParameter ParameterExtractor

	// ExtractInitializer extracts a binding from a constructor-call initializer.
	ExtractInitializer InitializerExtractor

	// ScanConstructorBinding scans for untyped `var = callee()` assignments.
	ScanConstructorBinding ConstructorBindingScanner

	// ExtractForLoopBinding extracts loop variable → type binding from a for-each AST node.
	ExtractForLoopBinding ForLoopExtractor

	// ExtractPendingAssignment extracts pending assignment for Tier 2 propagation.
	ExtractPendingAssignment PendingAssignmentExtractor

	// ExtractPatternBinding extracts a typed variable binding from a pattern-matching construct.
	ExtractPatternBinding PatternBindingExtractor

	// InferLiteralType infers the type name of a literal AST node.
	InferLiteralType LiteralTypeInferrer

	// DetectConstructorType detects constructor-style calls that don't use `new`.
	DetectConstructorType ConstructorTypeDetector

	// UnwrapDeclaredType unwraps a declared type to its inner type.
	UnwrapDeclaredType DeclaredTypeUnwrapper
}