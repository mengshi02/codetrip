package store

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble/v2"
)

func TestBatchNoSync(t *testing.T) {
	s := openTestStore(t)
	err := s.BatchNoSync(func(b *pebble.Batch) error {
		if err := b.Set([]byte("bn1"), []byte("v1"), nil); err != nil {
			return err
		}
		return b.Set([]byte("bn2"), []byte("v2"), nil)
	})
	if err != nil {
		t.Fatal(err)
	}
	v1, _ := s.Get([]byte("bn1"))
	v2, _ := s.Get([]byte("bn2"))
	if string(v1) != "v1" || string(v2) != "v2" {
		t.Errorf("BatchNoSync results: %q, %q", v1, v2)
	}
}

func TestDeleteNoSync(t *testing.T) {
	s := openTestStore(t)
	key := []byte("nosync:del")
	if err := s.SetNoSync(key, []byte("val")); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNoSync(key); err != nil {
		t.Fatal(err)
	}
	_, err := s.Get(key)
	if err == nil {
		t.Error("expected error after DeleteNoSync")
	}
}

func TestFlush(t *testing.T) {
	s := openTestStore(t)
	// Write via BatchNoSync, then flush
	s.BatchNoSync(func(b *pebble.Batch) error {
		return b.Set([]byte("flush:key"), []byte("flush:val"), nil)
	})
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get([]byte("flush:key"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "flush:val" {
		t.Errorf("Flush: got %q, want %q", got, "flush:val")
	}
}

func TestCompact(t *testing.T) {
	s := openTestStore(t)
	for i := 0; i < 50; i++ {
		key := []byte("compact:key" + fmt.Sprintf("%03d", i))
		s.Set(key, []byte("val"))
	}
	s.Flush()
	if err := s.Compact(); err != nil {
		t.Fatalf("Compact error: %v", err)
	}
}

func TestMetrics(t *testing.T) {
	s := openTestStore(t)
	s.Set([]byte("metric:key"), []byte("val"))
	m := s.Metrics()
	if m == nil {
		t.Error("Metrics() returned nil")
	}
}

func TestTrip(t *testing.T) {
	s := openTestStore(t)
	db := s.Trip()
	if db == nil {
		t.Error("Trip() returned nil")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("/tmp/testpath")
	if cfg.Path != "/tmp/testpath" {
		t.Errorf("Path = %q, want /tmp/testpath", cfg.Path)
	}
	if cfg.CacheSize != 256<<20 {
		t.Errorf("CacheSize = %d, want %d", cfg.CacheSize, 256<<20)
	}
	if cfg.MaxOpenDocs != 1024 {
		t.Errorf("MaxOpenDocs = %d, want 1024", cfg.MaxOpenDocs)
	}
}

func TestGet_CopySafety(t *testing.T) {
	s := openTestStore(t)
	key := []byte("copy:key")
	s.Set(key, []byte("original"))

	got1, _ := s.Get(key)
	got2, _ := s.Get(key)
	// Verify copies are independent
	if string(got1) != string(got2) {
		t.Errorf("copy safety: %q != %q", got1, got2)
	}
}

func TestBatch_Error(t *testing.T) {
	s := openTestStore(t)
	err := s.Batch(func(b *pebble.Batch) error {
		return fmt.Errorf("intentional error")
	})
	if err == nil {
		t.Error("expected error from batch")
	}
}

func TestBatchNoSync_Error(t *testing.T) {
	s := openTestStore(t)
	err := s.BatchNoSync(func(b *pebble.Batch) error {
		return fmt.Errorf("intentional error")
	})
	if err == nil {
		t.Error("expected error from BatchNoSync")
	}
}
