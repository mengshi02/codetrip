package utils

import (
	"fmt"
	"strings"
)

// MarkerKind distinguishes heritage vs property synthetic import markers.
type MarkerKind string

const (
	MarkerKindHeritage MarkerKind = "heritage"
	MarkerKindProperty MarkerKind = "property"
)

// PrefixByKind maps each marker kind to its wire-format prefix.
var PrefixByKind = map[MarkerKind]string{
	MarkerKindHeritage: "__heritage__:",
	MarkerKindProperty: "__property__:",
}

// HeritageMarkerPrefix is the constant prefix for heritage markers.
const HeritageMarkerPrefix = "__heritage__:"

// PropertyMarkerPrefix is the constant prefix for property markers.
const PropertyMarkerPrefix = "__property__:"

// DecodedMarker holds the result of decoding a marker string.
type DecodedMarker struct {
	Kind   MarkerKind
	Fields []string
}

// EncodeMarker builds a marker string "<prefix><field>:<field>:...".
// The ':' delimiter IS the wire format, so a field containing ':' is
// structurally invalid and causes a panic — callers must pre-normalize
// colon-bearing values (e.g. Outer::Mixin → Outer.Mixin).
func EncodeMarker(kind MarkerKind, fields []string) string {
	for _, field := range fields {
		if strings.Contains(field, ":") {
			panic(fmt.Sprintf(
				"EncodeMarker: field \"%s\" contains the ':' delimiter; normalize it before encoding",
				field))
		}
	}
	return PrefixByKind[kind] + strings.Join(fields, ":")
}

// DecodeMarker parses a marker string back into its kind + positional fields.
// Returns nil if raw is not a marker string.
func DecodeMarker(raw string) *DecodedMarker {
	if strings.HasPrefix(raw, PrefixByKind[MarkerKindHeritage]) {
		prefix := PrefixByKind[MarkerKindHeritage]
		fields := strings.Split(raw[len(prefix):], ":")
		return &DecodedMarker{Kind: MarkerKindHeritage, Fields: fields}
	}
	if strings.HasPrefix(raw, PrefixByKind[MarkerKindProperty]) {
		prefix := PrefixByKind[MarkerKindProperty]
		fields := strings.Split(raw[len(prefix):], ":")
		return &DecodedMarker{Kind: MarkerKindProperty, Fields: fields}
	}
	return nil
}

// IsHeritageMarker reports whether raw is a synthetic heritage/property marker.
func IsHeritageMarker(raw string) bool {
	return strings.HasPrefix(raw, HeritageMarkerPrefix) ||
		strings.HasPrefix(raw, PropertyMarkerPrefix)
}