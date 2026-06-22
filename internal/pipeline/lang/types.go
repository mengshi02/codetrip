package lang

import "github.com/mengshi02/codetrip/internal/pipeline"

// Type aliases pointing to pipeline package types.
// This avoids circular dependency: pipeline doesn't import codetrip,
// but lang providers can still use these familiar type names.

type CapturesConfig = pipeline.LangCapturesConfig
type CallExtractConfig = pipeline.LangCallExtractConfig
type ClassExtractConfig = pipeline.LangClassExtractConfig
type FieldExtractConfig = pipeline.LangFieldExtractConfig
type ImportResolveConfig = pipeline.LangImportResolveConfig
type CaptureResult = pipeline.LangCaptureResult
type Capture = pipeline.LangCapture
type InterpretResult = pipeline.LangInterpretResult
type ImportInfo = pipeline.LangImportInfo
type CallSite = pipeline.LangCallSiteInfo
type ClassInfo = pipeline.LangClassInfo
type FieldInfo = pipeline.LangFieldInfo

// Scope-based pipeline types
type QuerySet = pipeline.LangQuerySet
type ScopeInfo = pipeline.ScopeInfo
type TypeBindingInfo = pipeline.TypeBindingInfo
type ReferenceInfo = pipeline.ReferenceInfo
type ParamInfo = pipeline.ParamInfo

// Scope resolution types (moved from codetrip root to pipeline package)
type BindingSet = pipeline.BindingSet
type ScopeModel = pipeline.MutableSemanticModel
type ImportRef = pipeline.ImportInfo
type CallSiteRef = pipeline.CallSite
type FileSet = []*pipeline.ParsedFile
type IndexSet = pipeline.ScopeResolutionIndexes
type ScopeMapType = pipeline.ScopeResolutionIndexes
type RangeBindContext = pipeline.RangeBindContext
type ScopeContextOptions = pipeline.ScopeContextOptions
type EmitContext = pipeline.EmitContext
type GraphNodeLookup = pipeline.GraphNodeLookup

// NewBindingSet delegates to pipeline.NewBindingSet
var NewBindingSet = pipeline.NewBindingSet

// ImportSemantics and its constants (re-exported from pipeline)
type ImportSemantics = pipeline.ImportSemantics

const (
	ImportSemanticsNamed              = pipeline.ImportSemanticsNamed
	ImportSemanticsWildcardLeaf       = pipeline.ImportSemanticsWildcardLeaf
	ImportSemanticsWildcardTransitive = pipeline.ImportSemanticsWildcardTransitive
	ImportSemanticsNamespace          = pipeline.ImportSemanticsNamespace
)