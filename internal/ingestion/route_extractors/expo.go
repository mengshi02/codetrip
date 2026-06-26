package routeextractors

// Expo Router route extraction from M3 CFG site records.
//
// Based on app/ directory structure (similar to Next.js App Router):
//   - app/page.tsx → /
//   - app/about/page.tsx → /about
//   - app/[id]/page.tsx → /:id
//   - _layout.tsx → layout component
//   - _sitemap.ts → sitemap

import (
	"path/filepath"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/taint"
)

// ExpoRouterExtractor extracts routes from Expo Router file structure.
type ExpoRouterExtractor struct{}

func (ExpoRouterExtractor) Framework() string { return "expo-router" }

func (e ExpoRouterExtractor) Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	if !isExpoRouterFile(cfg.FilePath) {
		return nil
	}

	relPath := extractExpoAppRelPath(cfg.FilePath)
	if relPath == "" {
		return nil
	}

	routePath := appPathToRoute(relPath)
	handlerName := findHandlerFromSites(cfg, "default")
	if handlerName == "" {
		handlerName = findHandlerFromSites(cfg, "Page")
	}

	mw := detectLayoutMiddleware(cfg.FilePath, "app")

	return []ExtractedRoute{{
		Path:        routePath,
		Method:      "GET",
		HandlerName: handlerName,
		FilePath:    cfg.FilePath,
		Line:        findFirstLine(cfg),
		Middleware:  mw,
	}}
}

// isExpoRouterFile checks if the file is an Expo Router file.
func isExpoRouterFile(path string) bool {
	if !strings.Contains(path, "/app/") {
		return false
	}
	base := filepath.Base(path)
	return base == "page.tsx" || base == "page.jsx" ||
		base == "page.ts" || base == "page.js"
}

// extractExpoAppRelPath extracts the relative path under app/.
func extractExpoAppRelPath(path string) string {
	idx := strings.Index(path, "/app/")
	if idx < 0 {
		return ""
	}
	rel := path[idx+5:] // after "/app/"
	dir := filepath.Dir(rel)
	// Keep directory path only (remove filename)
	if dir == "." {
		return ""
	}
	return dir
}