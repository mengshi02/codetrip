package incremental

import (
	"context"
	"testing"

	"os"
	"path/filepath"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
)

func openTestIndexer(t *testing.T) (*IncrementalIndexer, *graph.GraphStore, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "idxtest-*")
	if err != nil {
		t.Fatal(err)
	}
	cfg := store.DefaultConfig(filepath.Join(dir, "db"))
	store, err := store.Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	gs := graph.NewGraphStore(store, "testrepo")
	idx := NewIncrementalIndexer(gs)
	cleanup := func() {
		store.Close()
		os.RemoveAll(dir)
	}
	return idx, gs, cleanup
}

func TestNewIncrementalIndexer(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()
	if idx == nil {
		t.Fatal("indexer is nil")
	}
	if idx.workers != 4 {
		t.Errorf("workers = %d, want 4", idx.workers)
	}
}

func TestWithParseFunc(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()
	idx.WithParseFunc(func(ctx context.Context, path string, content []byte, hash string) error {
		return nil
	})
	if idx.parseFn == nil {
		t.Error("parseFn should be set")
	}
}

func TestWithEmbedFunc(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()
	idx.WithEmbedFunc(func(ctx context.Context, nodeIDs []string) error {
		return nil
	})
	if idx.embedFn == nil {
		t.Error("embedFn should be set")
	}
}

func TestWithWorkers(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()
	idx.WithWorkers(8)
	if idx.workers != 8 {
		t.Errorf("workers = %d, want 8", idx.workers)
	}
	idx.WithWorkers(0) // should not change
	if idx.workers != 8 {
		t.Errorf("workers should remain 8, got %d", idx.workers)
	}
}

func TestDetectChanges_AllTypes(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()

	indexed := map[string]string{
		"a.go": "hash_a_v1",
		"b.go": "hash_b_v1",
		"c.go": "hash_c_v1",
	}
	current := map[string]string{
		"a.go": "hash_a_v1", // unchanged
		"b.go": "hash_b_v2", // modified
		"d.go": "hash_d_v1", // added
		// c.go deleted
	}

	changes := idx.detectChanges(indexed, current)

	counts := map[ChangeType]int{}
	for _, c := range changes {
		counts[c.Type]++
	}
	if counts[ChangeUnchanged] != 1 {
		t.Errorf("unchanged = %d, want 1", counts[ChangeUnchanged])
	}
	if counts[ChangeModified] != 1 {
		t.Errorf("modified = %d, want 1", counts[ChangeModified])
	}
	if counts[ChangeAdded] != 1 {
		t.Errorf("added = %d, want 1", counts[ChangeAdded])
	}
	if counts[ChangeDeleted] != 1 {
		t.Errorf("deleted = %d, want 1", counts[ChangeDeleted])
	}
}

func TestDetectChanges_Empty(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()

	changes := idx.detectChanges(nil, nil)
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}
}

func TestChangeType_String(t *testing.T) {
	tests := []struct {
		ct   ChangeType
		want string
	}{
		{ChangeAdded, "added"},
		{ChangeModified, "modified"},
		{ChangeDeleted, "deleted"},
		{ChangeUnchanged, "unchanged"},
		{ChangeType(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("ChangeType(%d).String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestIsSourceFile(t *testing.T) {
	supported := []string{".go", ".ts", ".tsx", ".js", ".py", ".java", ".rs", ".c", ".cpp", ".md"}
	for _, ext := range supported {
		if !isSourceFile(ext) {
			t.Errorf("isSourceFile(%q) = false, want true", ext)
		}
	}
	if isSourceFile(".exe") || isSourceFile(".bin") || isSourceFile("") {
		t.Error("expected false for non-source extensions")
	}
}

func TestIndex_Empty(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()

	result, err := idx.Index(context.Background(), "/nonexistent", map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 0 || result.Modified != 0 || result.Deleted != 0 {
		t.Errorf("expected all zeros, got %+v", result)
	}
}

func TestScanAndIndex_NonexistentDir(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()

	// ScanAndIndex on nonexistent dir returns empty result (filepath.Walk doesn't error)
	result, err := idx.ScanAndIndex(context.Background(), "/nonexistent/path")
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 0 || result.Modified != 0 || result.Deleted != 0 {
		t.Errorf("expected all zeros, got %+v", result)
	}
}

func TestGetIndexedFiles_Empty(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()

	files, err := idx.getIndexedFiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 indexed files, got %d", len(files))
	}
}