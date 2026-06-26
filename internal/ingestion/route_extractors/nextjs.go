package routeextractors

// Next.js route extraction from M3 CFG site records.
//
// Two router modes:
//   - Pages Router: /pages/api/*.js → API route, /pages/*.tsx → page route
//   - App Router: /app/page.tsx → GET page, /app/route.ts → HTTP method handlers
//
// M3 version operates on TaintFunctionCfg site records (call/member-read/
// construct) rather than the legacy ParsedFile.CallSites, and produces
// ExtractedRoute values consumable by the taint pipeline.

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/taint"
)

// ── extracted route ────────────────────────────────────────────────────────

// ExtractedRoute is a route discovered from CFG site analysis.
type ExtractedRoute struct {
	Path         string   // route path pattern (e.g. "/api/users/:id")
	Method       string   // HTTP method or "*" for all
	HandlerName  string   // callee name of the handler
	FilePath     string   // source file path
	Line         int      // line number
	Middleware   []string // layout/middleware names
	IsAPI        bool     // true for API routes (Pages: api/*, App: route.ts)
	ResponseKeys []string // known response shape keys
}

// RouteExtractor extracts routes from a single function CFG.
type RouteExtractor interface {
	// Framework returns the framework identifier.
	Framework() string
	// Extract extracts routes from the given function CFG.
	Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute
}

// ── Next.js Pages Router ───────────────────────────────────────────────────

var (
	nextJSPagesRe = regexp.MustCompile(`[/^]pages/(.+)`)
	nextJSAppRe   = regexp.MustCompile(`[/^]app/(.+)`)
	dynamicSegRe  = regexp.MustCompile(`\[(\.\.\.)?([^\]]+)\]`)
)

// NextJSPagesExtractor extracts routes from Next.js Pages Router files.
type NextJSPagesExtractor struct{}

func (NextJSPagesExtractor) Framework() string { return "nextjs-pages" }

func (e NextJSPagesExtractor) Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	relPath := extractPagesRelPath(cfg.FilePath)
	if relPath == "" {
		return nil
	}

	routePath := pagesPathToRoute(relPath)
	isAPI := strings.HasPrefix(relPath, "api/")

	method := "GET"
	if isAPI {
		method = "*"
	}

	handlerName := ""
	line := 0
	// Find the default-export or handler function from sites.
	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			stmt := &cfg.Blocks[bi].Statements[si]
			for _, site := range stmt.Sites {
				if site.Kind == "export-default" || site.Kind == "function" {
					handlerName = site.Callee
					line = stmt.Line
					break
				}
			}
			if handlerName != "" {
				break
			}
		}
		if handlerName != "" {
			break
		}
	}

	respKeys := []string(nil)
	if isAPI {
		respKeys = []string{"json"}
	}

	return []ExtractedRoute{{
		Path:         routePath,
		Method:       method,
		HandlerName:  handlerName,
		FilePath:     cfg.FilePath,
		Line:         line,
		IsAPI:        isAPI,
		ResponseKeys: respKeys,
	}}
}

// NextJSAppExtractor extracts routes from Next.js App Router files.
type NextJSAppExtractor struct{}

func (NextJSAppExtractor) Framework() string { return "nextjs-app" }

func (e NextJSAppExtractor) Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	relPath := extractAppRelPath(cfg.FilePath)
	if relPath == "" {
		return nil
	}

	routePath := appPathToRoute(relPath)
	base := filepath.Base(cfg.FilePath)
	isRouteTS := base == "route.ts" || base == "route.js"

	if isRouteTS {
		return e.extractRouteTS(cfg, routePath)
	}
	// page.tsx → GET page route
	handlerName := findHandlerFromSites(cfg, "default")
	return []ExtractedRoute{{
		Path:        routePath,
		Method:      "GET",
		HandlerName: handlerName,
		FilePath:    cfg.FilePath,
		Line:        findFirstLine(cfg),
		Middleware:  detectLayoutMiddleware(cfg.FilePath, "app"),
	}}
}

func (e NextJSAppExtractor) extractRouteTS(cfg *taint.TaintFunctionCfg, routePath string) []ExtractedRoute {
	// Detect exported HTTP method handlers from sites.
	httpMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	var methods []string
	methodSites := make(map[string]string) // method → callee

	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			for _, site := range cfg.Blocks[bi].Statements[si].Sites {
				if site.Kind == "export" || site.Kind == "function" {
					for _, m := range httpMethods {
						if site.Callee == m {
							methods = append(methods, m)
							methodSites[m] = site.Callee
							break
						}
					}
				}
			}
		}
	}

	if len(methods) == 0 {
		methods = []string{"GET", "POST", "PUT", "DELETE"}
	}

	mw := detectLayoutMiddleware(cfg.FilePath, "app")
	var routes []ExtractedRoute
	for _, m := range methods {
		hn := methodSites[m]
		routes = append(routes, ExtractedRoute{
			Path:        routePath,
			Method:      m,
			HandlerName: hn,
			FilePath:    cfg.FilePath,
			Line:        findFirstLine(cfg),
			Middleware:  mw,
			IsAPI:       true,
		})
	}
	return routes
}

// ── helpers ────────────────────────────────────────────────────────────────

func extractPagesRelPath(path string) string {
	m := nextJSPagesRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	return m[1]
}

func extractAppRelPath(path string) string {
	m := nextJSAppRe.FindStringSubmatch(path)
	if len(m) < 2 {
		return ""
	}
	dir := filepath.Dir(m[1])
	if dir == "." {
		return ""
	}
	return dir
}

func pagesPathToRoute(relPath string) string {
	noExt := stripExt(relPath)
	if noExt == "index" || filepath.Base(noExt) == "index" {
		noExt = filepath.Dir(noExt)
		if noExt == "." {
			noExt = ""
		}
	}
	path := dynamicSegRe.ReplaceAllStringFunc(noExt, func(match string) string {
		sub := dynamicSegRe.FindStringSubmatch(match)
		if sub[1] != "" {
			return ":" + sub[2] + "@" // catch-all
		}
		return ":" + sub[2]
	})
	if path == "" {
		return "/"
	}
	return "/" + path
}

func appPathToRoute(relPath string) string {
	path := dynamicSegRe.ReplaceAllStringFunc(relPath, func(match string) string {
		sub := dynamicSegRe.FindStringSubmatch(match)
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

func findHandlerFromSites(cfg *taint.TaintFunctionCfg, name string) string {
	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			for _, site := range cfg.Blocks[bi].Statements[si].Sites {
				if site.Callee == name {
					return name
				}
			}
		}
	}
	return ""
}

func findFirstLine(cfg *taint.TaintFunctionCfg) int {
	for _, b := range cfg.Blocks {
		for _, s := range b.Statements {
			return s.Line
		}
	}
	return 0
}

func detectLayoutMiddleware(filePath, rootDir string) []string {
	dir := filepath.Dir(filePath)
	var mw []string
	for {
		parent := filepath.Dir(dir)
		if parent == dir || !strings.Contains(dir, rootDir) {
			break
		}
		mw = append(mw, filepath.Join(dir, "layout.tsx"))
		dir = parent
	}
	return mw
}

func stripExt(path string) string {
	ext := filepath.Ext(path)
	if ext == "" {
		return path
	}
	return path[:len(path)-len(ext)]
}