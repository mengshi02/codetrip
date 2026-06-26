package routeextractors

// Laravel route extraction from M3 CFG site records.
//
// Scans for:
//   - Route::get('/path', handler), Route::post('/path', handler), etc.
//   - Route::any('/path', handler)
//   - Route::match(['get','post'], '/path', handler)
//   - Route::middleware(['auth'])->get('/path', handler)
//   - Route::group(['prefix' => '/api'], function() { ... })
//   - Route::resource('posts', 'PostController')
//   - Route::apiResource('posts', 'PostController')
//
// Laravel route files are typically in routes/web.php and routes/api.php.

import (
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/taint"
)

// Laravel HTTP method mapping from Route facade method names.
var laravelMethodMap = map[string]string{
	"get":     "GET",
	"post":    "POST",
	"put":     "PUT",
	"delete":  "DELETE",
	"patch":   "PATCH",
	"options": "OPTIONS",
	"any":     "*",
}

// LaravelRouteExtractor extracts routes from Laravel PHP route files.
type LaravelRouteExtractor struct{}

func (LaravelRouteExtractor) Framework() string { return "laravel" }

func (e LaravelRouteExtractor) Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	if !isLaravelRouteFileM3(cfg.FilePath) {
		return nil
	}

	var routes []ExtractedRoute

	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			stmt := &cfg.Blocks[bi].Statements[si]
			for siteIdx := range stmt.Sites {
				site := &stmt.Sites[siteIdx]
				info := parseLaravelRouteSite(cfg, site, stmt.Line)
				if info != nil {
					routes = append(routes, *info)
				}
			}
		}
	}

	return routes
}

// isLaravelRouteFileM3 checks if a PHP file is a Laravel route file.
func isLaravelRouteFileM3(path string) bool {
	if !strings.HasSuffix(path, ".php") {
		return false
	}
	return laravelRouteFilePattern.MatchString(path)
}

var laravelRouteFilePattern = regexp.MustCompile(`routes/(web|api|console|channels)\.php$`)

// parseLaravelRouteSite parses a Route facade call into an ExtractedRoute.
func parseLaravelRouteSite(cfg *taint.TaintFunctionCfg, site *taint.TaintSiteRecord, line int) *ExtractedRoute {
	if site.Kind != "call" {
		return nil
	}

	// Route::get/post/etc. — callee is the HTTP method name,
	// receiver is the "Route" facade binding.
	method, ok := laravelMethodMap[site.Callee]
	if !ok {
		// Also check for Route::match (multi-method)
		if site.Callee == "match" {
			method = "MATCH"
		} else if site.Callee == "resource" || site.Callee == "apiResource" {
			// Resource routes generate multiple standard CRUD routes.
			return parseLaravelResourceSite(cfg, site, line)
		} else if site.Callee == "group" {
			// Route::group is a prefix wrapper — we extract the prefix.
			return parseLaravelGroupSite(cfg, site, line)
		} else if site.Callee == "middleware" {
			// Middleware registration — not a route itself.
			return nil
		} else {
			return nil
		}
	}

	// Validate the receiver is the Route facade.
	if site.Receiver != nil {
		bindingIdx := *site.Receiver
		if bindingIdx >= 0 && bindingIdx < len(cfg.Bindings) {
			if cfg.Bindings[bindingIdx].Name != "Route" {
				return nil
			}
		}
	}

	// Extract path from RequireArg or args.
	path := "/"
	if site.RequireArg != "" {
		path = site.RequireArg
	}

	// Extract handler name from second arg position.
	handlerName := ""
	if len(site.Args) > 1 {
		for _, occ := range site.Args[1] {
			if !occ.IsVia && occ.Direct >= 0 && occ.Direct < len(cfg.Bindings) {
				handlerName = cfg.Bindings[occ.Direct].Name
				break
			}
		}
	}

	// Detect middleware chain if any middleware calls precede this site.
	mw := extractLaravelMiddlewareForSite(cfg, site)

	return &ExtractedRoute{
		Path:        normalizeLaravelPath(path),
		Method:      method,
		HandlerName: handlerName,
		FilePath:    cfg.FilePath,
		Line:        line,
		Middleware:  mw,
		IsAPI:       strings.Contains(cfg.FilePath, "routes/api"),
	}
}

