// Package typescript — TypeScript method receiver binding synthesis.
// TypeScript instance methods have a synthesized `this` type binding
// pointing to the enclosing class/interface type. Arrow functions
// inside class methods inherit `this` through the lexical scope chain.
// Static methods do NOT get a `this` binding.
//
// Ported from TS languages/typescript/receiver-binding.ts.
package typescript

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// SynthesizeTsReceiverBinding creates a synthetic `this` type binding
// for an instance method's function node. It:
//   1. Classifies the function's role (method/constructor/etc.)
//   2. Skips static methods (no `this` for static)
//   3. Finds the enclosing type declaration
//   4. Creates a @type-binding.this capture pointing to the type name
//
// Returns nil for:
//   - static methods / static fields
//   - free functions / module-level code
//   - non-method function nodes
//
// Mirrors TS synthesizeTsReceiverBinding(fnNode): CaptureMatch | null.
// TODO: full implementation — currently returns nil.
func SynthesizeTsReceiverBinding(fnNode interface{}) *shared.CaptureMatch {
	// TODO: classifyFunctionRole, isStaticMember, findEnclosingType,
	// getTypeDeclName, synthesize capture.
	return nil
}