package community

import (
	"testing"
)

func TestNewLeidenAlgorithm(t *testing.T) {
	l := NewLeidenAlgorithm()
	if l == nil {
		t.Fatal("algorithm is nil")
	}
	if l.seed != 0xc0de {
		t.Errorf("seed = %d, want 0xc0de", l.seed)
	}
	if l.resolution != 1.0 {
		t.Errorf("resolution = %f, want 1.0", l.resolution)
	}
}

func TestWithLeidenSeed(t *testing.T) {
	l := NewLeidenAlgorithm(WithLeidenSeed(42))
	if l.seed != 42 {
		t.Errorf("seed = %d, want 42", l.seed)
	}
}

func TestWithResolution(t *testing.T) {
	l := NewLeidenAlgorithm(WithResolution(0.5))
	if l.resolution != 0.5 {
		t.Errorf("resolution = %f, want 0.5", l.resolution)
	}
}

func TestLeidenDetect_Empty(t *testing.T) {
	l := NewLeidenAlgorithm()
	ag := &AdjGraph{
		Nodes:   []string{},
		NodeIdx: map[string]int{},
		Adj:     [][]neighbor{},
		Weight:  []float64{},
	}
	result := l.Detect(ag)
	if result == nil {
		t.Fatal("result is nil")
	}
	if len(result.Communities) != 0 {
		t.Errorf("expected 0 communities, got %d", len(result.Communities))
	}
}

func TestLeidenDetect_Single(t *testing.T) {
	l := NewLeidenAlgorithm()
	ag := &AdjGraph{
		Nodes:       []string{"A"},
		NodeIdx:     map[string]int{"A": 0},
		Adj:         [][]neighbor{{}},
		Weight:      []float64{0},
		TotalWeight: 0,
	}
	result := l.Detect(ag)
	if len(result.Communities) != 1 {
		t.Errorf("expected 1 community, got %d", len(result.Communities))
	}
}

func TestLeidenDetect_TwoConnected(t *testing.T) {
	l := NewLeidenAlgorithm()
	ag := &AdjGraph{
		Nodes:       []string{"A", "B"},
		NodeIdx:     map[string]int{"A": 0, "B": 1},
		Adj:         [][]neighbor{{{idx: 1, weight: 1.0}}, {{idx: 0, weight: 1.0}}},
		Weight:      []float64{1.0, 1.0},
		TotalWeight: 2.0,
	}
	result := l.Detect(ag)
	if len(result.Communities) != 1 {
		t.Errorf("expected 1 community (connected), got %d", len(result.Communities))
	}
}

func TestLeidenDetect_TwoDisconnected(t *testing.T) {
	l := NewLeidenAlgorithm()
	ag := &AdjGraph{
		Nodes:       []string{"A", "B"},
		NodeIdx:     map[string]int{"A": 0, "B": 1},
		Adj:         [][]neighbor{{}, {}},
		Weight:      []float64{0, 0},
		TotalWeight: 0,
	}
	result := l.Detect(ag)
	if len(result.Communities) != 2 {
		t.Errorf("expected 2 communities (disconnected), got %d", len(result.Communities))
	}
}

func TestLeidenDetect_Triangle(t *testing.T) {
	l := NewLeidenAlgorithm()
	ag := &AdjGraph{
		Nodes:       []string{"A", "B", "C"},
		NodeIdx:     map[string]int{"A": 0, "B": 1, "C": 2},
		Adj: [][]neighbor{
			{{idx: 1, weight: 1.0}, {idx: 2, weight: 1.0}},
			{{idx: 0, weight: 1.0}, {idx: 2, weight: 1.0}},
			{{idx: 0, weight: 1.0}, {idx: 1, weight: 1.0}},
		},
		Weight:      []float64{2.0, 2.0, 2.0},
		TotalWeight: 6.0,
	}
	result := l.Detect(ag)
	if len(result.Communities) != 1 {
		t.Errorf("expected 1 community (triangle), got %d", len(result.Communities))
	}
	if result.Stats.Modularity < 0 {
		t.Errorf("modularity should be non-negative, got %f", result.Stats.Modularity)
	}
}

func TestCommonPrefix(t *testing.T) {
	tests := []struct {
		names []string
		want  string
	}{
		{[]string{"pkg/handler", "pkg/service"}, "pkg/"},
		{[]string{"abc", "abd"}, "ab"},
		{[]string{"x", "y"}, ""},
		{[]string{"same"}, "same"},
		{[]string{}, ""},
	}
	for _, tt := range tests {
		got := commonPrefix(tt.names)
		if got != tt.want {
			t.Errorf("commonPrefix(%v) = %q, want %q", tt.names, got, tt.want)
		}
	}
}

func TestLeidenDetect_Stats(t *testing.T) {
	l := NewLeidenAlgorithm()
	ag := &AdjGraph{
		Nodes:       []string{"A", "B", "C"},
		NodeIdx:     map[string]int{"A": 0, "B": 1, "C": 2},
		Adj: [][]neighbor{
			{{idx: 1, weight: 1.0}},
			{{idx: 0, weight: 1.0}},
			{},
		},
		Weight:      []float64{1.0, 1.0, 0},
		TotalWeight: 2.0,
	}
	result := l.Detect(ag)
	if result.Stats.CommunityCount == 0 {
		t.Error("expected non-zero community count")
	}
	if result.Stats.AvgCohesion < 0 || result.Stats.AvgCohesion > 1 {
		t.Errorf("avg cohesion out of range: %f", result.Stats.AvgCohesion)
	}
}