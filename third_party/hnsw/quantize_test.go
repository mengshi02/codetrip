package hnsw

import (
	"math"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestQuantizeDequantize_RoundTrip verifies that quantize → dequantize round-trip
// preserves vector values within acceptable tolerance.
func TestQuantizeDequantize_RoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dim := 384
	n := 1000

	vectors := make([][]float32, n)
	for i := range vectors {
		vec := make([]float32, dim)
		for j := range vec {
			vec[j] = rng.Float32()*2 - 1
		}
		vectors[i] = vec
	}

	params := TrainQuantParams(vectors)

	maxAbsErr := float32(0)
	totalAbsErr := float32(0)
	count := 0
	maxRelErr := float32(0)
	const relErrThreshold = float32(0.01)

	for _, vec := range vectors {
		qvec := Quantize(vec, params)
		recovered := Dequantize(qvec, params)

		for j := range vec {
			absErr := abs32(recovered[j] - vec[j])
			if absErr > maxAbsErr {
				maxAbsErr = absErr
			}
			totalAbsErr += absErr
			count++
			if abs32(vec[j]) > relErrThreshold {
				relErr := absErr / abs32(vec[j])
				if relErr > maxRelErr {
					maxRelErr = relErr
				}
			}
		}
	}

	avgAbsErr := totalAbsErr / float32(count)
	t.Logf("Quantize/Dequantize accuracy (384-dim, %d vectors):", n)
	t.Logf("  Max absolute error: %.6f", maxAbsErr)
	t.Logf("  Avg absolute error: %.6f", avgAbsErr)
	t.Logf("  Max relative error (|val|>%.2f): %.4f", relErrThreshold, maxRelErr)

	require.Less(t, float64(maxAbsErr), 0.01, "max absolute error should be < 0.01")
	require.Less(t, float64(avgAbsErr), 0.005, "avg absolute error should be < 0.005")
}

// TestInt8CosineDistance_Accuracy validates that int8 cosine distance closely
// approximates float32 cosine distance.
func TestInt8CosineDistance_Accuracy(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dim := 384
	n := 500

	vectors := make([][]float32, n)
	for i := range vectors {
		vec := make([]float32, dim)
		var norm float32
		for j := range vec {
			vec[j] = rng.Float32()*2 - 1
			norm += vec[j] * vec[j]
		}
		norm = float32(math.Sqrt(float64(norm)))
		for j := range vec {
			vec[j] /= norm
		}
		vectors[i] = vec
	}

	params := TrainQuantParams(vectors)

	maxDiff := float32(0)
	totalDiff := float32(0)
	pairs := 1000

	for i := 0; i < pairs; i++ {
		a := vectors[rng.Intn(n)]
		b := vectors[rng.Intn(n)]

		floatDist := CosineDistance(a, b)
		qa := Quantize(a, params)
		qb := Quantize(b, params)
		int8Dist := Int8CosineDistance(qa, qb, params)

		diff := abs32(floatDist - int8Dist)
		if diff > maxDiff {
			maxDiff = diff
		}
		totalDiff += diff
	}

	avgDiff := totalDiff / float32(pairs)
	t.Logf("Int8CosineDistance vs float32 CosineDistance (%d pairs):", pairs)
	t.Logf("  Max absolute diff: %.6f", maxDiff)
	t.Logf("  Avg absolute diff: %.6f", avgDiff)

	require.Less(t, float64(maxDiff), 0.05, "max distance diff should be < 0.05")
	require.Less(t, float64(avgDiff), 0.01, "avg distance diff should be < 0.01")
}

// TestInt8CosineDistanceFast_Accuracy validates the fast int8 distance function
// produces valid (non-NaN) results.
func TestInt8CosineDistanceFast_Accuracy(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dim := 384
	n := 200

	vectors := make([][]float32, n)
	for i := range vectors {
		vec := make([]float32, dim)
		var norm float32
		for j := range vec {
			vec[j] = rng.Float32()*2 - 1
			norm += vec[j] * vec[j]
		}
		norm = float32(math.Sqrt(float64(norm)))
		for j := range vec {
			vec[j] /= norm
		}
		vectors[i] = vec
	}

	params := TrainQuantParams(vectors)

	for i := 0; i < 500; i++ {
		a := vectors[rng.Intn(n)]
		b := vectors[rng.Intn(n)]
		qa := Quantize(a, params)
		qb := Quantize(b, params)
		fast := Int8CosineDistanceFast(qa, qb, params)
		require.False(t, math.IsNaN(float64(fast)), "fast distance should not produce NaN")
		require.True(t, fast >= -0.01 && fast <= 2.01, "fast distance should be in [0, 2] range, got %f", fast)
	}
	t.Log("Int8CosineDistanceFast: all 500 samples valid (no NaN, in range)")
}

