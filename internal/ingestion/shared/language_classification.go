// Package shared — Language classification for scope-resolution behavior.
// Ported from gitnexus-shared scope-resolution/language-classification.ts (50 lines).
package shared

// LanguageClassification holds language-specific behavioral flags that affect
// scope resolution. Different languages have different rules for:
//   - Whether namespace imports contribute bindings
//   - Whether wildcard imports are allowed
//   - Whether multiple inheritance is supported
//   - Whether re-exports exist
type LanguageClassification struct {
	// HasNamespaces indicates whether the language supports namespace imports.
	// C++/C#/Rust: true (using namespace, import X, use X)
	// Python/Java/Go/JS/TS: false
	HasNamespaces bool
	// HasWildcardImports indicates whether the language supports wildcard/star imports.
	// Python: true (from X import *)
	// TS/JS: true (export * from)
	// Others: false
	HasWildcardImports bool
	// HasReexports indicates whether the language supports re-exports.
	// TS/JS: true (export { X } from './y')
	// Rust: true (pub use)
	// Others: false
	HasReexports bool
	// HasMultipleInheritance indicates whether the language supports multiple inheritance.
	// C++: true
	// Python: true
	// Others: false
	HasMultipleInheritance bool
	// HasInterfaceDefaults indicates whether the language supports default method
	// implementations on interfaces.
	// Java 8+/C# 8+/Rust: true
	// Others: false
	HasInterfaceDefaults bool
}

// LanguageClassifications maps each SupportedLanguage to its classification.
// All 9 core languages are listed.
var LanguageClassifications = map[SupportedLanguage]LanguageClassification{
	SupportedLanguageC: {
		HasNamespaces:          false,
		HasWildcardImports:     false,
		HasReexports:           false,
		HasMultipleInheritance: false,
		HasInterfaceDefaults:   false,
	},
	SupportedLanguageCpp: {
		HasNamespaces:          true,
		HasWildcardImports:     false,
		HasReexports:           false,
		HasMultipleInheritance: true,
		HasInterfaceDefaults:   false,
	},
	SupportedLanguageCSharp: {
		HasNamespaces:          true,
		HasWildcardImports:     false,
		HasReexports:           false,
		HasMultipleInheritance: false,
		HasInterfaceDefaults:   true,
	},
	SupportedLanguageGo: {
		HasNamespaces:          false,
		HasWildcardImports:     false,
		HasReexports:           false,
		HasMultipleInheritance: false,
		HasInterfaceDefaults:   false,
	},
	SupportedLanguageJava: {
		HasNamespaces:          false,
		HasWildcardImports:     false,
		HasReexports:           false,
		HasMultipleInheritance: false,
		HasInterfaceDefaults:   true,
	},
	SupportedLanguageJavaScript: {
		HasNamespaces:          false,
		HasWildcardImports:     true,
		HasReexports:           true,
		HasMultipleInheritance: false,
		HasInterfaceDefaults:   false,
	},
	SupportedLanguagePython: {
		HasNamespaces:          false,
		HasWildcardImports:     true,
		HasReexports:           false,
		HasMultipleInheritance: true,
		HasInterfaceDefaults:   false,
	},
	SupportedLanguageRust: {
		HasNamespaces:          true,
		HasWildcardImports:     true,
		HasReexports:           true,
		HasMultipleInheritance: false,
		HasInterfaceDefaults:   true,
	},
	SupportedLanguageTypeScript: {
		HasNamespaces:          false,
		HasWildcardImports:     true,
		HasReexports:           true,
		HasMultipleInheritance: false,
		HasInterfaceDefaults:   false,
	},
}

// IsProductionLanguage returns true for all 9 core languages.
// Non-production languages (COBOL, Markdown) are excluded from scope resolution.
func IsProductionLanguage(lang SupportedLanguage) bool {
	_, ok := LanguageClassifications[lang]
	return ok
}