package graph

// RelType represents relationship types (29 types)
type RelType string

const (
	RelContains          RelType = "CONTAINS"
	RelDefines           RelType = "DEFINES"
	RelCalls             RelType = "CALLS"
	RelImports           RelType = "IMPORTS"
	RelExtends           RelType = "EXTENDS"
	RelImplements        RelType = "IMPLEMENTS"
	RelInherits          RelType = "INHERITS"
	RelDecorates         RelType = "DECORATES"
	RelWraps             RelType = "WRAPS"
	RelHasMethod         RelType = "HAS_METHOD"
	RelHasProperty       RelType = "HAS_PROPERTY"
	RelAccesses          RelType = "ACCESSES"
	RelMethodOverrides   RelType = "METHOD_OVERRIDES"
	RelMethodImplements  RelType = "METHOD_IMPLEMENTS"
	RelMemberOf          RelType = "MEMBER_OF"
	RelStepInProcess     RelType = "STEP_IN_PROCESS"
	RelEntryPointOf      RelType = "ENTRY_POINT_OF"
	RelHandlesRoute      RelType = "HANDLES_ROUTE"
	RelFetches           RelType = "FETCHES"
	RelHandlesTool       RelType = "HANDLES_TOOL"
	RelQueries           RelType = "QUERIES"
	RelUses              RelType = "USES"
	RelBindsEventHandler RelType = "BINDS_EVENT_HANDLER"
	RelEmitsEvent        RelType = "EMITS_EVENT"
	RelCFG               RelType = "CFG"
	RelReachingDef       RelType = "REACHING_DEF"
	RelTainted           RelType = "TAINTED"
	RelSanitizes         RelType = "SANITIZES"
	RelTaintPath         RelType = "TAINT_PATH"
	// Cross-repository (ContractLink)
	RelContractLink RelType = "ContractLink"
	RelEmbraces     RelType = "EMBRACES"
)

// Edge represents a graph edge
type Edge struct {
	ID     string    `json:"id"`
	Type   RelType   `json:"type"`
	Source string    `json:"source"`
	Target string    `json:"target"`
	Props  EdgeProps `json:"props,omitempty"`
}

// NewEdge creates a new edge
func NewEdge(relType RelType, src, tgt string) *Edge {
	return &Edge{
		Type:   relType,
		Source: src,
		Target: tgt,
	}
}

// WithID sets the edge ID
func (e *Edge) WithID(id string) *Edge {
	e.ID = id
	return e
}

// WithProp sets a property
func (e *Edge) WithProp(key string, val any) *Edge {
	e.Props.SetProp(key, val)
	return e
}

// GetProp retrieves a property
func (e *Edge) GetProp(key string, defaultVal any) any {
	if v, ok := e.Props.GetProp(key); ok {
		return v
	}
	return defaultVal
}

// GetPropString safely retrieves a string property from the edge.
// Returns empty string if the property doesn't exist or isn't a string.
func (e *Edge) GetPropString(key string) string {
	if v, ok := e.Props.GetProp(key); ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// GetPropFloat64 retrieves a float64 property
func (e *Edge) GetPropFloat64(key string) float64 {
	v := e.GetProp(key, 0.0)
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	}
	return 0
}

// Confidence returns the edge confidence score
func (e *Edge) Confidence() float64 {
	v := e.GetProp("confidence", 1.0)
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	default:
		return 1.0
	}
}

// Key returns the Pebble KV storage key
func (e *Edge) Key() string {
	return edgeKey(e.ID)
}

// EdgeFilter is an edge filtering function
type EdgeFilter func(*Edge) bool

// TraverseDir represents traversal direction
type TraverseDir int

const (
	TraverseOut TraverseDir = iota // Outgoing edge direction
	TraverseIn                      // Incoming edge direction
	TraverseBoth                    // Both directions
)