package routeextractors

// FastAPI router binding extraction from M3 CFG site records.
//
// Scans for:
//   - @app.get("/path"), @app.post("/path"), etc. (app-level routes)
//   - @router.get("/path"), @router.post("/path"), etc. (APIRouter-level routes)
//   - @api_router.get("/path"), @api_router.post("/path") (aliased router)
//   - app.include_router(router, prefix="/api") (router inclusion)
//   - Depends() middleware injection (recorded as middleware metadata)
//
// FastAPI uses decorators on async/sync functions as route handlers.
// The decorator call site carries the HTTP method and path as the first argument.
// Router inclusion (include_router) carries prefix and optional tags.

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/taint"
)

// FastAPI HTTP method decorators.
var fastAPIMethodDecorators = map[string]string{
	"get":     "GET",
	"post":    "POST",
	"put":     "PUT",
	"delete":  "DELETE",
	"patch":   "PATCH",
	"options": "OPTIONS",
	"head":    "HEAD",
}

// FastAPI router receiver names (common conventions).
var fastAPIRouterReceivers = map[string]bool{
	"app":         true,
	"router":      true,
	"api_router":  true,
	"apiRouter":   true,
	"web_router":  true,
	"webRouter":   true,
	"main":        true,
}

// FastAPIRouterExtractor extracts routes from FastAPI Python handlers.
type FastAPIRouterExtractor struct{}

func (FastAPIRouterExtractor) Framework() string { return "fastapi" }

func (e FastAPIRouterExtractor) Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	if !isFastAPIFile(cfg.FilePath) {
		return nil
	}

	var routes []ExtractedRoute

	// Phase 1: extract route decorator sites.
	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			stmt := &cfg.Blocks[bi].Statements[si]
			for siteIdx := range stmt.Sites {
				site := &stmt.Sites[siteIdx]
				r := parseFastAPIDecoratorSite(cfg, site, stmt.Line)
				if r != nil {
					routes = append(routes, *r)
				}
			}
		}
	}

	// Phase 2: extract include_router calls and apply prefixes.
	includeRoutes := extractFastAPIIncludeRouterSites(cfg)
	if len(includeRoutes) > 0 {
		routes = append(routes, includeRoutes...)
	}

	// Phase 3: detect Depends() middleware injections.
	mw := extractFastAPIDependsMiddleware(cfg)
	for i := range routes {
		if routes[i].Middleware == nil {
			routes[i].Middleware = mw
		}
	}

	return routes
}

// isFastAPIFile checks if a Python file likely contains FastAPI routes.
func isFastAPIFile(path string) bool {
	if !strings.HasSuffix(path, ".py") {
		return false
	}
	// Common FastAPI file patterns
	lower := strings.ToLower(path)
	return strings.Contains(lower, "route") ||
		strings.Contains(lower, "router") ||
		strings.Contains(lower, "main") ||
		strings.Contains(lower, "app") ||
		strings.Contains(lower, "api")
}

// parseFastAPIDecoratorSite parses a single decorator site into a route.
func parseFastAPIDecoratorSite(cfg *taint.TaintFunctionCfg, site *taint.TaintSiteRecord, line int) *ExtractedRoute {
	// FastAPI decorators appear as "call" sites with callee like "get", "post"
	// and a receiver binding that holds the app/router object.
	if site.Kind != "call" {
		return nil
	}

	method, ok := fastAPIMethodDecorators[site.Callee]
	if !ok {
		return nil
	}

	// Validate the receiver is a known FastAPI router object.
	// In the M3 model, receiver is a binding index. We check
	// the binding name to see if it matches a router convention.
	if site.Receiver != nil {
		bindingIdx := *site.Receiver
		if bindingIdx >= 0 && bindingIdx < len(cfg.Bindings) {
			bindingName := cfg.Bindings[bindingIdx].Name
			if !fastAPIRouterReceivers[bindingName] {
				// Could still be a router — accept any receiver
				// for decorated calls with known method names.
			}
		}
	}

	// Extract path from RequireArg (string literal in first argument)
	// or from Args[0] occurrences.
	path := "/"
	if site.RequireArg != "" {
		path = site.RequireArg
	} else if len(site.Args) > 0 {
		// Try to infer path from the first arg occurrences.
		// The first argument to @app.get() is typically the path string.
		for _, occ := range site.Args[0] {
			if !occ.IsVia && occ.Direct >= 0 && occ.Direct < len(cfg.Bindings) {
				// Binding might hold the path string literal.
				// Since we can't access the runtime value, we use
				// RequireArg or default to "/".
				break
			}
		}
	}

	handlerName := findHandlerFromSites(cfg, site.Callee)
	if handlerName == "" {
		handlerName = findFirstLineFunctionName(cfg)
	}

	return &ExtractedRoute{
		Path:        normalizeFastAPIPath(path),
		Method:      method,
		HandlerName: handlerName,
		FilePath:    cfg.FilePath,
		Line:        line,
	}
}

// extractFastAPIIncludeRouterSites scans for app.include_router() calls.
func extractFastAPIIncludeRouterSites(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	var routes []ExtractedRoute

	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			stmt := &cfg.Blocks[bi].Statements[si]
			for siteIdx := range stmt.Sites {
				site := &stmt.Sites[siteIdx]
				if site.Kind != "call" || site.Callee != "include_router" {
					continue
				}
				// include_router typically carries a prefix argument.
				prefix := ""
				if site.RequireArg != "" {
					prefix = site.RequireArg
				}
				// The first arg is the router module/object.
				routerName := ""
				if len(site.Args) > 0 {
					for _, occ := range site.Args[0] {
						if !occ.IsVia && occ.Direct >= 0 && occ.Direct < len(cfg.Bindings) {
							routerName = cfg.Bindings[occ.Direct].Name
							break
						}
					}
				}
				_ = routerName // used for metadata only

				routes = append(routes, ExtractedRoute{
					Path:        normalizeFastAPIPath(prefix),
					Method:      "*",
					HandlerName: "include_router:" + routerName,
					FilePath:    cfg.FilePath,
					Line:        stmt.Line,
					IsAPI:       true,
				})
			}
		}
	}

	return routes
}

// extractFastAPIDependsMiddleware scans for Depends() references in function params.
func extractFastAPIDependsMiddleware(cfg *taint.TaintFunctionCfg) []string {
	var mw []string
	seen := make(map[string]bool)

	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			for siteIdx := range cfg.Blocks[bi].Statements[si].Sites {
				site := &cfg.Blocks[bi].Statements[si].Sites[siteIdx]
				if site.Kind == "call" && site.Callee == "Depends" {
					// Extract the dependency name from the first arg.
					if len(site.Args) > 0 {
						for _, occ := range site.Args[0] {
							if !occ.IsVia && occ.Direct >= 0 && occ.Direct < len(cfg.Bindings) {
								name := cfg.Bindings[occ.Direct].Name
								if !seen[name] {
									seen[name] = true
									mw = append(mw, name)
								}
								break
							}
						}
					}
				}
			}
		}
	}

	return mw
}

// normalizeFastAPIPath ensures the path starts with "/" and removes trailing slashes.
func normalizeFastAPIPath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return "/"
	}
	return path
}

// findFirstLineFunctionName tries to determine the handler function name from bindings.
func findFirstLineFunctionName(cfg *taint.TaintFunctionCfg) string {
	// The first param-binding or function-binding is likely the handler.
	for _, b := range cfg.Bindings {
		if b.Kind == "function" && !b.Synthetic {
			return b.Name
		}
	}
	return ""
}