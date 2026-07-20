package semantic

// EmbedConfig represents embedding configuration
type EmbedConfig struct {
	ModelID          string // model identifier (default "arctic-embed-xs")
	Dimensions       int    // vector dimensions (default 384)
	BatchSize        int    // batch size (default 16)
	SubBatchSize     int    // sub-batch size (default 8)
	MaxSnippetLength int    // max characters per code snippet (default 500)
	ChunkSize        int    // chunk size (default 1200)
	Overlap          int    // overlap characters (default 120)
}

// DefaultEmbedConfig returns default embedding configuration
func DefaultEmbedConfig() EmbedConfig {
	return EmbedConfig{
		ModelID:          "arctic-embed-xs",
		Dimensions:       384,
		BatchSize:        16,
		SubBatchSize:     8,
		MaxSnippetLength: 500,
		ChunkSize:        1200,
		Overlap:          120,
	}
}
