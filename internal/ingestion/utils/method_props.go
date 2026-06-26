package utils

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// SkipTypeHashLanguages are languages where class overload signatures
// are declaration-only contracts that should collapse to the
// implementation body's node ID.
var SkipTypeHashLanguages = map[shared.SupportedLanguage]bool{
	shared.SupportedLanguageTypeScript: true,
	shared.SupportedLanguageJavaScript: true,
}

// ArityForIdFromInfo computes arity for ID-generation purposes.
// Returns nil when any parameter is variadic (arity is indeterminate).
func ArityForIdFromInfo(info core.MethodInfo) *int {
	for _, p := range info.Parameters {
		if p.IsVariadic {
			return nil
		}
	}
	arity := len(info.Parameters)
	return &arity
}

// BuildCollisionGroups groups methods by "name#arity".
// Call once per class, then pass to TypeTagForId/ConstTagForId
// to avoid O(N²) scans.
func BuildCollisionGroups(methodMap map[string]core.MethodInfo) map[string][]core.MethodInfo {
	groups := make(map[string][]core.MethodInfo)
	for _, info := range methodMap {
		if infoHasVariadic(info) {
			continue
		}
		key := fmt.Sprintf("%s#%d", info.Name, len(info.Parameters))
		groups[key] = append(groups[key], info)
	}
	return groups
}

// TypeTagForId computes a type-based discriminator suffix for same-arity
// overloads. Returns "~type1,type2" when collision exists, "" otherwise.
func TypeTagForId(
	methodMap map[string]core.MethodInfo,
	methodName string,
	arity *int,
	currentInfo core.MethodInfo,
	language *shared.SupportedLanguage,
	collisionGroups *map[string][]core.MethodInfo,
) string {
	if arity == nil {
		return ""
	}
	a := *arity

	// Zero-arity methods have no parameter types to disambiguate.
	if a == 0 {
		return ""
	}

	// TS/JS class overload signatures collapse to implementation body ID.
	if language != nil && SkipTypeHashLanguages[*language] {
		return ""
	}

	// Check if all parameters have types
	if len(currentInfo.Parameters) > 0 && infoHasNullType(currentInfo) {
		return ""
	}

	// Use pre-built collision group if available
	groupKey := fmt.Sprintf("%s#%d", methodName, a)
	var sameArityGroup []core.MethodInfo
	if collisionGroups != nil {
		sameArityGroup = (*collisionGroups)[groupKey]
	} else {
		sameArityGroup = buildGroup(methodMap, methodName, a)
	}

	if len(sameArityGroup) == 0 {
		sameArityGroup = buildGroup(methodMap, methodName, a)
	}

	// No collision — single method with this name+arity
	if len(sameArityGroup) < 2 {
		return ""
	}

	// Check that ALL methods in the collision group have full type info
	for _, info := range sameArityGroup {
		if len(info.Parameters) > 0 && infoHasNullType(info) {
			return ""
		}
	}

	// Build type tag from current method's parameter types.
	// Prefer rawType over type (preserves generic/template args).
	types := make([]string, len(currentInfo.Parameters))
	for i, p := range currentInfo.Parameters {
		if p.RawType != nil {
			types[i] = *p.RawType
		} else if p.Type != nil {
			types[i] = *p.Type
		} else {
			types[i] = ""
		}
	}
	return "~" + strings.Join(types, ",")
}

// ConstTagForId computes a const-qualifier suffix for C++ const/non-const
// method collisions. Returns "$const" when collision exists, "" otherwise.
func ConstTagForId(
	methodMap map[string]core.MethodInfo,
	methodName string,
	arity *int,
	currentInfo core.MethodInfo,
	collisionGroups *map[string][]core.MethodInfo,
) string {
	if !currentInfo.IsConst {
		return ""
	}
	if arity == nil {
		return ""
	}
	a := *arity

	groupKey := fmt.Sprintf("%s#%d", methodName, a)
	var candidates []core.MethodInfo
	if collisionGroups != nil {
		candidates = (*collisionGroups)[groupKey]
	} else {
		candidates = buildGroup(methodMap, methodName, a)
	}

	// Check if a non-const method exists in the collision group
	for _, info := range candidates {
		// Skip self: compare by name + parameter count + const flag
		if info.Name == currentInfo.Name &&
			len(info.Parameters) == len(currentInfo.Parameters) &&
			info.IsConst == currentInfo.IsConst {
			continue
		}
		if info.IsConst {
			continue
		}
		return "$const"
	}
	return ""
}

