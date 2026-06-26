// Package core defines the fundamental types and constants for the ingestion pipeline.
// All other ingestion modules depend on these definitions.
//
// This is a 1:1 Go reimplementation of the GitNexus TypeScript ingestion core types,
// retaining only the 9 core languages: C, C++, C#, Go, Java, JavaScript, Python, Rust, TypeScript.
package core

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// SupportedLanguage is an alias for shared.SupportedLanguage.
// The canonical definition lives in the shared package; core re-exports it
// for backward compatibility with existing code that references core.SupportedLanguage.
type SupportedLanguage = shared.SupportedLanguage

// Short-form constants for convenience. These are aliases for the
// shared.SupportedLanguageXxx constants, using a shorter naming convention
// (LangXxx) that mirrors the TS SupportedLanguages enum naming.
const (
	LangJavaScript = shared.SupportedLanguageJavaScript
	LangTypeScript = shared.SupportedLanguageTypeScript
	LangPython    = shared.SupportedLanguagePython
	LangJava      = shared.SupportedLanguageJava
	LangC         = shared.SupportedLanguageC
	LangCpp       = shared.SupportedLanguageCpp
	LangCSharp    = shared.SupportedLanguageCSharp
	LangGo        = shared.SupportedLanguageGo
	LangRust      = shared.SupportedLanguageRust
)

// CoreLanguages returns the list of all 9 supported core languages.
func CoreLanguages() []SupportedLanguage {
	return []SupportedLanguage{
		LangJavaScript,
		LangTypeScript,
		LangPython,
		LangJava,
		LangC,
		LangCpp,
		LangCSharp,
		LangGo,
		LangRust,
	}
}

// IsCoreLanguage reports whether the given language is one of the 9 core languages.
func IsCoreLanguage(lang SupportedLanguage) bool {
	return shared.IsProductionLanguage(lang)
}

// MroStrategy represents the Method Resolution Order strategy for a language.
// Mirrors TS's MroStrategy type from gitnexus-shared.
type MroStrategy string

const (
	MroFirstWins       MroStrategy = "first-wins"       // Java/C#/Go: BFS, first match wins
	MroC3              MroStrategy = "c3"                // Python: C3-linearization
	MroLeftmostBase    MroStrategy = "leftmost-base"    // C++: BFS, leftmost base in diamond
	MroImplementsSplit MroStrategy = "implements-split" // Java/C#: BFS with interface-default ambiguity detection
	MroQualifiedSyntax MroStrategy = "qualified-syntax" // Rust: no auto-resolution, requires explicit <Type as Trait>::method
)