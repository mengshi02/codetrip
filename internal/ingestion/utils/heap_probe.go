package utils

import (
	"fmt"
	"os"
	"runtime"
)

// IsDebugHeapEnabled reports whether heap probing is active.
// Enabled when GITNEXUS_DEBUG_HEAP=1 or deferred resolution profiling is on.
func IsDebugHeapEnabled() bool {
	return ParseTruthyEnv(os.Getenv("GITNEXUS_DEBUG_HEAP")) || IsDeferredResolutionProfileEnabled()
}

// HeapUsedMB returns the current heap usage in megabytes.
func HeapUsedMB() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int(m.HeapAlloc / 1024 / 1024)
}

// RSSMB returns the current RSS (resident set size) in megabytes.
// Note: Go doesn't expose RSS directly; use Sys from MemStats as approximation.
func RSSMB() int {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return int(m.Sys / 1024 / 1024)
}

// LogHeapProbe writes a one-line heap snapshot to stderr.
// If GITNEXUS_HEAP_PROBE_FILE is set, also appends synchronously to that file
// so the line survives SIGKILL (the write(2) syscall completes before return).
func LogHeapProbe(label string, detail ...string) {
	if !IsDebugHeapEnabled() {
		return
	}
	suffix := ""
	if len(detail) > 0 && detail[0] != "" {
		suffix = " " + detail[0]
	}
	line := fmt.Sprintf("[gitnexus-heap] %s used_mb=%d rss_mb=%d%s\n",
		label, HeapUsedMB(), RSSMB(), suffix)

	// Write to stderr (async on pipe, but always attempted)
	os.Stderr.WriteString(line)

	// Synchronous file append for OOM investigation survival
	probeFile := os.Getenv("GITNEXUS_HEAP_PROBE_FILE")
	if probeFile != "" {
		f, err := os.OpenFile(probeFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			f.WriteString(line)
			f.Close()
		}
		// Best-effort diagnostics sink; never let a probe failure abort analyze.
	}
}