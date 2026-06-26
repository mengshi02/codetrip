package c

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// CaptureMatch represents a grouped set of captures for one query match.
// Maps capture tag (e.g., "@scope.function") → shared.Capture.
type CaptureMatch map[string]shared.Capture

// EmitCScopeCaptures executes the unified C scope query against the AST and
// synthesizes additional captures based on node structure.
// Ported from GitNexus c/captures.ts.
func EmitCScopeCaptures(source []byte, filePath string) []CaptureMatch {
	lang := grammars.CLanguage()
	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(source)
	if err != nil || tree == nil {
		return nil
	}
	defer tree.Release()

	root := tree.RootNode()
	if root == nil || root.NamedChildCount() == 0 {
		return nil
	}

	query := CScopeQueryCompiled()
	if query == nil {
		return nil
	}

	matches := query.ExecuteNode(root, lang, source)
	if len(matches) == 0 {
		return nil
	}

	var out []CaptureMatch

	// Track ranges where typedef-struct/union/enum was captured as its concrete
	// type so we can suppress the duplicate @declaration.typedef match.
	concreteTypedefRanges := make(map[string]bool)

	for _, m := range matches {
		grouped := make(CaptureMatch)

		for _, qc := range m.Captures {
			tag := "@" + qc.Name
			if strings.HasPrefix(qc.Name, "_") {
				continue // skip anonymous captures
			}
			grouped[tag] = utils.NodeToCapture(tag, qc.Node, source)
		}

		if len(grouped) == 0 {
			continue
		}

		// Handle #include statements
		if includeCap, ok := grouped["@import.statement"]; ok {
			includeNode := findNodeAtCapture(includeCap, root, lang)
			if includeNode != nil && includeNode.Type(lang) == "preproc_include" {
				split := SplitCInclude(lang, includeNode, source)
				if split != nil {
					includeMatch := CaptureMatch{
						"@import.statement": utils.SyntheticCapture("@import.statement", includeNode, includeNode.Text(source)),
						"@import.kind":      utils.SyntheticCapture("@import.kind", includeNode, split.Kind),
						"@import.source":    utils.SyntheticCapture("@import.source", includeNode, split.Source),
					}
					if split.IsSystem {
						includeMatch["@import.system"] = utils.SyntheticCapture("@import.system", includeNode, "true")
					}
					out = append(out, includeMatch)
					continue
				}
			}
		}

		// Track typedef struct/union/enum ranges to suppress duplicate typedef declarations
		for _, tag := range []string{"@declaration.struct", "@declaration.union", "@declaration.enum"} {
			if anchor, ok := grouped[tag]; ok {
				key := fmt.Sprintf("%d:%d:%d:%d", anchor.Range.StartLine, anchor.Range.StartCol, anchor.Range.EndLine, anchor.Range.EndCol)
				concreteTypedefRanges[key] = true
			}
		}

		// Suppress @declaration.typedef if the same range was already captured as a concrete type.
		if typedefAnchor, ok := grouped["@declaration.typedef"]; ok {
			key := fmt.Sprintf("%d:%d:%d:%d", typedefAnchor.Range.StartLine, typedefAnchor.Range.StartCol, typedefAnchor.Range.EndLine, typedefAnchor.Range.EndCol)
			if concreteTypedefRanges[key] {
				continue
			}
		}

		// Enrich function declarations with arity metadata and detect static linkage.
		if _, ok := grouped["@declaration.function"]; ok {
			fnNode := findNodeForCaptureType(grouped, "@declaration.function", root, lang)
			if fnNode != nil {
				nt := fnNode.Type(lang)
				if nt == "function_definition" || nt == "declaration" {
					arity := ComputeCDeclarationArity(lang, fnNode, source)
					if arity.HasParameterCount {
						grouped["@declaration.parameter-count"] = utils.SyntheticCapture(
							"@declaration.parameter-count", fnNode,
							fmt.Sprintf("%d", arity.ParameterCount),
						)
						grouped["@declaration.required-parameter-count"] = utils.SyntheticCapture(
							"@declaration.required-parameter-count", fnNode,
							fmt.Sprintf("%d", arity.RequiredParameterCount),
						)
					}
					if len(arity.ParameterTypes) > 0 {
						grouped["@declaration.parameter-types"] = utils.SyntheticCapture(
							"@declaration.parameter-types", fnNode,
							fmt.Sprintf("[%s]", strings.Join(arity.ParameterTypes, ",")),
						)
					}

					// Detect static storage class (file-local linkage)
					if hasStaticStorageClass(lang, fnNode, source) {
						if nameCap, ok := grouped["@declaration.name"]; ok {
							MarkStaticName(filePath, nameCap.Text)
						}
					}
				}
			}
		}

		// Enrich call references with arity
		if _, hasArity := grouped["@reference.arity"]; !hasArity {
			callNode := findNodeForCaptureType(grouped, "@reference.call.free", root, lang)
			if callNode == nil {
				callNode = findNodeForCaptureType(grouped, "@reference.call.member", root, lang)
			}
			if callNode != nil && callNode.Type(lang) == "call_expression" {
				callArity := ComputeCCallArity(lang, callNode, source)
				grouped["@reference.arity"] = utils.SyntheticCapture(
					"@reference.arity", callNode,
					fmt.Sprintf("%d", callArity),
				)
			}
		}

		out = append(out, grouped)
	}

	return out
}

// hasStaticStorageClass checks if a C function node has `static` storage class.
// Walks direct children for a `storage_class_specifier` node with text `static`.
// Ported from TS c/captures.ts hasStaticStorageClass.
func hasStaticStorageClass(tsLang *gotreesitter.Language, node *gotreesitter.Node, source []byte) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && child.Type(tsLang) == "storage_class_specifier" && child.Text(source) == "static" {
			return true
		}
	}
	return false
}

// findNodeAtCapture looks up the AST node for a capture by its source range.
func findNodeAtCapture(cap shared.Capture, root *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	return utils.FindNodeAtRange(root, cap.Range, nil, lang)
}

// findNodeForCaptureType finds a tree-sitter Node for a capture tag with the expected type.
func findNodeForCaptureType(grouped CaptureMatch, tag string, root *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	cap, ok := grouped[tag]
	if !ok {
		return nil
	}
	return utils.FindNodeAtRange(root, cap.Range, nil, lang)
}
