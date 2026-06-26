package cpp

import (
	"regexp"
	"strings"
)

// CppImportInfo represents a parsed C++ import.
type CppImportInfo struct {
	Kind        string // "wildcard" or "named"
	TargetRaw   string
	LocalName   string // for named imports (using X::name)
	ImportedName string
}

// InterpretCppImport interprets a C++ import capture.
// C++ has three import forms:
//   1. #include "file.h"  → wildcard import
//   2. using namespace X; → wildcard import
//   3. using X::name;     → named import
// System headers are not resolved to local files.
// Ported from GitNexus cpp/interpret.ts.
func InterpretCppImport(source, system, kind, importedName string) *CppImportInfo {
	if source == "" {
		return nil
	}
	if system != "" {
		return nil // system headers not resolved locally
	}

	if kind == "named" {
		if importedName == "" {
			return nil
		}
		return &CppImportInfo{
			Kind:         "named",
			TargetRaw:    source,
			LocalName:    importedName,
			ImportedName: importedName,
		}
	}

	return &CppImportInfo{
		Kind:      "wildcard",
		TargetRaw: source,
	}
}

// CppTypeBindingInfo represents a parsed C++ type binding.
type CppTypeBindingInfo struct {
	BoundName   string
	RawTypeName string
	Source      string
}

// InterpretCppTypeBinding interprets a C++ type-binding capture.
// Ported from GitNexus cpp/interpret.ts.
func InterpretCppTypeBinding(name, typeName, parameter, constructor, ret, field, memberAccess, memberAccessReceiver, alias, assignment, annotation string) *CppTypeBindingInfo {
	if name == "" || typeName == "" {
		return nil
	}

	source := "annotation"

	if parameter != "" {
		source = "parameter-annotation"
	} else if constructor != "" {
		source = "constructor-inferred"
	} else if ret != "" {
		source = "return-annotation"
	} else if field != "" {
		source = "annotation"
	} else if memberAccess != "" {
		if memberAccessReceiver != "" {
			return &CppTypeBindingInfo{
				BoundName:   name,
				RawTypeName: memberAccessReceiver + "." + typeName,
				Source:      "assignment-inferred",
			}
		}
		source = "assignment-inferred"
	} else if alias != "" {
		source = "assignment-inferred"
	} else if assignment != "" {
		source = "assignment-inferred"
	} else if annotation != "" {
		source = "annotation"
	}

	return &CppTypeBindingInfo{
		BoundName:   name,
		RawTypeName: NormalizeCppTypeName(typeName),
		Source:      source,
	}
}

// NormalizeCppTypeName normalizes a C++ type name: strips pointer/array/reference syntax,
// qualifiers, while preserving template arguments for specialization-aware receiver binding.
// Ported from GitNexus cpp/interpret.ts.
func NormalizeCppTypeName(text string) string {
	t := strings.TrimSpace(text)
	t = cppQualifierRe.ReplaceAllString(t, "")
	t = strings.TrimSpace(t)
	for strings.HasSuffix(t, "*") {
		t = strings.TrimSpace(t[:len(t)-1])
	}
	for strings.HasPrefix(t, "*") {
		t = strings.TrimSpace(t[1:])
	}
	for strings.HasSuffix(t, "&") {
		t = strings.TrimSpace(t[:len(t)-1])
	}
	t = cppArrayBracketRe.ReplaceAllString(t, "")
	t = strings.TrimSpace(t)
	t = cppStructPrefixRe.ReplaceAllString(t, "")
	t = strings.TrimPrefix(t, "::")
	return t
}

var (
	cppQualifierRe    = regexp.MustCompile(`\b(const|volatile|restrict|static|extern|inline|mutable|constexpr|consteval)\b`)
	cppArrayBracketRe = regexp.MustCompile(`\[.*?\]`)
	cppStructPrefixRe = regexp.MustCompile(`^(struct|union|enum|class)\s+`)
)