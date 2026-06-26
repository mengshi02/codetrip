// Package python — Synthesize Depends() reference captures for FastAPI.
// Inspects function_definition nodes for Depends(callable) parameter defaults
// and emits @reference.call.free captures for each dependency.
//
// Tree-sitter can't express "the first argument of a call named Depends
// inside a parameter default" in a single static query, so we synthesize
// reference captures in code, mirroring the receiver-binding pattern.
//
// Ported from TS languages/python/depends-references.ts (synthesizeDependsReferences).
package python

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// SynthesizeDependsReferences inspects a function_definition node's parameters
// for Depends(callable) defaults. Returns one @reference.call.free CaptureMatch
// per dependency.
//
// FastAPI's Depends(get_db) passes get_db as a callable that the DI framework
// calls on every request. The route handler is functionally a caller of the
// dependency — impact analysis needs that edge.
//
// Mirrors TS synthesizeDependsReferences(fnNode).
// TODO: full implementation — requires tree-sitter node traversal.
func SynthesizeDependsReferences(fnNode interface{}) []shared.CaptureMatch {
	// TODO: implement when tree-sitter Python integration is ready.
	// Steps:
	//   1. Get parameters child from fnNode
	//   2. For each typed_default_parameter / default_parameter:
	//      a. Get the default value
	//      b. If it's a call node with function name "Depends":
	//         - Emit @reference.call.free for identifier arguments
	//         - Emit @reference.call.member for attribute arguments
	return nil
}