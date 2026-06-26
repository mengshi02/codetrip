package core

import "github.com/mengshi02/codetrip/internal/ingestion/model"

// FieldVisibility represents visibility levels used across all supported languages.
//   - public / private / protected: universal modifiers
//   - internal: C# (assembly scope)
//   - protected internal: C# (accessible by same assembly OR derived classes)
//   - private protected: C# (accessible by derived classes within same assembly)
//   - package: Java (package-private, no keyword)
//   - fileprivate: Swift (file scope) — retained for interface compat
//   - open: Swift (subclassable across modules) — retained for interface compat
type FieldVisibility string

const (
	VisibilityPublic            FieldVisibility = "public"
	VisibilityPrivate           FieldVisibility = "private"
	VisibilityProtected         FieldVisibility = "protected"
	VisibilityInternal          FieldVisibility = "internal"
	VisibilityProtectedInternal FieldVisibility = "protected internal"
	VisibilityPrivateProtected  FieldVisibility = "private protected"
	VisibilityPackage           FieldVisibility = "package"
	VisibilityFilePrivate       FieldVisibility = "fileprivate"
	VisibilityOpen              FieldVisibility = "open"
)

// FieldInfo represents a field or property within a class/struct/interface.
type FieldInfo struct {
	Name       string
	Type       *string          // null = nil (untyped)
	Visibility FieldVisibility
	IsStatic   bool
	IsReadonly bool
	SourceFile string
	Line       int
}

// FieldTypeMap maps owner type FQN to its fields.
type FieldTypeMap map[string][]FieldInfo

// FieldExtractorContext provides context for field extraction.
type FieldExtractorContext struct {
	TypeEnv     *TypeEnv
	SymbolTable model.SymbolTableReader
	FilePath    string
	Language    SupportedLanguage
}

// ExtractedFields is the result of field extraction from a type declaration.
type ExtractedFields struct {
	OwnerFQN    string
	Fields      []FieldInfo
	NestedTypes []string
}