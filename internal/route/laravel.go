package route

import (
	"context"
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/collection"
)

// LaravelRouteExtractor Laravel route extractor
// Scans Route::get/Route::post/Route::put/Route::delete/Route::patch calls
// Parses route files routes/web.php, routes/api.php
type LaravelRouteExtractor struct{}

// Framework implements RouteExtractor
func (e *LaravelRouteExtractor) Framework() string { return "laravel" }

// Extract implements RouteExtractor
func (e *LaravelRouteExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*collection.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !isLaravelRouteFile(f.Path) {
			continue
		}

		routes := e.extractFromFile(f, g)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *LaravelRouteExtractor) extractFromFile(f *collection.ParsedFile, g *graph.GraphStore) []*Route {
	var routes []*Route

	// Extract Route::get/post/put/delete/patch from call sites
	for _, call := range f.CallSites {
		info := parseLaravelRouteCall(call)
		if info == nil {
			continue
		}

		handlerID := findHandlerNodeID(f, info.handlerName, g)

		r := &Route{
			Path:      info.path,
			Method:    info.method,
			HandlerID: handlerID,
			FilePath:  f.Path,
			Line:      call.Line,
		}

		// Detect middleware
		if info.middleware != nil {
			r.Middleware = info.middleware
		}

		routes = append(routes, r)
	}

	return routes
}

// ============ Laravel Helpers ============

var (
	laravelRouteFilePattern = regexp.MustCompile(`routes/(web|api|console|channels)\.php$`)
)

// isLaravelRouteFile checks if the file is a Laravel route file
func isLaravelRouteFile(path string) bool {
	return laravelRouteFilePattern.MatchString(path)
}

// laravelRouteCallInfo Laravel route call information
type laravelRouteCallInfo struct {
	method      string
	path        string
	handlerName string
	middleware  []string
}

// parseLaravelRouteCall parses Laravel route from call site
func parseLaravelRouteCall(call *collection.CallSite) *laravelRouteCallInfo {
	// Route::get('/path', [Controller::class, 'method'])
	// Route::post('/path', 'Controller@method')
	// Route::middleware(['auth'])->get('/path', ...)
	if call.Receiver != "Route" {
		return nil
	}

	method := laravelMethodFromCall(call.Name)
	if method == "" {
		return nil
	}

	// Infer path from call site (requires parameter information)
	// Actual path inference from CallSite.Args is limited, use symbol information as supplement
	path := ""
	handlerName := ""

	// Try to infer handler name from call
	if call.Args >= 2 {
		// Standard form: Route::get('/path', handler)
		// handler could be a closure or controller reference
		handlerName = call.Name // fallback
	}

	return &laravelRouteCallInfo{
		method:      method,
		path:        path,
		handlerName: handlerName,
	}
}

// laravelMethodFromCall maps Laravel Route method calls to HTTP methods
func laravelMethodFromCall(name string) string {
	switch strings.ToLower(name) {
	case "get", "post", "put", "delete", "patch", "options":
		return strings.ToUpper(name)
	case "any":
		return "*"
	case "match":
		return "MATCH" // needs parameter to specify exact method
	default:
		return ""
	}
}

// findHandlerNodeID finds the node ID for Laravel controller method
func findHandlerNodeID(f *collection.ParsedFile, handlerName string, g *graph.GraphStore) string {
	if handlerName == "" {
		return ""
	}

	// First search in current file symbols
	for _, sym := range f.Symbols {
		if sym.Name == handlerName {
			return sym.NodeID
		}
	}

	// Search by name through GraphStore
	if g != nil {
		nodes, err := g.GetNodesByName(g.Repo(), handlerName)
		if err == nil && len(nodes) > 0 {
			return nodes[0].ID
		}
	}

	return ""
}
