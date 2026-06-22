package route

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// ExpoRouterExtractor Expo Router extractor
// Based on app/ directory structure (similar to Next.js App Router):
//   - app/page.tsx → /
//   - app/about/page.tsx → /about
//   - app/[id]/page.tsx → /:id
//   - _layout.tsx → layout component
//   - _sitemap.ts → sitemap
type ExpoRouterExtractor struct{}

// Framework implements RouteExtractor
func (e *ExpoRouterExtractor) Framework() string { return "expo-router" }

// Extract implements RouteExtractor
func (e *ExpoRouterExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !isExpoRouterFile(f.Path) {
			continue
		}

		routes := e.extractFromFile(f, g)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *ExpoRouterExtractor) extractFromFile(f *pipeline.ParsedFile, g *graph.GraphStore) []*Route {
	relPath := extractExpoAppRelPath(f.Path)
	if relPath == "" {
		return nil
	}

	routePath := expoPathToRoute(relPath)
	base := filepath.Base(f.Path)

	// Special files do not generate routes
	if strings.HasPrefix(base, "_") {
		if base == "_layout.tsx" || base == "_layout.ts" {
			// Layout files are recorded as middleware, not directly generating routes
			return nil
		}
		if base == "_sitemap.ts" || base == "_sitemap.tsx" {
			return []*Route{{
				Path:      routePath,
				Method:    "GET",
				FilePath:  f.Path,
				Line:      1,
				HandlerID: findDefaultExportHandler(f, g),
			}}
		}
		return nil
	}

	handlerID := findDefaultExportHandler(f, g)
	middleware := detectExpoLayoutMiddleware(f.Path)

	r := &Route{
		Path:       routePath,
		Method:     "GET",
		HandlerID:  handlerID,
		FilePath:   f.Path,
		Line:       1,
		Middleware: middleware,
	}

	return []*Route{r}
}

// ============ Expo Helper Functions ============

var (
	expoAppPattern    = regexp.MustCompile(`/app/(.+)`)
	expoDynamicSeg    = regexp.MustCompile(`\[(\.\.\.)?([^\]]+)\]`)
)

// isExpoRouterFile checks if the file is an Expo Router file
func isExpoRouterFile(path string) bool {
	if !expoAppPattern.MatchString(path) {
		return false
	}
	return strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".ts") ||
		strings.HasSuffix(path, ".jsx") || strings.HasSuffix(path, ".js")
}

// extractExpoAppRelPath extracts relative path under app/ (directory part)
func extractExpoAppRelPath(path string) string {
	m := expoAppPattern.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	dir := filepath.Dir(m[1])
	if dir == "." {
		return ""
	}
	return dir
}

// expoPathToRoute converts Expo Router directory path to route path
func expoPathToRoute(relPath string) string {
	path := expoDynamicSeg.ReplaceAllStringFunc(relPath, func(match string) string {
		sub := expoDynamicSeg.FindStringSubmatch(match)
		if sub[1] != "" {
			return ":" + sub[2] + "@" // catch-all
		}
		return ":" + sub[2]
	})

	if path == "" || path == "." {
		return "/"
	}
	return "/" + path
}

// detectExpoLayoutMiddleware detects Expo _layout.tsx middleware chain
func detectExpoLayoutMiddleware(filePath string) []string {
	dir := filepath.Dir(filePath)
	var middleware []string
	for {
		parent := filepath.Dir(dir)
		if parent == dir || !strings.Contains(dir, "app") {
			break
		}
		layoutPath := filepath.Join(dir, "_layout.tsx")
		middleware = append(middleware, layoutPath)
		dir = parent
	}
	return middleware
}