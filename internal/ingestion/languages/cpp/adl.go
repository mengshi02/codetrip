package cpp

// C++ Argument-Dependent Lookup (ADL / Koenig lookup).
//
// When ordinary unqualified lookup fails for a free-call site, ADL also
// considers candidates declared in the associated namespaces of the
// call's argument types (ISO C++ [basic.lookup.argdep]).
//
// Current boundary:
//   - Class-typed arguments (value, pointer, reference)
//   - Template specializations with explicit type arguments
//   - Function-reference arguments (parameter/return type namespaces)
//   - Parenthesized-name suppression: (f)(s) does NOT trigger ADL
//
// State lifecycle — five pieces of module-level state, reset by ClearCppAdlState:
//   - argInfoBySite — per-call-site argument shape (capture-time)
//   - noAdlSites — call sites with parenthesized function (capture-time)
//   - classToNamespaceQName — class def → enclosing namespace qualified name
//   - adlIndex / adlIndexSource — lazily-built candidate index
//   - Per-file site-key indexes (lockstep with maps)
//
// Ported from TS languages/cpp/adl.ts.

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ────────────────────────────────────────────────────────────────────────────
// CppAdlArgInfo — per-argument shape information collected at capture time
// ────────────────────────────────────────────────────────────────────────────

// CppAdlArgInfo describes one ADL argument's type info for ADL namespace lookup.
type CppAdlArgInfo struct {
	SimpleClassName         string   // e.g. "Event" — last segment of qualified name; empty for primitives
	TemplateSimpleClassName string   // e.g. "vector" for std::vector<N::T>
	TemplateNamespace       string   // e.g. "std" — template's enclosing namespace
	TemplateArgClassNames   []string // class-like names from template type arguments
	TemplateArgNamespaces   []string // enclosing namespaces from template type arguments
	FunctionRefText         string   // set when arg is a potential free-function reference
}

// ────────────────────────────────────────────────────────────────────────────
// Module-level state
// ────────────────────────────────────────────────────────────────────────────

var adlMutex sync.RWMutex

// argInfoBySite maps "filePath:line:col" → []CppAdlArgInfo
var argInfoBySite map[string][]CppAdlArgInfo

// noAdlSites records call sites with parenthesized function (ADL suppressed).
var noAdlSites map[string]bool

// classToNamespaceQName maps class def NodeID → enclosing namespace qualified name (dot-joined).
var classToNamespaceQName map[string]string

// Per-file site-key indexes (lockstep with argInfoBySite / noAdlSites).
var argInfoSiteKeysByFile map[string][]string
var noAdlSiteKeysByFile map[string][]string

// ADL candidate index — lazily built, reused for every call site in a pipeline run.
var adlIndexState *AdlCandidateIndex
var adlIndexSourceRef []*shared.ParsedFile

func init() {
	argInfoBySite = make(map[string][]CppAdlArgInfo)
	noAdlSites = make(map[string]bool)
	classToNamespaceQName = make(map[string]string)
	argInfoSiteKeysByFile = make(map[string][]string)
	noAdlSiteKeysByFile = make(map[string][]string)
}

// ────────────────────────────────────────────────────────────────────────────
// AdlCandidateIndex — workspace-wide candidate index built once per pipeline
// ────────────────────────────────────────────────────────────────────────────

// AdlCandidateIndex is the prebuilt ADL candidate index for fast per-site lookup.
type AdlCandidateIndex struct {
	// ClassDefsBySimple maps simple name → class-like defs (Class/Struct/Interface/Enum)
	ClassDefsBySimple map[string][]shared.SymbolDefinition
	// NsCandidates maps namespace QName → simple name → callable defs
	NsCandidates map[string]map[string][]shared.SymbolDefinition
	// FriendCandidates maps associated-class enclosing-ns QName → simple name → hidden-friend + class-member callable defs
	FriendCandidates map[string]map[string][]shared.SymbolDefinition
	// NsFunctionsByQName maps namespace QName → simple name → Function/Method defs (for qualified function-ref ADL)
	NsFunctionsByQName map[string]map[string][]shared.SymbolDefinition
	// NsFunctionsBySimple maps simple name → Function/Method defs across all namespaces (for unqualified function-ref ADL)
	NsFunctionsBySimple map[string][]shared.SymbolDefinition
	// SeqByNodeID maps NodeID → visitation sequence number for deterministic candidate ordering
	SeqByNodeID map[string]int
}

// ────────────────────────────────────────────────────────────────────────────
// Site key helpers
// ────────────────────────────────────────────────────────────────────────────

