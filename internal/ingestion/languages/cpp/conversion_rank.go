package cpp

// C++ Conversion Rank — rank user-defined conversion sequences.
//
// C++ overload resolution ranks candidate functions by the conversion
// sequences required to match argument types to parameter types.
// This module computes the conversion rank for each candidate.
// Ported from TS languages/cpp/conversion-rank.ts.

import (
	"sync"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ConversionRank represents the rank of a conversion sequence.
type ConversionRank int

const (
	RankExactMatch     ConversionRank = 0 // no conversion needed
	RankPromotion      ConversionRank = 1 // integral promotion (e.g. int → long)
	RankStandardConv   ConversionRank = 2 // standard conversion (e.g. int → double)
	RankUserDefined    ConversionRank = 3 // user-defined conversion (constructor/operator)
	RankEllipsis       ConversionRank = 4 // ... match
	RankIncompatible   ConversionRank = 5 // no viable conversion
)

// conversionMutex guards user-defined conversion state.
var conversionMutex sync.RWMutex

// userDefinedConversions maps type NodeIDs to their conversion operators/constructors.
var userDefinedConversions map[string][]string // typeNodeID → []conversionDefNodeID

func init() {
	userDefinedConversions = make(map[string][]string)
}

// ClearCppUserDefinedConversions resets user-defined conversion state.
func ClearCppUserDefinedConversions() {
	conversionMutex.Lock()
	defer conversionMutex.Unlock()
	userDefinedConversions = make(map[string][]string)
}

// ComputeCppConversionRank computes the conversion rank between an argument type
// and a parameter type.
// TODO: full implementation
func ComputeCppConversionRank(arg shared.ParameterTypeClass, param shared.ParameterTypeClass) ConversionRank {
	if arg.Base == param.Base && arg.Indirection == param.Indirection && arg.PointerDepth == param.PointerDepth {
		return RankExactMatch
	}
	return RankIncompatible
}

// RegisterCppUserDefinedConversion registers a user-defined conversion.
// TODO: full implementation
func RegisterCppUserDefinedConversion(typeNodeID string, conversionDefNodeID string) {
	conversionMutex.Lock()
	defer conversionMutex.Unlock()
	userDefinedConversions[typeNodeID] = append(userDefinedConversions[typeNodeID], conversionDefNodeID)
}