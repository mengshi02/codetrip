// Package golang implements the Go language provider for the codetrip ingestion pipeline.
//
// provider.go — The LanguageProvider entry point for Go.
// This file wires all Go-specific extraction hooks into the core.LanguageProvider interface.
//
// Ported from TS languages/go.ts (goProvider factory).
package golang

import (
	"strconv"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// GoProvider returns the Go LanguageProvider instance.
func GoProvider() core.LanguageProvider {
	return &goProviderImpl{}
}

// goProviderImpl implements core.LanguageProvider for Go.
type goProviderImpl struct{}

func (p *goProviderImpl) Language() shared.SupportedLanguage { return shared.SupportedLanguageGo }

func (p *goProviderImpl) ExtractSymbols(source []byte, filePath string, config *core.LanguageConfig) ([]shared.SymbolDefinition, error) {
	captures := EmitGoScopeCaptures(source, filePath)
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
		var returnType *string

		if _, ok := cm["@declaration.function"]; ok {
			label = shared.LabelFunction
			paramCount, reqParamCount, paramTypes, returnType = extractGoArityFromCaptures(cm)
		} else if _, ok := cm["@declaration.method"]; ok {
			label = shared.LabelMethod
			paramCount, reqParamCount, paramTypes, returnType = extractGoArityFromCaptures(cm)
		} else if _, ok := cm["@declaration.struct"]; ok {
			label = shared.LabelStruct
		} else if _, ok := cm["@declaration.interface"]; ok {
			label = shared.LabelInterface
		} else if _, ok := cm["@declaration.type"]; ok {
			label = shared.LabelTypeAlias
		} else if _, ok := cm["@declaration.variable"]; ok {
			label = shared.LabelVariable
		} else if _, ok := cm["@declaration.const"]; ok {
			label = shared.LabelConst
		} else if _, ok := cm["@declaration.field"]; ok {
			label = shared.LabelProperty
		} else if _, ok := cm["@declaration.package"]; ok {
			label = shared.LabelPackage
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
			ReturnType:             returnType,
		}

		// Extract type binding if present
		if typeCap, ok := cm["@type-binding.type"]; ok {
			bindingName := ""
			if bn, ok := cm["@type-binding.name"]; ok {
				bindingName = bn.Text
			}
			binding := InterpretGoTypeBinding(cm)
			if binding.BoundName != "" {
				normalized := NormalizeGoTypeName(binding.RawTypeName)
				def.DeclaredType = &normalized
			}
			_ = typeCap
			_ = bindingName
		}

		// Mark exported via namespace prefix (uppercase first letter in Go)
		if len(nameCap.Text) > 0 && nameCap.Text[0] >= 'A' && nameCap.Text[0] <= 'Z' {
			exported := "exported"
			def.NamespacePrefix = &exported
		}

		defs = append(defs, def)
	}
	return defs, nil
}

func (p *goProviderImpl) ExtractImports(source []byte, filePath string, config *core.LanguageConfig) ([]core.ImportEdge, error) {
	captures := EmitGoScopeCaptures(source, filePath)
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

		parsed := InterpretGoImport(cm)
		if parsed.Kind == "" {
			continue
		}

		// Stdlib imports (no dots) are not resolved to local files
		if isGoStdlibPackage(sourceCap.Text) {
			continue
		}

		kind := core.ImportKindPackage
		switch parsed.Kind {
		case shared.ParsedImportAlias:
			kind = core.ImportKindDirect
		case shared.ParsedImportWildcard:
			kind = core.ImportKindPackage
		case shared.ParsedImportNamespace:
			kind = core.ImportKindPackage
		}

		edge := core.ImportEdge{
			SourceFile: filePath,
			ImportPath: sourceCap.Text,
			Kind:       kind,
			Line:       sourceCap.Range.StartLine,
		}
		edges = append(edges, edge)
	}
	return edges, nil
}

func (p *goProviderImpl) ResolveImportTarget(importPath string, fromFile string, workspace shared.WorkspaceIndex) (string, core.ImportKind) {
	allPaths := workspaceToMap(workspace)

	var resolutionConfig interface{}
	// Note: in a full implementation, this would come from the LanguageConfig
	// For now, we pass nil and rely on suffix/basename matching

	resolved := ResolveGoImportTarget(importPath, fromFile, allPaths, resolutionConfig)
	if len(resolved) == 0 {
		return "", core.ImportKindUnresolved
	}
	return resolved[0], core.ImportKindDirect
}

func (p *goProviderImpl) MergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	return GoMergeBindings(existing, incoming, scopeID)
}

func (p *goProviderImpl) ExtractScopes(source []byte, filePath string) ([]core.ScopeInfo, error) {
	captures := EmitGoScopeCaptures(source, filePath)
	if len(captures) == 0 {
		return nil, nil
	}

	var scopes []core.ScopeInfo
	for _, cm := range captures {
		var kind, name string
		var startRow, endRow int
		var isExported bool

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
			isExported = isGoExported(name)
		} else if scopeCap, ok := cm["@scope.function"]; ok {
			kind = "function"
			if nameCap, ok := cm["@declaration.name"]; ok {
				name = nameCap.Text
			}
			startRow = scopeCap.Range.StartLine
			endRow = scopeCap.Range.EndLine
			isExported = isGoExported(name)
		} else if scopeCap, ok := cm["@scope.block"]; ok {
			kind = "block"
			startRow = scopeCap.Range.StartLine
			endRow = scopeCap.Range.EndLine
		} else {
			continue
		}

		scopes = append(scopes, core.ScopeInfo{
			ScopeID:    shared.GenerateID(kind, filePath+":"+name),
			Name:       name,
			Kind:       kind,
			StartRow:   startRow,
			EndRow:     endRow,
			IsExported: isExported,
		})
	}
	return scopes, nil
}

func (p *goProviderImpl) IsTestFile(filePath string) bool {
	return strings.HasSuffix(filePath, "_test.go")
}

func (p *goProviderImpl) FrameworkHints() []core.FrameworkHintExt {
	// Go frameworks: standard library patterns
	return nil
}

// --- helper functions ---

func isGoExported(name string) bool {
	return len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z'
}

func extractGoArityFromCaptures(cm CaptureMatch) (*int, *int, []string, *string) {
	var paramCount *int
	var reqParamCount *int
	var paramTypes []string
	var returnType *string

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
			text := pt.Text
			text = strings.TrimPrefix(text, "[")
			text = strings.TrimSuffix(text, "]")
			if text != "" {
				paramTypes = strings.Split(text, ",")
			}
		}
	}
	if rt, ok := cm["@declaration.return-type"]; ok {
		if rt.Text != "" {
			returnType = &rt.Text
		}
	}
	return paramCount, reqParamCount, paramTypes, returnType
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
