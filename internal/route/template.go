package route

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// TemplateURLExtractor template URL extractor
// Extracts URLs from HTML/EJS/HBS/Blade/Smarty templates:
//   - <a href="/users">
//   - <form action="/submit">
//   - <img src="/logo.png">
//   - <link href="/style.css">
//   - <script src="/app.js">
//   - {{action '/api/data'}}
//   - @url('/api/users')
type TemplateURLExtractor struct{}

// Framework implements RouteExtractor
func (e *TemplateURLExtractor) Framework() string { return "template" }

// Extract implements RouteExtractor
func (e *TemplateURLExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		if !isTemplateFile(f.Path) {
			continue
		}

		routes := e.extractFromFile(f, g)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *TemplateURLExtractor) extractFromFile(f *pipeline.ParsedFile, g *graph.GraphStore) []*Route {
	var routes []*Route

	// Extract URL properties from symbol information
	for _, sym := range f.Symbols {
		url := extractURLFromSymbol(sym)
		if url == "" || !isRouteURL(url) {
			continue
		}

		method := inferMethodFromSymbol(sym)

		r := &Route{
			Path:     url,
			Method:   method,
			FilePath: f.Path,
			Line:     sym.StartLine,
		}

		if sym.NodeID != "" {
			r.HandlerID = sym.NodeID
		}

		routes = append(routes, r)
	}

	return routes
}

// ============ Template Helpers ============

var (
	templateExts = map[string]bool{
		".html": true, ".htm": true,
		".ejs": true, ".hbs": true, ".handlebars": true,
		".blade.php": true, ".smarty": true,
		".pug": true, ".jade": true,
		".mustache": true, ".njk": true,
	}
)

// isTemplateFile checks if the file is a template file
func isTemplateFile(path string) bool {
	ext := filepath.Ext(path)
	if templateExts[ext] {
		return true
	}
	// Blade: .blade.php
	if strings.HasSuffix(path, ".blade.php") {
		return true
	}
	return false
}

// extractURLFromSymbol extracts URL from symbol properties
func extractURLFromSymbol(sym *pipeline.SymbolInfo) string {
	for _, key := range []string{"href", "action", "src", "url", "path"} {
		if v, ok := sym.Props[key]; ok {
			if s, ok := v.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

// isRouteURL checks if URL is a route path (excludes static resources)
func isRouteURL(url string) bool {
	// Exclude external URLs
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") ||
		strings.HasPrefix(url, "//") || strings.HasPrefix(url, "mailto:") ||
		strings.HasPrefix(url, "tel:") || strings.HasPrefix(url, "#") {
		return false
	}

	// Exclude static resources
	staticExts := []string{".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".svg",
		".ico", ".woff", ".woff2", ".ttf", ".eot", ".map"}
	for _, ext := range staticExts {
		if strings.HasSuffix(url, ext) {
			return false
		}
	}

	return true
}

// inferMethodFromSymbol infers HTTP method from symbol
func inferMethodFromSymbol(sym *pipeline.SymbolInfo) string {
	// form action → POST
	if v, ok := sym.Props["action"]; ok && v != nil {
		if method, ok := sym.Props["method"]; ok {
			if s, ok := method.(string); ok {
				return strings.ToUpper(s)
			}
		}
		return "POST"
	}
	return "GET"
}

// ============ Template URL Regex ============

var (
	// href="..." or href='...'
	hrefPattern = regexp.MustCompile(`href=["'](/[^"']+)["']`)
	// action="..."
	actionPattern = regexp.MustCompile(`action=["'](/[^"']+)["']`)
	// src="..." (only non-static resources)
	srcPattern = regexp.MustCompile(`src=["'](/[^"']+)["']`)
)
