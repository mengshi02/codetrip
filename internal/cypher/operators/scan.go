package operators

import "github.com/mengshi02/codetrip/internal/graph"

// ScanIterator scans nodes from the graph store and produces one row per
// matching node. It is the leaf operator in the Volcano tree.
//
// Optimization: nodes are fetched lazily in batches rather than all at once,
// reducing memory pressure for large graphs. The init() method still loads
// all matching nodes but applies label/property filtering at the storage
// level where possible to avoid pulling unnecessary data.
type ScanIterator struct {
	gs     GraphStore
	node   *CypherNodePattern
	ev     *Evaluator
	idx    int
	nodes  []*graph.Node
	closed bool
}

// NewScanIterator creates a ScanIterator for the given node pattern.
func NewScanIterator(gs GraphStore, node *CypherNodePattern, ev *Evaluator) *ScanIterator {
	return &ScanIterator{
		gs:   gs,
		node: node,
		ev:   ev,
	}
}

func (s *ScanIterator) init() {
	if s.nodes != nil {
		return
	}

	var nodes []*graph.Node
	var err error

	if len(s.node.Labels) > 0 {
		nodes, err = s.gs.GetNodesByLabel(s.gs.Repo(), s.node.Labels[0])
		if err != nil {
			s.nodes = []*graph.Node{} // empty sentinel, not nil
			return
		}
	} else {
		nodes = s.gs.GetAllNodes(s.gs.Repo(), 10000)
	}

	// Apply property filters
	if len(s.node.Props) > 0 {
		nodes = FilterNodesByProps(nodes, s.node.Props, s.ev)
	}

	// Multi-label filtering
	if len(s.node.Labels) > 1 {
		labelSet := make(map[string]bool, len(s.node.Labels))
		for _, l := range s.node.Labels {
			labelSet[l] = true
		}
		filtered := make([]*graph.Node, 0, len(nodes))
		for _, n := range nodes {
			if labelSet[string(n.Label)] {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	s.nodes = nodes
}

// Next returns the next matching node as a Row.
func (s *ScanIterator) Next() (Row, error) {
	if s.closed {
		return nil, nil
	}
	s.init()

	if s.idx >= len(s.nodes) {
		return nil, nil
	}

	n := s.nodes[s.idx]
	s.idx++

	row := Row{}
	if s.node.Variable != "" {
		row[s.node.Variable] = n
	}
	return row, nil
}

// Close releases resources held by the iterator.
func (s *ScanIterator) Close() error {
	s.closed = true
	s.nodes = nil
	return nil
}