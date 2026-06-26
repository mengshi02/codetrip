package callextractors

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// CreateCallExtractor creates a CallExtractor from a declarative config.
//
// The factory converts a CallExtractionConfig into a runtime CallExtractor,
// whose Extract method:
//  1. Attempts config.ExtractLanguageCallSite(callNode, source, lang) for non-standard shapes
//  2. Falls back to the generic path using shared utilities from the callanalysis package
//     (InferCallForm, ExtractReceiverName, etc.)
func CreateCallExtractor(config core.CallExtractionConfig) core.CallExtractor {
	return &callExtractorImpl{
		language: config.Language,
		config:   config,
	}
}

// callExtractorImpl is the concrete implementation of CallExtractor
// produced by CreateCallExtractor.
type callExtractorImpl struct {
	language core.SupportedLanguage
	config   core.CallExtractionConfig
}

func (e *callExtractorImpl) Language() core.SupportedLanguage {
	return e.language
}

func (e *callExtractorImpl) Extract(callNode *gotreesitter.Node, callNameNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *core.ExtractedCallSite {
	// Path 1: Language-specific call site extraction.
	// Non-standard call shapes (e.g. Java :: method references) are entirely
	// handled by the config hook. When it returns a result, the generic path
	// is skipped — no argCount, no mixed chain.
	//
	// Note: ExtractLanguageCallSite is called on every Extract() invocation —
	// both extract(callNode, nil) (parse-worker path 1) and
	// extract(callNode, callNameNode) (parse-worker path 2).
	// Language hooks must be idempotent and cheap (e.g. single node type check).
	if e.config.ExtractLanguageCallSite != nil {
		seed := e.config.ExtractLanguageCallSite(callNode, source, lang)
		if seed != nil {
			result := *seed // value copy
			if e.config.TypeAsReceiverHeuristic {
				result.TypeAsReceiverHeuristic = boolPtr(true)
			}
			return &result
		}
	}

	// Path 2: Generic extraction via @call.name
	if callNameNode == nil {
		return nil
	}

	calledName := callNameNode.Text(source)
	callForm := utils.InferCallForm(callNode, callNameNode, lang)

	var receiverName *string
	var receiverMixedChain []core.MixedChainStep

	if callForm != nil && string(*callForm) == string(core.CallFormMember) {
		rn := utils.ExtractReceiverName(callNameNode, source, lang)
		receiverName = rn

		// When the receiver is a complex expression (call chain, field chain,
		// or mixed chain), ExtractReceiverName returns nil. Walk the receiver
		// node to build a unified mixed chain for deferred resolution.
		if rn == nil {
			receiverNode := utils.ExtractReceiverNode(callNameNode, lang)
			if receiverNode != nil {
				extracted := utils.ExtractMixedChain(receiverNode, source, lang)
				if extracted != nil && len(extracted.Chain) > 0 {
					receiverMixedChain = make([]core.MixedChainStep, len(extracted.Chain))
					for i, step := range extracted.Chain {
						receiverMixedChain[i] = core.MixedChainStep{Kind: step.Kind, Name: step.Name}
					}
					rn = extracted.BaseReceiverName
					receiverName = rn
				}
			}
		}
	}

	argCount := utils.CountCallArguments(callNode, lang)

	result := &core.ExtractedCallSite{
		CalledName:         calledName,
		ArgCount:           argCount,
		ReceiverMixedChain: receiverMixedChain,
	}
	if callForm != nil {
		cf := core.CallForm(string(*callForm))
		result.CallForm = &cf
	}
	if receiverName != nil {
		result.ReceiverName = receiverName
	}
	if e.config.TypeAsReceiverHeuristic {
		result.TypeAsReceiverHeuristic = boolPtr(true)
	}
	return result
}

func (e *callExtractorImpl) IsMethodDeclaration(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	return false
}

// boolPtr returns a pointer to a bool value.
func boolPtr(v bool) *bool { return &v }