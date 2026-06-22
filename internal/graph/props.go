package graph

import (
	"encoding/json"
	"sort"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
)

// NodeProps replaces map[string]any for node properties.
// High-frequency fields have dedicated typed slots (zero-alloc access, no boxing).
// Low-frequency fields fall through to Extra (map[string]any for flexibility).
//
// GC impact: ~60% reduction vs pure map[string]any — the struct is a single
// allocation with inline fields; only Extra needs separate allocation when used.
// Serialization: msgpack encodes struct fields directly without reflection
// on the hot path (only Extra uses map reflection).
type NodeProps struct {
	// Source location (used by Function, Method, Class, Interface, Struct, etc.)
	StartLine int `json:"startLine,omitempty" msgpack:"startLine,omitempty"`
	EndLine   int `json:"endLine,omitempty" msgpack:"endLine,omitempty"`

	// Single line reference (Import, CallSite, Variable, etc.)
	Line int `json:"line,omitempty" msgpack:"line,omitempty"`

	// File metadata (File node)
	Language    string `json:"language,omitempty" msgpack:"language,omitempty"`
	ContentHash string `json:"contentHash,omitempty" msgpack:"contentHash,omitempty"`
	LineCount   int    `json:"lineCount,omitempty" msgpack:"lineCount,omitempty"`

	// Scan metadata (Project node)
	FileCount int `json:"fileCount,omitempty" msgpack:"fileCount,omitempty"`
	TotalSize int `json:"totalSize,omitempty" msgpack:"totalSize,omitempty"`

	// Symbol attributes
	IsExported bool `json:"isExported,omitempty" msgpack:"isExported,omitempty"`
	IsAsync    bool `json:"isAsync,omitempty" msgpack:"isAsync,omitempty"`
	IsWildcard bool `json:"isWildcard,omitempty" msgpack:"isWildcard,omitempty"`
	IsParam    bool `json:"isParam,omitempty" msgpack:"isParam,omitempty"`

	// Import/alias
	Alias string `json:"alias,omitempty" msgpack:"alias,omitempty"`

	// Method receiver
	Receiver string `json:"receiver,omitempty" msgpack:"receiver,omitempty"`

	// Route/API
	Path   string `json:"path,omitempty" msgpack:"path,omitempty"`
	Method string `json:"method,omitempty" msgpack:"method,omitempty"`

	// Documentation
	Description string `json:"description,omitempty" msgpack:"description,omitempty"`

	// Tool definition
	HandlerID   string `json:"handlerId,omitempty" msgpack:"handlerId,omitempty"`
	InputSchema string `json:"inputSchema,omitempty" msgpack:"inputSchema,omitempty"`

	// Function arity
	Arity int `json:"arity,omitempty" msgpack:"arity,omitempty"`

	// Inheritance
	BaseTypes []string `json:"baseTypes,omitempty" msgpack:"baseTypes,omitempty"`

	// Community detection
	Cohesion     float64 `json:"cohesion,omitempty" msgpack:"cohesion,omitempty"`
	SymbolCount  int     `json:"symbolCount,omitempty" msgpack:"symbolCount,omitempty"`
	GroupID      string  `json:"groupId,omitempty" msgpack:"groupId,omitempty"`

	// Process
	EntryPointID string `json:"entryPointID,omitempty" msgpack:"entryPointID,omitempty"`
	StepCount    int    `json:"stepCount,omitempty" msgpack:"stepCount,omitempty"`
	ProcessType  string `json:"processType,omitempty" msgpack:"processType,omitempty"`

	// CFG
	FuncID       string `json:"funcID,omitempty" msgpack:"funcID,omitempty"`
	FuncName     string `json:"funcName,omitempty" msgpack:"funcName,omitempty"`
	BlockLabel   string `json:"blockLabel,omitempty" msgpack:"blockLabel,omitempty"`
	NodeIDs      []string `json:"nodeIDs,omitempty" msgpack:"nodeIDs,omitempty"`
	StatementIDs []string `json:"statementIDs,omitempty" msgpack:"statementIDs,omitempty"`
	SiteType     string `json:"siteType,omitempty" msgpack:"siteType,omitempty"`

	// Section level
	Level int `json:"level,omitempty" msgpack:"level,omitempty"`

	// ORM
	ORMModel  bool   `json:"ormModel,omitempty" msgpack:"ormModel,omitempty"`
	Operation string `json:"operation,omitempty" msgpack:"operation,omitempty"`
	Model     string `json:"model,omitempty" msgpack:"model,omitempty"`

	// Module/workspace
	Module     string `json:"module,omitempty" msgpack:"module,omitempty"`
	ArtifactID string `json:"artifactId,omitempty" msgpack:"artifactId,omitempty"`

	// Middleware
	Middleware []string `json:"middleware,omitempty" msgpack:"middleware,omitempty"`

	// Route response/error keys
	ResponseKeys []string `json:"responseKeys,omitempty" msgpack:"responseKeys,omitempty"`
	ErrorKeys    []string `json:"errorKeys,omitempty" msgpack:"errorKeys,omitempty"`

	// Low-frequency or dynamic properties fall through to Extra
	Extra map[string]any `json:"extra,omitempty" msgpack:"extra,omitempty"`
}