func adlSiteKey(filePath string, line int, col int) string {
	return fmt.Sprintf("%s:%d:%d", filePath, line, col)
}

var siteKeyRE = regexp.MustCompile(`^(.*):(\d+):(\d+)$`)

func parseAdlSiteKey(key string) (filePath string, line int, col int, ok bool) {
	m := siteKeyRE.FindStringSubmatch(key)
	if m == nil {
		return "", 0, 0, false
	}
	ln, err1 := strconv.Atoi(m[2])
	cc, err2 := strconv.Atoi(m[3])
	if err1 != nil || err2 != nil {
		return "", 0, 0, false
	}
	return m[1], ln, cc, true
}

func pushFileSiteKey(idx map[string][]string, filePath string, key string) {
	idx[filePath] = append(idx[filePath], key)
}

// ────────────────────────────────────────────────────────────────────────────
// Mark / record functions (called from captures.ts)
// ────────────────────────────────────────────────────────────────────────────

// MarkCppAdlSiteArgs records per-call-site argument info for ADL.
func MarkCppAdlSiteArgs(filePath string, line int, col int, args []CppAdlArgInfo) {
	adlMutex.Lock()
	defer adlMutex.Unlock()
	key := adlSiteKey(filePath, line, col)
	if _, exists := argInfoBySite[key]; !exists {
		pushFileSiteKey(argInfoSiteKeysByFile, filePath, key)
	}
	argInfoBySite[key] = args
}

// MarkCppAdlSiteNoAdl marks a call site as ADL-suppressed (parenthesized function).
func MarkCppAdlSiteNoAdl(filePath string, line int, col int) {
	adlMutex.Lock()
	defer adlMutex.Unlock()
	key := adlSiteKey(filePath, line, col)
	if !noAdlSites[key] {
		pushFileSiteKey(noAdlSiteKeysByFile, filePath, key)
	}
	noAdlSites[key] = true
}

// ────────────────────────────────────────────────────────────────────────────
// Side-channel serialization (worker → main boundary)
// ────────────────────────────────────────────────────────────────────────────

// CppAdlSideChannel is a JSON-serializable snapshot of per-file ADL capture state.
type CppAdlSideChannel struct {
	ArgInfoBySite [][3]interface{} // [line, col, []CppAdlArgInfo]
	NoAdlSites    [][2]int         // [line, col]
}

// CollectCppAdlSideChannel snapshots this file's ADL capture state for the worker→main side-channel.
func CollectCppAdlSideChannel(filePath string) CppAdlSideChannel {
	adlMutex.RLock()
	defer adlMutex.RUnlock()

	var args [][3]interface{}
	for _, key := range argInfoSiteKeysByFile[filePath] {
		value, ok := argInfoBySite[key]
		if !ok {
			continue
		}
		if _, line, col, parsed := parseAdlSiteKey(key); parsed {
			args = append(args, [3]interface{}{line, col, value})
		}
	}

	var noAdl [][2]int
	for _, key := range noAdlSiteKeysByFile[filePath] {
		if _, line, col, parsed := parseAdlSiteKey(key); parsed {
			noAdl = append(noAdl, [2]int{line, col})
		}
	}

	return CppAdlSideChannel{ArgInfoBySite: args, NoAdlSites: noAdl}
}

