// Package shared — scope ID construction and interning.
// Ported from gitnexus-shared scope-resolution/scope-id.ts.
package shared

import (
	"fmt"
	"sync"
)

// scopeInternPool — string interning pool for ScopeID identity-fast equality.
// Mirrors TS's implicit string interning via makeScopeId returning interned values.
var scopeInternPool sync.Pool

func init() {
	scopeInternPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]string)
		},
	}
}

// internMap returns the per-goroutine intern map from the pool.
// For simplicity and thread-safety, we use a global mutex-protected map instead.
var (
	internMap   = make(map[string]string)
	internMutex sync.Mutex
)

// MakeScopeId constructs a deterministic ScopeID from file path, range, and kind.
// Format: scope:{filePath}#{startLine}:{startCol}-{endLine}:{endCol}:{kind}
// The result is interned for identity-fast equality checks.
func MakeScopeID(filePath string, r Range, kind ScopeKind) ScopeID {
	raw := fmt.Sprintf("scope:%s#%d:%d-%d:%d:%s", filePath, r.StartLine, r.StartCol, r.EndLine, r.EndCol, kind)
	return internString(raw)
}

// internString returns an interned copy of s, ensuring identity comparison works.
func internString(s string) string {
	internMutex.Lock()
	defer internMutex.Unlock()
	if existing, ok := internMap[s]; ok {
		return existing
	}
	// Store a copy to avoid retaining the backing array of a mutable builder
	internMap[s] = s
	return s
}

// ParseScopeID extracts the file path and kind from a ScopeID string.
// Returns empty strings if the format is invalid.
func ParseScopeID(id ScopeID) (filePath string, kind ScopeKind) {
	// Expected format: scope:{filePath}#{startLine}:{startCol}-{endLine}:{endCol}:{kind}
	if len(id) < 6 || id[:6] != "scope:" {
		return "", ""
	}
	rest := id[6:]
	// Find the last colon for kind
	lastColon := -1
	for i := len(rest) - 1; i >= 0; i-- {
		if rest[i] == ':' {
			lastColon = i
			break
		}
	}
	if lastColon < 0 {
		return "", ""
	}
	kind = ScopeKind(rest[lastColon+1:])
	// Find the # separator for filePath
	for i := 0; i < lastColon; i++ {
		if rest[i] == '#' {
			filePath = rest[:i]
			return
		}
	}
	return "", kind
}