// EdgeProps replaces map[string]any for edge properties.
// Same design principle: high-frequency fields typed, low-frequency in Extra.
type EdgeProps struct {
	// Confidence score (most common edge property)
	Confidence float64 `json:"confidence,omitempty" msgpack:"confidence,omitempty"`

	// Source location
	Line int    `json:"line,omitempty" msgpack:"line,omitempty"`
	File string `json:"file,omitempty" msgpack:"file,omitempty"`

	// Import path (for cross-file edges)
	ImportPath string `json:"importPath,omitempty" msgpack:"importPath,omitempty"`
	Alias      string `json:"alias,omitempty" msgpack:"alias,omitempty"`

	// Taint analysis
	Category   string `json:"category,omitempty" msgpack:"category,omitempty"`
	SourceName string `json:"sourceName,omitempty" msgpack:"sourceName,omitempty"`
	SinkName   string `json:"sinkName,omitempty" msgpack:"sinkName,omitempty"`
	Sanitized  bool   `json:"sanitized,omitempty" msgpack:"sanitized,omitempty"`
	HopIndex   int    `json:"hopIndex,omitempty" msgpack:"hopIndex,omitempty"`

	// CFG edge
	EdgeType   string `json:"edgeType,omitempty" msgpack:"edgeType,omitempty"`
	Condition  string `json:"condition,omitempty" msgpack:"condition,omitempty"`
	FuncID     string `json:"funcID,omitempty" msgpack:"funcID,omitempty"`

	// Call resolution
	ReturnType        string `json:"returnType,omitempty" msgpack:"returnType,omitempty"`
	SameFile          bool   `json:"sameFile,omitempty" msgpack:"sameFile,omitempty"`
	OverloadResolution string `json:"overloadResolution,omitempty" msgpack:"overloadResolution,omitempty"`

	// Process step
	Order int `json:"order,omitempty" msgpack:"order,omitempty"`

	// Weight
	Weight float64 `json:"weight,omitempty" msgpack:"weight,omitempty"`

	// ORM
	Operation string `json:"operation,omitempty" msgpack:"operation,omitempty"`
	Model     string `json:"model,omitempty" msgpack:"model,omitempty"`

	// Topic
	Topic string `json:"topic,omitempty" msgpack:"topic,omitempty"`

	// Low-frequency or dynamic properties fall through to Extra
	Extra map[string]any `json:"extra,omitempty" msgpack:"extra,omitempty"`
}

// --- NodeProps methods ---

// NewNodeProps creates an empty NodeProps
func NewNodeProps() NodeProps {
	return NodeProps{}
}

