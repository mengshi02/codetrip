package c

import (
	"regexp"
	"strings"
)

// CImportInfo represents a parsed C #include import.
type CImportInfo struct {
	Kind       string // "wildcard" for C includes
	TargetRaw  string // The include path
	IsSystem   bool   // Whether this is a system header (<stdio.h>)
}

// InterpretCImport interprets a C #include capture into a CImportInfo.
// C includes are always wildcard imports (all symbols from the header).
// System headers (e.g. <stdio.h>) are not resolved to local files.
// Ported from GitNexus c/interpret.ts.
func InterpretCImport(source, system string) *CImportInfo {
	if source == "" {
		return nil
	}
	// System headers are not resolved to local files
	if system != "" {
		return nil
	}
	return &CImportInfo{
		Kind:      "wildcard",
		TargetRaw: source,
	}
}

// CTypeBindingInfo represents a parsed C type binding.
type CTypeBindingInfo struct {
	BoundName    string
	RawTypeName  string
	Source       string // "annotation", "parameter-annotation", "assignment-inferred"
}

// InterpretCTypeBinding interprets a C type-binding capture.
// Ported from GitNexus c/interpret.ts.
func InterpretCTypeBinding(name, typeName, parameter, assignment string) *CTypeBindingInfo {
	if name == "" || typeName == "" {
		return nil
	}

	source := "annotation"
	if parameter != "" {
		source = "parameter-annotation"
	} else if assignment != "" {
		source = "assignment-inferred"
	}

	return &CTypeBindingInfo{
		BoundName:   name,
		RawTypeName: NormalizeCTypeName(typeName),
		Source:      source,
	}
}

// NormalizeCTypeName normalizes a C type name: strips pointer/array syntax, qualifiers.
// Ported from GitNexus c/interpret.ts.
func NormalizeCTypeName(text string) string {
	t := strings.TrimSpace(text)
	// Strip const, volatile, restrict, static, extern, inline qualifiers
	t = cQualifierRe.ReplaceAllString(t, "")
	t = strings.TrimSpace(t)
	// Strip pointer stars
	for strings.HasSuffix(t, "*") {
		t = strings.TrimSpace(t[:len(t)-1])
	}
	for strings.HasPrefix(t, "*") {
		t = strings.TrimSpace(t[1:])
	}
	// Strip array brackets
	t = cArrayBracketRe.ReplaceAllString(t, "")
	t = strings.TrimSpace(t)
	// Strip struct/union/enum prefixes
	t = cStructPrefixRe.ReplaceAllString(t, "")
	return t
}

var (
	cQualifierRe    = regexp.MustCompile(`\b(const|volatile|restrict|static|extern|inline)\b`)
	cArrayBracketRe = regexp.MustCompile(`\[.*?\]`)
	cStructPrefixRe = regexp.MustCompile(`^(struct|union|enum)\s+`)
)