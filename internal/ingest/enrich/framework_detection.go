package enrich

// Framework Detection.
//
// Detects frameworks from file path patterns and provides
// entry point multipliers for process scoring.
// Returns nil for unknown frameworks (1.0 multiplier fallback).

import "strings"

// ─────────────────────────────────────────────────────────────────────────────
// Public types
// ─────────────────────────────────────────────────────────────────────────────

// FrameworkHint represents a detected framework with its entry point multiplier.
type FrameworkHint struct {
	Framework            string
	EntryPointMultiplier float64
	Reason               string
}

// ─────────────────────────────────────────────────────────────────────────────
// Path-based framework detection
// ─────────────────────────────────────────────────────────────────────────────

// DetectFrameworkFromPath detects a framework from file path patterns.
// Returns nil if no framework pattern is detected (falls back to 1.0 multiplier).
func DetectFrameworkFromPath(filePath string) *FrameworkHint {
	p := strings.ToLower(strings.ReplaceAll(filePath, "\\", "/"))
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	// ========== JAVASCRIPT / TYPESCRIPT FRAMEWORKS ==========

	// Next.js - Pages Router
	if strings.Contains(p, "/pages/") && !strings.Contains(p, "/_") && !strings.Contains(p, "/api/") {
		if hasExt(p, ".tsx", ".ts", ".jsx", ".js") {
			return &FrameworkHint{Framework: "nextjs-pages", EntryPointMultiplier: 3.0, Reason: "nextjs-page"}
		}
	}

	// Next.js - App Router
	if strings.Contains(p, "/app/") && (strings.HasSuffix(p, "page.tsx") || strings.HasSuffix(p, "page.ts") || strings.HasSuffix(p, "page.jsx") || strings.HasSuffix(p, "page.js")) {
		return &FrameworkHint{Framework: "nextjs-app", EntryPointMultiplier: 3.0, Reason: "nextjs-app-page"}
	}

	// Next.js - API Routes
	if strings.Contains(p, "/pages/api/") || (strings.Contains(p, "/app/") && strings.Contains(p, "/api/") && strings.HasSuffix(p, "route.ts")) {
		return &FrameworkHint{Framework: "nextjs-api", EntryPointMultiplier: 3.0, Reason: "nextjs-api-route"}
	}

	// Next.js - Layout files
	if strings.Contains(p, "/app/") && (strings.HasSuffix(p, "layout.tsx") || strings.HasSuffix(p, "layout.ts")) {
		return &FrameworkHint{Framework: "nextjs-app", EntryPointMultiplier: 2.0, Reason: "nextjs-layout"}
	}

	// Express / Node.js routes
	if strings.Contains(p, "/routes/") && hasExt(p, ".ts", ".js") {
		return &FrameworkHint{Framework: "express", EntryPointMultiplier: 2.5, Reason: "routes-folder"}
	}

	// Generic controllers (MVC)
	if strings.Contains(p, "/controllers/") && hasExt(p, ".ts", ".js") {
		return &FrameworkHint{Framework: "mvc", EntryPointMultiplier: 2.5, Reason: "controllers-folder"}
	}

	// Generic handlers
	if strings.Contains(p, "/handlers/") && hasExt(p, ".ts", ".js") {
		return &FrameworkHint{Framework: "handlers", EntryPointMultiplier: 2.5, Reason: "handlers-folder"}
	}

	// React components
	if (strings.Contains(p, "/components/") || strings.Contains(p, "/views/")) && (strings.HasSuffix(p, ".tsx") || strings.HasSuffix(p, ".jsx")) {
		fileName := lastPathComponent(p)
		if len(fileName) > 0 && fileName[0] >= 'A' && fileName[0] <= 'Z' {
			return &FrameworkHint{Framework: "react", EntryPointMultiplier: 1.5, Reason: "react-component"}
		}
	}

	// ========== PYTHON FRAMEWORKS ==========

	if strings.HasSuffix(p, "views.py") {
		return &FrameworkHint{Framework: "django", EntryPointMultiplier: 3.0, Reason: "django-views"}
	}
	if strings.HasSuffix(p, "urls.py") {
		return &FrameworkHint{Framework: "django", EntryPointMultiplier: 2.0, Reason: "django-urls"}
	}
	if (strings.Contains(p, "/routers/") || strings.Contains(p, "/endpoints/") || strings.Contains(p, "/routes/")) && strings.HasSuffix(p, ".py") {
		return &FrameworkHint{Framework: "fastapi", EntryPointMultiplier: 2.5, Reason: "api-routers"}
	}
	if strings.Contains(p, "/api/") && strings.HasSuffix(p, ".py") && !strings.HasSuffix(p, "__init__.py") {
		return &FrameworkHint{Framework: "python-api", EntryPointMultiplier: 2.0, Reason: "api-folder"}
	}

	// ========== JAVA FRAMEWORKS ==========

	if (strings.Contains(p, "/controller/") || strings.Contains(p, "/controllers/")) && strings.HasSuffix(p, ".java") {
		return &FrameworkHint{Framework: "spring", EntryPointMultiplier: 3.0, Reason: "spring-controller"}
	}
	if strings.HasSuffix(p, "controller.java") {
		return &FrameworkHint{Framework: "spring", EntryPointMultiplier: 3.0, Reason: "spring-controller-file"}
	}
	if (strings.Contains(p, "/service/") || strings.Contains(p, "/services/")) && strings.HasSuffix(p, ".java") {
		return &FrameworkHint{Framework: "java-service", EntryPointMultiplier: 1.8, Reason: "java-service"}
	}

	// ========== KOTLIN FRAMEWORKS ==========

	if (strings.Contains(p, "/controller/") || strings.Contains(p, "/controllers/")) && strings.HasSuffix(p, ".kt") {
		return &FrameworkHint{Framework: "spring-kotlin", EntryPointMultiplier: 3.0, Reason: "spring-kotlin-controller"}
	}
	if strings.HasSuffix(p, "controller.kt") {
		return &FrameworkHint{Framework: "spring-kotlin", EntryPointMultiplier: 3.0, Reason: "spring-kotlin-controller-file"}
	}
	if strings.Contains(p, "/routes/") && strings.HasSuffix(p, ".kt") {
		return &FrameworkHint{Framework: "ktor", EntryPointMultiplier: 2.5, Reason: "ktor-routes"}
	}
	if strings.Contains(p, "/plugins/") && strings.HasSuffix(p, ".kt") {
		return &FrameworkHint{Framework: "ktor", EntryPointMultiplier: 2.0, Reason: "ktor-plugin"}
	}
	if strings.HasSuffix(p, "routing.kt") || strings.HasSuffix(p, "routes.kt") {
		return &FrameworkHint{Framework: "ktor", EntryPointMultiplier: 2.5, Reason: "ktor-routing-file"}
	}
	if (strings.Contains(p, "/activity/") || strings.Contains(p, "/ui/")) && strings.HasSuffix(p, ".kt") {
		return &FrameworkHint{Framework: "android-kotlin", EntryPointMultiplier: 2.5, Reason: "android-ui"}
	}
	if strings.HasSuffix(p, "activity.kt") || strings.HasSuffix(p, "fragment.kt") {
		return &FrameworkHint{Framework: "android-kotlin", EntryPointMultiplier: 2.5, Reason: "android-component"}
	}
	if strings.HasSuffix(p, "/main.kt") {
		return &FrameworkHint{Framework: "kotlin", EntryPointMultiplier: 3.0, Reason: "kotlin-main"}
	}
	if strings.HasSuffix(p, "/application.kt") {
		return &FrameworkHint{Framework: "kotlin", EntryPointMultiplier: 2.5, Reason: "kotlin-application"}
	}

	// ========== C# / .NET FRAMEWORKS ==========

	if strings.Contains(p, "/controllers/") && strings.HasSuffix(p, ".cs") {
		return &FrameworkHint{Framework: "aspnet", EntryPointMultiplier: 3.0, Reason: "aspnet-controller"}
	}
	if strings.HasSuffix(p, "controller.cs") {
		return &FrameworkHint{Framework: "aspnet", EntryPointMultiplier: 3.0, Reason: "aspnet-controller-file"}
	}
	if (strings.Contains(p, "/services/") || strings.Contains(p, "/service/")) && strings.HasSuffix(p, ".cs") {
		return &FrameworkHint{Framework: "aspnet", EntryPointMultiplier: 1.8, Reason: "aspnet-service"}
	}
	if strings.Contains(p, "/middleware/") && strings.HasSuffix(p, ".cs") {
		return &FrameworkHint{Framework: "aspnet", EntryPointMultiplier: 2.5, Reason: "aspnet-middleware"}
	}
	if strings.Contains(p, "/hubs/") && strings.HasSuffix(p, ".cs") {
		return &FrameworkHint{Framework: "signalr", EntryPointMultiplier: 2.5, Reason: "signalr-hub"}
	}
	if strings.HasSuffix(p, "hub.cs") {
		return &FrameworkHint{Framework: "signalr", EntryPointMultiplier: 2.5, Reason: "signalr-hub-file"}
	}
	if strings.HasSuffix(p, "/program.cs") || strings.HasSuffix(p, "/startup.cs") {
		return &FrameworkHint{Framework: "aspnet", EntryPointMultiplier: 3.0, Reason: "aspnet-entry"}
	}
	if (strings.Contains(p, "/backgroundservices/") || strings.Contains(p, "/hostedservices/")) && strings.HasSuffix(p, ".cs") {
		return &FrameworkHint{Framework: "aspnet", EntryPointMultiplier: 2.0, Reason: "aspnet-background-service"}
	}
	if strings.Contains(p, "/pages/") && strings.HasSuffix(p, ".razor") {
		return &FrameworkHint{Framework: "blazor", EntryPointMultiplier: 2.5, Reason: "blazor-page"}
	}

	// ========== GO FRAMEWORKS ==========

	if (strings.Contains(p, "/handlers/") || strings.Contains(p, "/handler/")) && strings.HasSuffix(p, ".go") {
		return &FrameworkHint{Framework: "go-http", EntryPointMultiplier: 2.5, Reason: "go-handlers"}
	}
	if strings.Contains(p, "/routes/") && strings.HasSuffix(p, ".go") {
		return &FrameworkHint{Framework: "go-http", EntryPointMultiplier: 2.5, Reason: "go-routes"}
	}
	if strings.Contains(p, "/controllers/") && strings.HasSuffix(p, ".go") {
		return &FrameworkHint{Framework: "go-mvc", EntryPointMultiplier: 2.5, Reason: "go-controller"}
	}
	if strings.HasSuffix(p, "/main.go") {
		return &FrameworkHint{Framework: "go", EntryPointMultiplier: 3.0, Reason: "go-main"}
	}

	// ========== RUST FRAMEWORKS ==========

	if (strings.Contains(p, "/handlers/") || strings.Contains(p, "/routes/")) && strings.HasSuffix(p, ".rs") {
		return &FrameworkHint{Framework: "rust-web", EntryPointMultiplier: 2.5, Reason: "rust-handlers"}
	}
	if strings.HasSuffix(p, "/main.rs") {
		return &FrameworkHint{Framework: "rust", EntryPointMultiplier: 3.0, Reason: "rust-main"}
	}
	if strings.Contains(p, "/bin/") && strings.HasSuffix(p, ".rs") {
		return &FrameworkHint{Framework: "rust", EntryPointMultiplier: 2.5, Reason: "rust-bin"}
	}

	// ========== C / C++ ==========

	if strings.HasSuffix(p, "/main.c") || strings.HasSuffix(p, "/main.cpp") || strings.HasSuffix(p, "/main.cc") {
		return &FrameworkHint{Framework: "c-cpp", EntryPointMultiplier: 3.0, Reason: "c-main"}
	}
	if strings.Contains(p, "/src/") && (strings.HasSuffix(p, "/app.c") || strings.HasSuffix(p, "/app.cpp")) {
		return &FrameworkHint{Framework: "c-cpp", EntryPointMultiplier: 2.5, Reason: "c-app"}
	}

	// ========== PHP / LARAVEL ==========

	if strings.Contains(p, "/routes/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 3.0, Reason: "laravel-routes"}
	}
	if (strings.Contains(p, "/http/controllers/") || strings.Contains(p, "/controllers/")) && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 3.0, Reason: "laravel-controller"}
	}
	if strings.HasSuffix(p, "controller.php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 3.0, Reason: "laravel-controller-file"}
	}
	if (strings.Contains(p, "/console/commands/") || strings.Contains(p, "/commands/")) && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 2.5, Reason: "laravel-command"}
	}
	if strings.Contains(p, "/jobs/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 2.5, Reason: "laravel-job"}
	}
	if strings.Contains(p, "/listeners/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 2.5, Reason: "laravel-listener"}
	}
	if strings.Contains(p, "/http/middleware/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 2.5, Reason: "laravel-middleware"}
	}
	if strings.Contains(p, "/providers/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 1.8, Reason: "laravel-provider"}
	}
	if strings.Contains(p, "/policies/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 2.0, Reason: "laravel-policy"}
	}
	if strings.Contains(p, "/models/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 1.5, Reason: "laravel-model"}
	}
	if strings.Contains(p, "/services/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 1.8, Reason: "laravel-service"}
	}
	if strings.Contains(p, "/repositories/") && strings.HasSuffix(p, ".php") {
		return &FrameworkHint{Framework: "laravel", EntryPointMultiplier: 1.5, Reason: "laravel-repository"}
	}

	// ========== SWIFT / iOS ==========

	if strings.HasSuffix(p, "/appdelegate.swift") || strings.HasSuffix(p, "/scenedelegate.swift") || strings.HasSuffix(p, "/app.swift") {
		return &FrameworkHint{Framework: "ios", EntryPointMultiplier: 3.0, Reason: "ios-app-entry"}
	}
	if strings.HasSuffix(p, "app.swift") && strings.Contains(p, "/sources/") {
		return &FrameworkHint{Framework: "swiftui", EntryPointMultiplier: 3.0, Reason: "swiftui-app"}
	}
	if (strings.Contains(p, "/viewcontrollers/") || strings.Contains(p, "/controllers/") || strings.Contains(p, "/screens/")) && strings.HasSuffix(p, ".swift") {
		return &FrameworkHint{Framework: "uikit", EntryPointMultiplier: 2.5, Reason: "uikit-viewcontroller"}
	}
	if strings.HasSuffix(p, "viewcontroller.swift") || strings.HasSuffix(p, "vc.swift") {
		return &FrameworkHint{Framework: "uikit", EntryPointMultiplier: 2.5, Reason: "uikit-viewcontroller-file"}
	}
	if strings.Contains(p, "/coordinators/") && strings.HasSuffix(p, ".swift") {
		return &FrameworkHint{Framework: "ios-coordinator", EntryPointMultiplier: 2.5, Reason: "ios-coordinator"}
	}
	if strings.HasSuffix(p, "coordinator.swift") {
		return &FrameworkHint{Framework: "ios-coordinator", EntryPointMultiplier: 2.5, Reason: "ios-coordinator-file"}
	}
	if (strings.Contains(p, "/views/") || strings.Contains(p, "/scenes/")) && strings.HasSuffix(p, ".swift") {
		return &FrameworkHint{Framework: "swiftui", EntryPointMultiplier: 1.8, Reason: "swiftui-view"}
	}
	if strings.Contains(p, "/services/") && strings.HasSuffix(p, ".swift") {
		return &FrameworkHint{Framework: "ios-service", EntryPointMultiplier: 1.8, Reason: "ios-service"}
	}
	if strings.Contains(p, "/router/") && strings.HasSuffix(p, ".swift") {
		return &FrameworkHint{Framework: "ios-router", EntryPointMultiplier: 2.0, Reason: "ios-router"}
	}

	// ========== GENERIC PATTERNS ==========

	if strings.Contains(p, "/api/") && (strings.HasSuffix(p, "/index.ts") || strings.HasSuffix(p, "/index.js") || strings.HasSuffix(p, "/__init__.py")) {
		return &FrameworkHint{Framework: "api", EntryPointMultiplier: 1.8, Reason: "api-index"}
	}

	// No framework detected
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// AST-based framework detection
// ─────────────────────────────────────────────────────────────────────────────