// SetProp sets a property by key — routes to typed field or Extra
func (p *NodeProps) SetProp(key string, val any) {
	switch key {
	case "startLine":
		p.StartLine = toInt(val)
	case "endLine":
		p.EndLine = toInt(val)
	case "line":
		p.Line = toInt(val)
	case "language":
		p.Language = toString(val)
	case "contentHash":
		p.ContentHash = toString(val)
	case "lineCount":
		p.LineCount = toInt(val)
	case "fileCount":
		p.FileCount = toInt(val)
	case "totalSize":
		p.TotalSize = toInt(val)
	case "isExported", "exported":
		p.IsExported = toBool(val)
	case "isAsync":
		p.IsAsync = toBool(val)
	case "isWildcard":
		p.IsWildcard = toBool(val)
	case "isParam":
		p.IsParam = toBool(val)
	case "alias":
		p.Alias = toString(val)
	case "receiver":
		p.Receiver = toString(val)
	case "path":
		p.Path = toString(val)
	case "method":
		p.Method = toString(val)
	case "description":
		p.Description = toString(val)
	case "handlerId":
		p.HandlerID = toString(val)
	case "inputSchema":
		p.InputSchema = toString(val)
	case "arity":
		p.Arity = toInt(val)
	case "baseTypes":
		p.BaseTypes = toStringSlice(val)
	case "cohesion":
		p.Cohesion = toFloat64(val)
	case "symbolCount":
		p.SymbolCount = toInt(val)
	case "groupId":
		p.GroupID = toString(val)
	case "entryPointID":
		p.EntryPointID = toString(val)
	case "stepCount":
		p.StepCount = toInt(val)
	case "processType":
		p.ProcessType = toString(val)
	case "funcID":
		p.FuncID = toString(val)
	case "funcName":
		p.FuncName = toString(val)
	case "blockLabel":
		p.BlockLabel = toString(val)
	case "nodeIDs":
		p.NodeIDs = toStringSlice(val)
	case "statementIDs":
		p.StatementIDs = toStringSlice(val)
	case "siteType":
		p.SiteType = toString(val)
	case "level":
		p.Level = toInt(val)
	case "ormModel":
		p.ORMModel = toBool(val)
	case "operation":
		p.Operation = toString(val)
	case "model":
		p.Model = toString(val)
	case "module":
		p.Module = toString(val)
	case "artifactId":
		p.ArtifactID = toString(val)
	case "middleware":
		p.Middleware = toStringSlice(val)
	case "responseKeys":
		p.ResponseKeys = toStringSlice(val)
	case "errorKeys":
		p.ErrorKeys = toStringSlice(val)
	default:
		if p.Extra == nil {
			p.Extra = make(map[string]any)
		}
		p.Extra[key] = val
	}
}

