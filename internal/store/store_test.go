package store

import (
	"fmt"
	"os"
	"testing"

	"github.com/cockroachdb/pebble/v2"
)

func tempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "db-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	return dir
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := tempDir(t)
	cfg := DefaultConfig(dir)
	cfg.CacheSize = 8 << 20 // 8MB for tests
	s, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenClose(t *testing.T) {
	dir := tempDir(t)
	cfg := DefaultConfig(dir)
	s, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestGetSet(t *testing.T) {
	s := openTestStore(t)
	key := []byte("test:key1")
	val := []byte("hello world")
	if err := s.Set(key, val); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(val) {
		t.Errorf("Get() = %q, want %q", got, val)
	}
}

func TestGet_NotFound(t *testing.T) {
	s := openTestStore(t)
	_, err := s.Get([]byte("nonexistent"))
	if err == nil {
		t.Error("expected error for nonexistent key")
	}
}

func TestDelete(t *testing.T) {
	s := openTestStore(t)
	key := []byte("test:del")
	if err := s.Set(key, []byte("value")); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(key); err != nil {
		t.Fatal(err)
	}
	_, err := s.Get(key)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestHasKey(t *testing.T) {
	s := openTestStore(t)
	key := []byte("test:has")
	found, err := s.HasKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if found {
		t.Error("should not find key before set")
	}
	if err := s.Set(key, []byte("val")); err != nil {
		t.Fatal(err)
	}
	found, err = s.HasKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Error("should find key after set")
	}
}

func TestBatch(t *testing.T) {
	s := openTestStore(t)
	err := s.Batch(func(b *pebble.Batch) error {
		if err := b.Set([]byte("b1"), []byte("v1"), nil); err != nil {
			return err
		}
		return b.Set([]byte("b2"), []byte("v2"), nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	v1, _ := s.Get([]byte("b1"))
	v2, _ := s.Get([]byte("b2"))
	if string(v1) != "v1" || string(v2) != "v2" {
		t.Errorf("batch results: %q, %q", v1, v2)
	}
}

func TestScanPrefix(t *testing.T) {
	s := openTestStore(t)
	for i := 0; i < 10; i++ {
		key := fmt.Sprintf("prefix:key%d", i)
		if err := s.Set([]byte(key), []byte(fmt.Sprintf("val%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.Set([]byte("other:key"), []byte("other")); err != nil {
		t.Fatal(err)
	}
	var results []string
	err := s.ScanPrefix([]byte("prefix:"), func(key, val []byte) error {
		results = append(results, string(key))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 10 {
		t.Errorf("ScanPrefix returned %d results, want 10", len(results))
	}
}

func TestScanPrefixLimit(t *testing.T) {
	s := openTestStore(t)
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("lim:key%d", i)
		if err := s.Set([]byte(key), []byte("v")); err != nil {
			t.Fatal(err)
		}
	}
	var results []string
	err := s.ScanPrefixLimit([]byte("lim:"), 10, func(key, val []byte) error {
		results = append(results, string(key))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 10 {
		t.Errorf("ScanPrefixLimit returned %d, want 10", len(results))
	}
	var allResults []string
	err = s.ScanPrefixLimit([]byte("lim:"), 0, func(key, val []byte) error {
		allResults = append(allResults, string(key))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(allResults) != 100 {
		t.Errorf("ScanPrefixLimit(0) returned %d, want 100", len(allResults))
	}
}

func TestSetNoSync(t *testing.T) {
	s := openTestStore(t)
	key := []byte("nosync:key")
	if err := s.SetNoSync(key, []byte("val")); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(key)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "val" {
		t.Errorf("SetNoSync: got %q, want %q", got, "val")
	}
}

func TestNewIterator(t *testing.T) {
	s := openTestStore(t)
	for i := 0; i < 5; i++ {
		key := fmt.Sprintf("iter:key%d", i)
		if err := s.Set([]byte(key), []byte(fmt.Sprintf("v%d", i))); err != nil {
			t.Fatal(err)
		}
	}
	iter := s.NewIterator(&IterOptions{LowerBound: []byte("iter:")})
	defer iter.Close()
	count := 0
	for iter.First(); iter.Valid(); iter.Next() {
		count++
	}
	if count != 5 {
		t.Errorf("iterator count = %d, want 5", count)
	}
}

func TestPath(t *testing.T) {
	dir := tempDir(t)
	cfg := DefaultConfig(dir)
	s, err := Open(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if s.Path() != dir {
		t.Errorf("Path() = %s, want %s", s.Path(), dir)
	}
}

// Benchmarks

func BenchmarkStoreSet(b *testing.B) {
	dir, _ := os.MkdirTemp("", "db-bench-*")
	defer os.RemoveAll(dir)
	cfg := DefaultConfig(dir)
	cfg.CacheSize = 64 << 20
	s, _ := Open(cfg)
	defer s.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Set([]byte(fmt.Sprintf("bench:key%d", i)), []byte("value"))
	}
}

func BenchmarkStoreGet(b *testing.B) {
	dir, _ := os.MkdirTemp("", "db-bench-*")
	defer os.RemoveAll(dir)
	cfg := DefaultConfig(dir)
	cfg.CacheSize = 64 << 20
	s, _ := Open(cfg)
	defer s.Close()
	for i := 0; i < 10000; i++ {
		s.Set([]byte(fmt.Sprintf("bench:key%d", i)), []byte("value"))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Get([]byte(fmt.Sprintf("bench:key%d", i%10000)))
	}
}

func BenchmarkScanPrefix(b *testing.B) {
	dir, _ := os.MkdirTemp("", "db-bench-*")
	defer os.RemoveAll(dir)
	cfg := DefaultConfig(dir)
	cfg.CacheSize = 64 << 20
	s, _ := Open(cfg)
	defer s.Close()
	for i := 0; i < 1000; i++ {
		s.Set([]byte(fmt.Sprintf("scan:key%d", i)), []byte("value"))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		s.ScanPrefix([]byte("scan:"), func(_, _ []byte) error {
			count++
			return nil
		})
	}
}
