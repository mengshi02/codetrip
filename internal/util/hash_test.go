package util

import (
	"testing"
)

func TestContentHash(t *testing.T) {
	tests := []struct {
		name  string
		input []byte
	}{
		{"empty", []byte{}},
		{"hello", []byte("hello")},
		{"world", []byte("world")},
		{"binary", []byte{0x00, 0x01, 0x02, 0xff}},
		{"large", make([]byte, 10000)},
	}

	hashes := make(map[string]string)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := ContentHash(tt.input)
			if len(h) != 40 { // SHA1 hex = 40 chars
				t.Errorf("ContentHash() length = %d, want 40", len(h))
			}
			// 相同输入必须产生相同哈希
			h2 := ContentHash(tt.input)
			if h != h2 {
				t.Errorf("ContentHash() not deterministic: %s != %s", h, h2)
			}
			hashes[tt.name] = h
		})
	}

	// 不同输入必须产生不同哈希
	if hashes["hello"] == hashes["world"] {
		t.Error("different inputs produced same hash")
	}
}

func TestContentHashString(t *testing.T) {
	h1 := ContentHashString("hello")
	h2 := ContentHash([]byte("hello"))
	if h1 != h2 {
		t.Errorf("ContentHashString != ContentHash: %s != %s", h1, h2)
	}
}

func TestFingerprint(t *testing.T) {
	fp1 := Fingerprint("foo", "Function", "bar.go", nil)
	fp2 := Fingerprint("foo", "Function", "bar.go", nil)
	if fp1 != fp2 {
		t.Errorf("Fingerprint not deterministic: %s != %s", fp1, fp2)
	}
	if len(fp1) != 40 {
		t.Errorf("Fingerprint length = %d, want 40", len(fp1))
	}

	// 不同输入产生不同指纹
	fp3 := Fingerprint("baz", "Function", "bar.go", nil)
	if fp1 == fp3 {
		t.Error("different inputs produced same fingerprint")
	}

	// 带属性的指纹
	props := map[string]any{"key": "value"}
	fp4 := Fingerprint("foo", "Function", "bar.go", props)
	if fp1 == fp4 {
		t.Error("props should change fingerprint")
	}
}

func BenchmarkContentHash(b *testing.B) {
	data := []byte("benchmark test data for content hash")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ContentHash(data)
	}
}

func BenchmarkContentHashLarge(b *testing.B) {
	data := make([]byte, 65536) // 64KB
	for i := range data {
		data[i] = byte(i % 256)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ContentHash(data)
	}
}

func BenchmarkFingerprint(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Fingerprint("repo", "Function", "file.go", nil)
	}
}