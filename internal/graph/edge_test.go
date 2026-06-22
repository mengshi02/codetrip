package graph

import (
	"testing"
)

func TestNewEdge(t *testing.T) {
	e := NewEdge(RelCalls, "src1", "tgt1")
	if e.Type != RelCalls {
		t.Errorf("Type = %s, want CALLS", e.Type)
	}
	if e.Source != "src1" {
		t.Errorf("Source = %s, want src1", e.Source)
	}
	if e.Target != "tgt1" {
		t.Errorf("Target = %s, want tgt1", e.Target)
	}
}

func TestEdge_WithID(t *testing.T) {
	e := NewEdge(RelCalls, "src1", "tgt1").WithID("edge123")
	if e.ID != "edge123" {
		t.Errorf("ID = %s, want edge123", e.ID)
	}
}

func TestEdge_WithProp(t *testing.T) {
	e := NewEdge(RelCalls, "src1", "tgt1").WithProp("weight", 0.8)
	if e.GetProp("weight", 0.0) != 0.8 {
		t.Error("WithProp/GetProp mismatch")
	}
}

func TestEdge_GetPropFloat64(t *testing.T) {
	e := NewEdge(RelCalls, "src1", "tgt1").WithProp("weight", 0.8)
	if e.GetPropFloat64("weight") != 0.8 {
		t.Errorf("GetPropFloat64 = %f, want 0.8", e.GetPropFloat64("weight"))
	}
}

func TestEdge_Confidence(t *testing.T) {
	e := NewEdge(RelCalls, "src1", "tgt1").WithProp("confidence", 0.9)
	if c := e.Confidence(); c != 0.9 {
		t.Errorf("Confidence = %f, want 0.9", c)
	}
	// 默认置信度应为 1.0
	e2 := NewEdge(RelCalls, "src1", "tgt1")
	if c := e2.Confidence(); c != 1.0 {
		t.Errorf("default Confidence = %f, want 1.0", c)
	}
}

func TestEdge_Key(t *testing.T) {
	e := NewEdge(RelCalls, "src1", "tgt1").WithID("edge123")
	key := e.Key()
	if key != "e:edge123" {
		t.Errorf("Key = %s, want e:edge123", key)
	}
}

func TestEdge_NilProps(t *testing.T) {
	e := &Edge{Type: RelCalls, Source: "s", Target: "t"}
	// GetProp on nil Props should return default
	if e.GetProp("key", "default") != "default" {
		t.Error("GetProp on nil Props should return default")
	}
	if e.Confidence() != 1.0 {
		t.Error("Confidence on nil Props should be 1.0")
	}
}

func TestRelTypeConstants(t *testing.T) {
	// 验证关键关系类型常量
	if RelCalls != "CALLS" {
		t.Errorf("RelCalls = %s, want CALLS", RelCalls)
	}
	if RelContains != "CONTAINS" {
		t.Errorf("RelContains = %s, want CONTAINS", RelContains)
	}
	if RelImports != "IMPORTS" {
		t.Errorf("RelImports = %s, want IMPORTS", RelImports)
	}
}