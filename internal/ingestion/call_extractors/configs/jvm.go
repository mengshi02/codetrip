package configs

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// JavaCallConfig is the call extraction configuration for Java.
// Handles method references (::).
var JavaCallConfig = core.CallExtractionConfig{
	Language:                core.LangJava,
	ExtractLanguageCallSite: parseJavaMethodReference,
	TypeAsReceiverHeuristic: true,
}

// parseJavaMethodReference parses Java method_reference nodes.
// Handles: expr::method, Type::new, this::m, super::m
func parseJavaMethodReference(callNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *core.ExtractedCallSite {
	if callNode.Type(lang) != "method_reference" {
		return nil
	}

	recv := callNode.NamedChild(0)
	if recv == nil {
		return nil
	}

	// Type::new → constructor call
	for i := 0; i < callNode.ChildCount(); i++ {
		c := callNode.Child(i)
		if c != nil && c.Type(lang) == "new" {
			if recv.Type(lang) != "identifier" {
				return nil
			}
			text := recv.Text(source)
			ctor := core.CallFormConstructor
			return &core.ExtractedCallSite{
				CalledName: text,
				CallForm:   &ctor,
			}
		}
	}

	// expr::method → member call with receiver
	rhs := callNode.Child(callNode.ChildCount() - 1)
	if rhs == nil || rhs.Type(lang) != "identifier" {
		return nil
	}
	methodName := rhs.Text(source)

	switch recv.Type(lang) {
	case "identifier":
		recvText := recv.Text(source)
		member := core.CallFormMember
		return &core.ExtractedCallSite{
			CalledName:   methodName,
			CallForm:     &member,
			ReceiverName: &recvText,
		}
	case "this":
		thisText := "this"
		member := core.CallFormMember
		return &core.ExtractedCallSite{
			CalledName:   methodName,
			CallForm:     &member,
			ReceiverName: &thisText,
		}
	case "super":
		superText := "super"
		member := core.CallFormMember
		return &core.ExtractedCallSite{
			CalledName:   methodName,
			CallForm:     &member,
			ReceiverName: &superText,
		}
	default:
		return nil
	}
}