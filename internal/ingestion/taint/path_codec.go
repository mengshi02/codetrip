package taint

// PathCodec encodes and decodes taint propagation paths for storage
// and comparison. A taint path is a sequence of hops from source to sink,
// where each hop represents a variable assignment or function call that
// propagates tainted data.
//
// Mirrors TS taint/path-codec.ts.
//
// Current status: skeleton — full implementation deferred.

// EncodePath serializes a taint path to a compact string representation.
func EncodePath(hops []HopInfo) string {
	_ = hops
	// TODO: encode hops as "line:var→line:var→..." format
	return ""
}

// DecodePath deserializes a taint path from its compact representation.
func DecodePath(encoded string) []HopInfo {
	_ = encoded
	// TODO: parse encoded string back to HopInfo slice
	return nil
}

// HopInfo represents a single hop in a taint propagation path.
// This mirrors collection.HopInfo but is defined here to avoid
// circular imports when the collection package is not available.
type HopInfo struct {
	NodeID  string // Node identifier in the CFG
	Line    int    // Source line number
	VarName string // Variable name
	BlockID string // Basic block identifier
	Op      string // assign, call, return, param
}