package framework

// defaultPathRules returns default file path matching rules
func defaultPathRules() []PathRule {
	return []PathRule{
		// Next.js Pages Router
		{
			Pattern:    "pages/",
			Framework:  "nextjs-pages",
			Multiplier: 1.5,
			Reason:     "Next.js pages/ directory detected",
		},
		// Next.js App Router
		{
			Pattern:    "app/",
			Framework:  "nextjs-app",
			Multiplier: 1.5,
			Reason:     "Next.js app/ directory detected",
		},
		// Laravel
		{
			Pattern:    "routes/web.php",
			Framework:  "laravel",
			Multiplier: 1.4,
			Reason:     "Laravel routes/web.php detected",
		},
		{
			Pattern:    "routes/api.php",
			Framework:  "laravel",
			Multiplier: 1.4,
			Reason:     "Laravel routes/api.php detected",
		},
		// Django
		{
			Pattern:    "urls.py",
			Framework:  "django",
			Multiplier: 1.2,
			Reason:     "Django urls.py detected",
		},
		{
			Pattern:    "views.py",
			Framework:  "django",
			Multiplier: 1.1,
			Reason:     "Django views.py detected",
		},
		// Rails
		{
			Pattern:    "config/routes.rb",
			Framework:  "rails",
			Multiplier: 1.4,
			Reason:     "Rails config/routes.rb detected",
		},
		// Spring Boot
		{
			Pattern:    "Application.java",
			Framework:  "springboot",
			Multiplier: 1.1,
			Reason:     "Spring Boot Application.java detected",
		},
		// ASP.NET
		{
			Pattern:    "Program.cs",
			Framework:  "aspnet",
			Multiplier: 1.1,
			Reason:     "ASP.NET Program.cs detected",
		},
		{
			Pattern:    "Startup.cs",
			Framework:  "aspnet",
			Multiplier: 1.1,
			Reason:     "ASP.NET Startup.cs detected",
		},
		{
			Pattern:    "Controller.cs",
			Framework:  "aspnet",
			Multiplier: 1.2,
			Reason:     "ASP.NET Controller detected",
		},
	}
}

// defaultSymbolRules returns default symbol/decorator matching rules
func defaultSymbolRules() []SymbolRule {
	return []SymbolRule{
		// Spring Boot
		{
			Name:       "Controller",
			Label:      "Decorator",
			Framework:  "springboot",
			Multiplier: 1.5,
			Reason:     "Spring @Controller annotation detected",
		},
		{
			Name:       "RestController",
			Label:      "Decorator",
			Framework:  "springboot",
			Multiplier: 1.5,
			Reason:     "Spring @RestController annotation detected",
		},
		{
			Name:       "GetMapping",
			Label:      "Decorator",
			Framework:  "springboot",
			Multiplier: 1.3,
			Reason:     "Spring @GetMapping annotation detected",
		},
		{
			Name:       "PostMapping",
			Label:      "Decorator",
			Framework:  "springboot",
			Multiplier: 1.3,
			Reason:     "Spring @PostMapping annotation detected",
		},
		{
			Name:       "RequestMapping",
			Label:      "Decorator",
			Framework:  "springboot",
			Multiplier: 1.3,
			Reason:     "Spring @RequestMapping annotation detected",
		},
		// Express
		{
			Name:       "express",
			Receiver:   "",
			Framework:  "express",
			Multiplier: 1.4,
			Reason:     "Express framework call detected",
		},
		{
			Name:       "get",
			Receiver:   "app",
			Framework:  "express",
			Multiplier: 1.3,
			Reason:     "Express app.get() detected",
		},
		{
			Name:       "post",
			Receiver:   "app",
			Framework:  "express",
			Multiplier: 1.3,
			Reason:     "Express app.post() detected",
		},
		// Gin (Go)
		{
			Name:       "Default",
			Receiver:   "gin",
			Framework:  "gin",
			Multiplier: 1.4,
			Reason:     "Gin gin.Default() detected",
		},
		{
			Name:       "New",
			Receiver:   "gin",
			Framework:  "gin",
			Multiplier: 1.4,
			Reason:     "Gin gin.New() detected",
		},
		// FastAPI
		{
			Name:       "get",
			Receiver:   "app",
			Framework:  "fastapi",
			Multiplier: 1.3,
			Reason:     "FastAPI @app.get decorator detected",
		},
		{
			Name:       "post",
			Receiver:   "app",
			Framework:  "fastapi",
			Multiplier: 1.3,
			Reason:     "FastAPI @app.post decorator detected",
		},
		// ASP.NET
		{
			Name:       "ApiController",
			Label:      "Decorator",
			Framework:  "aspnet",
			Multiplier: 1.5,
			Reason:     "ASP.NET [ApiController] attribute detected",
		},
		{
			Name:       "HttpGet",
			Label:      "Decorator",
			Framework:  "aspnet",
			Multiplier: 1.3,
			Reason:     "ASP.NET [HttpGet] attribute detected",
		},
		{
			Name:       "HttpPost",
			Label:      "Decorator",
			Framework:  "aspnet",
			Multiplier: 1.3,
			Reason:     "ASP.NET [HttpPost] attribute detected",
		},
		// Expo/React Native
		{
			Name:       "expo",
			Receiver:   "",
			Framework:  "expo",
			Multiplier: 1.3,
			Reason:     "Expo import detected",
		},
	}
}

