package wiki

import (
	"fmt"
	"strings"
)

// SymbolInfo represents symbol information (used for building prompts)
type SymbolInfo struct {
	Name     string
	Kind     string // Function, Class, Interface, Method, etc.
	FilePath string
	Doc      string // Brief documentation description
}

// RouteInfo represents route information (used for building API prompts)
type RouteInfo struct {
	Method      string // GET, POST, etc.
	Path        string // URL path
	Handler     string // Handler function name
	Description string // Route description
}

// CommunityInfo represents community information (used for building architecture prompts)
type CommunityInfo struct {
	Name        string
	Description string
	Members     []string // Community member name list
}

// EdgeInfo represents edge information (used for building architecture prompts)
type EdgeInfo struct {
	Source string
	Target string
	Type   string // Dependency type
}

// BuildOverviewPrompt builds project overview prompt
func BuildOverviewPrompt(communityInfo, processInfo, routeInfo string) string {
	var sb strings.Builder

	sb.WriteString("You are analyzing a software project's knowledge graph. Generate a comprehensive project overview document in Markdown format.\n\n")
	sb.WriteString("## Available Data\n\n")

	if communityInfo != "" {
		sb.WriteString("### Communities (Modules)\n")
		sb.WriteString(communityInfo)
		sb.WriteString("\n\n")
	}

	if processInfo != "" {
		sb.WriteString("### Processes (Data Flows)\n")
		sb.WriteString(processInfo)
		sb.WriteString("\n\n")
	}

	if routeInfo != "" {
		sb.WriteString("### Routes (API Endpoints)\n")
		sb.WriteString(routeInfo)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("Generate a project overview that:\n")
	sb.WriteString("1. Summarizes the overall architecture and purpose\n")
	sb.WriteString("2. Lists the main modules/communities and their responsibilities\n")
	sb.WriteString("3. Describes key data flows and processes\n")
	sb.WriteString("4. Highlights the API surface if routes are present\n")
	sb.WriteString("5. Uses clear Markdown headings and bullet points\n")

	return sb.String()
}

// BuildModulePrompt builds module documentation prompt
func BuildModulePrompt(moduleName string, symbols []SymbolInfo, deps []string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Generate detailed documentation for the module \"%s\" in Markdown format.\n\n", moduleName))

	if len(symbols) > 0 {
		sb.WriteString("## Symbols in this module\n\n")
		for _, sym := range symbols {
			sb.WriteString(fmt.Sprintf("- **%s** (%s)", sym.Name, sym.Kind))
			if sym.FilePath != "" {
				sb.WriteString(fmt.Sprintf(" — `%s`", sym.FilePath))
			}
			if sym.Doc != "" {
				sb.WriteString(fmt.Sprintf(": %s", sym.Doc))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(deps) > 0 {
		sb.WriteString("## Dependencies\n\n")
		for _, dep := range deps {
			sb.WriteString(fmt.Sprintf("- %s\n", dep))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString(fmt.Sprintf("Generate module documentation for \"%s\" that:\n", moduleName))
	sb.WriteString("1. Describes the module's purpose and responsibility\n")
	sb.WriteString("2. Documents each symbol with its role and usage\n")
	sb.WriteString("3. Explains key dependencies and why they are needed\n")
	sb.WriteString("4. Includes code examples where appropriate\n")

	return sb.String()
}

// BuildAPIPrompt builds API documentation prompt
func BuildAPIPrompt(routes []RouteInfo) string {
	var sb strings.Builder

	sb.WriteString("Generate API documentation in Markdown format based on the following route definitions.\n\n")

	if len(routes) > 0 {
		sb.WriteString("## Routes\n\n")
		sb.WriteString("| Method | Path | Handler | Description |\n")
		sb.WriteString("|--------|------|---------|-------------|\n")
		for _, r := range routes {
			sb.WriteString(fmt.Sprintf("| %s | `%s` | `%s` | %s |\n", r.Method, r.Path, r.Handler, r.Description))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("Generate API documentation that:\n")
	sb.WriteString("1. Groups routes by logical resource/domain\n")
	sb.WriteString("2. Documents request/response formats for each endpoint\n")
	sb.WriteString("3. Notes authentication requirements if evident\n")
	sb.WriteString("4. Includes example requests and responses\n")

	return sb.String()
}

// BuildArchitecturePrompt builds architecture diagram documentation prompt
func BuildArchitecturePrompt(communities []CommunityInfo, edges []EdgeInfo) string {
	var sb strings.Builder

	sb.WriteString("Generate an architecture document in Markdown format based on the following community and dependency structure.\n\n")

	if len(communities) > 0 {
		sb.WriteString("## Communities\n\n")
		for _, c := range communities {
			sb.WriteString(fmt.Sprintf("### %s\n", c.Name))
			if c.Description != "" {
				sb.WriteString(fmt.Sprintf("%s\n\n", c.Description))
			}
			if len(c.Members) > 0 {
				sb.WriteString("Members: ")
				sb.WriteString(strings.Join(c.Members, ", "))
				sb.WriteString("\n\n")
			}
		}
	}

	if len(edges) > 0 {
		sb.WriteString("## Dependencies\n\n")
		for _, e := range edges {
			sb.WriteString(fmt.Sprintf("- `%s` → `%s` (%s)\n", e.Source, e.Target, e.Type))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## Instructions\n\n")
	sb.WriteString("Generate an architecture document that:\n")
	sb.WriteString("1. Describes the high-level system design\n")
	sb.WriteString("2. Explains each community's role and boundaries\n")
	sb.WriteString("3. Analyzes dependency patterns and coupling\n")
	sb.WriteString("4. Includes a text-based dependency diagram (Mermaid syntax)\n")
	sb.WriteString("5. Identifies potential architectural improvements\n")

	return sb.String()
}