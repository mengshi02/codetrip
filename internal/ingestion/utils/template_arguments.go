package utils

import (
	"encoding/json"
	"hash/fnv"
	"strings"
)

// ExtractTemplateArguments parses generic type arguments from text like "Foo<Bar, Baz<T>>".
// Returns the list of argument strings, or nil if no angle brackets found.
// Tracks angle-bracket depth so nested generics are handled correctly.
func ExtractTemplateArguments(text string) []string {
	depth := 0
	start := -1
	var args []string

	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '<' {
			if depth == 0 {
				start = i + 1
			}
			depth++
		} else if ch == '>' {
			depth--
			if depth == 0 && start >= 0 {
				arg := strings.TrimSpace(text[start:i])
				if arg != "" {
					args = append(args, arg)
				}
			}
		} else if ch == ',' && depth == 1 && start >= 0 {
			arg := strings.TrimSpace(text[start:i])
			if arg != "" {
				args = append(args, arg)
			}
			start = i + 1
		}
	}

	// Handle single argument with no comma at depth 1
	if len(args) == 0 && start >= 0 && len(text) > 0 && text[len(text)-1] == '>' {
		arg := strings.TrimSpace(text[start : len(text)-1])
		if arg != "" {
			args = append(args, arg)
		}
	}

	if len(args) == 0 {
		return nil
	}
	return args
}

// StripTemplateArguments removes all generic type arguments from text.
// "Foo<Bar>" → "Foo", "Foo<Bar<Baz>>" → "Foo"
func StripTemplateArguments(text string) string {
	depth := 0
	var result []byte
	for i := 0; i < len(text); i++ {
		ch := text[i]
		if ch == '<' {
			depth++
		} else if ch == '>' {
			depth--
		} else if depth == 0 {
			result = append(result, ch)
		}
	}
	return string(result)
}

// TemplateArgumentsIdTag builds an ID suffix from template arguments.
// E.g. ["Bar", "Baz"] → "~Bar,Baz"
func TemplateArgumentsIdTag(templateArguments []string) string {
	if len(templateArguments) == 0 {
		return ""
	}
	return "~" + strings.Join(templateArguments, ",")
}

// ConstraintsHash computes a deterministic short hash from a JSON string
// using FNV-1a 32-bit, encoded as base-36 (alphanumeric).
// This is the same algorithm as the TS implementation for cross-language
// ID compatibility.
func ConstraintsHash(jsonText string) string {
	h := fnv.New32a()
	h.Write([]byte(jsonText))
	hashVal := h.Sum32()
	return formatBase36(hashVal)
}

// formatBase36 converts a uint32 to base-36 string (0-9, a-z).
func formatBase36(val uint32) string {
	if val == 0 {
		return "0"
	}
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	var result []byte
	for val > 0 {
		result = append(result, digits[val%36])
		val /= 36
	}
	// Reverse for correct order
	for i, j := 0, len(result)-1; i < j; i, j = i+1, j-1 {
		result[i], result[j] = result[j], result[i]
	}
	return string(result)
}

// TemplateConstraintsIdTag builds an ID suffix from template constraint data.
// Serializes payload to JSON, hashes with ConstraintsHash, returns "~c:<hash>".
func TemplateConstraintsIdTag(payload interface{}) string {
	jsonText, err := json.Marshal(payload)
	if err != nil {
		// Fallback: use a deterministic placeholder for non-serializable data
		return "~c:err"
	}
	return "~c:" + ConstraintsHash(string(jsonText))
}