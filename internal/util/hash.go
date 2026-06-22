package util

import (
	"crypto/sha1"
	"encoding/hex"
)

// ContentHash computes SHA1 hash of content and returns hex string
func ContentHash(data []byte) string {
	h := sha1.Sum(data)
	return hex.EncodeToString(h[:])
}

// ContentHashString computes SHA1 hash of string content
func ContentHashString(s string) string {
	return ContentHash([]byte(s))
}

// Fingerprint computes node fingerprint for incremental comparison
func Fingerprint(name, label, filePath string, props map[string]any) string {
	h := sha1.New()
	h.Write([]byte(name))
	h.Write([]byte(label))
	h.Write([]byte(filePath))
	for k, v := range props {
		h.Write([]byte(k))
		h.Write([]byte(anyToString(v)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func anyToString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	default:
		return ""
	}
}