// ApplyCppAdlSideChannel restores this file's ADL capture state from the side-channel.
func ApplyCppAdlSideChannel(filePath string, data CppAdlSideChannel) {
	adlMutex.Lock()
	defer adlMutex.Unlock()
	for _, entry := range data.ArgInfoBySite {
		line := entry[0].(int)
		col := entry[1].(int)
		args := entry[2].([]CppAdlArgInfo)
		key := adlSiteKey(filePath, line, col)
		if _, exists := argInfoBySite[key]; !exists {
			pushFileSiteKey(argInfoSiteKeysByFile, filePath, key)
		}
		argInfoBySite[key] = args
	}
	for _, entry := range data.NoAdlSites {
		line := entry[0]
		col := entry[1]
		key := adlSiteKey(filePath, line, col)
		if !noAdlSites[key] {
			pushFileSiteKey(noAdlSiteKeysByFile, filePath, key)
		}
		noAdlSites[key] = true
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Clear state
// ────────────────────────────────────────────────────────────────────────────

// ClearCppAdlState resets all ADL state for a new pipeline invocation.
func ClearCppAdlState() {
	adlMutex.Lock()
	defer adlMutex.Unlock()
	argInfoBySite = make(map[string][]CppAdlArgInfo)
	noAdlSites = make(map[string]bool)
	argInfoSiteKeysByFile = make(map[string][]string)
	noAdlSiteKeysByFile = make(map[string][]string)
	classToNamespaceQName = make(map[string]string)
	adlIndexState = nil
	adlIndexSourceRef = nil
}

// ────────────────────────────────────────────────────────────────────────────
// Legacy API compatibility (used by scope_resolver.go)
// ────────────────────────────────────────────────────────────────────────────

// RegisterCppAdlNamespace registers a namespace's function declarations for ADL.
func RegisterCppAdlNamespace(namespaceScopeID string, functions map[string][]string) {
	// This is a legacy stub; the real ADL uses AdlCandidateIndex now.
}

// RegisterCppAdlArgumentType registers a type's enclosing namespace for ADL.
func RegisterCppAdlArgumentType(typeNodeID string, namespaceScopeIDs []string) {
	// This is a legacy stub; the real ADL uses classToNamespaceQName now.
}

// LookupCppAdl performs ADL for a function name given argument types (legacy stub).
func LookupCppAdl(functionName string, argumentTypes []shared.SymbolDefinition) []string {
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// Associated namespace computation
// ────────────────────────────────────────────────────────────────────────────

// PopulateCppAssociatedNamespaces walks parsed.scopes to record each Class/Enum
// def's enclosing namespace qualified name. Run from the cpp resolver's
// populateOwners hook so the index is available before any resolution pass.
func PopulateCppAssociatedNamespaces(parsed *shared.ParsedFile) {
	scopesByID := make(map[shared.ScopeID]*shared.Scope)
	for _, scope := range parsed.Scopes {
		scopesByID[scope.ID] = scope
	}

	adlMutex.Lock()
	defer adlMutex.Unlock()

	for _, scope := range parsed.Scopes {
		if scope.Kind != shared.ScopeKindClass {
			continue
		}
		nsQName := computeEnclosingNamespaceQName(scope, scopesByID)
		if nsQName == "" {
			continue
		}
		for j := range scope.OwnedDefs {
			def := &scope.OwnedDefs[j]
			if def.Type == "Class" || def.Type == "Struct" || def.Type == "Interface" {
				classToNamespaceQName[def.NodeID] = nsQName
			}
		}
	}

	// Enum defs live in Namespace scopes directly.
	for _, scope := range parsed.Scopes {
		if scope.Kind != shared.ScopeKindNamespace {
			continue
		}
		nsQName := computeNamespaceQName(scope, scopesByID)
		if nsQName == "" {
			continue
		}
		for j := range scope.OwnedDefs {
			def := &scope.OwnedDefs[j]
			if def.Type == "Enum" {
				classToNamespaceQName[def.NodeID] = nsQName
			}
		}
	}
}

// computeEnclosingNamespaceQName walks upward from a Class scope to find the
// innermost enclosing Namespace scope's qualified name (dot-joined).
func computeEnclosingNamespaceQName(classScope *shared.Scope, scopesByID map[shared.ScopeID]*shared.Scope) string {
	parentID := classScope.Parent
	for parentID != nil {
		parent, ok := scopesByID[*parentID]
		if !ok {
			return ""
		}
		if parent.Kind == shared.ScopeKindNamespace {
			return computeNamespaceQName(parent, scopesByID)
		}
		parentID = parent.Parent
	}
	return ""
}

// computeNamespaceQName walks upward from a Namespace scope collecting each
// enclosing Namespace's simple name (innermost last). Returns dot-joined
// qualified name (e.g. "outer.inner").
func computeNamespaceQName(nsScope *shared.Scope, scopesByID map[shared.ScopeID]*shared.Scope) string {
	var segments []string
	currentID := nsScope.Parent
	current := nsScope
	safety := 64
	for current != nil && safety > 0 {
		safety--
		nsDef := findNamespaceDefInScope(current.OwnedDefs)
		if nsDef == nil {
			return ""
		}
		simple := adlSimpleName(*nsDef)
		segments = append([]string{simple}, segments...)
		// Walk up to next enclosing namespace
		var nextNs *shared.Scope
		nextID := currentID
		for nextID != nil {
			nx, ok := scopesByID[*nextID]
			if !ok {
				break
			}
			if nx.Kind == shared.ScopeKindNamespace {
				nextNs = nx
				currentID = nx.Parent
				break
			}
			nextID = nx.Parent
		}
		current = nextNs
	}
	return strings.Join(segments, ".")
}

func findNamespaceDefInScope(defs []shared.SymbolDefinition) *shared.SymbolDefinition {
	for i := range defs {
		if defs[i].Type == "Namespace" {
			return &defs[i]
		}
	}
	return nil
}

func adlSimpleName(def shared.SymbolDefinition) string {
	if def.QualifiedName != nil {
		parts := strings.Split(*def.QualifiedName, ".")
		return parts[len(parts)-1]
	}
	return ""
}

// ────────────────────────────────────────────────────────────────────────────
// ADL candidate selection — PickCppAdlCandidates
// ────────────────────────────────────────────────────────────────────────────

// PickCppAdlCandidates returns ADL candidates for a call site, or nil if no ADL applies.
func PickCppAdlCandidates(
	siteName string,
	callerFilePath string,
	atLine int, atCol int,
) []shared.SymbolDefinition {
	adlMutex.RLock()
	defer adlMutex.RUnlock()

	key := adlSiteKey(callerFilePath, atLine, atCol)
	if noAdlSites[key] {
		return nil
	}
	args, ok := argInfoBySite[key]
	if !ok || len(args) == 0 {
		return nil
	}

	idx := adlIndexState
	if idx == nil {
		return nil
	}

	// Collect associated namespace QNames
	associatedNamespaces := make(map[string]bool)
	for _, arg := range args {
		collectAssociatedNamespacesForAdlArg(arg, associatedNamespaces)
		if arg.FunctionRefText != "" {
			collectFunctionTypeAssociatedNamespaces(arg.FunctionRefText, associatedNamespaces)
		}
	}
	if len(associatedNamespaces) == 0 {
		return nil
	}

	// Gather candidates from the prebuilt index
	bySeq := make(map[int]shared.SymbolDefinition)
	seenKey := make(map[string]bool)
	collectFrom := func(buckets map[string]map[string][]shared.SymbolDefinition) {
		for ns := range associatedNamespaces {
			matches, ok := buckets[ns][siteName]
			if !ok {
				continue
			}
			for _, def := range matches {
				nodeID := def.NodeID
				if seenKey[nodeID] {
					continue
				}
				seenKey[nodeID] = true
				seq := 0
				if s, ok := idx.SeqByNodeID[nodeID]; ok {
					seq = s
				}
				bySeq[seq] = def
			}
		}
	}
	collectFrom(idx.NsCandidates)
	collectFrom(idx.FriendCandidates)

	if len(bySeq) == 0 {
		return nil
	}

	// Sort by sequence number
	seqs := make([]int, 0, len(bySeq))
	for s := range bySeq {
		seqs = append(seqs, s)
	}
	sort.Ints(seqs)
	result := make([]shared.SymbolDefinition, 0, len(seqs))
	for _, s := range seqs {
		result = append(result, bySeq[s])
	}
	return result
}

func collectAssociatedNamespacesForAdlArg(arg CppAdlArgInfo, out map[string]bool) {
	addAssociatedNamespaceForClassName(arg.SimpleClassName, out)
	if arg.TemplateNamespace != "" {
		out[arg.TemplateNamespace] = true
	}
	for _, ns := range arg.TemplateArgNamespaces {
		if ns != "" {
			out[ns] = true
		}
	}
	for _, className := range arg.TemplateArgClassNames {
		addAssociatedNamespaceForClassName(className, out)
	}
}

func addAssociatedNamespaceForClassName(simpleClassName string, out map[string]bool) {
	if simpleClassName == "" {
		return
	}
	adlMutex.RLock()
	defer adlMutex.RUnlock()
	classDef, ambiguous := findCppClassDefBySimpleName(simpleClassName)
	if classDef == nil {
		return
	}
	if classDef.NodeID != "" {
		if nsQName, ok := classToNamespaceQName[classDef.NodeID]; ok {
			out[nsQName] = true
		}
	}
	// If ambiguous, don't walk MRO (V1 collision behavior)
	if ambiguous {
		return
	}
	// TODO: walk MRO chain for ancestor namespaces when MRO is available
}

func findCppClassDefBySimpleName(simpleName string) (*shared.SymbolDefinition, bool) {
	if adlIndexState == nil {
		return nil, false
	}
	matches, ok := adlIndexState.ClassDefsBySimple[simpleName]
	if !ok || len(matches) == 0 {
		return nil, false
	}
	return &matches[0], len(matches) > 1
}

func collectFunctionTypeAssociatedNamespaces(refText string, out map[string]bool) {
	idx := adlIndexState
	if idx == nil {
		return
	}
	colonIdx := strings.LastIndex(refText, "::")
	if colonIdx != -1 {
		// Qualified ref: extract namespace prefix, normalize :: → dot notation
		nsText := strings.ReplaceAll(refText[:colonIdx], "::", ".")
		if nsText == "" {
			return
		}
		simpleName := refText[colonIdx+2:]
		matches, ok := idx.NsFunctionsByQName[nsText][simpleName]
		if ok {
			for i := range matches {
				collectAssociatedNamespacesForFunctionDef(&matches[i], out)
			}
		}
		return
	}
	// Unqualified function references — workspace-wide approximation
	matches, ok := idx.NsFunctionsBySimple[refText]
	if ok {
		for i := range matches {
			collectAssociatedNamespacesForFunctionDef(&matches[i], out)
		}
	}
}

func collectAssociatedNamespacesForFunctionDef(def *shared.SymbolDefinition, out map[string]bool) {
	// Collect from parameter types
	if def.ParameterTypeClasses != nil {
		for _, tc := range def.ParameterTypeClasses {
			collectAssociatedNamespacesForFunctionTypeText(tc.Base, out)
		}
	} else if def.ParameterTypes != nil {
		for _, pt := range def.ParameterTypes {
			collectAssociatedNamespacesForFunctionTypeText(pt, out)
		}
	}
	// Collect from return type
	if def.ReturnType != nil {
		collectAssociatedNamespacesForFunctionTypeText(*def.ReturnType, out)
	}
}

func collectAssociatedNamespacesForFunctionTypeText(typeText string, out map[string]bool) {
	for _, token := range extractCppTypeNameTokens(typeText) {
		if isIgnoredCppAdlNamespace(token.NamespaceName) {
			continue
		}
		addAssociatedNamespaceForClassName(token.SimpleName, out)
		if token.NamespaceName != "" {
			out[token.NamespaceName] = true
		}
	}
}

// cppTypeNameToken represents a parsed type-name token with namespace and simple name.
type cppTypeNameToken struct {
	SimpleName    string
	NamespaceName string
}

func extractCppTypeNameTokens(typeText string) []cppTypeNameToken {
	cleaned := NormalizeCppParamType(typeText)
	if cleaned == "" || isPrimitiveCppAdlType(cleaned) {
		return nil
	}
	var out []cppTypeNameToken
	seen := make(map[string]bool)
	tokenSource := cleaned
	if strings.Contains(typeText, "<") {
		tokenSource = cleaned + " " + typeText
	}
	re := regexp.MustCompile(`[A-Za-z_]\w*(?:::[A-Za-z_]\w*)*`)
	for _, rawToken := range re.FindAllString(tokenSource, -1) {
		if isPrimitiveCppAdlType(rawToken) {
			continue
		}
		segments := strings.Split(rawToken, "::")
		// Filter empty segments
		var filtered []string
		for _, s := range segments {
			if s != "" {
				filtered = append(filtered, s)
			}
		}
		if len(filtered) == 0 {
			continue
		}
		simpleName := filtered[len(filtered)-1]
		if simpleName == "" || isPrimitiveCppAdlType(simpleName) {
			continue
		}
		nsName := ""
		if len(filtered) > 1 {
			nsName = strings.Join(filtered[:len(filtered)-1], ".")
		}
		key := nsName + "\x00" + simpleName
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, cppTypeNameToken{SimpleName: simpleName, NamespaceName: nsName})
	}
	return out
}

var cppAdlPrimitiveOrKeywordTypes = map[string]bool{
	"alignas": true, "alignof": true, "auto": true, "bool": true,
	"char": true, "char8_t": true, "char16_t": true, "char32_t": true,
	"class": true, "const": true, "consteval": true, "constexpr": true,
	"constinit": true, "decltype": true, "double": true, "enum": true,
	"explicit": true, "extern": true, "float": true, "inline": true,
	"int": true, "long": true, "mutable": true, "noexcept": true,
	"null": true, "register": true, "short": true, "signed": true,
	"static": true, "string": true, "struct": true, "template": true,
	"thread_local": true, "typename": true, "union": true, "unknown": true,
	"unsigned": true, "void": true, "volatile": true, "wchar_t": true,
	"...": true,
}

func isPrimitiveCppAdlType(typeText string) bool {
	return cppAdlPrimitiveOrKeywordTypes[typeText]
}

func isIgnoredCppAdlNamespace(namespaceName string) bool {
	return namespaceName == "std" || strings.HasPrefix(namespaceName, "std.")
}
