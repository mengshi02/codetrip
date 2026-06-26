package cpp

// TypeClass represents a coarse-grained type classification for C++ constraint
// evaluation (is_integral, is_floating_point, etc.).
// Maps a normalized type token to one of the categories the <type_traits>
// predicate registry uses for SFINAE filtering.
// Ported from GitNexus cpp/type-classifier.ts.
type TypeClass string

const (
	TypeClassIntegral  TypeClass = "integral"
	TypeClassFloating  TypeClass = "floating"
	TypeClassBool      TypeClass = "bool"
	TypeClassChar      TypeClass = "char"
	TypeClassString    TypeClass = "string"
	TypeClassNull      TypeClass = "null"
	TypeClassVoid      TypeClass = "void"
	TypeClassEnum      TypeClass = "enum"
	TypeClassClass     TypeClass = "class"
	TypeClassPointer   TypeClass = "pointer"
	TypeClassReference TypeClass = "reference"
	TypeClassUnknown   TypeClass = "unknown"
)

// ClassifyType classifies a normalized C++ type token into a TypeClass.
// The mapping mirrors the literal-inference table in captures.ts:inferCppLiteralType
// plus the std:: normalization in arity-metadata.ts:normalizeCppParamType.
//
// Caller note: token should be normalized for overload matching. Enum tokens
// produced by the C++ adapter use the internal `enum:<Name>` prefix so
// `is_enum_v` does not have to guess that every user token is class-like.
// Ported from GitNexus cpp/type-classifier.ts.
func ClassifyType(token string) TypeClass {
	if len(token) == 0 {
		return TypeClassUnknown
	}
	if len(token) > 5 && token[:5] == "enum:" {
		return TypeClassEnum
	}
	switch token {
	case "void":
		return TypeClassVoid
	case "int":
		return TypeClassIntegral
	case "double", "float":
		return TypeClassFloating
	case "bool":
		return TypeClassBool
	case "char":
		return TypeClassChar
	case "string":
		return TypeClassString
	case "null":
		return TypeClassNull
	default:
		// After normalization, anything that isn't a recognized primitive
		// is assumed to be a class-like type. The Tier-A predicate registry
		// doesn't introspect class types — `is_integral_v` etc. simply
		// returns `false` for `'class'`, matching ISO behavior.
		return TypeClassClass
	}
}