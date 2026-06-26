package routeextractors

// Spring Boot route extraction from M3 CFG site records.
//
// Scans for @RequestMapping, @GetMapping, @PostMapping, @PutMapping,
// @DeleteMapping, @PatchMapping annotations on controller methods.
// Uses spring_shared.go utilities for annotation parsing.

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/taint"
)

// SpringRouteExtractor extracts routes from Spring Boot controllers.
type SpringRouteExtractor struct{}

func (SpringRouteExtractor) Framework() string { return "spring-boot" }

func (e SpringRouteExtractor) Extract(cfg *taint.TaintFunctionCfg) []ExtractedRoute {
	if !isSpringFile(cfg.FilePath) {
		return nil
	}

	mappings := extractSpringMappings(cfg)
	if len(mappings) == 0 {
		return nil
	}

	// Detect class-level @RequestMapping prefix.
	classPath := extractClassLevelPath(cfg)

	var routes []ExtractedRoute
	for _, m := range mappings {
		path := joinPaths(classPath, m.Path)
		method := m.Method
		if method == "" {
			method = "*"
		}
		routes = append(routes, ExtractedRoute{
			Path:        path,
			Method:      method,
			HandlerName: m.MethodName,
			FilePath:    cfg.FilePath,
			Line:        findFirstLine(cfg),
		})
	}

	return routes
}

// isSpringFile checks if the file is a Spring Java file.
func isSpringFile(path string) bool {
	return strings.HasSuffix(path, ".java") &&
		(strings.Contains(path, "Controller") ||
			strings.Contains(path, "controller") ||
			strings.Contains(path, "Resource") ||
			strings.Contains(path, "resource"))
}

// extractClassLevelPath looks for a class-level @RequestMapping prefix.
func extractClassLevelPath(cfg *taint.TaintFunctionCfg) string {
	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			for siteIdx := range cfg.Blocks[bi].Statements[si].Sites {
				site := &cfg.Blocks[bi].Statements[si].Sites[siteIdx]
				if site.Kind == "decorator" && site.Callee == "RequestMapping" {
					if site.RequireArg != "" {
						return site.RequireArg
					}
				}
			}
		}
	}
	return ""
}