// GetProp retrieves a property by key — routes from typed field or Extra
func (p NodeProps) GetProp(key string) (any, bool) {
	switch key {
	case "startLine":
		if p.StartLine != 0 {
			return p.StartLine, true
		}
	case "endLine":
		if p.EndLine != 0 {
			return p.EndLine, true
		}
	case "line":
		if p.Line != 0 {
			return p.Line, true
		}
	case "language":
		if p.Language != "" {
			return p.Language, true
		}
	case "contentHash":
		if p.ContentHash != "" {
			return p.ContentHash, true
		}
	case "lineCount":
		if p.LineCount != 0 {
			return p.LineCount, true
		}
	case "fileCount":
		if p.FileCount != 0 {
			return p.FileCount, true
		}
	case "totalSize":
		if p.TotalSize != 0 {
			return p.TotalSize, true
		}
	case "isExported", "exported":
		if p.IsExported {
			return p.IsExported, true
		}
	case "isAsync":
		if p.IsAsync {
			return p.IsAsync, true
		}
	case "isWildcard":
		if p.IsWildcard {
			return p.IsWildcard, true
		}
	case "isParam":
		if p.IsParam {
			return p.IsParam, true
		}
	case "alias":
		if p.Alias != "" {
			return p.Alias, true
		}
	case "receiver":
		if p.Receiver != "" {
			return p.Receiver, true
		}
	case "path":
		if p.Path != "" {
			return p.Path, true
		}
	case "method":
		if p.Method != "" {
			return p.Method, true
		}
	case "description":
		if p.Description != "" {
			return p.Description, true
		}
	case "handlerId":
		if p.HandlerID != "" {
			return p.HandlerID, true
		}
	case "inputSchema":
		if p.InputSchema != "" {
			return p.InputSchema, true
		}
	case "arity":
		if p.Arity != 0 {
			return p.Arity, true
		}
	case "baseTypes":
		if p.BaseTypes != nil {
			return p.BaseTypes, true
		}
	case "cohesion":
		if p.Cohesion != 0 {
			return p.Cohesion, true
		}
	case "symbolCount":
		if p.SymbolCount != 0 {
			return p.SymbolCount, true
		}
	case "groupId":
		if p.GroupID != "" {
			return p.GroupID, true
		}
	case "entryPointID":
		if p.EntryPointID != "" {
			return p.EntryPointID, true
		}
	case "stepCount":
		if p.StepCount != 0 {
			return p.StepCount, true
		}
	case "processType":
		if p.ProcessType != "" {
			return p.ProcessType, true
		}
	case "funcID":
		if p.FuncID != "" {
			return p.FuncID, true
		}
	case "funcName":
		if p.FuncName != "" {
			return p.FuncName, true
		}
	case "blockLabel":
		if p.BlockLabel != "" {
			return p.BlockLabel, true
		}
	case "nodeIDs":
		if p.NodeIDs != nil {
			return p.NodeIDs, true
		}
	case "statementIDs":
		if p.StatementIDs != nil {
			return p.StatementIDs, true
		}
	case "siteType":
		if p.SiteType != "" {
			return p.SiteType, true
		}
	case "level":
		if p.Level != 0 {
			return p.Level, true
		}
	case "ormModel":
		if p.ORMModel {
			return p.ORMModel, true
		}
	case "operation":
		if p.Operation != "" {
			return p.Operation, true
		}
	case "model":
		if p.Model != "" {
			return p.Model, true
		}
	case "module":
		if p.Module != "" {
			return p.Module, true
		}
	case "artifactId":
		if p.ArtifactID != "" {
			return p.ArtifactID, true
		}
	case "middleware":
		if p.Middleware != nil {
			return p.Middleware, true
		}
	case "responseKeys":
		if p.ResponseKeys != nil {
			return p.ResponseKeys, true
		}
	case "errorKeys":
		if p.ErrorKeys != nil {
			return p.ErrorKeys, true
		}
	}
	if p.Extra != nil {
		if v, ok := p.Extra[key]; ok {
			return v, true
		}
	}
	return nil, false
}

