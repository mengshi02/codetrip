package graph

import "strings"

// Label represents node type labels (38 types)
type Label string

const (
	// Project organization
	LabelProject Label = "Project"
	LabelPackage Label = "Package"
	// File system
	LabelFile   Label = "File"
	LabelFolder Label = "Folder"
	// Core code structure
	LabelFunction    Label = "Function"
	LabelClass       Label = "Class"
	LabelInterface   Label = "Interface"
	LabelMethod      Label = "Method"
	LabelConstructor Label = "Constructor"
	// Type system
	LabelStruct    Label = "Struct"
	LabelEnum      Label = "Enum"
	LabelMacro     Label = "Macro"
	LabelTypedef   Label = "Typedef"
	LabelUnion     Label = "Union"
	LabelTypeAlias Label = "TypeAlias"
	LabelType      Label = "Type"
	// Code organization
	LabelNamespace Label = "Namespace"
	LabelTrait     Label = "Trait"
	LabelImpl      Label = "Impl"
	LabelModule    Label = "Module"
	LabelTemplate  Label = "Template"
	// Members
	LabelProperty Label = "Property"
	LabelConst    Label = "Const"
	LabelStatic   Label = "Static"
	LabelRecord   Label = "Record"
	LabelDelegate Label = "Delegate"
	LabelVariable Label = "Variable"
	// Metaprogramming
	LabelAnnotation Label = "Annotation"
	LabelDecorator  Label = "Decorator"
	// Imports
	LabelImport Label = "Import"
	// General
	LabelCodeElement Label = "CodeElement"
	// Analysis outputs
	LabelCommunity Label = "Community"
	LabelProcess   Label = "Process"
	LabelRoute     Label = "Route"
	LabelTool      Label = "Tool"
	LabelSection   Label = "Section"
	// CFG
	LabelBasicBlock Label = "BasicBlock"
	// Cross-repository
	LabelContract Label = "Contract"
	// Language file types
	LabelGoFile       Label = "GoFile"
	LabelTSFile       Label = "TSFile"
	LabelJSFile       Label = "JSFile"
	LabelPythonFile   Label = "PythonFile"
	LabelJavaFile     Label = "JavaFile"
	LabelRustFile     Label = "RustFile"
	LabelCFile        Label = "CFile"
	LabelCPPFile      Label = "CPPFile"
	LabelCSharpFile   Label = "CSharpFile"
	LabelMarkdownFile Label = "MarkdownFile"
	// Supplementary code structure
	LabelField    Label = "Field"
	LabelCallSite Label = "CallSite"
	LabelUnknown  Label = "Unknown"
)

// IsSymbol determines whether a label is a code symbol (node types that need parsing)
func (l Label) IsSymbol() bool {
	switch l {
	case LabelFunction, LabelClass, LabelInterface, LabelMethod, LabelConstructor,
		LabelStruct, LabelEnum, LabelMacro, LabelTypedef, LabelUnion, LabelTypeAlias, LabelType,
		LabelNamespace, LabelTrait, LabelImpl, LabelModule, LabelTemplate,
		LabelProperty, LabelConst, LabelStatic, LabelRecord, LabelDelegate, LabelVariable,
		LabelAnnotation, LabelDecorator, LabelCodeElement:
		return true
	}
	return false
}

// Node represents a graph node
type Node struct {
	ID       string    `json:"id"`
	Label    Label     `json:"label"`
	Repo     string    `json:"repo"`
	FilePath string    `json:"filePath,omitempty"`
	Name     string    `json:"name"`
	Props    NodeProps `json:"props,omitempty"`
}

// NewNode creates a new node
func NewNode(repo string, label Label, name string) *Node {
	return &Node{
		ID:    "",
		Label: label,
		Repo:  repo,
		Name:  name,
	}
}

// WithID sets the node ID
func (n *Node) WithID(id string) *Node {
	n.ID = id
	return n
}

// WithFile sets the file path
func (n *Node) WithFile(fp string) *Node {
	n.FilePath = fp
	return n
}

// WithProp sets a property
func (n *Node) WithProp(key string, val any) *Node {
	n.Props.SetProp(key, val)
	return n
}

// GetProp retrieves a property with a default value
func (n *Node) GetProp(key string, defaultVal any) any {
	if v, ok := n.Props.GetProp(key); ok {
		return v
	}
	return defaultVal
}

// GetPropString retrieves a string property
func (n *Node) GetPropString(key string) string {
	v := n.GetProp(key, "")
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// GetPropInt retrieves an integer property
func (n *Node) GetPropInt(key string) int {
	v := n.GetProp(key, 0)
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	}
	return 0
}

// GetPropBool retrieves a boolean property
func (n *Node) GetPropBool(key string) bool {
	v := n.GetProp(key, false)
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

// GetPropFloat64 retrieves a float64 property
func (n *Node) GetPropFloat64(key string) float64 {
	v := n.GetProp(key, 0.0)
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

// UID generates a deterministic UID (for cross-file references)
func (n *Node) UID() string {
	var b strings.Builder
	b.Grow(len(n.Repo) + 1 + len(n.FilePath) + 1 + len(n.Label) + 1 + len(n.Name))
	b.WriteString(n.Repo)
	b.WriteByte(':')
	b.WriteString(n.FilePath)
	b.WriteByte(':')
	b.WriteString(string(n.Label))
	b.WriteByte(':')
	b.WriteString(n.Name)
	return b.String()
}

// Key returns the Pebble KV storage key
func (n *Node) Key() string {
	return nodeKey(n.Repo, n.ID)
}

// SymbolDescription returns a brief description of the node
func (n *Node) SymbolDescription() string {
	parts := []string{string(n.Label), n.Name}
	if n.FilePath != "" {
		parts = append(parts, n.FilePath)
	}
	return strings.Join(parts, " ")
}

// NodeIterator is a node iterator (streaming traversal, reduces memory allocation)
type NodeIterator interface {
	Next() bool
	Node() *Node
	Close() error
}
