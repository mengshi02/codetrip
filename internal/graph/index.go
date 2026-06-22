package graph

// IndexType represents index types
type IndexType int

const (
	IndexByName  IndexType = iota // Name index
	IndexByLabel                  // Label index
	IndexByFile                   // File path index
	IndexByUID                    // UID index
)

// IndexStats represents index statistics
type IndexStats struct {
	NameCount  int
	LabelCount int
	FileCount  int
	UIDCount   int
}

// GetIndexStats retrieves index statistics
func (s *GraphStore) GetIndexStats(repo string) (*IndexStats, error) {
	stats := &IndexStats{}

	// Count name index
	_ = s.store.ScanPrefix(NameRepoPrefix(repo), func(_, _ []byte) error {
		stats.NameCount++
		return nil
	})

	// Count label index
	_ = s.store.ScanPrefix(TypeRepoPrefix(repo), func(_, _ []byte) error {
		stats.LabelCount++
		return nil
	})

	// Count file index
	_ = s.store.ScanPrefix(FileRepoPrefix(repo), func(_, _ []byte) error {
		stats.FileCount++
		return nil
	})

	return stats, nil
}
