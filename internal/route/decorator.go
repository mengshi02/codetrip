package route

import (
	"context"
	"regexp"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/pipeline"
)

// DecoratorRouteExtractor decorator route extractor
// Supports:
//   - NestJS: @Get(), @Post(), @Put(), @Delete(), @Patch()
//   - Spring Boot: @GetMapping, @PostMapping, @PutMapping, @DeleteMapping, @PatchMapping
//   - ASP.NET: [HttpGet], [HttpPost], [HttpPut], [HttpDelete], [HttpPatch]
type DecoratorRouteExtractor struct{}

// Framework implements RouteExtractor
func (e *DecoratorRouteExtractor) Framework() string { return "decorator" }

// Extract implements RouteExtractor
func (e *DecoratorRouteExtractor) Extract(ctx context.Context, g *graph.GraphStore, files []*pipeline.ParsedFile) ([]*Route, error) {
	slice := acquireRouteSlice()
	defer releaseRouteSlice(slice)

	for _, f := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		routes := e.extractFromFile(f, g)
		*slice = append(*slice, routes...)
	}

	result := make([]*Route, len(*slice))
	copy(result, *slice)
	return result, nil
}

func (e *DecoratorRouteExtractor) extractFromFile(f *pipeline.ParsedFile, g *graph.GraphStore) []*Route {
	var routes []*Route

	// Iterate through ClassInfo to detect decorators
	for _, cls := range f.ClassInfos {
		classRoute := e.extractClassBaseRoute(cls, f)
		for _, method := range cls.Methods {
			// Find method decorator routes
			methodRoutes := e.extractMethodRoutes(method, cls, f, g, classRoute)
			routes = append(routes, methodRoutes...)
		}
	}

	// Iterate through SymbolInfo to find decorators
	for _, sym := range f.Symbols {
		if sym.Label == graph.LabelDecorator {
			r := e.extractDecoratorRoute(sym, f, g)
			if r != nil {
				routes = append(routes, r)
			}
		}
	}

	return routes
}

// extractClassBaseRoute extracts class-level route prefix
func (e *DecoratorRouteExtractor) extractClassBaseRoute(cls *pipeline.ClassInfo, f *pipeline.ParsedFile) string {
	// NestJS: @Controller('path')
	// Spring: @RequestMapping(path = "/api")
	for _, sym := range f.Symbols {
		if sym.Name == "Controller" || sym.Name == "RequestMapping" ||
			sym.Name == "RestController" {
			if path, ok := sym.Props["path"]; ok {
				if s, ok := path.(string); ok {
					return s
				}
			}
		}
	}
	return ""
}

// extractMethodRoutes extracts routes from method decorators
func (e *DecoratorRouteExtractor) extractMethodRoutes(method string, cls *pipeline.ClassInfo, f *pipeline.ParsedFile, g *graph.GraphStore, basePath string) []*Route {
	var routes []*Route

	// Find decorators for this method in call sites
	for _, call := range f.CallSites {
		if call.Name != method {
			continue
		}

		info := parseDecoratorCall(call.Name)
		if info == nil {
			continue
		}

		fullPath := joinRoutePath(basePath, info.path)
		handlerID := cls.NodeID // 类节点 ID

		// Try to find method node ID
		for _, sym := range f.Symbols {
			if sym.Name == method && sym.FilePath == f.Path {
				handlerID = sym.NodeID
				break
			}
		}

		routes = append(routes, &Route{
			Path:      fullPath,
			Method:    info.method,
			HandlerID: handlerID,
			FilePath:  f.Path,
			Line:      call.Line,
		})
	}

	return routes
}

// extractDecoratorRoute extracts route from decorator symbol
func (e *DecoratorRouteExtractor) extractDecoratorRoute(sym *pipeline.SymbolInfo, f *pipeline.ParsedFile, g *graph.GraphStore) *Route {
	info := parseDecoratorName(sym.Name)
	if info == nil {
		return nil
	}

	path := ""
	if p, ok := sym.Props["path"]; ok {
		if s, ok := p.(string); ok {
			path = s
		}
	}

	return &Route{
		Path:      path,
		Method:    info.method,
		HandlerID: sym.NodeID,
		FilePath:  f.Path,
		Line:      sym.StartLine,
	}
}

// ============ Decorator Parsing ============

type decoratorInfo struct {
	method string
	path   string
}

var (
	// NestJS: @Get, @Post, @Put, @Delete, @Patch
	nestjsPattern = regexp.MustCompile(`^(Get|Post|Put|Delete|Patch|Head|Options|All)$`)
	// Spring: @GetMapping, @PostMapping, @PutMapping, @DeleteMapping, @PatchMapping
	springPattern = regexp.MustCompile(`^(Get|Post|Put|Delete|Patch)Mapping$`)
	// ASP.NET: HttpGet, HttpPost, HttpPut, HttpDelete, HttpPatch
	dotnetPattern = regexp.MustCompile(`^Http(Get|Post|Put|Delete|Patch|Head|Options)$`)
)

// parseDecoratorName parses HTTP method and path from decorator name
func parseDecoratorName(name string) *decoratorInfo {
	// NestJS
	if m := nestjsPattern.FindStringSubmatch(name); m != nil {
		return &decoratorInfo{method: strings.ToUpper(m[1]), path: ""}
	}
	// Spring Boot
	if m := springPattern.FindStringSubmatch(name); m != nil {
		return &decoratorInfo{method: strings.ToUpper(m[1]), path: ""}
	}
	// ASP.NET
	if m := dotnetPattern.FindStringSubmatch(name); m != nil {
		return &decoratorInfo{method: strings.ToUpper(m[1]), path: ""}
	}
	return nil
}

// parseDecoratorCall parses decorator from call site (same logic)
func parseDecoratorCall(name string) *decoratorInfo {
	return parseDecoratorName(name)
}

// joinRoutePath joins route prefix and path
func joinRoutePath(base, path string) string {
	if base == "" {
		return path
	}
	if path == "" {
		return base
	}
	base = strings.TrimRight(base, "/")
	path = strings.TrimLeft(path, "/")
	return base + "/" + path
}