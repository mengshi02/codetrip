package route

import (
	"context"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/collection"
)

// MiddlewareChainExtractor middleware chain extractor
// Scans:
//   - Express: app.use(middleware), router.use(middleware)
//   - Koa: app.use(middleware)
//   - Fastify: fastify.addHook('onRequest', middleware)
//   - Django: middleware_classes configuration
//   - Laravel: $app->middleware([])
type MiddlewareChainExtractor struct{}

// Framework implements RouteExtractor
func (e *MiddlewareChainExtractor) Framework() string { return "middleware" }

// Extract implements RouteExtractor
// Middleware extractor produces virtual routes (Method=MIDDLEWARE) to record middleware chains
func (e *MiddlewareChainExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*collection.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		routes := e.extractFromFile(f, g)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *MiddlewareChainExtractor) extractFromFile(f *collection.ParsedFile, g *graph.GraphStore) []*Route {
	var routes []*Route

	for _, call := range f.CallSites {
		mw := parseMiddlewareCall(call)
		if mw == nil {
			continue
		}

		handlerID := ""
		// Find middleware function node
		for _, sym := range f.Symbols {
			if sym.Name == mw.handlerName {
				handlerID = sym.NodeID
				break
			}
		}

		r := &Route{
			Path:      mw.path,
			Method:    "MIDDLEWARE",
			HandlerID: handlerID,
			Middleware: []string{mw.handlerName},
			FilePath:  f.Path,
			Line:      call.Line,
		}

		routes = append(routes, r)
	}

	return routes
}

// middlewareCallInfo middleware call information
type middlewareCallInfo struct {
	handlerName string
	path        string // middleware mount path (e.g., app.use('/api', auth))
}

// parseMiddlewareCall parses middleware call from call site
func parseMiddlewareCall(call *collection.CallSite) *middlewareCallInfo {
	switch {
	// Express/Koa: app.use(), router.use()
	case call.Name == "use" &&
		(call.Receiver == "app" || call.Receiver == "router" ||
			strings.HasSuffix(call.Receiver, "Router") ||
			strings.HasSuffix(call.Receiver, "router")):
		path := ""
		handlerName := "anonymous"
		if call.Args >= 2 {
			// app.use('/path', middleware) → has path prefix
			path = "/"
		} else if call.Args >= 1 {
			// app.use(middleware) → global middleware
			path = "/"
		}
		return &middlewareCallInfo{handlerName: handlerName, path: path}

	// Fastify: fastify.addHook('onRequest', middleware)
	case call.Name == "addHook" && call.Args >= 2:
		return &middlewareCallInfo{handlerName: "anonymous", path: "/"}

	// Django: MIDDLEWARE = [...] configuration (detected via assignment)
	case call.Name == "MIDDLEWARE" || call.Name == "middleware_classes":
		return &middlewareCallInfo{handlerName: call.Name, path: "/"}

	default:
		return nil
	}
}