// ============ Built-in framework detector implementations ============

// NextJSPagesDetector is the Next.js Pages Router detector
type NextJSPagesDetector struct{}

func (d *NextJSPagesDetector) DetectFromPath(filePath string) *FrameworkHint {
	if containsSegment(filePath, "pages") {
		return &FrameworkHint{
			Framework:            "nextjs-pages",
			EntryPointMultiplier: 1.5,
			Reason:               "Next.js pages/ directory",
		}
	}
	return nil
}

func (d *NextJSPagesDetector) DetectFromAST(_ string, _ []SymbolMatch) *FrameworkHint {
	return nil
}

// NextJSAppDetector is the Next.js App Router detector
type NextJSAppDetector struct{}

func (d *NextJSAppDetector) DetectFromPath(filePath string) *FrameworkHint {
	if containsSegment(filePath, "app") {
		return &FrameworkHint{
			Framework:            "nextjs-app",
			EntryPointMultiplier: 1.5,
			Reason:               "Next.js app/ directory",
		}
	}
	return nil
}

func (d *NextJSAppDetector) DetectFromAST(_ string, _ []SymbolMatch) *FrameworkHint {
	return nil
}

// ExpoDetector is the Expo detector
type ExpoDetector struct{}

func (d *ExpoDetector) DetectFromPath(filePath string) *FrameworkHint {
	if containsSegment(filePath, "app") {
		return &FrameworkHint{
			Framework:            "expo",
			EntryPointMultiplier: 1.3,
			Reason:               "Expo app/ directory",
		}
	}
	return nil
}

func (d *ExpoDetector) DetectFromAST(_ string, symbols []SymbolMatch) *FrameworkHint {
	for _, sym := range symbols {
		if sym.Name == "expo" || sym.Name == "Expo" {
			return &FrameworkHint{
				Framework:            "expo",
				EntryPointMultiplier: 1.3,
				Reason:               "Expo import detected",
			}
		}
	}
	return nil
}

// LaravelDetector is the Laravel detector
type LaravelDetector struct{}

func (d *LaravelDetector) DetectFromPath(filePath string) *FrameworkHint {
	if containsSegment(filePath, "routes") && hasSuffix(filePath, ".php") {
		return &FrameworkHint{
			Framework:            "laravel",
			EntryPointMultiplier: 1.4,
			Reason:               "Laravel routes/ directory",
		}
	}
	return nil
}

func (d *LaravelDetector) DetectFromAST(_ string, _ []SymbolMatch) *FrameworkHint {
	return nil
}

// SpringBootDetector is the Spring Boot detector
type SpringBootDetector struct{}

func (d *SpringBootDetector) DetectFromPath(_ string) *FrameworkHint {
	return nil
}

func (d *SpringBootDetector) DetectFromAST(_ string, symbols []SymbolMatch) *FrameworkHint {
	for _, sym := range symbols {
		if sym.Label == "Decorator" || sym.Label == "Annotation" {
			switch sym.Name {
			case "Controller", "RestController", "GetMapping", "PostMapping", "RequestMapping":
				return &FrameworkHint{
					Framework:            "springboot",
					EntryPointMultiplier: 1.5,
					Reason:               "Spring @" + sym.Name + " annotation",
				}
			}
		}
	}
	return nil
}

// ExpressDetector is the Express detector
type ExpressDetector struct{}

func (d *ExpressDetector) DetectFromPath(_ string) *FrameworkHint {
	return nil
}

func (d *ExpressDetector) DetectFromAST(_ string, symbols []SymbolMatch) *FrameworkHint {
	for _, sym := range symbols {
		if sym.Name == "express" {
			return &FrameworkHint{
				Framework:            "express",
				EntryPointMultiplier: 1.4,
				Reason:               "express() call detected",
			}
		}
		if sym.Receiver == "app" && (sym.Name == "get" || sym.Name == "post" || sym.Name == "put" || sym.Name == "delete") {
			return &FrameworkHint{
				Framework:            "express",
				EntryPointMultiplier: 1.3,
				Reason:               "Express app." + sym.Name + "() detected",
			}
		}
	}
	return nil
}

