package importresolvers

// CreateImportResolver creates an ImportResolverFn from a declarative config.
//
// Chains strategies in declaration order — first non-nil result wins.
// Returns nil only if every strategy returns nil.
//
// Error behaviour: if a strategy panics, the panic propagates immediately
// and remaining strategies are not tried. Strategies are expected to be
// pure data transforms that never panic; any unexpected panic indicates
// a bug in the strategy implementation.
func CreateImportResolver(config ImportResolutionConfig) ImportResolverFn {
	strategies := config.Strategies
	return func(rawImportPath, filePath string, ctx *ResolveCtx) *ImportResult {
		for _, strategy := range strategies {
			result := strategy(rawImportPath, filePath, ctx)
			if result != nil {
				return result
			}
		}
		return nil
	}
}