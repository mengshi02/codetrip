package graph

import (
	"fmt"
	"strings"
)

// Key encoding helpers — zero-alloc string concatenation replacing fmt.Sprintf.
// fmt.Sprintf uses reflect + io.Writer internally; strings.Builder with Grow
// pre-allocates once and appends directly, ~5-10x faster for key construction.

// --- Node keys ---

// nodeKey builds "n:{repo}:{id}"
func nodeKey(repo, id string) string {
	var b strings.Builder
	b.Grow(2 + len(repo) + 1 + len(id))
	b.WriteString("n:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(id)
	return b.String()
}

// nodePrefix builds "n:{repo}:"
func nodePrefix(repo string) []byte {
	var b strings.Builder
	b.Grow(2 + len(repo) + 1)
	b.WriteString("n:")
	b.WriteString(repo)
	b.WriteByte(':')
	return []byte(b.String())
}

// --- Edge keys ---

// edgeKey builds "e:{id}"
func edgeKey(id string) string {
	var b strings.Builder
	b.Grow(1 + len(id))
	b.WriteByte('e')
	b.WriteByte(':')
	b.WriteString(id)
	return b.String()
}

// --- Index keys ---

// typeKey builds "type:{repo}:{label}:{id}"
func typeKey(repo, label, id string) string {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1 + len(label) + 1 + len(id))
	b.WriteString("type:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(label)
	b.WriteByte(':')
	b.WriteString(id)
	return b.String()
}

// typePrefix builds "type:{repo}:{label}:"
func typePrefix(repo, label string) []byte {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1 + len(label) + 1)
	b.WriteString("type:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(label)
	b.WriteByte(':')
	return []byte(b.String())
}

// TypeRepoPrefix builds "type:{repo}:" — exported for use by Trip.Verify index consistency checks.
func TypeRepoPrefix(repo string) []byte {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1)
	b.WriteString("type:")
	b.WriteString(repo)
	b.WriteByte(':')
	return []byte(b.String())
}

// nameKey builds "name:{repo}:{name}:{id}"
func nameKey(repo, name, id string) string {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1 + len(name) + 1 + len(id))
	b.WriteString("name:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(name)
	b.WriteByte(':')
	b.WriteString(id)
	return b.String()
}

// namePrefix builds "name:{repo}:{name}:"
func namePrefix(repo, name string) []byte {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1 + len(name) + 1)
	b.WriteString("name:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(name)
	b.WriteByte(':')
	return []byte(b.String())
}

// NameRepoPrefix builds "name:{repo}:" — exported for use by Trip.Verify index consistency checks.
func NameRepoPrefix(repo string) []byte {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1)
	b.WriteString("name:")
	b.WriteString(repo)
	b.WriteByte(':')
	return []byte(b.String())
}

// fileKey builds "file:{repo}:{filePath}:{id}"
func fileKey(repo, filePath, id string) string {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1 + len(filePath) + 1 + len(id))
	b.WriteString("file:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(filePath)
	b.WriteByte(':')
	b.WriteString(id)
	return b.String()
}

// filePrefix builds "file:{repo}:{filePath}:"
func filePrefix(repo, filePath string) []byte {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1 + len(filePath) + 1)
	b.WriteString("file:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(filePath)
	b.WriteByte(':')
	return []byte(b.String())
}

// FileRepoPrefix builds "file:{repo}:" — exported for use by Trip.Verify index consistency checks.
func FileRepoPrefix(repo string) []byte {
	var b strings.Builder
	b.Grow(5 + len(repo) + 1)
	b.WriteString("file:")
	b.WriteString(repo)
	b.WriteByte(':')
	return []byte(b.String())
}

// --- Adjacency keys ---

