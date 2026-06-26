package core

// TreeSitterBufferSize is the default minimum buffer size for tree-sitter parsing (512 KB).
// tree-sitter requires bufferSize >= file size in bytes.
const TreeSitterBufferSize = 512 * 1024

// TreeSitterMaxBuffer is the maximum buffer size cap (32 MB) to prevent OOM on huge files.
// Also used as the file-size skip threshold — files larger than this are not parsed.
const TreeSitterMaxBuffer = 32 * 1024 * 1024

// GetTreeSitterContentByteLength returns the UTF-8 byte length of sourceText.
// Equivalent to TS's Buffer.byteLength(sourceText, 'utf8').
// Go strings are UTF-8 encoded; converting to []byte yields the same byte count.
func GetTreeSitterContentByteLength(sourceText string) int {
	return len([]byte(sourceText))
}

// GetTreeSitterBufferSize computes an adaptive buffer size for tree-sitter parsing.
// Uses 2x UTF-8 byte size, clamped between TreeSitterBufferSize (512KB) and TreeSitterMaxBuffer (32MB).
// Keeps tree-sitter's byte-sized buffer above large ASCII and multibyte sources.
func GetTreeSitterBufferSize(sourceText string) int {
	byteLen := GetTreeSitterContentByteLength(sourceText)
	bufSize := byteLen * 2
	if bufSize < TreeSitterBufferSize {
		bufSize = TreeSitterBufferSize
	}
	if bufSize > TreeSitterMaxBuffer {
		bufSize = TreeSitterMaxBuffer
	}
	return bufSize
}