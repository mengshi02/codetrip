// Package python — Synthesize @type-binding.self / @type-binding.cls captures for methods.
// Tree-sitter can't easily express "the first parameter of a function defined
// directly inside a class body" via a single static query. Doing this in code
// keeps the embedded scope query declarative and lets us encode the
// @classmethod / @staticmethod decorator awareness that Python's runtime depends on.
//
// Ported from TS languages/python/receiver-binding.ts (synthesizeReceiverTypeBinding).
package python

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// SynthesizeReceiverTypeBinding walks up to the enclosing class_definition
// of a function_definition node and emits a @type-binding.self (or
// @type-binding.cls for @classmethod) capture linking the method's
// implicit receiver to the class type.
//
// Returns nil for free functions (no enclosing class), lambda-bodied,
// or nested inside another function.
//
// Mirrors TS synthesizeReceiverTypeBinding(fnNode).
// TODO: full implementation — requires tree-sitter node traversal.
func SynthesizeReceiverTypeBinding(fnNode interface{}) shared.CaptureMatch {
	// TODO: implement when tree-sitter Python integration is ready.
	// Steps (from TS):
	//   1. Walk up from fnNode to find enclosing class_definition
	//      (ignoring decorated_definition wrapper)
	//   2. If no enclosing class, return nil (free function)
	//   3. Check decorators for @classmethod → emit @type-binding.cls
	//      (classmethod's first param is cls → bound to class type)
	//   4. Check decorators for @staticmethod → return nil (no receiver)
	//   5. Default: emit @type-binding.self with class name as type
	//      (instance method's first param is self → bound to class type)
	return nil
}