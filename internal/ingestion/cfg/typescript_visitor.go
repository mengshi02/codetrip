package cfg

import (
	"github.com/mengshi02/codetrip/internal/graph"
)

// TypeScriptCFGVisitor builds a CFG by walking a TypeScript AST node tree.
// It uses the tree-sitter query patterns from the ingestion pipeline to
// identify control flow constructs (if/else, for, while, try/catch, etc.).
//
// Mirrors TS cfg/visitors/typescript.ts.
//
// Current status: skeleton — full implementation deferred.
type TypeScriptCFGVisitor struct {
	// ctx is the mutable control flow context during construction.
	ctx *ControlFlowContext
}

// NewTypeScriptCFGVisitor creates a new TypeScript CFG visitor.
func NewTypeScriptCFGVisitor() *TypeScriptCFGVisitor {
	return &TypeScriptCFGVisitor{
		ctx: NewControlFlowContext(),
	}
}

// BuildCFG constructs a FunctionCFG from a TypeScript function node.
//
// Current status: skeleton — full implementation deferred.
func (v *TypeScriptCFGVisitor) BuildCFG(funcNode *graph.Node, sourceCode string) (*FunctionCFG, error) {
	_ = funcNode
	_ = sourceCode
	// TODO: parse sourceCode with tree-sitter TypeScript grammar
	// Walk AST nodes, building basic blocks and edges
	// Use ControlFlowContext to track state
	return nil, nil
}