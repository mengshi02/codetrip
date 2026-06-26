package cfg

// BindingEntry represents variable binding
type BindingEntry struct {
	Name    string // variable name
	NodeID  string // declaration node ID
	Line    int    // declaration line number
	IsParam bool   // whether it is a function parameter
}

// BasicBlock represents a basic block
type BasicBlock struct {
	ID           string   // unique block ID
	Label        string   // entry/exit/normal/branch/loop
	StartLine    int      // start line number
	EndLine      int      // end line number
	NodeIDs      []string // AST node IDs within the block
	StatementIDs []string // statement IDs within the block
}

// CFGEdge represents a control flow edge
type CFGEdge struct {
	From      string // source block ID
	To        string // target block ID
	EdgeType  string // normal/true/false/exception
	Condition string // condition expression (optional)
}

// SiteRecord represents call/construct/member-read sites
type SiteRecord struct {
	SiteType string // call/construct/member-read
	Symbol   string // call symbol
	Line     int    // line number
	NodeID   string // associated node ID
}

// StatementFacts represents statement facts (for data flow analysis)
type StatementFacts struct {
	StatementID string   // unique statement ID
	Line        int      // line number
	Defines     []string // defined variables
	Uses        []string // used variables
	Kills       []string // killed definitions
}

// FunctionCFG represents a function control flow graph
type FunctionCFG struct {
	FuncID     string           // function node ID
	FuncName   string           // function name
	Bindings   []BindingEntry   // variable bindings
	Blocks     []BasicBlock     // basic blocks
	Edges      []CFGEdge        // control flow edges
	Sites      []SiteRecord     // call/construct/member-read sites
	Statements []StatementFacts // statement facts
}