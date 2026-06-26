package utils

import (
	"fmt"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// Tuning constants for profile gates.
const (
	logEveryNVerbose = 10
	logEveryNProfile = 100

	defaultSlowMsVerbose            = 3000
	defaultSlowMs                   = 5000
	defaultAlwaysOnSlowFileWarnMs   = 15000
)

// AlwaysOnSlowFileWarnThrottleMS is the minimum wall-clock gap between
// always-on slow-file warnings (throttle volume under burst conditions).
const AlwaysOnSlowFileWarnThrottleMS = 30000

// droppedLogLines counts logDeferredProfile calls whose underlying logger threw.
var droppedLogLines int64

// IsDeferredResolutionProfileEnabled reports whether deferred-stage
// timing / progress logs should emit.
func IsDeferredResolutionProfileEnabled() bool {
	return IsVerboseIngestionEnabled() || ParseTruthyEnv(os.Getenv("GITNEXUS_PROFILE_DEFERRED"))
}

// DeferredCallLogEveryN returns the file count interval for progress logs.
// Finer (10) when verbose; coarser (100) when profile-only.
func DeferredCallLogEveryN() int {
	if IsVerboseIngestionEnabled() {
		return logEveryNVerbose
	}
	return logEveryNProfile
}

// DeferredCallFileSlowMs returns the per-file call-resolution log threshold (ms).
// Lower default when verbose. Override via GITNEXUS_PROFILE_DEFERRED_SLOW_MS.
func DeferredCallFileSlowMs() int {
	raw := os.Getenv("GITNEXUS_PROFILE_DEFERRED_SLOW_MS")
	if raw != "" {
		n, err := strconv.Atoi(raw)
		if err == nil && n > 0 {
			return n
		}
	}
	if IsVerboseIngestionEnabled() {
		return defaultSlowMsVerbose
	}
	return defaultSlowMs
}

// AlwaysOnSlowFileWarnMs returns the always-on per-file slow threshold (ms).
// 0 / negative / non-finite → disabled. Override via GITNEXUS_SLOW_FILE_WARN_MS.
func AlwaysOnSlowFileWarnMs() int {
	raw := os.Getenv("GITNEXUS_SLOW_FILE_WARN_MS")
	if raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return 0 // disabled
		}
		return n
	}
	return defaultAlwaysOnSlowFileWarnMs
}

// ProfileNow returns a monotonic timestamp (nanoseconds since epoch).
func ProfileNow() int64 {
	return time.Now().UnixNano()
}

// ProfileElapsedMs computes elapsed milliseconds from a start timestamp.
func ProfileElapsedMs(start int64) float64 {
	return float64(time.Now().UnixNano()-start) / 1e6
}

// GetDeferredProfileDroppedCount returns the number of logDeferredProfile
// calls whose underlying logger.Info threw.
func GetDeferredProfileDroppedCount() int {
	return int(atomic.LoadInt64(&droppedLogLines))
}

// ResetDeferredProfileDroppedCount resets the dropped-line counter.
// Call from test afterEach or at processCallsFromExtracted entry.
func ResetDeferredProfileDroppedCount() {
	atomic.StoreInt64(&droppedLogLines, 0)
}

// LogDeferredProfile emits a [deferred-profile] log line.
// Counts (but does not re-throw) if the underlying logger fails.
func LogDeferredProfile(message string) {
	defer func() {
		if recovered := recover(); recovered != nil {
			// Do not call the failing logger from the handler.
			atomic.AddInt64(&droppedLogLines, 1)
		}
	}()
	// Simple stderr output — avoids importing a logger package
	fmt.Fprintf(os.Stderr, "[deferred-profile] %s\n", message)
}

// StartTimer captures a monotonic timestamp when profiling is enabled;
// otherwise returns 0 (profiling disabled). The zero sentinel is
// structurally distinct from "zero elapsed time" — callers must
// check before calling EndTimer.
func StartTimer(enabled bool) int64 {
	if enabled {
		return ProfileNow()
	}
	return 0
}

// EndTimer emits a [deferred-profile] log line for a captured timer.
// No-op when start is 0 (profiling was disabled at capture time).
// The formatter receives elapsed ms; if the formatter throws,
// a single "formatter error: …" line is logged instead.
func EndTimer(start int64, format func(elapsedMs float64) string) {
	if start == 0 {
		return
	}
	elapsedMs := ProfileElapsedMs(start)
	var message string
	defer func() {
		if recovered := recover(); recovered != nil {
			errMsg := fmt.Sprintf("%v", recovered)
			LogDeferredProfile(fmt.Sprintf("formatter error: %s", errMsg))
		}
	}()
	message = format(elapsedMs)
	LogDeferredProfile(message)
}