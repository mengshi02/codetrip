package cfg

// TypeScriptHarvest extracts sites (calls, constructions, member-reads)
// from a TypeScript function AST for taint analysis.
//
// Mirrors TS cfg/visitors/typescript-harvest.ts.
//
// The harvested sites are stored in FunctionCFG.Sites and used by the
// taint analysis engine to identify source→sink data flow paths.
//
// Current status: skeleton — full implementation deferred.
type TypeScriptHarvest struct{}

// NewTypeScriptHarvest creates a new TypeScript harvest extractor.
func NewTypeScriptHarvest() *TypeScriptHarvest {
	return &TypeScriptHarvest{}
}

// HarvestSites extracts all call/construct/member-read sites from the
// given function's source code.
//
// Current status: skeleton — full implementation deferred.
func (h *TypeScriptHarvest) HarvestSites(sourceCode string) ([]SiteRecord, error) {
	_ = sourceCode
	// TODO: parse sourceCode with tree-sitter TypeScript grammar
	// Walk AST to find:
	//   - Call expressions → SiteType="call"
	//   - New expressions → SiteType="construct"
	//   - Member access expressions → SiteType="member-read"
	return nil, nil
}