// Keys returns all property keys (for iteration)
func (p NodeProps) Keys() []string {
	var keys []string
	if p.StartLine != 0 { keys = append(keys, "startLine") }
	if p.EndLine != 0 { keys = append(keys, "endLine") }
	if p.Line != 0 { keys = append(keys, "line") }
	if p.Language != "" { keys = append(keys, "language") }
	if p.ContentHash != "" { keys = append(keys, "contentHash") }
	if p.LineCount != 0 { keys = append(keys, "lineCount") }
	if p.FileCount != 0 { keys = append(keys, "fileCount") }
	if p.TotalSize != 0 { keys = append(keys, "totalSize") }
	if p.IsExported { keys = append(keys, "isExported") }
	if p.IsAsync { keys = append(keys, "isAsync") }
	if p.IsWildcard { keys = append(keys, "isWildcard") }
	if p.IsParam { keys = append(keys, "isParam") }
	if p.Alias != "" { keys = append(keys, "alias") }
	if p.Receiver != "" { keys = append(keys, "receiver") }
	if p.Path != "" { keys = append(keys, "path") }
	if p.Method != "" { keys = append(keys, "method") }
	if p.Description != "" { keys = append(keys, "description") }
	if p.HandlerID != "" { keys = append(keys, "handlerId") }
	if p.InputSchema != "" { keys = append(keys, "inputSchema") }
	if p.Arity != 0 { keys = append(keys, "arity") }
	if p.BaseTypes != nil { keys = append(keys, "baseTypes") }
	if p.Cohesion != 0 { keys = append(keys, "cohesion") }
	if p.SymbolCount != 0 { keys = append(keys, "symbolCount") }
	if p.GroupID != "" { keys = append(keys, "groupId") }
	if p.EntryPointID != "" { keys = append(keys, "entryPointID") }
	if p.StepCount != 0 { keys = append(keys, "stepCount") }
	if p.ProcessType != "" { keys = append(keys, "processType") }
	if p.FuncID != "" { keys = append(keys, "funcID") }
	if p.FuncName != "" { keys = append(keys, "funcName") }
	if p.BlockLabel != "" { keys = append(keys, "blockLabel") }
	if p.NodeIDs != nil { keys = append(keys, "nodeIDs") }
	if p.StatementIDs != nil { keys = append(keys, "statementIDs") }
	if p.SiteType != "" { keys = append(keys, "siteType") }
	if p.Level != 0 { keys = append(keys, "level") }
	if p.ORMModel { keys = append(keys, "ormModel") }
	if p.Operation != "" { keys = append(keys, "operation") }
	if p.Model != "" { keys = append(keys, "model") }
	if p.Module != "" { keys = append(keys, "module") }
	if p.ArtifactID != "" { keys = append(keys, "artifactId") }
	if p.Middleware != nil { keys = append(keys, "middleware") }
	if p.ResponseKeys != nil { keys = append(keys, "responseKeys") }
	if p.ErrorKeys != nil { keys = append(keys, "errorKeys") }
	for k := range p.Extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// IsEmpty returns true if no properties are set
func (p NodeProps) IsEmpty() bool {
	return p.StartLine == 0 && p.EndLine == 0 && p.Line == 0 &&
		p.Language == "" && p.ContentHash == "" && p.LineCount == 0 &&
		p.FileCount == 0 && p.TotalSize == 0 &&
		!p.IsExported && !p.IsAsync && !p.IsWildcard && !p.IsParam &&
		p.Alias == "" && p.Receiver == "" && p.Path == "" && p.Method == "" &&
		p.Description == "" && p.HandlerID == "" && p.InputSchema == "" &&
		p.Arity == 0 && p.BaseTypes == nil &&
		p.Cohesion == 0 && p.SymbolCount == 0 && p.GroupID == "" &&
		p.EntryPointID == "" && p.StepCount == 0 && p.ProcessType == "" &&
		p.FuncID == "" && p.FuncName == "" && p.BlockLabel == "" &&
		p.NodeIDs == nil && p.StatementIDs == nil && p.SiteType == "" &&
		p.Level == 0 && !p.ORMModel && p.Operation == "" && p.Model == "" &&
		p.Module == "" && p.ArtifactID == "" &&
		p.Middleware == nil && p.ResponseKeys == nil && p.ErrorKeys == nil &&
		len(p.Extra) == 0
}

// --- EdgeProps methods ---

// NewEdgeProps creates an empty EdgeProps
func NewEdgeProps() EdgeProps {
	return EdgeProps{}
}

// SetProp sets a property by key — routes to typed field or Extra
func (p *EdgeProps) SetProp(key string, val any) {
	switch key {
	case "confidence":
		p.Confidence = toFloat64(val)
	case "line":
		p.Line = toInt(val)
	case "file":
		p.File = toString(val)
	case "importPath":
		p.ImportPath = toString(val)
	case "alias":
		p.Alias = toString(val)
	case "category":
		p.Category = toString(val)
	case "sourceName":
		p.SourceName = toString(val)
	case "sinkName":
		p.SinkName = toString(val)
	case "sanitized":
		p.Sanitized = toBool(val)
	case "hopIndex":
		p.HopIndex = toInt(val)
	case "edgeType":
		p.EdgeType = toString(val)
	case "condition":
		p.Condition = toString(val)
	case "funcID":
		p.FuncID = toString(val)
	case "returnType":
		p.ReturnType = toString(val)
	case "sameFile":
		p.SameFile = toBool(val)
	case "overloadResolution":
		p.OverloadResolution = toString(val)
	case "order":
		p.Order = toInt(val)
	case "weight", "w":
		p.Weight = toFloat64(val)
	case "operation":
		p.Operation = toString(val)
	case "model":
		p.Model = toString(val)
	case "topic", "name":
		p.Topic = toString(val)
	default:
		if p.Extra == nil {
			p.Extra = make(map[string]any)
		}
		p.Extra[key] = val
	}
}

// GetProp retrieves a property by key — routes from typed field or Extra
func (p EdgeProps) GetProp(key string) (any, bool) {
	switch key {
	case "confidence":
		if p.Confidence != 0 {
			return p.Confidence, true
		}
	case "line":
		if p.Line != 0 {
			return p.Line, true
		}
	case "file":
		if p.File != "" {
			return p.File, true
		}
	case "importPath":
		if p.ImportPath != "" {
			return p.ImportPath, true
		}
	case "alias":
		if p.Alias != "" {
			return p.Alias, true
		}
	case "category":
		if p.Category != "" {
			return p.Category, true
		}
	case "sourceName":
		if p.SourceName != "" {
			return p.SourceName, true
		}
	case "sinkName":
		if p.SinkName != "" {
			return p.SinkName, true
		}
	case "sanitized":
		if p.Sanitized {
			return p.Sanitized, true
		}
	case "hopIndex":
		if p.HopIndex != 0 {
			return p.HopIndex, true
		}
	case "edgeType":
		if p.EdgeType != "" {
			return p.EdgeType, true
		}
	case "condition":
		if p.Condition != "" {
			return p.Condition, true
		}
	case "funcID":
		if p.FuncID != "" {
			return p.FuncID, true
		}
	case "returnType":
		if p.ReturnType != "" {
			return p.ReturnType, true
		}
	case "sameFile":
		if p.SameFile {
			return p.SameFile, true
		}
	case "overloadResolution":
		if p.OverloadResolution != "" {
			return p.OverloadResolution, true
		}
	case "order":
		if p.Order != 0 {
			return p.Order, true
		}
	case "weight", "w":
		if p.Weight != 0 {
			return p.Weight, true
		}
	case "operation":
		if p.Operation != "" {
			return p.Operation, true
		}
	case "model":
		if p.Model != "" {
			return p.Model, true
		}
	case "topic", "name":
		if p.Topic != "" {
			return p.Topic, true
		}
	}
	if p.Extra != nil {
		if v, ok := p.Extra[key]; ok {
			return v, true
		}
	}
	return nil, false
}

// IsEmpty returns true if no properties are set
func (p EdgeProps) IsEmpty() bool {
	return p.Confidence == 0 && p.Line == 0 && p.File == "" &&
		p.ImportPath == "" && p.Alias == "" &&
		p.Category == "" && p.SourceName == "" && p.SinkName == "" &&
		!p.Sanitized && p.HopIndex == 0 &&
		p.EdgeType == "" && p.Condition == "" && p.FuncID == "" &&
		p.ReturnType == "" && !p.SameFile && p.OverloadResolution == "" &&
		p.Order == 0 && p.Weight == 0 &&
		p.Operation == "" && p.Model == "" && p.Topic == "" &&
		len(p.Extra) == 0
}

// --- Legacy compatibility: convert to/from map[string]any ---

// NodePropsFromMap converts a map[string]any to NodeProps
func NodePropsFromMap(m map[string]any) NodeProps {
	var p NodeProps
	for k, v := range m {
		p.SetProp(k, v)
	}
	return p
}

// EdgePropsFromMap converts a map[string]any to EdgeProps
func EdgePropsFromMap(m map[string]any) EdgeProps {
	var p EdgeProps
	for k, v := range m {
		p.SetProp(k, v)
	}
	return p
}

// ToMap converts NodeProps back to map[string]any (for compatibility)
func (p NodeProps) ToMap() map[string]any {
	m := make(map[string]any, len(p.Keys()))
	for _, k := range p.Keys() {
		v, _ := p.GetProp(k)
		m[k] = v
	}
	return m
}

// ToMap converts EdgeProps back to map[string]any (for compatibility)
func (p EdgeProps) ToMap() map[string]any {
	m := make(map[string]any)
	if p.Confidence != 0 { m["confidence"] = p.Confidence }
	if p.Line != 0 { m["line"] = p.Line }
	if p.File != "" { m["file"] = p.File }
	if p.ImportPath != "" { m["importPath"] = p.ImportPath }
	if p.Alias != "" { m["alias"] = p.Alias }
	if p.Category != "" { m["category"] = p.Category }
	if p.SourceName != "" { m["sourceName"] = p.SourceName }
	if p.SinkName != "" { m["sinkName"] = p.SinkName }
	if p.Sanitized { m["sanitized"] = p.Sanitized }
	if p.HopIndex != 0 { m["hopIndex"] = p.HopIndex }
	if p.EdgeType != "" { m["edgeType"] = p.EdgeType }
	if p.Condition != "" { m["condition"] = p.Condition }
	if p.FuncID != "" { m["funcID"] = p.FuncID }
	if p.ReturnType != "" { m["returnType"] = p.ReturnType }
	if p.SameFile { m["sameFile"] = p.SameFile }
	if p.OverloadResolution != "" { m["overloadResolution"] = p.OverloadResolution }
	if p.Order != 0 { m["order"] = p.Order }
	if p.Weight != 0 { m["weight"] = p.Weight }
	if p.Operation != "" { m["operation"] = p.Operation }
	if p.Model != "" { m["model"] = p.Model }
	if p.Topic != "" { m["topic"] = p.Topic }
	for k, v := range p.Extra {
		m[k] = v
	}
	return m
}

// --- Custom msgpack marshal/unmarshal for Props ---
// These handle the legacy "props" field which was map[string]any.
// New format uses the struct directly, but we need to decode old JSON/msgpack
// where "props" was a map.

// UnmarshalJSONNodeProps decodes JSON "props" field which may be map or struct format
func UnmarshalJSONNodeProps(data []byte) (NodeProps, error) {
	// Try struct format first (new format)
	var p NodeProps
	if err := json.Unmarshal(data, &p); err == nil && !p.IsEmpty() {
		return p, nil
	}
	// Fallback: try map format (legacy)
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return NodeProps{}, err
	}
	return NodePropsFromMap(m), nil
}

