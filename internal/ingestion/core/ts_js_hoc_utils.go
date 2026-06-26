// TS/JS HOC Utils — Higher-Order Component detection and unwrapping for TypeScript/JavaScript.
//
// Mirrors TS ts-js-hoc-utils.ts, skeleton for codetrip.
// Detects common HOC patterns (React.forwardRef, React.memo, withRouter,
// connect, etc.) and unwraps them so the inner component gets proper
// symbol identity. Deferred to Phase 3.

package core

// HOCPattern describes a higher-order component wrapper we know about.
type HOCPattern struct {
	Name       string // e.g., "forwardRef", "memo", "connect"
	Wrapper    string // import path of the wrapper, e.g., "react"
	UnwrapTo   int    // which argument is the inner component (0-based)
	IsMethod   bool   // true if HOC is called as obj.method()
}

// Known HOC patterns in React ecosystem.
var builtinHOCPatterns = []HOCPattern{
	{Name: "forwardRef", Wrapper: "react", UnwrapTo: 0, IsMethod: true},
	{Name: "memo", Wrapper: "react", UnwrapTo: 0, IsMethod: true},
	{Name: "lazy", Wrapper: "react", UnwrapTo: 0, IsMethod: true},
	{Name: "connect", Wrapper: "react-redux", UnwrapTo: 1, IsMethod: false},
	{Name: "withRouter", Wrapper: "react-router-dom", UnwrapTo: 0, IsMethod: false},
	{Name: "withStyles", Wrapper: "@material-ui/core/styles", UnwrapTo: 1, IsMethod: false},
	{Name: "compose", Wrapper: "recompose", UnwrapTo: -1, IsMethod: false}, // compose is variadic
}

// HOCUnwrapResult holds the result of unwrapping an HOC call.
type HOCUnwrapResult struct {
	InnerComponentName string
	HOCName            string
	IsHOC              bool
}

// DetectHOC checks whether a function call matches a known HOC pattern.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func DetectHOC(callName string, importSource string) *HOCUnwrapResult {
	// TODO(Phase 3): match against builtinHOCPatterns
	return &HOCUnwrapResult{IsHOC: false}
}

// UnwrapHOC extracts the inner component name from an HOC-wrapped definition.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func UnwrapHOC(wrappedName string, hocPattern HOCPattern) string {
	// TODO(Phase 3): parse call expression arguments
	return wrappedName
}