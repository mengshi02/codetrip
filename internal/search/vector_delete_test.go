package search

import (
	"fmt"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/util"
)

// Helper functions for test node/vector creation

func createTestNodes(t *testing.T, gs *graph.GraphStore, count int) []string {
	t.Helper()
	var ids []string
	for i := 0; i < count; i++ {
		node := graph.NewNode("testrepo", graph.LabelFunction, fmt.Sprintf("testFunc%d", i)).WithFile("test.go")
		if err := gs.AddNode(node); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, node.ID)
	}
	return ids
}

func createTestNodesWithOffset(t *testing.T, gs *graph.GraphStore, count int, offset int) []string {
	t.Helper()
	var ids []string
	for i := offset; i < offset+count; i++ {
		node := graph.NewNode("testrepo", graph.LabelFunction, fmt.Sprintf("testFunc%d", i)).WithFile("test.go")
		if err := gs.AddNode(node); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, node.ID)
	}
	return ids
}

// storeTestVectors writes dual-modal embedding vectors (desc + code) and
// both modality index keys to the KV store, which is required before
// BuildDualModalHNSWIndex can work.
func storeTestVectors(vs *VectorSearch, gs *graph.GraphStore, nodeIDs []string, count int, dim int) {
	for i := 0; i < count; i++ {
		vec := make([]float32, dim)
		for j := 0; j < dim; j++ {
			vec[j] = float32(i*dim+j) / float32(dim*count)
		}
		vecData := util.EncodeFloat32Vec(vec)
		// Write both desc and code modality vectors
		vs.store.Set([]byte(fmt.Sprintf("embdesc:testrepo:%s", nodeIDs[i])), vecData)
		vs.store.Set([]byte(fmt.Sprintf("embcode:testrepo:%s", nodeIDs[i])), vecData)
	}
	idxData := util.EncodeStringList(nodeIDs)
	// Write both desc and code modality index keys
	vs.store.Set([]byte("embdescidx:testrepo"), idxData)
	vs.store.Set([]byte("embcodeidx:testrepo"), idxData)
	gs.Flush()
	vs.store.Flush()
}

// createTestVectors generates test float32 vectors with an offset for variety.
func createTestVectors(count int, dim int, offset int) [][]float32 {
	vecs := make([][]float32, count)
	for i := 0; i < count; i++ {
		vec := make([]float32, dim)
		for j := 0; j < dim; j++ {
			vec[j] = float32(offset+i*dim+j) / float32(dim*count+offset)
		}
		vecs[i] = vec
	}
	return vecs
}