// parseLaravelResourceSite generates CRUD routes from Route::resource/apiResource.
func parseLaravelResourceSite(cfg *taint.TaintFunctionCfg, site *taint.TaintSiteRecord, line int) *ExtractedRoute {
	// Route::resource('posts', 'PostController') generates:
	//   GET    /posts          → index
	//   GET    /posts/create   → create
	//   POST   /posts          → store
	//   GET    /posts/{id}     → show
	//   GET    /posts/{id}/edit → edit
	//   PUT    /posts/{id}     → update
	//   DELETE /posts/{id}     → destroy
	resourceName := ""
	if site.RequireArg != "" {
		resourceName = site.RequireArg
	} else if len(site.Args) > 0 {
		for _, occ := range site.Args[0] {
			if !occ.IsVia && occ.Direct >= 0 && occ.Direct < len(cfg.Bindings) {
				resourceName = cfg.Bindings[occ.Direct].Name
				break
			}
		}
	}

	controllerName := ""
	if len(site.Args) > 1 {
		for _, occ := range site.Args[1] {
			if !occ.IsVia && occ.Direct >= 0 && occ.Direct < len(cfg.Bindings) {
				controllerName = cfg.Bindings[occ.Direct].Name
				break
			}
		}
	}

	// Return a single aggregated route with method "*" for the resource.
	// Full CRUD expansion would be done by a downstream consumer.
	path := "/" + resourceName
	isAPI := site.Callee == "apiResource"

	return &ExtractedRoute{
		Path:        normalizeLaravelPath(path),
		Method:      "*",
		HandlerName: controllerName,
		FilePath:    cfg.FilePath,
		Line:        line,
		IsAPI:       isAPI || strings.Contains(cfg.FilePath, "routes/api"),
	}
}

// parseLaravelGroupSite extracts prefix from Route::group calls.
func parseLaravelGroupSite(cfg *taint.TaintFunctionCfg, site *taint.TaintSiteRecord, line int) *ExtractedRoute {
	// Route::group(['prefix' => '/api'], ...) — extract the prefix.
	prefix := ""
	if site.RequireArg != "" {
		prefix = site.RequireArg
	}

	return &ExtractedRoute{
		Path:        normalizeLaravelPath(prefix),
		Method:      "GROUP",
		HandlerName: "group",
		FilePath:    cfg.FilePath,
		Line:        line,
	}
}

// extractLaravelMiddlewareForSite scans for preceding middleware chain calls.
func extractLaravelMiddlewareForSite(cfg *taint.TaintFunctionCfg, site *taint.TaintSiteRecord) []string {
	// In the M3 model, middleware chaining (Route::middleware(['auth'])->get(...))
	// is represented as separate call sites with callee "middleware".
	// We scan all sites in the same function for middleware calls.
	var mw []string

	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			for sIdx := range cfg.Blocks[bi].Statements[si].Sites {
				s := &cfg.Blocks[bi].Statements[si].Sites[sIdx]
				if s.Kind == "call" && s.Callee == "middleware" {
					if s.Receiver != nil {
						bIdx := *s.Receiver
						if bIdx >= 0 && bIdx < len(cfg.Bindings) && cfg.Bindings[bIdx].Name == "Route" {
							// Extract middleware names from RequireArg.
							if s.RequireArg != "" {
								for _, m := range strings.Split(s.RequireArg, ",") {
									m = strings.TrimSpace(m)
									m = strings.Trim(m, "'\"[]")
									if m != "" {
										mw = append(mw, m)
									}
								}
							}
						}
					}
				}
			}
		}
	}

	return mw
}

// normalizeLaravelPath ensures the path starts with "/".
func normalizeLaravelPath(path string) string {
	if path == "" {
		return "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}