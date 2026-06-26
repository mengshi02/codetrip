package cpp

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ClassifyCppParameterType classifies a C/C++ parameter's passing semantics
// based on the type text, declarator text, and full parameter text.
//
// Classification rules:
//   - "ptr"  : pointer type (type* or type* const), array declarator (type name[])
//   - "ref"  : lvalue reference (type&), forwarding reference (type&&)
//   - "out"  : not natively supported in C/C++; always returns value unless pointer
//   - "value": pass-by-value (default)
func ClassifyCppParameterType(typeText, declText, paramText string) core.ParameterTypeClass {
	typeText = strings.TrimSpace(typeText)
	declText = strings.TrimSpace(declText)
	paramText = strings.TrimSpace(paramText)

	// Array declarator: int name[] — passed as pointer
	if strings.Contains(declText, "[") && strings.Contains(declText, "]") {
		return core.ParamTypeClassPtr
	}

	// Reference type: type& or type&&
	if strings.Contains(declText, "&&") {
		return core.ParamTypeClassRef
	}
	if strings.Contains(declText, "&") && !strings.Contains(declText, "&&") {
		return core.ParamTypeClassRef
	}

	// Pointer type: type* or type* const
	if strings.Contains(typeText, "*") || strings.Contains(declText, "*") {
		return core.ParamTypeClassPtr
	}

	// Full parameter text fallback: check for pointer/reference patterns
	if strings.Contains(paramText, "*") {
		return core.ParamTypeClassPtr
	}
	if strings.Contains(paramText, "&") {
		return core.ParamTypeClassRef
	}

	return core.ParamTypeClassValue
}