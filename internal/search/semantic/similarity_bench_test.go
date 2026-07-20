package semantic

import "testing"

func BenchmarkCosineSimilarity(b *testing.B) {
	a := make([]float32, 384)
	v := make([]float32, 384)
	for i := range a {
		a[i] = float32(i) / 384.0
		v[i] = float32(i+1) / 384.0
	}
	b.ResetTimer()
	for b.Loop() {
		cosineSimilarity(a, v)
	}
}
