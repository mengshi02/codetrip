package model

// NodeLabel represents the type of a graph node.
type NodeLabel string

const (
	LabelFile        NodeLabel = "File"
	LabelFolder      NodeLabel = "Folder"
	LabelFunction    NodeLabel = "Function"
	LabelClass       NodeLabel = "Class"
	LabelInterface   NodeLabel = "Interface"
	LabelMethod      NodeLabel = "Method"
	LabelConstructor NodeLabel = "Constructor"
	LabelProperty    NodeLabel = "Property"
	LabelEnum        NodeLabel = "Enum"
	LabelStruct      NodeLabel = "Struct"
	LabelTrait       NodeLabel = "Trait"
	LabelMacro       NodeLabel = "Macro"
	LabelNamespace   NodeLabel = "Namespace"
	LabelModule      NodeLabel = "Module"
	LabelVariable    NodeLabel = "Variable"
	LabelConst       NodeLabel = "Const"
	LabelStatic      NodeLabel = "Static"
	LabelTypedef     NodeLabel = "Typedef"
	LabelConstant    NodeLabel = "Constant"
	LabelTypeAlias   NodeLabel = "TypeAlias"
	LabelRecord      NodeLabel = "Record"
	LabelDelegate    NodeLabel = "Delegate"
	LabelAnnotation  NodeLabel = "Annotation"
	LabelTemplate    NodeLabel = "Template"
	LabelUnion       NodeLabel = "Union"
	LabelImpl        NodeLabel = "Impl"
	LabelDecorator   NodeLabel = "Decorator"
	LabelPackage     NodeLabel = "Package"
	LabelMixin       NodeLabel = "Mixin"
	LabelExtension   NodeLabel = "Extension"
	LabelProtocol    NodeLabel = "Protocol"
	LabelCategory    NodeLabel = "Category"
	LabelProtocolExt NodeLabel = "ProtocolExtension"
	LabelComponent   NodeLabel = "Component"
	LabelService     NodeLabel = "Service"
	LabelRepository  NodeLabel = "Repository"
	LabelController  NodeLabel = "Controller"
	LabelConfig      NodeLabel = "Config"
	LabelTest        NodeLabel = "Test"
	LabelCodeElement NodeLabel = "CodeElement"
	LabelCommunity   NodeLabel = "Community"
	LabelProcess     NodeLabel = "Process"
)

// NodeProperties holds all properties for a graph node.
type NodeProperties struct {
	Name           string   `json:"name"`
	FilePath       string   `json:"filePath"`
	StartLine      *int     `json:"startLine,omitempty"`
	EndLine        *int     `json:"endLine,omitempty"`
	IsExported     *bool    `json:"isExported,omitempty"`
	Content        string   `json:"content,omitempty"`
	Description    string   `json:"description,omitempty"`
	ParameterCount *int     `json:"parameterCount,omitempty"`
	ReturnType     string   `json:"returnType,omitempty"`
	Visibility     string   `json:"visibility,omitempty"`
	IsAbstract     *bool    `json:"isAbstract,omitempty"`
	IsStatic       *bool    `json:"isStatic,omitempty"`
	IsAsync        *bool    `json:"isAsync,omitempty"`
	IsTest         *bool    `json:"isTest,omitempty"`
	Modifiers      string   `json:"modifiers,omitempty"`
	Language       string   `json:"language,omitempty"`
	HeuristicLabel string   `json:"heuristicLabel,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
	EnrichedBy     string   `json:"enrichedBy,omitempty"`
	Cohesion       *float64 `json:"cohesion,omitempty"`
	SymbolCount    *float64 `json:"symbolCount,omitempty"`
	ProcessType    string   `json:"processType,omitempty"`
	StepCount      *float64 `json:"stepCount,omitempty"`
	Communities    []string `json:"communities,omitempty"`
	EntryPointID   string   `json:"entryPointId,omitempty"`
	TerminalID     string   `json:"terminalId,omitempty"`
	FileSize       int64    `json:"fileSize,omitempty"`
	IsBinary       *bool    `json:"isBinary,omitempty"`
	LanguageID     string   `json:"languageId,omitempty"`
	PackageName    string   `json:"packageName,omitempty"`
	Version        string   `json:"version,omitempty"`
}

// GraphNode represents a node in the knowledge graph.
type GraphNode struct {
	ID         string         `json:"id"`
	Label      NodeLabel      `json:"label"`
	Properties NodeProperties `json:"properties"`
}
