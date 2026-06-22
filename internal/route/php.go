package route

import (
	"context"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// PHPFileRouteExtractor PHP file route extractor
// Detects:
//   - $_SERVER['REQUEST_URI'] routing dispatch
//   - switch/case routing pattern
//   - $_GET/$_POST routing parameters
//   - header('Location: ...') redirect
type PHPFileRouteExtractor struct{}

// Framework implements RouteExtractor
func (e *PHPFileRouteExtractor) Framework() string { return "php-file" }

// Extract implements RouteExtractor
func (e *PHPFileRouteExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !isPHPFile(f.Path) {
			continue
		}

		routes := e.extractFromFile(f, g)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *PHPFileRouteExtractor) extractFromFile(f *pipeline.ParsedFile, g *graph.GraphStore) []*Route {
	var routes []*Route

	// Detect $_SERVER['REQUEST_URI'] routing dispatch
	for _, call := range f.CallSites {
		if call.Name == "_SERVER" || call.Receiver == "_SERVER" {
			routes = append(routes, e.extractServerRoutes(f, g)...)
			break // only need to detect once
		}
	}

	// Detect switch/case routing pattern
	for _, sym := range f.Symbols {
		if sym.Label == graph.LabelFunction || sym.Label == graph.LabelMethod {
			r := e.extractSwitchRoute(sym, f, g)
			if r != nil {
				routes = append(routes, r)
			}
		}
	}

	// Detect $_GET/$_POST parameter routing
	for _, call := range f.CallSites {
		if call.Name == "_GET" {
			routes = append(routes, &Route{
				Path:     "/",
				Method:   "GET",
				FilePath: f.Path,
				Line:     call.Line,
			})
		} else if call.Name == "_POST" {
			routes = append(routes, &Route{
				Path:     "/",
				Method:   "POST",
				FilePath: f.Path,
				Line:     call.Line,
			})
		}
	}

	return routes
}

// extractServerRoutes extracts routes from $_SERVER['REQUEST_URI'] pattern
func (e *PHPFileRouteExtractor) extractServerRoutes(f *pipeline.ParsedFile, g *graph.GraphStore) []*Route {
	var routes []*Route

	// Find symbols related to REQUEST_URI
	for _, sym := range f.Symbols {
		if v, ok := sym.Props["routePath"]; ok {
			if path, ok := v.(string); ok && path != "" {
				method := "GET"
				if m, ok := sym.Props["method"]; ok {
					if s, ok := m.(string); ok {
						method = s
					}
				}

				routes = append(routes, &Route{
					Path:      path,
					Method:    method,
					HandlerID: sym.NodeID,
					FilePath:  f.Path,
					Line:      sym.StartLine,
				})
			}
		}
	}

	return routes
}

// extractSwitchRoute extracts routes from switch/case pattern
func (e *PHPFileRouteExtractor) extractSwitchRoute(sym *pipeline.SymbolInfo, f *pipeline.ParsedFile, g *graph.GraphStore) *Route {
	// Check if function properties have switch-case routing information
	path, hasPath := sym.Props["caseValue"]
	if !hasPath {
		return nil
	}

	pathStr, ok := path.(string)
	if !ok || pathStr == "" || !strings.HasPrefix(pathStr, "/") {
		return nil
	}

	return &Route{
		Path:      pathStr,
		Method:    "GET",
		HandlerID: sym.NodeID,
		FilePath:  f.Path,
		Line:      sym.StartLine,
	}
}

// ============ PHP Helpers ============

// isPHPFile checks if the file is a PHP file
func isPHPFile(path string) bool {
	return strings.HasSuffix(path, ".php")
}