// astFrameworkPatterns maps language → list of (framework, multiplier, reason, patterns).
type astPatternConfig struct {
	framework            string
	entryPointMultiplier float64
	reason               string
	patterns             []string
}

var astPatternsByLanguage = map[string][]astPatternConfig{
	"javascript": {
		{framework: "nestjs", entryPointMultiplier: 3.2, reason: "nestjs-decorator", patterns: []string{"@controller", "@get", "@post", "@put", "@delete", "@patch"}},
	},
	"typescript": {
		{framework: "nestjs", entryPointMultiplier: 3.2, reason: "nestjs-decorator", patterns: []string{"@controller", "@get", "@post", "@put", "@delete", "@patch"}},
	},
	"tsx": {
		{framework: "nestjs", entryPointMultiplier: 3.2, reason: "nestjs-decorator", patterns: []string{"@controller", "@get", "@post", "@put", "@delete", "@patch"}},
	},
	"python": {
		{framework: "fastapi", entryPointMultiplier: 3.0, reason: "fastapi-decorator", patterns: []string{"@app.get", "@app.post", "@app.put", "@app.delete", "@router.get"}},
		{framework: "flask", entryPointMultiplier: 2.8, reason: "flask-decorator", patterns: []string{"@app.route", "@blueprint.route"}},
	},
	"java": {
		{framework: "spring", entryPointMultiplier: 3.2, reason: "spring-annotation", patterns: []string{"@restcontroller", "@controller", "@getmapping", "@postmapping", "@requestmapping"}},
		{framework: "jaxrs", entryPointMultiplier: 3.0, reason: "jaxrs-annotation", patterns: []string{"@path", "@get", "@post", "@put", "@delete"}},
	},
	"kotlin": {
		{framework: "spring-kotlin", entryPointMultiplier: 3.2, reason: "spring-kotlin-annotation", patterns: []string{"@restcontroller", "@controller", "@getmapping", "@postmapping", "@requestmapping"}},
		{framework: "jaxrs", entryPointMultiplier: 3.0, reason: "jaxrs-annotation", patterns: []string{"@path", "@get", "@post", "@put", "@delete"}},
		{framework: "ktor", entryPointMultiplier: 2.8, reason: "ktor-routing", patterns: []string{"routing", "embeddedserver", "application.module"}},
		{framework: "android-kotlin", entryPointMultiplier: 2.5, reason: "android-annotation", patterns: []string{"@androidentrypoint", "appcompatactivity", "fragment("}},
	},
	"csharp": {
		{framework: "aspnet", entryPointMultiplier: 3.2, reason: "aspnet-attribute", patterns: []string{"[apicontroller]", "[httpget]", "[httppost]", "[httpput]", "[httpdelete]", "[route]", "[authorize]", "[allowanonymous]"}},
		{framework: "signalr", entryPointMultiplier: 2.8, reason: "signalr-attribute", patterns: []string{"[hubmethodname]", ": hub", ": hub<"}},
		{framework: "blazor", entryPointMultiplier: 2.5, reason: "blazor-attribute", patterns: []string{"@page", "[parameter]", "@inject"}},
		{framework: "efcore", entryPointMultiplier: 2.0, reason: "efcore-pattern", patterns: []string{"dbcontext", "dbset<", "onmodelcreating"}},
	},
	"php": {
		{framework: "laravel", entryPointMultiplier: 3.0, reason: "php-route-attribute", patterns: []string{"route::get", "route::post", "route::put", "route::delete", "route::resource", "route::apiresource", "#[route("}},
	},
}

// DetectFrameworkFromAST detects framework entry points from AST definition text.
// Returns nil if no known pattern is found.
func DetectFrameworkFromAST(language string, definitionText string) *FrameworkHint {
	if language == "" || definitionText == "" {
		return nil
	}

	configs, ok := astPatternsByLanguage[strings.ToLower(language)]
	if !ok || len(configs) == 0 {
		return nil
	}

	normalized := strings.ToLower(definitionText)

	for _, cfg := range configs {
		for _, pattern := range cfg.patterns {
			if strings.Contains(normalized, pattern) {
				return &FrameworkHint{
					Framework:            cfg.framework,
					EntryPointMultiplier: cfg.entryPointMultiplier,
					Reason:               cfg.reason,
				}
			}
		}
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────────────────────────────────────

func hasExt(p string, exts ...string) bool {
	for _, ext := range exts {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

func lastPathComponent(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return p
	}
	return p[idx+1:]
}
