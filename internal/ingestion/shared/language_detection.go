// Language Detection — maps file paths to SupportedLanguage enum values.
//
// Mirrors TS gitnexus-shared/src/language-detection.ts, adapted for
// codetrip's 9 core languages (no Ruby, PHP, Kotlin, Swift, Dart, Vue, Cobol).
//
// The reverse lookup map (extToLang) is built once at init time; all
// subsequent calls to GetLanguageFromFilename are O(1) on the extension.
// This avoids rebuilding the map on every call, which the TS version also
// does via a module-level pre-built Map.

package shared

import (
	"path/filepath"
	"strings"
)

// ─── Extension → Language forward map ──────────────────────────

// extensionMap maps each SupportedLanguage to its recognised file extensions.
// Only the 9 core languages are included (no Ruby/PHP/Kotlin/Swift/Dart/Vue/Cobol).
//
// When adding a new language:
//  1. Add the SupportedLanguage constant in types.go
//  2. Add the extension slice here
//  3. The init() builder will pick it up automatically
var extensionMap = map[SupportedLanguage][]string{
	SupportedLanguageTypeScript: {".ts", ".tsx", ".mts", ".cts"},
	SupportedLanguageJavaScript: {".js", ".jsx", ".mjs", ".cjs"},
	SupportedLanguagePython:     {".py"},
	SupportedLanguageJava:       {".java"},
	SupportedLanguageC:          {".c"},
	SupportedLanguageCpp:        {".cpp", ".cc", ".cxx", ".h", ".hpp", ".hxx", ".hh"},
	SupportedLanguageCSharp:     {".cs"},
	SupportedLanguageGo:         {".go"},
	SupportedLanguageRust:       {".rs"},
}

// ─── Reverse lookup (extension → language) ────────────────────

// extToLang is the pre-built reverse lookup: lowercase extension → SupportedLanguage.
// Populated once at init time from extensionMap.
var extToLang map[string]SupportedLanguage

func init() {
	extToLang = make(map[string]SupportedLanguage, 40) // ~40 total extensions across 9 languages
	for lang, exts := range extensionMap {
		for _, ext := range exts {
			extToLang[strings.ToLower(ext)] = lang
		}
	}
}

// ─── Public API ──────────────────────────────────────────────

// GetLanguageFromFilename maps a filename (or full path) to its SupportedLanguage.
// Returns the empty string ("") if the file extension is not recognised.
//
// Mirrors TS getLanguageFromFilename, simplified:
//   - No Blade template check (PHP not in core languages)
//   - No Ruby extensionless filenames (Ruby not in core languages)
//   - Uses filepath.Ext for robust extension extraction
func GetLanguageFromFilename(filename string) SupportedLanguage {
	ext := strings.ToLower(filepath.Ext(filename))
	if ext == "" {
		return ""
	}
	if lang, ok := extToLang[ext]; ok {
		return lang
	}
	return ""
}

// IsKnownLanguage returns true if the given SupportedLanguage value is one
// of the 9 core languages recognised by the ingestion pipeline.
func IsKnownLanguage(lang SupportedLanguage) bool {
	_, ok := extensionMap[lang]
	return ok
}

// ExtensionsForLanguage returns the file extensions associated with a
// SupportedLanguage. Returns nil if the language is not in the core set.
func ExtensionsForLanguage(lang SupportedLanguage) []string {
	return extensionMap[lang]
}

// ─── Syntax highlighting map ──────────────────────────────────

// syntaxMap maps each SupportedLanguage to a Prism-compatible syntax
// identifier for highlighting.  Only the 9 core languages.
var syntaxMap = map[SupportedLanguage]string{
	SupportedLanguageTypeScript: "typescript",
	SupportedLanguageJavaScript: "javascript",
	SupportedLanguagePython:     "python",
	SupportedLanguageJava:       "java",
	SupportedLanguageC:          "c",
	SupportedLanguageCpp:        "cpp",
	SupportedLanguageCSharp:     "csharp",
	SupportedLanguageGo:         "go",
	SupportedLanguageRust:       "rust",
}

// auxiliarySyntaxMap maps non-code file extensions to Prism identifiers.
// Mirrors TS AUXILIARY_SYNTAX_MAP (subset relevant for ingestion context).
var auxiliarySyntaxMap = map[string]string{
	"json":      "json",
	"yaml":      "yaml",
	"yml":       "yaml",
	"md":        "markdown",
	"mdx":       "markdown",
	"html":      "markup",
	"htm":       "markup",
	"xml":       "markup",
	"css":       "css",
	"scss":      "css",
	"sass":      "css",
	"sh":        "bash",
	"bash":      "bash",
	"zsh":       "bash",
	"sql":       "sql",
	"toml":      "toml",
	"ini":       "ini",
	"dockerfile": "docker",
}

// auxiliaryBasenameMap maps extensionless filenames to Prism identifiers.
var auxiliaryBasenameMap = map[string]string{
	"Makefile":   "makefile",
	"Dockerfile": "docker",
}

// GetSyntaxLanguageFromFilename maps a file path to a Prism-compatible syntax
// highlight language string.  Covers all 9 core SupportedLanguages plus
// common non-code formats.  Returns "text" for unrecognised files.
//
// Mirrors TS getSyntaxLanguageFromFilename, adapted for 9 core languages.
func GetSyntaxLanguageFromFilename(filePath string) string {
	// Try core language first
	lang := GetLanguageFromFilename(filePath)
	if lang != "" {
		if s, ok := syntaxMap[lang]; ok {
			return s
		}
	}

	// Try auxiliary extension map
	ext := strings.ToLower(filepath.Ext(filePath))
	if ext != "" {
		// Strip the leading dot for auxiliary lookup
		extNoDot := ext[1:]
		if s, ok := auxiliarySyntaxMap[extNoDot]; ok {
			return s
		}
	}

	// Try auxiliary basename map
	base := filepath.Base(filePath)
	if s, ok := auxiliaryBasenameMap[base]; ok {
		return s
	}

	return "text"
}