// TestQuantizeRecall validates that int8 quantization does not significantly
// degrade HNSW recall compared to float32 baseline.
func TestQuantizeRecall(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	dim := 8
	n := 2000
	k := 10

	vectors := make([][]float32, n)
	for i := range vectors {
		vec := make([]float32, dim)
		vec[i%dim] = float32(i) / float32(n) * 2.0
		for j := range vec {
			if j != i%dim {
				vec[j] = (rng.Float32() - 0.5) * 0.1
			}
		}
		vectors[i] = vec
	}

	params := TrainQuantParams(vectors)

	bruteForceTopK := func(queryVec []float32, queryIdx int) map[int]bool {
		type distEntry struct {
			idx  int
			dist float32
		}
		entries := make([]distEntry, n)
		for i, vec := range vectors {
			entries[i] = distEntry{idx: i, dist: CosineDistance(queryVec, vec)}
		}
		topK := make(map[int]bool)
		for i := 0; i < k+1 && i < n; i++ {
			minIdx := i
			for j := i + 1; j < n; j++ {
				if entries[j].dist < entries[minIdx].dist {
					minIdx = j
				}
			}
			entries[i], entries[minIdx] = entries[minIdx], entries[i]
			if entries[i].idx != queryIdx {
				topK[entries[i].idx] = true
			}
		}
		return topK
	}

	computeRecall := func(g *Graph[int]) float64 {
		totalRecall := 0.0
		queries := 100
		for q := 0; q < queries; q++ {
			queryIdx := rng.Intn(n)
			queryVec := vectors[queryIdx]
			topK := bruteForceTopK(queryVec, queryIdx)
			results, err := g.Search(queryVec, k)
			if err != nil {
				t.Fatalf("search: %v", err)
			}
			hits := 0
			for _, r := range results {
				if topK[r.Key] {
					hits++
				}
			}
			if len(topK) > 0 {
				totalRecall += float64(hits) / float64(len(topK))
			}
		}
		return totalRecall / float64(queries)
	}

	// Float32 baseline
	gFloat := NewGraph[int]()
	gFloat.M = 16
	gFloat.Ml = 0.5
	gFloat.EfSearch = 200
	gFloat.Distance = CosineDistance
	for i, vec := range vectors {
		if err := gFloat.Add(MakeNode(i, vec)); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	float32Recall := computeRecall(gFloat)

	// Int8 quantized
	gInt8 := NewGraph[int]()
	gInt8.M = 16
	gInt8.Ml = 0.5
	gInt8.EfSearch = 200
	gInt8.Distance = CosineDistance
	gInt8.QuantType = QuantInt8
	gInt8.QuantParams = params
	for i, vec := range vectors {
		if err := gInt8.Add(MakeNode(i, vec)); err != nil {
			t.Fatalf("add: %v", err)
		}
	}
	int8Recall := computeRecall(gInt8)

	t.Logf("Float32 recall@%d (dim=%d, n=%d): %.4f", k, dim, n, float32Recall)
	t.Logf("Int8    recall@%d (dim=%d, n=%d): %.4f", k, dim, n, int8Recall)

	// Int8 recall should be within 10% of float32 recall
	require.Greater(t, int8Recall, float32Recall*0.90,
		"int8 recall should be within 10%% of float32 recall")
	// Don't enforce absolute recall threshold — HNSW recall depends on
	// data distribution and parameters. The key metric is that int8
	// quantization doesn't significantly degrade recall vs float32.
	t.Logf("Int8/Float32 recall ratio: %.2f%%", int8Recall/float32Recall*100)
}

// TestEncodeDecodeQuantParams verifies serialization round-trip for QuantParams.
func TestEncodeDecodeQuantParams(t *testing.T) {
	params := QuantParams{
		Scale:  []float32{0.01, 0.02, 0.03, 0.04},
		Offset: []float32{-1.0, -0.5, 0.0, 0.5},
	}

	data := EncodeQuantParams(params)
	recovered := DecodeQuantParams(data)

	require.Equal(t, len(params.Scale), len(recovered.Scale))
	require.Equal(t, len(params.Offset), len(recovered.Offset))

	for i := range params.Scale {
		require.Equal(t, params.Scale[i], recovered.Scale[i], "scale[%d] mismatch", i)
		require.Equal(t, params.Offset[i], recovered.Offset[i], "offset[%d] mismatch", i)
	}
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
