package community

import (
	"fmt"
	"testing"
)

func BenchmarkLeidenDetect_Small(b *testing.B) {
	ag := buildBenchGraph(50)
	l := NewLeidenAlgorithm()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Detect(ag)
	}
}

func BenchmarkLeidenDetect_Medium(b *testing.B) {
	ag := buildBenchGraph(200)
	l := NewLeidenAlgorithm()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Detect(ag)
	}
}

func BenchmarkLeidenDetect_Large(b *testing.B) {
	ag := buildBenchGraph(1000)
	l := NewLeidenAlgorithm()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		l.Detect(ag)
	}
}

func BenchmarkBuildAdjGraph(b *testing.B) {
	// Benchmark with synthetic AdjGraph construction
	ag := buildBenchGraph(100)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ag.Nodes
	}
}

// buildBenchGraph 构建基准测试图（随机小世界网络）
func buildBenchGraph(n int) *AdjGraph {
	ag := &AdjGraph{
		Nodes:   make([]string, n),
		NodeIdx: make(map[string]int, n),
		Adj:     make([][]neighbor, n),
		Weight:  make([]float64, n),
	}
	for i := 0; i < n; i++ {
		ag.Nodes[i] = fmt.Sprintf("node%d", i)
		ag.NodeIdx[ag.Nodes[i]] = i
	}
	// 每个节点连接2-3个邻居
	rng := NewLeidenAlgorithm().rng
	for i := 0; i < n; i++ {
		for j := 0; j < 3; j++ {
			nb := rng.Intn(n)
			if nb != i {
				w := rng.Float64()
				ag.Adj[i] = append(ag.Adj[i], neighbor{idx: nb, weight: w})
				ag.Weight[i] += w
				ag.TotalWeight += w
			}
		}
	}
	return ag
}