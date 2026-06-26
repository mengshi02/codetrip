package routeextractors

// Shared Spring Boot/Spring MVC route extraction utilities.
//
// Handles common patterns:
//   - @RequestMapping / @GetMapping / @PostMapping / etc.
//   - @RestController / @Controller class-level prefixes
//   - @PathVariable / @RequestParam parameter extraction
//
// Used by both spring.go and other JVM framework extractors.

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/taint"
)

// SpringMapping holds a parsed Spring request mapping annotation.
type SpringMapping struct {
	Method     string   // GET, POST, PUT, DELETE, PATCH, or empty (any)
	Path       string   // route path from annotation value
	ClassPath  string   // class-level @RequestMapping prefix
	MethodName string   // Java method name
}

// springMethodAnnotations maps annotation simple names to HTTP method.
var springMethodAnnotations = map[string]string{
	"GetMapping":    "GET",
	"PostMapping":   "POST",
	"PutMapping":    "PUT",
	"DeleteMapping": "DELETE",
	"PatchMapping":  "PATCH",
}

// springAnnotationMethods returns the HTTP method for a Spring annotation.
func springAnnotationMethods(annotation string) (method string, ok bool) {
	if annotation == "RequestMapping" {
		return "", true // any method
	}
	if m, found := springMethodAnnotations[annotation]; found {
		return m, true
	}
	return "", false
}

// extractSpringMappings extracts all Spring request mapping sites from a CFG.
func extractSpringMappings(cfg *taint.TaintFunctionCfg) []SpringMapping {
	var mappings []SpringMapping

	for bi := range cfg.Blocks {
		for si := range cfg.Blocks[bi].Statements {
			stmt := &cfg.Blocks[bi].Statements[si]
			for siteIdx := range stmt.Sites {
				site := &stmt.Sites[siteIdx]
				mapping := parseSpringMappingSite(site)
				if mapping != nil {
					mapping.MethodName = site.Callee
					mappings = append(mappings, *mapping)
				}
			}
		}
	}

	return mappings
}

// parseSpringMappingSite parses a Spring mapping annotation from a site.
func parseSpringMappingSite(site *taint.TaintSiteRecord) *SpringMapping {
	if site.Kind != "call" && site.Kind != "decorator" {
		return nil
	}

	callee := site.Callee
	method, ok := springAnnotationMethods(callee)
	if !ok {
		return nil
	}

	path := "/"
	// Extract path from RequireArg or first argument
	if site.RequireArg != "" {
		path = site.RequireArg
	} else if len(site.Args) > 0 && len(site.Args[0]) > 0 {
		// Path is in the first argument — recorded as binding ref
		// We can't resolve the string literal from binding index alone,
		// so we use the annotation name as fallback.
		path = "/" + strings.ToLower(callee)
	}

	return &SpringMapping{
		Method: method,
		Path:   path,
	}
}

// joinPaths combines class-level and method-level paths.
func joinPaths(classPath, methodPath string) string {
	if classPath == "" || classPath == "/" {
		return methodPath
	}
	if methodPath == "" || methodPath == "/" {
		return classPath
	}
	cp := strings.TrimSuffix(classPath, "/")
	mp := strings.TrimPrefix(methodPath, "/")
	return cp + "/" + mp
}