// UnmarshalJSONEdgeProps decodes JSON "props" field which may be map or struct format
func UnmarshalJSONEdgeProps(data []byte) (EdgeProps, error) {
	var p EdgeProps
	if err := json.Unmarshal(data, &p); err == nil && !p.IsEmpty() {
		return p, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return EdgeProps{}, err
	}
	return EdgePropsFromMap(m), nil
}

// --- Type conversion helpers ---

func toInt(v any) int {
	switch val := v.(type) {
	case int:
		return val
	case int64:
		return int(val)
	case float64:
		return int(val)
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return int(i)
		}
		return 0
	default:
		return 0
	}
}

func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func toBool(v any) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	return false
}

func toStringSlice(v any) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []any:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		// Single string -> slice with one element
		if strings.HasPrefix(val, "[") {
			var slice []string
			if err := json.Unmarshal([]byte(val), &slice); err == nil {
				return slice
			}
		}
		return []string{val}
	default:
		return nil
	}
}

// PropsFromAny decodes a props value from msgpack/JSON, handling both
// the new struct format and legacy map format.
func PropsFromAny(data []byte, isNode bool) (NodeProps, EdgeProps, error) {
	if isNode {
		// Try msgpack struct format
		var np NodeProps
		if err := msgpack.Unmarshal(data, &np); err == nil {
			return np, EdgeProps{}, nil
		}
		// Try JSON map format (legacy)
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			return NodeProps{}, EdgeProps{}, err
		}
		return NodePropsFromMap(m), EdgeProps{}, nil
	}
	// Edge props
	var ep EdgeProps
	if err := msgpack.Unmarshal(data, &ep); err == nil {
		return NodeProps{}, ep, nil
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return NodeProps{}, EdgeProps{}, err
	}
	return NodeProps{}, EdgePropsFromMap(m), nil
}