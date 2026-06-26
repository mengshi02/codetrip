package utils

import (
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// DefaultMaxFileSizeBytes is the default threshold (512 KB).
// Files larger than this are skipped by the walker.
const DefaultMaxFileSizeBytes = 512 * 1024

// MaxFileSizeUpperBoundBytes is the hard upper bound — tree-sitter refuses
// buffers above this regardless. Imported from constants.go.
var MaxFileSizeUpperBoundBytes = core.TreeSitterMaxBuffer

// warnOnceCache tracks warnings already emitted (one-time only).
var warnOnceCache struct {
	mu   sync.Mutex
	seen map[string]bool
}

func init() {
	warnOnceCache.seen = make(map[string]bool)
}

// warnOnce emits a warning message exactly once per key.
// Uses stderr output since the project logger is not yet available.
func warnOnce(key string, message string) {
	warnOnceCache.mu.Lock()
	defer warnOnceCache.mu.Unlock()
	if warnOnceCache.seen[key] {
		return
	}
	warnOnceCache.seen[key] = true
	fmt.Fprintf(os.Stderr, "[gitnexus-warn] %s\n", message)
}

// GetMaxFileSizeBytes resolves the effective file-size skip threshold (bytes).
// Reads GITNEXUS_MAX_FILE_SIZE (KB). Invalid values fall back to the default
// and emit a one-time warning. Values above the tree-sitter ceiling are clamped.
func GetMaxFileSizeBytes() int {
	raw := os.Getenv("GITNEXUS_MAX_FILE_SIZE")
	if raw == "" {
		return DefaultMaxFileSizeBytes
	}

	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed <= 0 {
		warnOnce(
			fmt.Sprintf("invalid:%s", raw),
			fmt.Sprintf("  GITNEXUS_MAX_FILE_SIZE must be a positive integer (KB), got \"%s\" — using default %dKB",
				raw, DefaultMaxFileSizeBytes/1024),
		)
		return DefaultMaxFileSizeBytes
	}

	bytes := parsed * 1024
	if bytes > MaxFileSizeUpperBoundBytes {
		warnOnce(
			fmt.Sprintf("clamp:%s", raw),
			fmt.Sprintf("  GITNEXUS_MAX_FILE_SIZE=%dKB exceeds tree-sitter ceiling (%dKB) — clamping",
				parsed, MaxFileSizeUpperBoundBytes/1024),
		)
		return MaxFileSizeUpperBoundBytes
	}
	return bytes
}

// GetMaxFileSizeBannerMessage builds the CLI banner message announcing an active
// file-size override. Returns nil when the effective threshold equals the default.
func GetMaxFileSizeBannerMessage() *string {
	effectiveBytes := GetMaxFileSizeBytes()
	if effectiveBytes == DefaultMaxFileSizeBytes {
		return nil
	}
	effectiveKb := effectiveBytes / 1024
	defaultKb := DefaultMaxFileSizeBytes / 1024
	msg := fmt.Sprintf("  GITNEXUS_MAX_FILE_SIZE: effective threshold %dKB (default %dKB)",
		effectiveKb, defaultKb)
	return &msg
}

// ResetMaxFileSizeWarnings clears the warn-once cache (test-only).
func ResetMaxFileSizeWarnings() {
	warnOnceCache.mu.Lock()
	warnOnceCache.seen = make(map[string]bool)
	warnOnceCache.mu.Unlock()
}