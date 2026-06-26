// Framework Detection — detects frameworks from file path patterns and
// provides entry point multipliers for process scoring.
//
// Mirrors TS framework-detection.ts (563 lines), simplified for codetrip:
//   - Path-based detection only (AST decorator detection deferred to Phase 6)
//   - Covers: Next.js, Express, Django, Flask, Spring, Rails, Laravel, Gin
//   - Returns nil for unknown frameworks (1.0 multiplier, no bonus/penalty)
//
// DESIGN: Returns nil for unknown frameworks, which causes a 1.0 multiplier
// (no bonus, no penalty) - same behavior as before this feature.

package core

import (
	"strings"
)

// ─── Types ──────────────────────────────────────────────────

// FrameworkHintExt extends FrameworkHint with the detected framework name.
// Used internally by DetectFrameworkFromPath; the EntryPointScoreResult
// uses the simpler FrameworkHint for scoring.
type FrameworkHintExt struct {
	Framework            string
	EntryPointMultiplier float64
	Reason               string
}

// ─── Path-based framework detection ──────────────────────────

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
		if strings.HasSuffix(p, ".tsx") || strings.HasSuffix(p, ".ts") ||
			strings.HasSuffix(p, ".jsx") || strings.HasSuffix(p, ".js") {
			return &FrameworkHint{EntryPointMultiplier: 3.0, Reason: "nextjs-page"}
		}
	}

	// Next.js - App Router (page.tsx files)
	if strings.Contains(p, "/app/") &&
		(strings.HasSuffix(p, "page.tsx") || strings.HasSuffix(p, "page.ts") ||
			strings.HasSuffix(p, "page.jsx") || strings.HasSuffix(p, "page.js")) {
		return &FrameworkHint{EntryPointMultiplier: 3.0, Reason: "nextjs-app-page"}
	}

	// Next.js - API Routes
	if strings.Contains(p, "/pages/api/") ||
		(strings.Contains(p, "/app/") && strings.Contains(p, "/api/") && strings.HasSuffix(p, "route.ts")) {
		return &FrameworkHint{EntryPointMultiplier: 3.0, Reason: "nextjs-api-route"}
	}

	// Next.js - Layout files
	if strings.Contains(p, "/app/") &&
		(strings.HasSuffix(p, "layout.tsx") || strings.HasSuffix(p, "layout.ts")) {
		return &FrameworkHint{EntryPointMultiplier: 2.0, Reason: "nextjs-layout"}
	}

	// Express / Fastify / Hapi — route handler files
	if strings.Contains(p, "/routes/") && !strings.Contains(p, "/test/") {
		if strings.HasSuffix(p, ".ts") || strings.HasSuffix(p, ".js") {
			return &FrameworkHint{EntryPointMultiplier: 2.5, Reason: "express-route"}
		}
	}

	// ========== PYTHON FRAMEWORKS ==========

	// Django — views.py, urls.py files
	if strings.HasSuffix(p, "/views.py") && !strings.Contains(p, "/test") {
		return &FrameworkHint{EntryPointMultiplier: 2.5, Reason: "django-view"}
	}
	if strings.HasSuffix(p, "/urls.py") {
		return &FrameworkHint{EntryPointMultiplier: 2.0, Reason: "django-url-config"}
	}

	// Flask — app.py or files in routes/
	if strings.HasSuffix(p, "/app.py") && !strings.Contains(p, "/test") {
		return &FrameworkHint{EntryPointMultiplier: 2.5, Reason: "flask-app"}
	}

	// ========== JAVA FRAMEWORKS ==========

	// Spring — Controller, Service, Repository classes
	if strings.Contains(p, "/controller/") && strings.HasSuffix(p, ".java") {
		return &FrameworkHint{EntryPointMultiplier: 3.0, Reason: "spring-controller"}
	}
	if strings.Contains(p, "/service/") && strings.HasSuffix(p, ".java") {
		return &FrameworkHint{EntryPointMultiplier: 2.0, Reason: "spring-service"}
	}

	// ========== GO FRAMEWORKS ==========

	// Gin — handler files
	if strings.HasSuffix(p, "_handler.go") || strings.HasSuffix(p, "_router.go") ||
		strings.Contains(p, "/handler/") && strings.HasSuffix(p, ".go") {
		return &FrameworkHint{EntryPointMultiplier: 2.5, Reason: "gin-handler"}
	}

	// ========== C# FRAMEWORKS ==========

	// ASP.NET — Controller files
	if strings.Contains(p, "/controllers/") && strings.HasSuffix(p, ".cs") {
		return &FrameworkHint{EntryPointMultiplier: 3.0, Reason: "aspnet-controller"}
	}

	// ========== RUST FRAMEWORKS ==========

	// Actix-web / Axum — handler modules
	if strings.Contains(p, "/handlers/") && strings.HasSuffix(p, ".rs") {
		return &FrameworkHint{EntryPointMultiplier: 2.5, Reason: "rust-handler"}
	}

	// ========== C/C++ FRAMEWORKS ==========

	// No specific framework patterns for C/C++ — return nil

	return nil
}

// DetectFrameworkFromAST detects frameworks from AST definition text
// (decorators, annotations, attributes).  Deferred to Phase 6 when
// LanguageProviders are implemented.
// TODO(Phase 6): Implement AST-based framework detection per language provider.
func DetectFrameworkFromAST(_ string, _ string) *FrameworkHint {
	return nil
}