// DjangoDetector is the Django detector
type DjangoDetector struct{}

func (d *DjangoDetector) DetectFromPath(filePath string) *FrameworkHint {
	if hasSuffix(filePath, "urls.py") || hasSuffix(filePath, "views.py") {
		return &FrameworkHint{
			Framework:            "django",
			EntryPointMultiplier: 1.3,
			Reason:               "Django " + filePath + " detected",
		}
	}
	return nil
}

func (d *DjangoDetector) DetectFromAST(_ string, _ []SymbolMatch) *FrameworkHint {
	return nil
}

// RailsDetector is the Rails detector
type RailsDetector struct{}

func (d *RailsDetector) DetectFromPath(filePath string) *FrameworkHint {
	if containsSegment(filePath, "config") && hasSuffix(filePath, "routes.rb") {
		return &FrameworkHint{
			Framework:            "rails",
			EntryPointMultiplier: 1.4,
			Reason:               "Rails config/routes.rb detected",
		}
	}
	return nil
}

func (d *RailsDetector) DetectFromAST(_ string, _ []SymbolMatch) *FrameworkHint {
	return nil
}

// ASPNetDetector is the ASP.NET detector
type ASPNetDetector struct{}

func (d *ASPNetDetector) DetectFromPath(_ string) *FrameworkHint {
	return nil
}

func (d *ASPNetDetector) DetectFromAST(_ string, symbols []SymbolMatch) *FrameworkHint {
	for _, sym := range symbols {
		if sym.Label == "Decorator" || sym.Label == "Annotation" {
			switch sym.Name {
			case "ApiController", "Controller", "HttpGet", "HttpPost", "HttpPut", "HttpDelete":
				return &FrameworkHint{
					Framework:            "aspnet",
					EntryPointMultiplier: 1.5,
					Reason:               "ASP.NET [" + sym.Name + "] attribute",
				}
			}
		}
	}
	return nil
}

// GinDetector is the Gin detector
type GinDetector struct{}

func (d *GinDetector) DetectFromPath(_ string) *FrameworkHint {
	return nil
}

func (d *GinDetector) DetectFromAST(_ string, symbols []SymbolMatch) *FrameworkHint {
	for _, sym := range symbols {
		if sym.Receiver == "gin" && (sym.Name == "Default" || sym.Name == "New") {
			return &FrameworkHint{
				Framework:            "gin",
				EntryPointMultiplier: 1.4,
				Reason:               "Gin gin." + sym.Name + "() detected",
			}
		}
		if sym.Name == "GET" || sym.Name == "POST" || sym.Name == "PUT" || sym.Name == "DELETE" {
			// Check if it's a method call on gin.Engine
			if sym.Receiver == "r" || sym.Receiver == "router" || sym.Receiver == "engine" {
				return &FrameworkHint{
					Framework:            "gin",
					EntryPointMultiplier: 1.3,
					Reason:               "Gin router." + sym.Name + "() detected",
				}
			}
		}
	}
	return nil
}

// FastAPIDetector is the FastAPI detector
type FastAPIDetector struct{}

func (d *FastAPIDetector) DetectFromPath(_ string) *FrameworkHint {
	return nil
}

func (d *FastAPIDetector) DetectFromAST(_ string, symbols []SymbolMatch) *FrameworkHint {
	for _, sym := range symbols {
		if sym.Receiver == "app" && (sym.Name == "get" || sym.Name == "post" || sym.Name == "put" || sym.Name == "delete") {
			if sym.Label == "Decorator" {
				return &FrameworkHint{
					Framework:            "fastapi",
					EntryPointMultiplier: 1.4,
					Reason:               "FastAPI @app." + sym.Name + " decorator",
				}
			}
		}
	}
	return nil
}

// ============ Helper functions ============

// containsSegment checks if path contains the specified segment
func containsSegment(path, segment string) bool {
	parts := splitPath(path)
	for _, p := range parts {
		if p == segment {
			return true
		}
	}
	return false
}

// hasSuffix checks if path ends with the specified suffix
func hasSuffix(path, suffix string) bool {
	return len(path) >= len(suffix) && path[len(path)-len(suffix):] == suffix
}

// splitPath splits the path
func splitPath(path string) []string {
	var parts []string
	current := ""
	for _, c := range path {
		if c == '/' || c == '\\' {
			if current != "" {
				parts = append(parts, current)
				current = ""
			}
		} else {
			current += string(c)
		}
	}
	if current != "" {
		parts = append(parts, current)
	}
	return parts
}
