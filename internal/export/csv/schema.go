package csv

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/model"
)

type NodeTableName string

var NodeTables = []NodeTableName{
	"File", "Folder", "Function", "Class", "Interface",
	"Method", "Enum", "Struct", "Trait", "Macro",
	"Namespace", "Module", "Variable", "Const", "Static",
	"Typedef", "Constant", "TypeAlias", "Union", "Impl",
	"Record", "Delegate", "Annotation", "Template",
	"Decorator", "Package", "Mixin", "Extension", "Protocol",
	"Category", "ProtocolExtension", "Config",
}

// MultiLangTypes lists types that use the MultiLangHeader CSV format,
// Original: ['Struct', 'Enum', 'Macro', 'Typedef', 'Union', 'Namespace', 'Trait', 'Impl',
//
//	'TypeAlias', 'Const', 'Static', 'Property', 'Record', 'Delegate', 'Annotation', 'Constructor', 'Template', 'Module']
var MultiLangTypes = []NodeTableName{
	"Struct", "Enum", "Macro", "Typedef", "Union", "Namespace", "Trait", "Impl",
	"TypeAlias", "Const", "Static", "Property", "Record", "Delegate",
	"Annotation", "Constructor", "Template", "Module",
}

var CodeElementTypes = []NodeTableName{
	"Function", "Class", "Interface", "CodeElement",
}

const (
	FileHeader        = "id,name,filePath,content"
	FolderHeader      = "id,name,filePath"
	CodeElementHeader = "id,name,filePath,startLine,endLine,isExported,content,description"
	MethodHeader      = "id,name,filePath,startLine,endLine,isExported,content,description,parameterCount,returnType"
	CommunityHeader   = "id,label,heuristicLabel,keywords,description,enrichedBy,cohesion,symbolCount"
	ProcessHeader     = "id,label,heuristicLabel,processType,stepCount,communities,entryPointId,terminalId"
	MultiLangHeader   = "id,name,filePath,startLine,endLine,content,description"
	RelationHeader    = "from,to,type,confidence,reason,step"
)

func HeaderForLabel(label model.NodeLabel) string {
	switch label {
	case model.LabelFile:
		return FileHeader
	case model.LabelFolder:
		return FolderHeader
	case model.LabelMethod:
		return MethodHeader
	case model.LabelCommunity:
		return CommunityHeader
	case model.LabelProcess:
		return ProcessHeader
	default:
		if IsMultiLangType(string(label)) {
			return MultiLangHeader
		}
		if IsCodeElementType(string(label)) {
			return CodeElementHeader
		}
		return MultiLangHeader
	}
}

func IsMultiLangType(label string) bool {
	for _, t := range MultiLangTypes {
		if string(t) == label {
			return true
		}
	}
	return false
}

func IsCodeElementType(label string) bool {
	for _, t := range CodeElementTypes {
		if string(t) == label {
			return true
		}
	}
	return false
}

func LabelToTableName(label model.NodeLabel) NodeTableName {
	return NodeTableName(label)
}

func TableNameToFilename(name NodeTableName) string {
	return strings.ToLower(string(name))
}
