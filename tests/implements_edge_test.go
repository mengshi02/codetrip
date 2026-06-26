package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mengshi02/codetrip"
	"github.com/mengshi02/codetrip/internal/graph"
)

// createImplementsTestRepo creates a Go repo with interface+struct implementation
// to test IMPLEMENTS edge creation end-to-end.
func createImplementsTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	files := map[string]string{
		"iface.go": `package main

// Reader is an interface for reading data
type Reader interface {
	Read() string
	Close() error
}
`,
		"impl.go": `package main

import "fmt"

// FileReader implements Reader using a file
type FileReader struct {
	path string
}

func (f *FileReader) Read() string {
	return fmt.Sprintf("reading from %s", f.path)
}

func (f *FileReader) Close() error {
	return nil
}
`,
		"service.go": `package main

// Writer is an interface for writing data
type Writer interface {
	Write(data string) error
	Flush() error
}

// ConsoleWriter implements Writer using console output
type ConsoleWriter struct{}

func (c *ConsoleWriter) Write(data string) error {
	return nil
}

func (c *ConsoleWriter) Flush() error {
	return nil
}
`,
		"partial.go": `package main

// PartialImpl only partially implements Reader (missing Close)
type PartialImpl struct{}

func (p *PartialImpl) Read() string {
	return "partial"
}
`,
	}

	for relPath, content := range files {
		fullPath := filepath.Join(dir, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	return dir
}

// Note: we access the graph store via trip.GraphStore() after indexing

// TestImplementsEdge_E2E verifies that IMPLEMENTS edges are created
// when a struct implements all methods of an interface.
func TestImplementsEdge_E2E(t *testing.T) {
	trip := openTrip(t)
	repoPath := createImplementsTestRepo(t)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := trip.IndexRepo(ctx, repoPath, codetrip.WithRepoName("impltest"))
	if err != nil {
		t.Fatalf("index repo: %v", err)
	}

	t.Logf("indexed: repo=%s files=%d nodes=%d edges=%d duration=%.2fs",
		result.Repo, result.Files, result.Nodes, result.Edges, result.Duration)

	// Access the graph store via Trip's public method
	gs := trip.GraphStore("impltest")
	if gs == nil {
		t.Fatal("no graph store found for repo impltest")
	}

	// Find all IMPLEMENTS edges
	implementsEdges := 0
	hasMethodEdges := 0
	inheritsEdges := 0

	iter := gs.IterNodes("impltest")
	defer iter.Close()
	for iter.Next() {
		node := iter.Node()
		// Check outgoing edges for each Interface node
		if node.Label == graph.LabelInterface {
			inEdges, err := gs.GetAllInEdges(node.ID)
			if err != nil {
				continue
			}
			for _, e := range inEdges {
				t.Logf("Interface %s incoming edge: type=%s source=%s", node.Name, e.Type, e.Source)
				if e.Type == graph.RelImplements {
					implementsEdges++
				}
				if e.Type == graph.RelInherits {
					// Check if it's an implements-kind INHERITS
					if kind, ok := e.Props.GetProp("kind"); ok && kind == "implements" {
						t.Logf("  INHERITS(kind=implements) from %s", e.Source)
					}
				}
			}
		}
		// Also check HAS_METHOD edges for each Struct node
		if node.Label == graph.LabelStruct {
			outEdges, err := gs.GetAllOutEdges(node.ID)
			if err != nil {
				continue
			}
			for _, e := range outEdges {
				if e.Type == graph.RelHasMethod {
					hasMethodEdges++
					t.Logf("Struct %s HAS_METHOD → %s", node.Name, e.Target)
				}
				if e.Type == graph.RelImplements {
					implementsEdges++
					t.Logf("Struct %s IMPLEMENTS → %s", node.Name, e.Target)
				}
				if e.Type == graph.RelInherits {
					inheritsEdges++
				}
			}
		}
	}

	t.Logf("=== Summary ===")
	t.Logf("HAS_METHOD edges: %d", hasMethodEdges)
	t.Logf("IMPLEMENTS edges: %d", implementsEdges)
	t.Logf("INHERITS edges: %d", inheritsEdges)

	if hasMethodEdges == 0 {
		t.Error("Expected HAS_METHOD edges on struct nodes, found 0 — parse phase may not be creating them")
	}

	if implementsEdges == 0 {
		t.Error("Expected IMPLEMENTS edges, found 0 — scope resolution phase may not be creating them")
	}

	// Specifically check: FileReader should implement Reader
	iter2 := gs.IterNodes("impltest")
	defer iter2.Close()
	foundFileReaderImplementsReader := false
	for iter2.Next() {
		node := iter2.Node()
		if node.Label == graph.LabelStruct && node.Name == "FileReader" {
			outEdges, _ := gs.GetAllOutEdges(node.ID)
			for _, e := range outEdges {
				if e.Type == graph.RelImplements {
					target, err := gs.GetNode(e.Target)
					if err == nil && target.Name == "Reader" {
						foundFileReaderImplementsReader = true
						conf, _ := e.Props.GetProp("confidence")
						inf, _ := e.Props.GetProp("inferred")
						t.Logf("FileReader IMPLEMENTS Reader (confidence=%v, inferred=%v)", conf, inf)
					}
				}
			}
		}
	}

	if !foundFileReaderImplementsReader {
		t.Error("Expected FileReader IMPLEMENTS Reader edge, not found")
	}

	// Check: PartialImpl should NOT implement Reader (missing Close method)
	iter3 := gs.IterNodes("impltest")
	defer iter3.Close()
	foundPartialImplementsReader := false
	for iter3.Next() {
		node := iter3.Node()
		if node.Label == graph.LabelStruct && node.Name == "PartialImpl" {
			outEdges, _ := gs.GetAllOutEdges(node.ID)
			for _, e := range outEdges {
				if e.Type == graph.RelImplements {
					target, err := gs.GetNode(e.Target)
					if err == nil && target.Name == "Reader" {
						foundPartialImplementsReader = true
					}
				}
			}
		}
	}

	if foundPartialImplementsReader {
		t.Error("PartialImpl should NOT implement Reader (missing Close method)")
	}
}