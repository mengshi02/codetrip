//go:build windows

package hnsw

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
)

// Save writes the graph to the file.
//
// On Windows, atomic file replacement via rename is not guaranteed
// (see https://github.com/google/renameio/issues/1), so this implementation
// uses a non-atomic write-then-rename approach as a best-effort fallback.
func (g *SavedGraph[K]) Save() error {
	dir := filepath.Dir(g.Path)
	tmp, err := os.CreateTemp(dir, ".hnsw-tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()

	wr := bufio.NewWriter(tmp)
	if err = g.Export(wr); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("exporting: %w", err)
	}

	if err = wr.Flush(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("flushing: %w", err)
	}

	if err = tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("syncing: %w", err)
	}

	if err = tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing: %w", err)
	}

	return os.Rename(tmpPath, g.Path)
}