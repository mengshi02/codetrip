package routeextractors

// Middleware chain extraction from M3 CFG site records.
//
// Scans:
//   - Express: app.use(middleware), router.use(middleware)
//   - Koa: app.use(middleware)
//   - Fastify: fastify.addHook('onRequest', middleware)
//   - Django: MIDDLEWARE = [...] configuration
//
// Produces virtual routes (Method=MIDDLEWARE) to record middleware chains.

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/taint"
)

// MiddlewareExtractor extracts middleware registrations from CFG sites.
type MiddlewareExtractor struct{}

func (MiddlewareExtractor) Framework() string { return "middleware" }

func (e MiddlewareExtractor) Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	var routes []ExtractedRoute

	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			stmt := &cfg.Blocks[bi].Statements[si]
			for siteIdx := range stmt.Sites {
				site := &stmt.Sites[siteIdx]
				mw := parseMiddlewareSite(site)
				if mw == nil {
					continue
				}
				routes = append(routes, ExtractedRoute{
					Path:        mw.path,
					Method:      "MIDDLEWARE",
					HandlerName: mw.handlerName,
					FilePath:    cfg.FilePath,
					Line:        stmt.Line,
					Middleware:   []string{mw.handlerName},
				})
			}
		}
	}

	return routes
}

// middlewareInfo holds parsed middleware registration info.
type middlewareInfo struct {
	handlerName string
	path        string
}

// parseMiddlewareSite parses a middleware registration from a site record.
func parseMiddlewareSite(site *taint.TaintSiteRecord) *middlewareInfo {
	if site.Kind != "call" {
		return nil
	}

	callee := site.Callee

	// Express/Koa: app.use(), router.use()
	if callee == "use" {
		// Check receiver via parent chain or requireArg
		receiver := ""
		if site.Receiver != nil {
			// Receiver binding index — we record the callee as "use" and
			// let the consumer resolve the receiver binding.
			receiver = "this"
		}
		_ = receiver
		path := "/"
		handlerName := "anonymous"
		// If the site has arguments, the first arg may be a path string.
		if site.RequireArg != "" {
			path = site.RequireArg
		}
		return &middlewareInfo{handlerName: handlerName, path: path}
	}

	// Fastify: fastify.addHook('onRequest', middleware)
	if callee == "addHook" {
		return &middlewareInfo{handlerName: "anonymous", path: "/"}
	}

	// Django-style MIDDLEWARE configuration
	if callee == "MIDDLEWARE" || callee == "middleware_classes" {
		return &middlewareInfo{handlerName: callee, path: "/"}
	}

	// NestJS middleware: app.use(), module.configure()
	if callee == "configure" && strings.Contains(site.RequireArg, "middleware") {
		return &middlewareInfo{handlerName: "configure", path: "/"}
	}

	return nil
}