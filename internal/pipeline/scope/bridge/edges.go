package bridge

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// EdgeEmitter graph edge emitter
// Converts scope resolution results to graph edges (CALLS, IMPORTS, EMBRACES, etc.)
type EdgeEmitter struct {
	graph   *graph.GraphStore
	repo    string
	batch   []*graph.Edge // batch buffer
	batchSz int           // batch size
}

// NewEdgeEmitter creates an edge emitter
func NewEdgeEmitter(gs *graph.GraphStore, repo string, batchSize int) *EdgeEmitter {
	if batchSize <= 0 {
		batchSize = 64
	}
	return &EdgeEmitter{
		graph:   gs,
		repo:    repo,
		batch:   make([]*graph.Edge, 0, batchSize),
		batchSz: batchSize,
	}
}

// EmitCall emits CALLS edge
func (e *EdgeEmitter) EmitCall(callerID, targetID string, confidence float64) error {
	edge := &graph.Edge{
		Type:   graph.RelCalls,
		Source: callerID,
		Target: targetID,
	}
	edge.WithProp("confidence", confidence)
	return e.emit(edge)
}

// EmitImport emits IMPORTS edge
func (e *EdgeEmitter) EmitImport(sourceID, targetID string, importPath string, confidence float64) error {
	edge := &graph.Edge{
		Type:   graph.RelImports,
		Source: sourceID,
		Target: targetID,
	}
	edge.WithProp("importPath", importPath)
	edge.WithProp("confidence", confidence)
	return e.emit(edge)
}

// EmitImplements emits IMPLEMENTS edge
func (e *EdgeEmitter) EmitImplements(classID, interfaceID string, confidence float64) error {
	edge := &graph.Edge{
		Type:   graph.RelImplements,
		Source: classID,
		Target: interfaceID,
	}
	edge.WithProp("confidence", confidence)
	return e.emit(edge)
}

// EmitContains emits CONTAINS edge
func (e *EdgeEmitter) EmitContains(parentID, childID string) error {
	edge := &graph.Edge{
		Type:   graph.RelContains,
		Source: parentID,
		Target: childID,
	}
	return e.emit(edge)
}

// EmitReferences emits USES edge (symbol reference)
func (e *EdgeEmitter) EmitReferences(sourceID, targetID string, confidence float64) error {
	edge := &graph.Edge{
		Type:   graph.RelUses,
		Source: sourceID,
		Target: targetID,
	}
	edge.WithProp("confidence", confidence)
	return e.emit(edge)
}

// EmitCustom emits custom edge
func (e *EdgeEmitter) EmitCustom(edgeType graph.RelType, sourceID, targetID string, props map[string]any) error {
	edge := &graph.Edge{
		Type:   edgeType,
		Source: sourceID,
		Target: targetID,
	}
	for k, v := range props {
		edge.WithProp(k, v)
	}
	return e.emit(edge)
}

// emit adds edge to batch buffer
func (e *EdgeEmitter) emit(edge *graph.Edge) error {
	e.batch = append(e.batch, edge)
	if len(e.batch) >= e.batchSz {
		return e.Flush()
	}
	return nil
}

// Flush flushes batch buffer to graph store
func (e *EdgeEmitter) Flush() error {
	if len(e.batch) == 0 {
		return nil
	}
	for _, edge := range e.batch {
		if err := e.graph.BufferEdge(edge); err != nil {
			return fmt.Errorf("add edge %s→%s (%s): %w", edge.Source, edge.Target, edge.Type, err)
		}
	}
	e.batch = e.batch[:0]
	return nil
}

// EmitFromCallSite emits CALLS edge from CallSite
// Unified call site → target edge emission logic
func (e *EdgeEmitter) EmitFromCallSite(cs *pipeline.CallSite, targetID string, confidence float64) error {
	if cs.CallerID == "" || targetID == "" {
		return nil
	}
	return e.EmitCall(cs.CallerID, targetID, confidence)
}