// ParameterShapeIdTag disambiguates function-template overloads whose
// normalized parameter types collapse to the same placeholder token
// (T, U, ...) but whose C++ sidecar shape is semantically different
// (T vs T* / T&).
func ParameterShapeIdTag(
	parameterTypes []string,
	parameterTypeClasses []shared.ParameterTypeClass,
) string {
	if len(parameterTypes) == 0 || parameterTypeClasses == nil {
		return ""
	}

	templatePlaceholderRegex := regexp.MustCompile(`^[A-Z]\w*$`)

	hasTemplatePlaceholder := false
	hasDisambiguatingShape := false
	parts := make([]string, 0, len(parameterTypes))

	for i, type_ := range parameterTypes {
		typeClass := parameterTypeClasses[i]
		if templatePlaceholderRegex.MatchString(type_) {
			hasTemplatePlaceholder = true
		}
		if typeClass.Indirection != shared.IndirectionValue ||
			typeClass.PointerDepth > 0 ||
			(typeClass.CV != shared.CVNone && typeClass.CV != shared.CVUnknown) {
			hasDisambiguatingShape = true
		}
		parts = append(parts, fmt.Sprintf("%s:%s:%s:%d",
			type_, typeClass.CV, typeClass.Indirection, typeClass.PointerDepth))
	}

	if !hasTemplatePlaceholder || !hasDisambiguatingShape {
		return ""
	}
	return "~shape:" + strings.Join(parts, "|")
}

// BuildMethodProps converts MethodInfo into flat properties for a graph node.
func BuildMethodProps(info core.MethodInfo) map[string]interface{} {
	types := make([]string, 0)
	typeClasses := make([]core.ParameterTypeClass, 0)
	var optionalCount int
	var hasVariadic bool

	for _, p := range info.Parameters {
		if p.Type != nil {
			types = append(types, *p.Type)
		}
		if p.TypeClass != nil {
			typeClasses = append(typeClasses, *p.TypeClass)
		}
		if p.IsOptional {
			optionalCount++
		}
		if p.IsVariadic {
			hasVariadic = true
		}
	}

	props := make(map[string]interface{})

	if !hasVariadic {
		props["parameterCount"] = len(info.Parameters)
		if optionalCount > 0 {
			props["requiredParameterCount"] = len(info.Parameters) - optionalCount
		}
	}

	if len(types) > 0 {
		props["parameterTypes"] = types
	}
	if len(typeClasses) == len(info.Parameters) && len(typeClasses) > 0 {
		props["parameterTypeClasses"] = typeClasses
	}

	if info.ReturnType != nil {
		props["returnType"] = *info.ReturnType
	}
	props["visibility"] = info.Visibility
	props["isStatic"] = info.IsStatic
	props["isAbstract"] = info.IsAbstract
	props["isFinal"] = info.IsFinal

	if info.IsVirtual {
		props["isVirtual"] = true
	}
	if info.IsOverride {
		props["isOverride"] = true
	}
	if info.IsAsync {
		props["isAsync"] = true
	}
	if info.IsPartial {
		props["isPartial"] = true
	}
	if info.IsConst {
		props["isConst"] = true
	}
	if info.IsDeleted {
		props["isDeleted"] = true
	}
	if len(info.Annotations) > 0 {
		props["annotations"] = info.Annotations
	}

	return props
}

// ── Internal helpers ──────────────────────────────────────────

func infoHasVariadic(info core.MethodInfo) bool {
	for _, p := range info.Parameters {
		if p.IsVariadic {
			return true
		}
	}
	return false
}

func infoHasNullType(info core.MethodInfo) bool {
	for _, p := range info.Parameters {
		if p.RawType == nil && p.Type == nil {
			return true
		}
	}
	return false
}

func buildGroup(methodMap map[string]core.MethodInfo, methodName string, arity int) []core.MethodInfo {
	var group []core.MethodInfo
	for _, info := range methodMap {
		if info.Name != methodName {
			continue
		}
		if infoHasVariadic(info) {
			continue
		}
		if len(info.Parameters) != arity {
			continue
		}
		group = append(group, info)
	}
	return group
}
