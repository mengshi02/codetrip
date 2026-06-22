package graph

import (
	"encoding/json"

	"github.com/vmihailenco/msgpack/v5"
)

// Codec provides binary encoding/decoding for graph entities using msgpack.
// Msgpack reduces encoded size by ~40% compared to JSON and encodes/decodes
// 2-4x faster for structured data. JSON fallback is preserved for data that
// was written before the msgpack migration — Decode attempts msgpack first,
// falling back to JSON on error.
//
// Note: AdjEntry Merge operations in store.go still use JSON internally
// (json.RawMessage for merging), but the final merged result is encoded
// with msgpack when stored via this codec. The merger handles its own
// serialization internally.

// Encode marshals a value to msgpack binary format.
func Encode(v any) ([]byte, error) {
	return msgpack.Marshal(v)
}

// Decode unmarshals a value from msgpack binary format, with JSON fallback
// for data written before the msgpack migration.
func Decode(data []byte, v any) error {
	err := msgpack.Unmarshal(data, v)
	if err != nil {
		// Fallback: try JSON for legacy data
		return json.Unmarshal(data, v)
	}
	return nil
}

// DecodeMsgpack unmarshals a value strictly from msgpack (no JSON fallback).
// Used when we know the data was definitely written by msgpack.
func DecodeMsgpack(data []byte, v any) error {
	return msgpack.Unmarshal(data, v)
}

// EncodeAdjEntries encodes a slice of AdjEntry to msgpack.
// AdjEntry serialization uses msgpack for compact storage.
func EncodeAdjEntries(entries []AdjEntry) ([]byte, error) {
	return msgpack.Marshal(entries)
}

// DecodeAdjEntries decodes a slice of AdjEntry from msgpack, with JSON fallback.
func DecodeAdjEntries(data []byte) ([]AdjEntry, error) {
	var entries []AdjEntry
	err := msgpack.Unmarshal(data, &entries)
	if err != nil {
		// Fallback: try JSON for legacy data
		var jsonEntries []AdjEntry
		if jsonErr := json.Unmarshal(data, &jsonEntries); jsonErr != nil {
			return nil, jsonErr // Return JSON error (original format)
		}
		return jsonEntries, nil
	}
	return entries, nil
}

// EncodeAdjEntry encodes a single AdjEntry (for Merge operations).
// Used by appendAdjEntry to create the merge operand.
func EncodeAdjEntry(entry AdjEntry) ([]byte, error) {
	return msgpack.Marshal([]AdjEntry{entry})
}

// IsMsgpackData checks if the data starts with a msgpack format marker.
// Msgpack arrays start with 0x90-0x9f (fixarray) or 0xdc (array 16) or 0xdd (array 32).
// Msgpack maps start with 0x80-0x8f (fixmap) or 0xde (map 16) or 0xdf (map 32).
// JSON text starts with '{' (0x7b) or '[' (0x5b).
func IsMsgpackData(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	// JSON text data starts with '{' or '['
	// Msgpack binary data starts with different byte ranges
	b := data[0]
	return b != '{' && b != '[' // If not JSON text, assume msgpack
}