package route

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ============ Next.js Pages Router ============

// NextJSPagesExtractor Next.js Pages Router extractor
// Based on /pages/ directory file system routing:
//   - pages/api/*.js → API route
//   - pages/*.tsx → page route
//   - dynamic route [id] → :id
type NextJSPagesExtractor struct{}

// Framework implements RouteExtractor
func (e *NextJSPagesExtractor) Framework() string { return "nextjs-pages" }

// Extract implements RouteExtractor
func (e *NextJSPagesExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !isNextJSPagesFile(f.Path) {
			continue
		}

		routes := e.extractFromFile(f, g)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *NextJSPagesExtractor) extractFromFile(f *pipeline.ParsedFile, g *graph.GraphStore) []*Route {
	relPath := extractPagesRelPath(f.Path)
	if relPath == "" {
		return nil
	}

	routePath := pagesPathToRoute(relPath)
	isAPI := strings.HasPrefix(relPath, "api/")

	method := "GET"
	if isAPI {
		method = "*" // API route supports all HTTP methods
	}

	// Find default export function as handler
	handlerID := findDefaultExportHandler(f, g)

	r := &Route{
		Path:      routePath,
		Method:    method,
		HandlerID: handlerID,
		FilePath:  f.Path,
		Line:      1,
	}

	if isAPI {
		r.ResponseKeys = []string{"json"}
	}

	return []*Route{r}
}

// ============ Next.js App Router ============

// NextJSAppExtractor Next.js App Router extractor
// Based on /app/ directory routing:
//   - page.tsx → GET page route
//   - route.ts → API handler (GET/POST/PUT/DELETE)
//   - layout.tsx → layout component
type NextJSAppExtractor struct{}

// Framework implements RouteExtractor
func (e *NextJSAppExtractor) Framework() string { return "nextjs-app" }

// Extract implements RouteExtractor
func (e *NextJSAppExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !isNextJSAppFile(f.Path) {
			continue
		}

		routes := e.extractFromFile(f, g)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *NextJSAppExtractor) extractFromFile(f *pipeline.ParsedFile, g *graph.GraphStore) []*Route {
	relPath := extractAppRelPath(f.Path)
	if relPath == "" {
		return nil
	}

	routePath := appPathToRoute(relPath)
	base := filepath.Base(f.Path)
	isRouteTS := base == "route.ts" || base == "route.js"

	var routes []*Route

	if isRouteTS {
		// route.ts → detect exported HTTP methods
		methods := detectExportedMethods(f)
		if len(methods) == 0 {
			methods = []string{"GET", "POST", "PUT", "DELETE"}
		}
		for _, m := range methods {
			handlerID := findNamedExportHandler(f, m, g)
			routes = append(routes, &Route{
				Path:       routePath,
				Method:     m,
				HandlerID:  handlerID,
				FilePath:   f.Path,
				Line:       1,
				Middleware: detectLayoutMiddleware(f.Path, "app"),
			})
		}
	} else {
		// page.tsx → GET page route
		handlerID := findDefaultExportHandler(f, g)
		routes = append(routes, &Route{
			Path:       routePath,
			Method:     "GET",
			HandlerID:  handlerID,
			FilePath:   f.Path,
			Line:       1,
			Middleware: detectLayoutMiddleware(f.Path, "app"),
		})
	}

	return routes
}

// ============ Helper Functions ============

var (
	nextJSPagesPattern = regexp.MustCompile(`/pages/(.+)`)
	nextJSAppPattern   = regexp.MustCompile(`/app/(.+)`)
	dynamicSegment     = regexp.MustCompile(`\[(\.\.\.)?([^\]]+)\]`)
)

// isNextJSPagesFile checks if the file is a Next.js Pages Router file
func isNextJSPagesFile(path string) bool {
	return nextJSPagesPattern.MatchString(path) &&
		(strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".jsx") ||
			strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx"))
}

// isNextJSAppFile checks if the file is a Next.js App Router file
func isNextJSAppFile(path string) bool {
	if !nextJSAppPattern.MatchString(path) {
		return false
	}
	base := filepath.Base(path)
	return base == "page.tsx" || base == "page.jsx" || base == "route.ts" || base == "route.js"
}

// extractPagesRelPath extracts relative path under pages/
func extractPagesRelPath(path string) string {
	m := nextJSPagesPattern.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

// extractAppRelPath extracts relative path under app/
func extractAppRelPath(path string) string {
	m := nextJSAppPattern.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	// Remove filename, keep only directory path
	dir := filepath.Dir(m[1])
	if dir == "." {
		return ""
	}
	return dir
}

// pagesPathToRoute converts Pages Router file path to route path
func pagesPathToRoute(relPath string) string {
	// Remove extension
	noExt := stripExt(relPath)

	// index → root path
	if noExt == "index" || filepath.Base(noExt) == "index" {
		noExt = filepath.Dir(noExt)
		if noExt == "." {
			noExt = ""
		}
	}

	// Dynamic route [id] → :id, [...slug] → :slug@
	path := dynamicSegment.ReplaceAllStringFunc(noExt, func(match string) string {
		sub := dynamicSegment.FindStringSubmatch(match)
		if sub[1] != "" {
			return ":" + sub[2] + "@" // catch-all parameter marker
		}
		return ":" + sub[2]
	})

	if path == "" {
		return "/"
	}
	return "/" + path
}

// appPathToRoute converts App Router directory path to route path
func appPathToRoute(relPath string) string {
	path := dynamicSegment.ReplaceAllStringFunc(relPath, func(match string) string {
		sub := dynamicSegment.FindStringSubmatch(match)
		if sub[1] != "" {
			return ":" + sub[2] + "@"
		}
		return ":" + sub[2]
	})

	if path == "" || path == "." {
		return "/"
	}
	return "/" + path
}

// detectExportedMethods detects exported HTTP methods in route.ts
func detectExportedMethods(f *pipeline.ParsedFile) []string {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	var exported []string
	for _, sym := range f.Symbols {
		for _, m := range methods {
			if sym.Name == m && sym.Label == graph.LabelFunction {
				exported = append(exported, m)
				break
			}
		}
	}
	return exported
}

// findDefaultExportHandler finds the node ID of default export function
func findDefaultExportHandler(f *pipeline.ParsedFile, g *graph.GraphStore) string {
	for _, sym := range f.Symbols {
		if sym.Name == "default" || sym.Name == "Default" {
			return sym.NodeID
		}
	}
	return ""
}

// findNamedExportHandler finds the node ID of named export function
func findNamedExportHandler(f *pipeline.ParsedFile, name string, g *graph.GraphStore) string {
	for _, sym := range f.Symbols {
		if sym.Name == name {
			return sym.NodeID
		}
	}
	return ""
}

// detectLayoutMiddleware detects layout middleware (search upward for layout.tsx)
func detectLayoutMiddleware(filePath, rootDir string) []string {
	// Search upward from file path for layout files
	dir := filepath.Dir(filePath)
	var middleware []string
	for {
		parent := filepath.Dir(dir)
		if parent == dir || !strings.Contains(dir, rootDir) {
			break
		}
		layoutPath := filepath.Join(dir, "layout.tsx")
		middleware = append(middleware, layoutPath)
		dir = parent
	}
	return middleware
}

// stripExt removes file extension
func stripExt(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path
	}
	return path[:len(path)-len(ext)]
}