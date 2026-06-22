package util

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync/atomic"
	"time"
)

var counter atomic.Uint64

// GenerateID generates unique node/edge ID, format: {repo}:{label}:{name}:{shortHash}
// High performance: atomic counter + timestamp, no lock needed
func GenerateID(repo, label, name string) string {
	c := counter.Add(1)
	t := time.Now().UnixNano()
	short := hex.EncodeToString(append(
		make([]byte, 0, 8),
		byte(t>>56), byte(t>>48), byte(t>>40), byte(t>>32),
		byte(c>>24), byte(c>>16), byte(c>>8), byte(c),
	))
	var b strings.Builder
	b.Grow(len(repo) + 1 + len(label) + 1 + len(name) + 1 + 8)
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(label)
	b.WriteByte(':')
	b.WriteString(name)
	b.WriteByte(':')
	b.WriteString(short[:8])
	return b.String()
}

// GenerateEdgeID generates unique edge ID, format: e:{repo}:{src}:{type}:{target}:{shortHash}
func GenerateEdgeID(repo, src, relType, target string) string {
	c := counter.Add(1)
	t := time.Now().UnixNano()
	short := hex.EncodeToString(append(
		make([]byte, 0, 8),
		byte(t>>56), byte(t>>48), byte(t>>40), byte(t>>32),
		byte(c>>24), byte(c>>16), byte(c>>8), byte(c),
	))
	var b strings.Builder
	b.Grow(1 + len(repo) + 1 + len(src) + 1 + len(relType) + 1 + len(target) + 1 + 8)
	b.WriteByte('e')
	b.WriteByte(':')
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(src)
	b.WriteByte(':')
	b.WriteString(relType)
	b.WriteByte(':')
	b.WriteString(target)
	b.WriteByte(':')
	b.WriteString(short[:8])
	return b.String()
}

// GenerateRandomID generates random ID (16 bytes hex)
func GenerateRandomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// NodeUID generates deterministic node UID: {repo}:{filePath}:{label}:{name}
// Used for unique identification across file references
func NodeUID(repo, filePath, label, name string) string {
	var b strings.Builder
	b.Grow(len(repo) + 1 + len(filePath) + 1 + len(label) + 1 + len(name))
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(filePath)
	b.WriteByte(':')
	b.WriteString(label)
	b.WriteByte(':')
	b.WriteString(name)
	return b.String()
}
