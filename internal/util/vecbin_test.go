package util

import (
	"math"
	"testing"
)

func TestEncodeDecodeFloat32Vec(t *testing.T) {
	vec := []float32{1.0, -2.5, 0.0, 3.14, -0.001, 999.99}
	encoded := EncodeFloat32Vec(vec)
	decoded := DecodeFloat32Vec(encoded)

	if len(decoded) != len(vec) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(vec))
	}
	for i, v := range vec {
		if decoded[i] != v {
			t.Errorf("element %d: got %v, want %v", i, decoded[i], v)
		}
	}
}

func TestEncodeDecodeFloat32Vec_Empty(t *testing.T) {
	vec := []float32{}
	encoded := EncodeFloat32Vec(vec)
	if len(encoded) != 0 {
		t.Fatalf("empty vec should encode to empty bytes, got %d bytes", len(encoded))
	}
	decoded := DecodeFloat32Vec(encoded)
	if len(decoded) != 0 {
		t.Fatalf("empty data should decode to empty vec, got %d elements", len(decoded))
	}
}

func TestEncodeDecodeFloat32Vec_SpecialValues(t *testing.T) {
	vec := []float32{float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN()), 0.0, -0.0}
	encoded := EncodeFloat32Vec(vec)
	decoded := DecodeFloat32Vec(encoded)

	if len(decoded) != len(vec) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(vec))
	}
	if !math.IsInf(float64(decoded[0]), 1) {
		t.Errorf("+Inf not preserved: got %v", decoded[0])
	}
	if !math.IsInf(float64(decoded[1]), -1) {
		t.Errorf("-Inf not preserved: got %v", decoded[1])
	}
	if !math.IsNaN(float64(decoded[2])) {
		t.Errorf("NaN not preserved: got %v", decoded[2])
	}
}

func TestEncodeDecodeStringList(t *testing.T) {
	list := []string{"node1", "node2", "node_with_underscore", "node-with-dash", ""}
	encoded := EncodeStringList(list)
	decoded, err := DecodeStringList(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decoded) != len(list) {
		t.Fatalf("length mismatch: got %d, want %d", len(decoded), len(list))
	}
	for i, s := range list {
		if decoded[i] != s {
			t.Errorf("element %d: got %q, want %q", i, decoded[i], s)
		}
	}
}

func TestEncodeDecodeStringList_Empty(t *testing.T) {
	list := []string{}
	encoded := EncodeStringList(list)
	decoded, err := DecodeStringList(encoded)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(decoded) != 0 {
		t.Fatalf("empty list should decode to empty, got %d elements", len(decoded))
	}
}

func TestDecodeStringList_Truncated(t *testing.T) {
	// Truncated data should return partial results without panic
	encoded := EncodeStringList([]string{"a", "b", "c"})
	truncated := encoded[:5] // truncate in the middle
	decoded, err := DecodeStringList(truncated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should return partial results, not panic
	_ = decoded
}

func BenchmarkEncodeFloat32Vec_384(b *testing.B) {
	vec := make([]float32, 384)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeFloat32Vec(vec)
	}
}

func BenchmarkDecodeFloat32Vec_384(b *testing.B) {
	vec := make([]float32, 384)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}
	encoded := EncodeFloat32Vec(vec)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DecodeFloat32Vec(encoded)
	}
}

func BenchmarkEncodeStringList_10K(b *testing.B) {
	list := make([]string, 10000)
	for i := range list {
		list[i] = "node_" + string(rune('A'+i%26)) + string(rune('0'+i%10))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EncodeStringList(list)
	}
}

func BenchmarkDecodeStringList_10K(b *testing.B) {
	list := make([]string, 10000)
	for i := range list {
		list[i] = "node_" + string(rune('A'+i%26)) + string(rune('0'+i%10))
	}
	encoded := EncodeStringList(list)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		DecodeStringList(encoded)
	}
}