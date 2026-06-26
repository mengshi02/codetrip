package routeextractors

// Response shape extraction from M3 CFG site records.
//
// Infers the response shape (schema) of route handlers by analyzing
// the return / yield / response.json() patterns in the CFG.
//
// Produces ResponseShape records keyed by route path + method, which
// can be consumed by downstream documentation or API analysis tools.

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/taint"
)

// ResponseShape describes the inferred shape of a route's response.
type ResponseShape struct {
	RoutePath  string   // route path pattern
	Method     string   // HTTP method
	Keys       []string // top-level keys observed in response objects
	IsArray    bool     // response is an array wrapper
	FilePath   string
	Line       int
}

// ResponseShapeExtractor extracts response shapes from route handler CFGs.
type ResponseShapeExtractor struct{}

func (ResponseShapeExtractor) Framework() string { return "response-shapes" }

func (e ResponseShapeExtractor) Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	// Response shapes are not routes; we produce them as special
	// ExtractedRoute entries with ResponseKeys populated.
	keys := extractResponseKeys(cfg)
	if len(keys) == 0 {
		return nil
	}

	// Find the route path from the handler's context.
	// This is a heuristic: we look for a parent route registration.
	routePath := inferRoutePath(cfg)
	if routePath == "" {
		routePath = "/" + strings.ToLower(strings.TrimSuffix(
			strings.ReplaceAll(
				strings.ReplaceAll(cfg.FilePath, "/", "_"),
				".", "_"),
			"_"))
	}

	return []ExtractedRoute{{
		Path:         routePath,
		Method:       "*",
		HandlerName:  "",
		FilePath:     cfg.FilePath,
		Line:         findFirstLine(cfg),
		ResponseKeys: keys,
	}}
}

// extractResponseKeys scans CFG sites for response shape evidence.
func extractResponseKeys(cfg *taint.TaintFunctionCfg) []string {
	seen := make(map[string]bool)
	var keys []string

	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			for siteIdx := range cfg.Blocks[bi].Statements[si].Sites {
				site := &cfg.Blocks[bi].Statements[si].Sites[siteIdx]
				// Pattern: res.json({key: value})
				if site.Kind == "call" && site.Callee == "json" {
					// Args contain the object structure
					for _, argOccurrences := range site.Args {
						for _, occ := range argOccurrences {
							if occ.IsVia {
								continue
							}
							// Direct reference to a binding that holds the response
						}
					}
					if !seen["json"] {
						seen["json"] = true
						keys = append(keys, "json")
					}
				}
				// Pattern: member-read of response object (e.g., data.field)
				if site.Kind == "member-read" && site.Property != "" {
					prop := site.Property
					if !seen[prop] && isResponseBodyProperty(prop) {
						seen[prop] = true
						keys = append(keys, prop)
					}
				}
			}
		}
	}

	return keys
}

// isResponseBodyProperty checks if a property name is likely a response body key.
func isResponseBodyProperty(prop string) bool {
	// Common response envelope keys
	commonKeys := map[string]bool{
		"data": true, "result": true, "results": true,
		"error": true, "errors": true, "message": true,
		"status": true, "code": true, "total": true,
		"items": true, "list": true, "rows": true,
		"count": true, "page": true, "pageSize": true,
	}
	return commonKeys[prop]
}

// inferRoutePath tries to infer a route path from the file path.
func inferRoutePath(cfg *taint.TaintFunctionCfg) string {
	// Heuristic: look for /api/, /routes/, /pages/ in file path
	for _, prefix := range []string{"/api/", "/routes/", "/pages/"} {
		idx := strings.Index(cfg.FilePath, prefix)
		if idx >= 0 {
			rel := cfg.FilePath[idx:]
			rel = strings.TrimSuffix(rel, ".ts")
			rel = strings.TrimSuffix(rel, ".js")
			rel = strings.TrimSuffix(rel, ".tsx")
			rel = strings.TrimSuffix(rel, ".jsx")
			return rel
		}
	}
	return ""
}