// adjKey builds "adj:{repo}:{nodeID}:{dir}:{relType}"
func adjKey(repo, nodeID, dir, relType string) string {
	var b strings.Builder
	b.Grow(4 + len(repo) + 1 + len(nodeID) + 1 + len(dir) + 1 + len(relType))
	b.WriteString("adj:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(nodeID)
	b.WriteByte(':')
	b.WriteString(dir)
	b.WriteByte(':')
	b.WriteString(relType)
	return b.String()
}

// adjPrefix builds "adj:{repo}:{nodeID}:{dir}:"
func adjPrefix(repo, nodeID, dir string) []byte {
	var b strings.Builder
	b.Grow(4 + len(repo) + 1 + len(nodeID) + 1 + len(dir) + 1)
	b.WriteString("adj:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(nodeID)
	b.WriteByte(':')
	b.WriteString(dir)
	b.WriteByte(':')
	return []byte(b.String())
}

// adjScanPrefix builds "adj:{repo}:{nodeID}:{dir}:" as string (for ScanPrefix with string key parsing)
func adjScanPrefix(repo, nodeID, dir string) string {
	var b strings.Builder
	b.Grow(4 + len(repo) + 1 + len(nodeID) + 1 + len(dir) + 1)
	b.WriteString("adj:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(nodeID)
	b.WriteByte(':')
	b.WriteString(dir)
	b.WriteByte(':')
	return b.String()
}

// AdjRepoPrefix builds "adj:{repo}:" — exported for use by Trip.Verify index consistency checks.
func AdjRepoPrefix(repo string) []byte {
	var b strings.Builder
	b.Grow(4 + len(repo) + 1)
	b.WriteString("adj:")
	b.WriteString(repo)
	b.WriteByte(':')
	return []byte(b.String())
}

// --- Embedding keys ---

// EmbHashKey builds "embhash:{repo}:{nodeID}" — content hash for embedding incremental check
func EmbHashKey(repo, nodeID string) string {
	var b strings.Builder
	b.Grow(8 + len(repo) + 1 + len(nodeID))
	b.WriteString("embhash:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(nodeID)
	return b.String()
}

// EmbHashRepoPrefix builds "embhash:{repo}:" — scans all embedding hash keys for a repo
func EmbHashRepoPrefix(repo string) []byte {
	var b strings.Builder
	b.Grow(8 + len(repo) + 1)
	b.WriteString("embhash:")
	b.WriteString(repo)
	b.WriteByte(':')
	return []byte(b.String())
}

// --- Dual-Modal Embedding keys ---

// EmbDescKey builds "embdesc:{repo}:{nodeID}" — description modality vector key.
// Description modality captures symbol signature + relationship summary.
func EmbDescKey(repo, nodeID string) string {
	var b strings.Builder
	b.Grow(8 + len(repo) + 1 + len(nodeID))
	b.WriteString("embdesc:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(nodeID)
	return b.String()
}

// EmbDescChunkKey builds "embdesc:{repo}:{nodeID}:{chunkIdx}" — description chunk vector key.
func EmbDescChunkKey(repo, nodeID string, chunkIdx int) string {
	idxStr := fmt.Sprintf("%d", chunkIdx)
	var b strings.Builder
	b.Grow(8 + len(repo) + 1 + len(nodeID) + 1 + len(idxStr))
	b.WriteString("embdesc:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(nodeID)
	b.WriteByte(':')
	b.WriteString(idxStr)
	return b.String()
}

// EmbDescPrefix builds "embdesc:{repo}:" — prefix for scanning all description vectors.
func EmbDescPrefix(repo string) []byte {
	var b strings.Builder
	b.Grow(8 + len(repo) + 1)
	b.WriteString("embdesc:")
	b.WriteString(repo)
	b.WriteByte(':')
	return []byte(b.String())
}

// EmbCodeKey builds "embcode:{repo}:{nodeID}" — code modality vector key.
// Code modality captures source code snippet chunking.
func EmbCodeKey(repo, nodeID string) string {
	var b strings.Builder
	b.Grow(8 + len(repo) + 1 + len(nodeID))
	b.WriteString("embcode:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(nodeID)
	return b.String()
}

// EmbCodeChunkKey builds "embcode:{repo}:{nodeID}:{chunkIdx}" — code chunk vector key.
func EmbCodeChunkKey(repo, nodeID string, chunkIdx int) string {
	idxStr := fmt.Sprintf("%d", chunkIdx)
	var b strings.Builder
	b.Grow(8 + len(repo) + 1 + len(nodeID) + 1 + len(idxStr))
	b.WriteString("embcode:")
	b.WriteString(repo)
	b.WriteByte(':')
	b.WriteString(nodeID)
	b.WriteByte(':')
	b.WriteString(idxStr)
	return b.String()
}

// EmbCodePrefix builds "embcode:{repo}:" — prefix for scanning all code vectors.
func EmbCodePrefix(repo string) []byte {
	var b strings.Builder
	b.Grow(8 + len(repo) + 1)
	b.WriteString("embcode:")
	b.WriteString(repo)
	b.WriteByte(':')
	return []byte(b.String())
}

// --- Dual-Modal Embedding Index keys ---

// EmbDescIdxKey builds "embdescidx:{repo}" — index of nodeIDs with description vectors.
// Used for building the description-modality HNSW index.
func EmbDescIdxKey(repo string) string {
	var b strings.Builder
	b.Grow(12 + len(repo))
	b.WriteString("embdescidx:")
	b.WriteString(repo)
	return b.String()
}

// EmbCodeIdxKey builds "embcodeidx:{repo}" — index of nodeIDs with code vectors.
// Used for building the code-modality HNSW index.
func EmbCodeIdxKey(repo string) string {
	var b strings.Builder
	b.Grow(12 + len(repo))
	b.WriteString("embcodeidx:")
	b.WriteString(repo)
	return b.String()
}