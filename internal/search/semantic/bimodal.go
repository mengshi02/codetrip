package semantic

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
)

// embedContext holds pre-loaded adjacency data to avoid N+1 queries
// when building description texts for all nodes.
type embedContext struct {
	nameIndex map[string]string   // nodeID → node.Name (avoids GetNode per edge)
	callOut   map[string][]string // nodeID → []targetNodeID (CALLS out-edges)
	callIn    map[string][]string // nodeID → []sourceNodeID (CALLS in-edges)
}

// buildEmbedContext pre-loads all node names and CALLS edges into memory.
// This replaces N+1 GetNode calls (1 per edge endpoint) with just 2 scans
// (one for nodes, one for edges), reducing IO from ~1.2M to 2 operations
// for a 100K-node repo.
func buildEmbedContext(gs *graph.GraphStore) (*embedContext, error) {
	ec := &embedContext{
		nameIndex: make(map[string]string),
		callOut:   make(map[string][]string),
		callIn:    make(map[string][]string),
	}

	// 1. Single pass: nodeID → Name
	iter := gs.IterNodes(gs.Repo())
	for iter.Next() {
		n := iter.Node()
		ec.nameIndex[n.ID] = n.Name
	}
	iter.Close()

	// 2. Single ScanAllOutEdgesByRelType("CALLS"): build out-edges
	outEdges, err := gs.ScanAllOutEdgesByRelType("CALLS")
	if err != nil {
		return nil, fmt.Errorf("scan CALLS edges: %w", err)
	}
	for _, e := range outEdges {
		ec.callOut[e.Source] = append(ec.callOut[e.Source], e.Target)
	}

	// 3. Reverse out-edges to get in-edges (zero IO)
	for src, targets := range ec.callOut {
		for _, tgt := range targets {
			ec.callIn[tgt] = append(ec.callIn[tgt], src)
		}
	}

	return ec, nil
}

// buildDescriptionText generates the description modality text for a node.
// It uses the pre-loaded embedContext to avoid N+1 queries.
//
// Output format:
//
//	func processOrder(order Order) error
//	  Calls: validateOrder, calculateTotal, saveToDB
//	  Called by: handleHTTPRequest
//	  Type: Method, Visibility: public, Receiver: *OrderService
func buildDescriptionText(node *graph.Node, ec *embedContext) string {
	var b strings.Builder

	// 1. Symbol signature
	signature := buildNodeSignature(node)
	b.WriteString(signature)
	b.WriteByte('\n')

	// 2. Calls (outgoing CALLS edges)
	if targets, ok := ec.callOut[node.ID]; ok && len(targets) > 0 {
		var names []string
		for _, tgtID := range targets {
			if name, ok := ec.nameIndex[tgtID]; ok && name != "" {
				names = append(names, name)
			} else {
				names = append(names, tgtID)
			}
		}
		b.WriteString("  Calls: ")
		b.WriteString(strings.Join(names, ", "))
		b.WriteByte('\n')
	}

	// 3. Called by (incoming CALLS edges)
	if sources, ok := ec.callIn[node.ID]; ok && len(sources) > 0 {
		var names []string
		for _, srcID := range sources {
			if name, ok := ec.nameIndex[srcID]; ok && name != "" {
				names = append(names, name)
			} else {
				names = append(names, srcID)
			}
		}
		b.WriteString("  Called by: ")
		b.WriteString(strings.Join(names, ", "))
		b.WriteByte('\n')
	}

	// 4. Metadata
	b.WriteString("  Type: ")
	b.WriteString(string(node.Label))

	if v, ok := node.Props.GetProp("visibility"); ok {
		b.WriteString(", Visibility: ")
		b.WriteString(fmt.Sprintf("%v", v))
	}
	if v, ok := node.Props.GetProp("receiver"); ok {
		b.WriteString(", Receiver: ")
		b.WriteString(fmt.Sprintf("%v", v))
	}

	return b.String()
}

// buildNodeSignature builds a readable signature from node properties.
func buildNodeSignature(node *graph.Node) string {
	var b strings.Builder

	// Label prefix
	switch node.Label {
	case graph.LabelMethod:
		if v, ok := node.Props.GetProp("receiver"); ok {
			b.WriteString(fmt.Sprintf("%v.", v))
		}
		b.WriteString(node.Name)
	case graph.LabelFunction, graph.LabelConstructor:
		b.WriteString(node.Name)
	case graph.LabelClass, graph.LabelInterface, graph.LabelStruct:
		b.WriteString(string(node.Label))
		b.WriteByte(' ')
		b.WriteString(node.Name)
	default:
		b.WriteString(string(node.Label))
		b.WriteByte(' ')
		b.WriteString(node.Name)
	}

	// Parameters (if available)
	if v, ok := node.Props.GetProp("signature"); ok {
		b.WriteString(fmt.Sprintf("%v", v))
	} else if v, ok := node.Props.GetProp("params"); ok {
		b.WriteString(fmt.Sprintf("(%v)", v))
	}

	// Return type (if available)
	if v, ok := node.Props.GetProp("returnType"); ok {
		b.WriteString(fmt.Sprintf(" %v", v))
	}

	return b.String()
}

// modalityRef tracks which modality each text in a combined batch belongs to.
type modalityRef struct {
	nodeIdx  int    // index in the batch
	modality string // "desc" or "code"
	chunkIdx int    // chunk index within the node (for code modality)
}
