package util

import (
	"encoding/binary"
	"math"
)

// EncodeFloat32Vec encodes a float32 vector to binary (little-endian).
// Each float32 takes 4 bytes, so len(vec)*4 bytes are written.
// This is ~3x more compact than JSON and ~5-10x faster to encode/decode.
func EncodeFloat32Vec(vec []float32) []byte {
	buf := make([]byte, len(vec)*4)
	for i, v := range vec {
		binary.LittleEndian.PutUint32(buf[i*4:], math.Float32bits(v))
	}
	return buf
}

// DecodeFloat32Vec decodes a binary-encoded float32 vector (little-endian).
// The data length must be a multiple of 4.
func DecodeFloat32Vec(data []byte) []float32 {
	n := len(data) / 4
	vec := make([]float32, n)
	for i := range vec {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[i*4:]))
	}
	return vec
}

// EncodeStringList encodes a list of strings to binary for index storage.
// Format: uint32 count + (uint32 len + bytes data)*
// More compact and faster than JSON for string arrays.
func EncodeStringList(list []string) []byte {
	// Calculate total size
	total := 4 // count
	for _, s := range list {
		total += 4 + len(s)
	}
	buf := make([]byte, total)
	binary.LittleEndian.PutUint32(buf[0:], uint32(len(list)))
	off := 4
	for _, s := range list {
		binary.LittleEndian.PutUint32(buf[off:], uint32(len(s)))
		off += 4
		copy(buf[off:], s)
		off += len(s)
	}
	return buf
}

// DecodeStringList decodes a binary-encoded string list.
func DecodeStringList(data []byte) ([]string, error) {
	if len(data) < 4 {
		return nil, nil
	}
	count := int(binary.LittleEndian.Uint32(data[0:]))
	if count == 0 {
		return nil, nil
	}
	list := make([]string, 0, count)
	off := 4
	for i := 0; i < count; i++ {
		if off+4 > len(data) {
			return list, nil
		}
		sLen := int(binary.LittleEndian.Uint32(data[off:]))
		off += 4
		if off+sLen > len(data) {
			return list, nil
		}
		list = append(list, string(data[off:off+sLen]))
		off += sLen
	}
	return list, nil
}