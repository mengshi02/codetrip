package incremental

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mengshi02/codetrip/internal/graph"
)

func TestScanFiles_RealDir(t *testing.T) {
	idx, _, cleanup := openTestIndexer(t)
	defer cleanup()

	// 创建临时目录结构
	dir, _ := os.MkdirTemp("", "scantest-*")
	defer os.RemoveAll(dir)
	os.MkdirAll(filepath.Join(dir, "pkg"), 0755)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644)
	os.WriteFile(filepath.Join(dir, "pkg", "handler.go"), []byte("package pkg"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.bin"), []byte("binary"), 0644) // 非源码文件
	os.MkdirAll(filepath.Join(dir, ".git"), 0755)                          // 应跳过
	os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("cfg"), 0644)

	files, err := idx.scanFiles(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 source files, got %d: %v", len(files), files)
	}
}

func TestIndex_WithAddAndDelete(t *testing.T) {
	idx, gs, cleanup := openTestIndexer(t)
	defer cleanup()

	// 创建临时目录
	dir, _ := os.MkdirTemp("", "indextest-*")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main"), 0644)

	// 首次索引：添加文件
	parseCalled := false
	idx.WithParseFunc(func(ctx context.Context, filePath string, content []byte, contentHash string) error {
		parseCalled = true
		// 模拟创建 File 节点
		n := graph.NewNode("testrepo", graph.LabelFile, filePath)
		n.FilePath = filePath
		n.Props = graph.NodePropsFromMap(map[string]any{"contentHash": contentHash})
		gs.AddNode(n)
		gs.Flush()
		return nil
	})

	result, err := idx.ScanAndIndex(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Added != 1 {
		t.Errorf("added = %d, want 1", result.Added)
	}
	if !parseCalled {
		t.Error("parseFn should be called for added file")
	}

	// 删除文件后再次索引
	os.Remove(filepath.Join(dir, "a.go"))
	result2, err := idx.ScanAndIndex(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result2.Deleted != 1 {
		t.Errorf("deleted = %d, want 1", result2.Deleted)
	}
}

func TestIndex_WithModify(t *testing.T) {
	idx, gs, cleanup := openTestIndexer(t)
	defer cleanup()

	dir, _ := os.MkdirTemp("", "modtest-*")
	defer os.RemoveAll(dir)
	filePath := filepath.Join(dir, "b.go")
	os.WriteFile(filePath, []byte("package main // v1"), 0644)

	idx.WithParseFunc(func(ctx context.Context, fp string, content []byte, hash string) error {
		n := graph.NewNode("testrepo", graph.LabelFile, fp)
		n.FilePath = fp
		n.Props = graph.NodePropsFromMap(map[string]any{"contentHash": hash})
		gs.AddNode(n)
		gs.Flush()
		return nil
	})

	// 首次索引
	idx.ScanAndIndex(context.Background(), dir)

	// 修改文件
	os.WriteFile(filePath, []byte("package main // v2"), 0644)

	result, err := idx.ScanAndIndex(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Modified != 1 {
		t.Errorf("modified = %d, want 1", result.Modified)
	}
}

func TestIndex_WithEmbedFunc(t *testing.T) {
	idx, gs, cleanup := openTestIndexer(t)
	defer cleanup()

	dir, _ := os.MkdirTemp("", "embedtest-*")
	defer os.RemoveAll(dir)

	// 先用 parseFn 创建一个 File 节点（模拟已索引状态）
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package main // v1"), 0644)
	idx.WithParseFunc(func(ctx context.Context, fp string, content []byte, hash string) error {
		n := graph.NewNode("testrepo", graph.LabelFile, fp)
		n.FilePath = fp
		n.Props = graph.NodePropsFromMap(map[string]any{"contentHash": hash})
		gs.AddNode(n)
		gs.Flush()
		return nil
	})
	// 首次索引
	idx.ScanAndIndex(context.Background(), dir)

	// 修改文件触发 modify 路径，embedFn 会被调用
	os.WriteFile(filepath.Join(dir, "c.go"), []byte("package main // v2"), 0644)
	embedCalled := false
	idx.WithEmbedFunc(func(ctx context.Context, nodeIDs []string) error {
		embedCalled = true
		return nil
	})
	result, err := idx.ScanAndIndex(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	if result.Modified != 1 {
		t.Errorf("modified = %d, want 1", result.Modified)
	}
	if !embedCalled {
		t.Error("embedFn should be called on modify path")
	}
}

func TestDetectFileChanges(t *testing.T) {
	idx, gs, cleanup := openTestIndexer(t)
	defer cleanup()

	dir, _ := os.MkdirTemp("", "detecttest-*")
	defer os.RemoveAll(dir)
	os.WriteFile(filepath.Join(dir, "d.go"), []byte("package main"), 0644)

	// 首次索引
	idx.WithParseFunc(func(ctx context.Context, fp string, content []byte, hash string) error {
		n := graph.NewNode("testrepo", graph.LabelFile, fp)
		n.FilePath = fp
		n.Props = graph.NodePropsFromMap(map[string]any{"contentHash": hash})
		gs.AddNode(n)
		gs.Flush()
		return nil
	})
	idx.ScanAndIndex(context.Background(), dir)

	// 检测变更（无变更）
	changes, err := idx.DetectFileChanges(context.Background(), dir)
	if err != nil {
		t.Fatal(err)
	}
	hasUnchanged := false
	for _, c := range changes {
		if c.Type == ChangeUnchanged {
			hasUnchanged = true
		}
	}
	if !hasUnchanged {
		t.Error("expected at least one unchanged file")
	}
}

func TestDeleteFileNodes(t *testing.T) {
	idx, gs, cleanup := openTestIndexer(t)
	defer cleanup()

	n := graph.NewNode("testrepo", graph.LabelFile, "test.go")
	n.FilePath = "test.go"
	gs.AddNode(n)
	gs.Flush()

	if err := idx.deleteFileNodes("test.go"); err != nil {
		t.Fatal(err)
	}
}