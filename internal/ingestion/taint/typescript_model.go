package taint

// Built-in TS/JS taint model (#2083 M3 U2, plan KTD7).
//
// The canonical Express/Node source/sink/sanitizer set, registered for the
// "typescript" and "javascript" language ids via the explicit
// RegisterBuiltinTaintModels seam — deliberately not an init() side-effect,
// so the U4 emit path controls WHEN registration happens (call it once before
// the pdg window runs; it is idempotent — the registry is last-write-wins on
// the same language id).
//
// taintModelVersion is a deterministic digest of the FULL model content
// (entries, kinds, args, modules). It joins the RepoMeta pdg stamp in U5 so
// that ANY model change trips full writeback on an existing --pdg index (R7):
// persisted findings must never outlive the model that produced them.

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// TSJSTaintModel is the built-in TS/JS model. Module provenance uses bare
// specifier names — the matcher normalizes the "node:" scheme prefix, so
// import { exec } from 'node:child_process' resolves identically.
var TSJSTaintModel = SourceSinkSanitizerSpec{
	Sources: []TaintMemberSourceEntry{
		{
			Kind:       SourceKindRemoteInput,
			Objects:    []string{"req", "request"},
			Properties: []string{"body", "query", "params", "headers", "cookies"},
		},
	},
	Sinks: []TaintSinkEntry{
		// Command execution — the command string is argument 0.
		{TaintCallableMatcher: TaintCallableMatcher{Name: "exec", Kind: "command-injection", Args: []int{0}}, Module: "child_process"},
		{TaintCallableMatcher: TaintCallableMatcher{Name: "execSync", Kind: "command-injection", Args: []int{0}}, Module: "child_process"},
		{TaintCallableMatcher: TaintCallableMatcher{Name: "spawn", Kind: "command-injection", Args: []int{0}}, Module: "child_process"},
		// Code evaluation. eval takes code at 0; new Function(...) treats
		// EVERY argument as source text (params + body), so Args is nil
		// (= all positions) rather than pinned to 0.
		{TaintCallableMatcher: TaintCallableMatcher{Name: "eval", Kind: "code-injection", Args: []int{0}}, Global: true},
		{TaintCallableMatcher: TaintCallableMatcher{Name: "Function", Kind: "code-injection"}, Global: true, NewOnly: true},
		// Filesystem path consumption — path argument 0.
		{TaintCallableMatcher: TaintCallableMatcher{Name: "readFile", Kind: "path-traversal", Args: []int{0}}, Module: "fs"},
		{TaintCallableMatcher: TaintCallableMatcher{Name: "readFileSync", Kind: "path-traversal", Args: []int{0}}, Module: "fs"},
		{TaintCallableMatcher: TaintCallableMatcher{Name: "writeFile", Kind: "path-traversal", Args: []int{0}}, Module: "fs"},
		{TaintCallableMatcher: TaintCallableMatcher{Name: "writeFileSync", Kind: "path-traversal", Args: []int{0}}, Module: "fs"},
		// SQL — .query(sql) / .execute(sql) member calls on ANY receiver
		// (mysql2/pg/knex handles go by many names; receiver-conventional).
		{TaintCallableMatcher: TaintCallableMatcher{Name: "query", Kind: "sql-injection", Args: []int{0}}, AnyReceiver: true},
		{TaintCallableMatcher: TaintCallableMatcher{Name: "execute", Kind: "sql-injection", Args: []int{0}}, AnyReceiver: true},
		// Reflected XSS — Express response writes, conventional receiver "res".
		{TaintCallableMatcher: TaintCallableMatcher{Name: "send", Kind: "xss", Args: []int{0}}, Receivers: []string{"res"}},
		{TaintCallableMatcher: TaintCallableMatcher{Name: "write", Kind: "xss", Args: []int{0}}, Receivers: []string{"res"}},
	},
	Sanitizers: []TaintSanitizerEntry{
		// URL-encoding: neutralizes markup injection AND path separators.
		{Name: "encodeURIComponent", Neutralizes: []SinkKind{SinkKindXSS, SinkKindPathTraversal}, Global: true},
		// escape-html exports its function as the module default.
		{Name: "default", Neutralizes: []SinkKind{SinkKindXSS}, Module: "escape-html"},
		{Name: "encode", Neutralizes: []SinkKind{SinkKindXSS}, Module: "he"},
		{Name: "basename", Neutralizes: []SinkKind{SinkKindPathTraversal}, Module: "path"},
		{Name: "escape", Neutralizes: []SinkKind{SinkKindXSS}, Module: "validator"},
	},
}

// ComputeTaintModelVersion returns a deterministic digest of a spec's full
// content. Key order is canonicalized (recursively sorted) so the version
// reflects CONTENT, not literal layout; array order is semantic (entry
// identity) and intentionally preserved.
func ComputeTaintModelVersion(spec *SourceSinkSanitizerSpec) string {
	h := sha256.New()
	h.Write([]byte(canonicalJSON(spec)))
	sum := fmt.Sprintf("%x", h.Sum(nil))
	if len(sum) > 12 {
		return sum[:12]
	}
	return sum
}

// TaintModelVersion is the version stamp of the built-in TS/JS model
// (joins the RepoMeta pdg key in U5).
var TaintModelVersion = ComputeTaintModelVersion(&TSJSTaintModel)

// RegisterBuiltinTaintModels registers the built-in model for TypeScript and
// JavaScript. Explicit init seam for the U4 emit path (call before the pdg
// window consumes the registry); idempotent. Vue and other TS-adjacent
// language ids are deliberately NOT registered — the M3 scope is TS/JS only.
func RegisterBuiltinTaintModels() {
	RegisterSourceSinkConfig("typescript", TSJSTaintModel)
	RegisterSourceSinkConfig("javascript", TSJSTaintModel)
}

// canonicalJSON produces a deterministic JSON representation by sorting
// object keys recursively and omitting undefined/zero values.
func canonicalJSON(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "null"
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int, int64, float64, string:
		b, _ := json.Marshal(val)
		return string(b)
	case []interface{}:
		parts := make([]string, len(val))
		for i, elem := range val {
			parts[i] = canonicalJSON(elem)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case map[string]interface{}:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			ev := val[k]
			if ev == nil {
				continue
			}
			parts = append(parts, fmt.Sprintf("%q:%s", k, canonicalJSON(ev)))
		}
		return "{" + strings.Join(parts, ",") + "}"
	default:
		// Use encoding/json for other types
		b, _ := json.Marshal(val)
		return string(b)
	}
}