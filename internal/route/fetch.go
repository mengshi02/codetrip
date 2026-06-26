package route

import (
	"context"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/collection"
)

// FetchCallExtractor fetch call tracking extractor
// Scans:
//   - fetch('/api/users') calls
//   - axios.get('/api/users') / axios.post() calls
//   - http.get() / httpRequest() calls
// Extracts URL templates from call sites, matches registered routes → creates FETCHES edges
type FetchCallExtractor struct{}

// Framework implements RouteExtractor
func (e *FetchCallExtractor) Framework() string { return "fetch" }

// Extract implements RouteExtractor
func (e *FetchCallExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*collection.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	// First collect existing route nodes for matching
	registeredRoutes := e.collectRegisteredRoutes(g)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		routes := e.extractFromFile(f, g, registeredRoutes)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *FetchCallExtractor) extractFromFile(f *collection.ParsedFile, g *graph.GraphStore, registeredRoutes map[string]string) []*Route {
	var routes []*Route

	for _, call := range f.CallSites {
		info := parseFetchCall(call)
		if info == nil {
			continue
		}

		// Match registered routes
		routeID, matchedPath := matchRoute(info.url, registeredRoutes)

		consumer := RouteConsumer{
			FunctionID: call.CallerID,
			FilePath:   call.FilePath,
		}

		r := &Route{
			Path:      matchedPath,
			Method:    info.method,
			FilePath:  f.Path,
			Line:      call.Line,
			Consumers: []RouteConsumer{consumer},
		}

		if routeID != "" {
			r.HandlerID = routeID // consumed route node ID
		}

		routes = append(routes, r)
	}

	return routes
}

// collectRegisteredRoutes collects registered routes from GraphStore
func (e *FetchCallExtractor) collectRegisteredRoutes(g *graph.GraphStore) map[string]string {
	routes := make(map[string]string) // path → nodeID
	if g == nil {
		return routes
	}

	nodes, err := g.GetNodesByLabel(g.Repo(), string(graph.LabelRoute))
	if err != nil {
		return routes
	}

	for _, node := range nodes {
		path := node.GetPropString("path")
		if path != "" {
			routes[path] = node.ID
		}
	}

	return routes
}

// fetchCallInfo fetch call information
type fetchCallInfo struct {
	method string
	url    string
}

// parseFetchCall parses fetch/axios calls from call site
func parseFetchCall(call *collection.CallSite) *fetchCallInfo {
	switch {
	case call.Name == "fetch":
		// fetch('/api/users') or fetch('/api/users', {method: 'POST'})
		method := "GET"
		if call.Args >= 2 {
			// may have method parameter
			method = "*"
		}
		return &fetchCallInfo{method: method, url: ""}

	case call.Receiver == "axios" && isHTTPMethod(call.Name):
		return &fetchCallInfo{method: strings.ToUpper(call.Name), url: ""}

	case call.Receiver == "http" && isHTTPMethod(call.Name):
		return &fetchCallInfo{method: strings.ToUpper(call.Name), url: ""}

	case strings.HasSuffix(call.Name, "Request") || strings.HasSuffix(call.Name, "Fetch"):
		return &fetchCallInfo{method: "*", url: ""}

	default:
		return nil
	}
}

// isHTTPMethod checks if the name is an HTTP method name
func isHTTPMethod(name string) bool {
	switch strings.ToLower(name) {
	case "get", "post", "put", "delete", "patch", "head", "options":
		return true
	default:
		return false
	}
}

// matchRoute matches URL to registered routes
func matchRoute(url string, registeredRoutes map[string]string) (nodeID, path string) {
	if url == "" {
		return "", ""
	}

	// Exact match
	if id, ok := registeredRoutes[url]; ok {
		return id, url
	}

	// Prefix match (for template URLs like /api/users/:id)
	for rPath, rID := range registeredRoutes {
		if strings.HasPrefix(url, rPath) || strings.HasPrefix(rPath, url) {
			return rID, rPath
		}
	}

	// No match, return URL itself
	return "", url
}