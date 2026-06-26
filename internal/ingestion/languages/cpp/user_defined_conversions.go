package cpp

// C++ User-Defined Conversions — track converting constructors and conversion operators.
//
// C++ allows user-defined conversions via:
//   - Converting constructors: constructor that can be called with a single argument
//   - Conversion operators: operator T() member functions
//
// These conversions participate in overload resolution and are ranked by
// conversion rank. This module provides utilities for resolving user-defined
// conversion sequences.
// Ported from TS languages/cpp/user-defined-conversions.ts.

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CppConversionKind discriminates the two kinds of user-defined conversions.
type CppConversionKind int

const (
	ConversionConstructor CppConversionKind = iota // converting constructor
	ConversionOperator                              // operator T()
)

// CppUserDefinedConversion represents a user-defined conversion path.
type CppUserDefinedConversion struct {
	Kind       CppConversionKind
	SourceType string // the type being converted from
	TargetType string // the type being converted to
	DefNodeID  string // the definition node ID of the constructor/operator
}

// FindCppUserDefinedConversions finds user-defined conversion paths
// from a source type to a target type.
// TODO: full implementation — walk conversion registry.
func FindCppUserDefinedConversions(sourceType string, targetType string) []CppUserDefinedConversion {
	return nil
}

// IsCppConvertingConstructor checks whether a constructor is a converting constructor
// (can be called with a single argument and is not marked explicit).
// TODO: full implementation
func IsCppConvertingConstructor(def shared.SymbolDefinition) bool {
	return false
}

// IsCppConversionOperator checks whether a method is a conversion operator.
// TODO: full implementation
func IsCppConversionOperator(def shared.SymbolDefinition) bool {
	return false
}