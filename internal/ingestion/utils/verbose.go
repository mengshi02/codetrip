package utils

import "os"

// IsVerboseIngestionEnabled reports whether verbose ingestion logging is active.
// Enabled when GITNEXUS_VERBOSE=1.
func IsVerboseIngestionEnabled() bool {
	return os.Getenv("GITNEXUS_VERBOSE") == "1"
}