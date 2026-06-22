package util

import (
	"strings"
	"testing"
)

func TestGenerateID(t *testing.T) {
	id1 := GenerateID("repo1", "Function", "foo")

	// 应包含格式: {repo}:{label}:{name}:{shortHash}
	if !strings.HasPrefix(id1, "repo1:Function:foo:") {
		t.Errorf("GenerateID format wrong: %s", id1)
	}

	// 格式验证: 4 段以 : 分隔
	parts := strings.Split(id1, ":")
	if len(parts) != 4 {
		t.Errorf("GenerateID should have 4 parts, got %d: %s", len(parts), id1)
	}
	if len(parts[3]) != 8 {
		t.Errorf("shortHash should be 8 chars, got %d: %s", len(parts[3]), parts[3])
	}
}

func TestGenerateID_DifferentInputs(t *testing.T) {
	id1 := GenerateID("repo1", "Function", "foo")
	id2 := GenerateID("repo2", "Function", "foo")
	if id1 == id2 {
		t.Error("different repos should produce different IDs")
	}

	id3 := GenerateID("repo1", "Class", "foo")
	if id1 == id3 {
		t.Error("different labels should produce different IDs")
	}

	id4 := GenerateID("repo1", "Function", "bar")
	if id1 == id4 {
		t.Error("different names should produce different IDs")
	}
}

func TestGenerateEdgeID(t *testing.T) {
	id1 := GenerateEdgeID("repo1", "src1", "CALLS", "tgt1")

	// 格式: e:{repo}:{src}:{type}:{target}:{shortHash}
	if !strings.HasPrefix(id1, "e:repo1:src1:CALLS:tgt1:") {
		t.Errorf("GenerateEdgeID format wrong: %s", id1)
	}
}

func TestGenerateEdgeID_Unique(t *testing.T) {
	// 格式验证
	id1 := GenerateEdgeID("repo1", "src1", "CALLS", "tgt1")
	parts := strings.Split(id1, ":")
	// e:repo1:src1:CALLS:tgt1:shortHash
	if len(parts) != 6 {
		t.Errorf("GenerateEdgeID should have 6 parts, got %d: %s", len(parts), id1)
	}
	if parts[0] != "e" {
		t.Errorf("GenerateEdgeID should start with 'e', got %s", parts[0])
	}
}

func TestGenerateEdgeID_DifferentTargets(t *testing.T) {
	id1 := GenerateEdgeID("repo1", "src1", "CALLS", "tgt1")
	id2 := GenerateEdgeID("repo1", "src1", "CALLS", "tgt2")
	if id1 == id2 {
		t.Error("different targets should produce different edge IDs")
	}
}

func TestGenerateRandomID(t *testing.T) {
	id1 := GenerateRandomID()
	id2 := GenerateRandomID()

	if len(id1) != 32 { // 16 bytes hex = 32 chars
		t.Errorf("GenerateRandomID length = %d, want 32", len(id1))
	}
	if id1 == id2 {
		t.Error("GenerateRandomID should produce unique IDs")
	}
}

func TestNodeUID(t *testing.T) {
	uid := NodeUID("repo1", "main.go", "Function", "foo")
	expected := "repo1:main.go:Function:foo"
	if uid != expected {
		t.Errorf("NodeUID = %s, want %s", uid, expected)
	}

	// 确定性
	uid2 := NodeUID("repo1", "main.go", "Function", "foo")
	if uid != uid2 {
		t.Error("NodeUID should be deterministic")
	}
}

func BenchmarkGenerateID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateID("repo", "Function", "foo")
	}
}

func BenchmarkGenerateEdgeID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateEdgeID("repo", "src", "CALLS", "tgt")
	}
}

func BenchmarkGenerateRandomID(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateRandomID()
	}
}