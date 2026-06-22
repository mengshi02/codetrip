//go:build !windows

package hnsw

import (
	"bufio"
	"fmt"

	"github.com/google/renameio"
)

// Save writes the graph to the file atomically using renameio.
func (g *SavedGraph[K]) Save() error {
	tmp, err := renameio.TempFile("", g.Path)
	if err != nil {
		return err
	}
	defer tmp.Cleanup()

	wr := bufio.NewWriter(tmp)
	err = g.Export(wr)
	if err != nil {
		return fmt.Errorf("exporting: %w", err)
	}

	err = wr.Flush()
	if err != nil {
		return fmt.Errorf("flushing: %w", err)
	}

	err = tmp.CloseAtomicallyReplace()
	if err != nil {
		return fmt.Errorf("closing atomically: %w", err)
	}

	return nil
}