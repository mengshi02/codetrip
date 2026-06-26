// Package c implements the C language provider for the codetrip ingestion pipeline.
//
// provider.go — The LanguageProvider entry point for C.
// This file wires all C-specific extraction hooks into the core.LanguageProvider interface.
//
// C specifics:
//   - #include as imports
//   - Static linkage (file-local) functions excluded from cross-file resolution
//   - No overloading — arity compatibility is straightforward
//   - No namespaces — flat symbol space with file-level scoping via static
//
// Ported from TS languages/c.ts (cProvider factory).
package c

import (
	"strconv"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CProvider returns the C LanguageProvider instance.
func CProvider() core.LanguageProvider {
	return &cProviderImpl{}
}

// cProviderImpl implements core.LanguageProvider for C.
type cProviderImpl struct{}

func (p *cProviderImpl) Language() shared.SupportedLanguage { return shared.SupportedLanguageC }

func (p *cProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	captures := EmitCScopeCaptures(source, filePath)
	if len(captures) == 0 {
		return nil, nil
	}

	var defs []shared.SymbolDefinition
	for _, cm := range captures {
		nameCap, hasName := cm["@declaration.name"]
		if !hasName {
			continue
		}

		var label shared.NodeLabel
		var paramCount, reqParamCount *int
		var paramTypes []string

		if _, ok := cm["@declaration.function"]; ok {
			label = shared.LabelFunction
			paramCount, reqParamCount, paramTypes = extractArityFromCaptures(cm)
		} else if _, ok := cm["@declaration.struct"]; ok {
			label = shared.LabelStruct
		} else if _, ok := cm["@declaration.union"]; ok {
			label = shared.LabelUnion
		} else if _, ok := cm["@declaration.enum"]; ok {
			label = shared.LabelEnum
		} else if _, ok := cm["@declaration.typedef"]; ok {
			label = shared.LabelTypedef
		} else if _, ok := cm["@declaration.variable"]; ok {
			label = shared.LabelVariable
		} else if _, ok := cm["@declaration.macro"]; ok {
			label = shared.LabelMacro
		} else if _, ok := cm["@declaration.const"]; ok {
			label = shared.LabelConst
		} else if _, ok := cm["@declaration.field"]; ok {
			label = shared.LabelProperty
		} else {
			continue
		}

		def := shared.SymbolDefinition{
			NodeID:                 shared.GenerateID(string(label), nameCap.Text),
			FilePath:               filePath,
			Type:                   label,
			ParameterCount:         paramCount,
			RequiredParameterCount: reqParamCount,
			ParameterTypes:         paramTypes,
		}

		// Mark static linkage
		if IsStaticName(filePath, nameCap.Text) {
			def.NamespacePrefix = strPtr("static")
		}

		// Extract type binding if present
		if typeCap, ok := cm["@type-binding.type"]; ok {
			bindingName := ""
			if bn, ok := cm["@type-binding.name"]; ok {
				bindingName = bn.Text
			}
			binding := InterpretCTypeBinding(bindingName, typeCap.Text, captureText(cm, "@type-binding.parameter"), captureText(cm, "@type-binding.assignment"))
			if binding != nil {
				def.DeclaredType = &binding.RawTypeName
			}
		}

		defs = append(defs, def)
	}
	return defs, nil
}

func (p *cProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	captures := EmitCScopeCaptures(source, filePath)
	if len(captures) == 0 {
		return nil, nil
	}

	var edges []core.ImportEdge
	for _, cm := range captures {
		if _, ok := cm["@import.statement"]; !ok {
			continue
		}
		sourceCap, hasSource := cm["@import.source"]
		if !hasSource || sourceCap.Text == "" {
			continue
		}

		// System includes are not resolved to local files
		if _, isSystem := cm["@import.system"]; isSystem {
			continue
		}

		edge := core.ImportEdge{
			SourceFile: filePath,
			ImportPath: sourceCap.Text,
			Kind:       core.ImportKindPackage, // C includes are wildcard/package-level
			Line:       sourceCap.Range.StartLine,
		}
		edges = append(edges, edge)
	}
	return edges, nil
}

func (p *cProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	allPaths := workspaceToMap(workspace)
	resolved := ResolveCImportTarget(importPath, fromFile, allPaths)
	if resolved == "" {
		return "", core.ImportKindUnresolved
	}
	return resolved, core.ImportKindDirect
}

func (p *cProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	return CMergeBindings(existing, incoming)
}

func (p *cProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	captures := EmitCScopeCaptures(source, filePath)
	if len(captures) == 0 {
		return nil, nil
	}

	var scopes []core.ScopeInfo
	for _, cm := range captures {
		var kind string
		var name string
		var startRow, endRow int

		if scopeCap, ok := cm["@scope.module"]; ok {
			kind = "module"
			name = filePath
			startRow = scopeCap.Range.StartLine
			endRow = scopeCap.Range.EndLine
		} else if scopeCap, ok := cm["@scope.class"]; ok {
			kind = "class"
			if nameCap, ok := cm["@declaration.name"]; ok {
				name = nameCap.Text
			}
			startRow = scopeCap.Range.StartLine
			endRow = scopeCap.Range.EndLine
		} else if scopeCap, ok := cm["@scope.function"]; ok {
			kind = "function"
			if nameCap, ok := cm["@declaration.name"]; ok {
				name = nameCap.Text
			}
			startRow = scopeCap.Range.StartLine
			endRow = scopeCap.Range.EndLine
		} else if scopeCap, ok := cm["@scope.block"]; ok {
			kind = "block"
			startRow = scopeCap.Range.StartLine
			endRow = scopeCap.Range.EndLine
		} else {
			continue
		}

		scopes = append(scopes, core.ScopeInfo{
			ScopeID:  shared.GenerateID(kind, filePath+":"+name),
			Name:     name,
			Kind:     kind,
			StartRow: startRow,
			EndRow:   endRow,
		})
	}
	return scopes, nil
}

func (p *cProviderImpl) IsTestFile(filePath string) bool {
	lower := strings.ToLower(filePath)
	return strings.Contains(lower, "/test/") ||
		strings.Contains(lower, "\\test\\") ||
		strings.Contains(lower, "test_") ||
		strings.Contains(lower, "_test.") ||
		strings.Contains(lower, "/tests/") ||
		strings.Contains(lower, "\\tests\\")
}

func (p *cProviderImpl) FrameworkHints() []core.FrameworkHintExt {
	// C frameworks: Check, Unity, CMocka patterns
	return nil
}

// --- helper functions ---

func strPtr(s string) *string { return &s }

func captureText(cm CaptureMatch, tag string) string {
	if c, ok := cm[tag]; ok {
		return c.Text
	}
	return ""
}

func extractArityFromCaptures(cm CaptureMatch) (*int, *int, []string) {
	var paramCount *int
	var reqParamCount *int
	var paramTypes []string

	if pc, ok := cm["@declaration.parameter-count"]; ok {
		if v := parseIntSafe(pc.Text); v >= 0 {
			paramCount = &v
		}
	}
	if rpc, ok := cm["@declaration.required-parameter-count"]; ok {
		if v := parseIntSafe(rpc.Text); v >= 0 {
			reqParamCount = &v
		}
	}
	if pt, ok := cm["@declaration.parameter-types"]; ok {
		if pt.Text != "" {
			// TS serializes as JSON array: ["int","char*"]
			// Go serializes as bracket-enclosed: [int,char*]
			text := pt.Text
			text = strings.TrimPrefix(text, "[")
			text = strings.TrimSuffix(text, "]")
			if text != "" {
				paramTypes = strings.Split(text, ",")
			}
		}
	}
	return paramCount, reqParamCount, paramTypes
}

func parseIntSafe(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return -1
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return -1
	}
	return n
}

func workspaceToMap(workspace shared.WorkspaceIndex) map[string]bool {
	if workspace == nil {
		return nil
	}
	switch v := workspace.(type) {
	case map[string]bool:
		return v
	case []string:
		m := make(map[string]bool, len(v))
		for _, s := range v {
			m[s] = true
		}
		return m
	default:
		return nil
	}
}
