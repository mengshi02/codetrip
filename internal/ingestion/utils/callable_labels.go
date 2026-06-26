package utils

// CallableOverloadableLabels contains node labels that represent callable
// symbols which can have multiple overloads (same name, different signatures).
var CallableOverloadableLabels = map[string]bool{
	"Method":   true,
	"Function": true,
}

// IsOverloadableCallable reports whether the given label represents a callable
// that can appear in multiple overload signatures.
func IsOverloadableCallable(label string) bool {
	return CallableOverloadableLabels[label]
}