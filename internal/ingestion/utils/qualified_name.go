package utils

import "strings"

// NormalizeQualifiedName normalizes a qualified name by:
// 1. Removing whitespace
// 2. Stripping leading "::" (C++/Ruby scope)
// 3. Converting "::" and "\" to "." (multi-language scope separator)
// 4. Collapsing repeated "." to single
// 5. Stripping leading/trailing "."
func NormalizeQualifiedName(value string) string {
	// Remove whitespace
	result := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return -1
		}
		return r
	}, value)

	// Strip leading "::"
	if strings.HasPrefix(result, "::") {
		result = result[2:]
	}

	// Convert "::" to "."
	result = strings.ReplaceAll(result, "::", ".")

	// Convert "\" to "."
	result = strings.ReplaceAll(result, `\`, ".")

	// Collapse repeated "." to single
	prevDot := false
	var collapsed []byte
	for i := 0; i < len(result); i++ {
		if result[i] == '.' {
			if prevDot {
				continue
			}
			prevDot = true
		} else {
			prevDot = false
		}
		collapsed = append(collapsed, result[i])
	}
	result = string(collapsed)

	// Strip leading and trailing "."
	result = strings.Trim(result, ".")

	return result
}

// SplitQualifiedName normalizes and then splits a qualified name on ".".
// Returns empty slice for empty/whitespace-only inputs.
func SplitQualifiedName(value string) []string {
	normalized := NormalizeQualifiedName(value)
	if normalized == "" {
		return []string{}
	}
	parts := strings.Split(normalized, ".")
	// Filter empty parts (shouldn't exist after normalization, but be safe)
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// StripTrailingTypeArguments strips trailing generic/type arguments from a name.
// Walks backwards tracking angle-bracket depth; when depth returns to 0,
// returns the substring before the opening "<".
//
// Examples: "Foo<Bar>" → "Foo", "Foo" → "Foo", "Foo<Bar<Baz>>" → "Foo"
func StripTrailingTypeArguments(value string) string {
	depth := 0
	for i := len(value) - 1; i >= 0; i-- {
		ch := value[i]
		if ch == '>' {
			depth++
		} else if ch == '<' {
			depth--
			if depth == 0 {
				return value[:i]
			}
		}
	}
	return value
}
