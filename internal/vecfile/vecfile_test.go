package vecfile

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/coder/hnsw"
)

// TestVectorFile_WriteReadRoundTrip verifies that writing a quantized vector file
// and reading it back via mmap produces consistent data.
func TestVectorFile_WriteReadRoundTrip(t *testing.T) {
	dim := 384
	nodeCount := 100
	chunkCount := 50

	// Generate test vectors and quantization parameters
	vectors := make([][]float32, nodeCount+chunkCount)
	for i := range vectors {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = float32(i)*0.01 + float32(j)*0.001 // deterministic pattern
		}
		vectors[i] = vec
	}

	params := hnsw.TrainQuantParams(vectors)

	// Build writer
	w := NewVectorFileWriter(dim, params.Scale, params.Offset)
	for i := 0; i < nodeCount; i++ {
		qvec := hnsw.Quantize(vectors[i], params)
		w.AddNodeVector(qvec)
	}
	for i := nodeCount; i < nodeCount+chunkCount; i++ {
		qvec := hnsw.Quantize(vectors[i], params)
		w.AddChunkVector(qvec)
	}

	// Write file
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	if err := w.Write(path); err != nil {
		t.Fatalf("write vector file: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat vector file: %v", err)
	}

	// Read file
	reader, err := OpenVectorFile(path)
	if err != nil {
		t.Fatalf("open vector file: %v", err)
	}
	defer reader.Close()

	// Verify metadata
	if reader.Dim() != dim {
		t.Errorf("dim = %d, want %d", reader.Dim(), dim)
	}
	if reader.NodeCount() != nodeCount {
		t.Errorf("nodeCount = %d, want %d", reader.NodeCount(), nodeCount)
	}
	if reader.ChunkCount() != chunkCount {
		t.Errorf("chunkCount = %d, want %d", reader.ChunkCount(), chunkCount)
	}

	// Verify quantized vectors match
	for i := 0; i < nodeCount; i++ {
		expected := hnsw.Quantize(vectors[i], params)
		got := reader.NodeQVec(i)
		if got == nil {
			t.Errorf("NodeQVec(%d) = nil", i)
			continue
		}
		if len(got) != len(expected) {
			t.Errorf("NodeQVec(%d) len = %d, want %d", i, len(got), len(expected))
			continue
		}
		for j := range expected {
			if got[j] != expected[j] {
				t.Errorf("NodeQVec(%d)[%d] = %d, want %d", i, j, got[j], expected[j])
				break
			}
		}
	}

	// Verify chunk vectors
	for i := 0; i < chunkCount; i++ {
		expected := hnsw.Quantize(vectors[nodeCount+i], params)
		got := reader.ChunkQVec(i)
		if got == nil {
			t.Errorf("ChunkQVec(%d) = nil", i)
			continue
		}
		for j := range expected {
			if got[j] != expected[j] {
				t.Errorf("ChunkQVec(%d)[%d] = %d, want %d", i, j, got[j], expected[j])
				break
			}
		}
	}

	t.Logf("Vector file round-trip: dim=%d nodes=%d chunks=%d OK", dim, nodeCount, chunkCount)
}

// TestVectorFile_Float32Dequantization verifies that dequantized float32 vectors
// from the mmap'd file are close to the original vectors.
func TestVectorFile_Float32Dequantization(t *testing.T) {
	dim := 64
	nodeCount := 50

	vectors := make([][]float32, nodeCount)
	for i := range vectors {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = float32(i)*0.05 + float32(j)*0.01
		}
		vectors[i] = vec
	}

	params := hnsw.TrainQuantParams(vectors)

	w := NewVectorFileWriter(dim, params.Scale, params.Offset)
	for i := 0; i < nodeCount; i++ {
		w.AddNodeVector(hnsw.Quantize(vectors[i], params))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	if err := w.Write(path); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader, err := OpenVectorFile(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reader.Close()

	// Verify dequantized vectors are close to originals
	maxAbsErr := float32(0)
	for i := 0; i < nodeCount; i++ {
		recovered := reader.NodeFloat32(i)
		if recovered == nil {
			t.Errorf("NodeFloat32(%d) = nil", i)
			continue
		}
		for j := range vectors[i] {
			err := abs32(recovered[j] - vectors[i][j])
			if err > maxAbsErr {
				maxAbsErr = err
			}
		}
	}

	t.Logf("Max dequantization error: %.6f", maxAbsErr)
	if maxAbsErr > 0.02 {
		t.Errorf("dequantization error too large: %.6f", maxAbsErr)
	}
}

// TestVectorFile_OutOfBounds verifies that out-of-bounds access returns nil.
func TestVectorFile_OutOfBounds(t *testing.T) {
	dim := 16
	nodeCount := 5
	chunkCount := 3

	vectors := make([][]float32, nodeCount+chunkCount)
	for i := range vectors {
		vectors[i] = make([]float32, dim)
	}
	params := hnsw.TrainQuantParams(vectors)

	w := NewVectorFileWriter(dim, params.Scale, params.Offset)
	for i := 0; i < nodeCount; i++ {
		w.AddNodeVector(hnsw.Quantize(vectors[i], params))
	}
	for i := 0; i < chunkCount; i++ {
		w.AddChunkVector(hnsw.Quantize(vectors[nodeCount+i], params))
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	if err := w.Write(path); err != nil {
		t.Fatalf("write: %v", err)
	}

	reader, err := OpenVectorFile(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer reader.Close()

	// Negative index
	if reader.NodeQVec(-1) != nil {
		t.Error("NodeQVec(-1) should be nil")
	}
	if reader.ChunkQVec(-1) != nil {
		t.Error("ChunkQVec(-1) should be nil")
	}
	// Out of range
	if reader.NodeQVec(nodeCount) != nil {
		t.Error("NodeQVec(nodeCount) should be nil")
	}
	if reader.ChunkQVec(chunkCount) != nil {
		t.Error("ChunkQVec(chunkCount) should be nil")
	}
}

// TestVectorFilePath verifies the path helper functions.
func TestVectorFilePath(t *testing.T) {
	path := VectorFilePath("/data", "myrepo")
	expected := filepath.Join("/data", "vectors", "myrepo.bin")
	if path != expected {
		t.Errorf("VectorFilePath = %q, want %q", path, expected)
	}
}

// TestVectorFileExists verifies the existence check.
func TestVectorFileExists(t *testing.T) {
	dir := t.TempDir()
	if VectorFileExists(dir, "myrepo") {
		t.Error("should not exist initially")
	}
	// Create the file
	path := VectorFilePath(dir, "myrepo")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if !VectorFileExists(dir, "myrepo") {
		t.Error("should exist after creation")
	}
}

// TestVectorFile_InvalidMagic verifies that invalid files are rejected.
func TestVectorFile_InvalidMagic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.bin")
	if err := os.WriteFile(path, make([]byte, 128), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := OpenVectorFile(path)
	if err == nil {
		t.Error("expected error for invalid magic")
	}
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// Ensure math is used (for potential future extensions)
var _ = math.Pi