package wiki

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"text/template"

	"github.com/mengshi02/codetrip/internal/graph"
)

// WikiConfig is the Wiki generation configuration
type WikiConfig struct {
	OutputDir   string // Output directory
	Format      string // html | markdown
	LLMProvider string // noop | cli | http
}

// WikiResult is the Wiki generation result
type WikiResult struct {
	Sections      []WikiSection
	TotalSections int
}

// WikiSection is a Wiki document section
type WikiSection struct {
	Title   string
	Content string
	Type    string // overview | module | api | architecture
}

// WikiGenerator automatically generates project Wiki documentation based on knowledge graph
type WikiGenerator struct {
	graph  *graph.GraphStore
	llm    LLMClient
	config WikiConfig
}

// NewWikiGenerator creates a Wiki generator
func NewWikiGenerator(g *graph.GraphStore, llm LLMClient, config WikiConfig) *WikiGenerator {
	if llm == nil {
		llm = &NoopLLMClient{}
	}
	if config.Format == "" {
		config.Format = "markdown"
	}
	return &WikiGenerator{
		graph:  g,
		llm:    llm,
		config: config,
	}
}

// Generate extracts structure from knowledge graph, calls LLM to generate documentation
func (g *WikiGenerator) Generate(ctx context.Context, repo string) (*WikiResult, error) {
	// 1. Extract community nodes from GraphStore (MEMBER_OF edges)
	communities := g.extractCommunities(repo)
	// 2. Extract process nodes from GraphStore (STEP_IN_PROCESS edges)
	processes := g.extractProcesses(repo)
	// 3. Extract route nodes from GraphStore (Route label)
	routes := g.extractRoutes(repo)
	// 4. Build module information
	modules := g.extractModules(repo, communities)

	// 5. Build sections concurrently
	type sectionResult struct {
		section WikiSection
		err     error
	}

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Add fixed sections
	tasks := []struct {
		title string
		typ   string
		fn    func() (string, error)
	}{
		{"Project Overview", "overview", func() (string, error) {
			prompt := BuildOverviewPrompt(
				formatCommunityInfo(communities),
				formatProcessInfo(processes),
				formatRouteInfo(routes),
			)
			return g.llm.Complete(ctx, prompt)
		}},
		{"API Documentation", "api", func() (string, error) {
			prompt := BuildAPIPrompt(routes)
			return g.llm.Complete(ctx, prompt)
		}},
		{"Architecture", "architecture", func() (string, error) {
			prompt := BuildArchitecturePrompt(
				communitiesToInfo(communities),
				extractEdges(g.graph, repo),
			)
			return g.llm.Complete(ctx, prompt)
		}},
	}

	// Add module sections
	for _, mod := range modules {
		mod := mod // capture
		tasks = append(tasks, struct {
			title string
			typ   string
			fn    func() (string, error)
		}{
			title: fmt.Sprintf("Module: %s", mod.Name),
			typ:   "module",
			fn: func() (string, error) {
				prompt := BuildModulePrompt(mod.Name, mod.Symbols, mod.Deps)
				return g.llm.Complete(ctx, prompt)
			},
		})
	}

	results := make([]sectionResult, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t struct {
			title string
			typ   string
			fn    func() (string, error)
		}) {
			defer wg.Done()
			content, err := t.fn()
			mu.Lock()
			results[idx] = sectionResult{
				section: WikiSection{
					Title:   t.title,
					Content: content,
					Type:    t.typ,
				},
				err: err,
			}
			mu.Unlock()
		}(i, task)
	}

	wg.Wait()

	// Collect results
	var wikiSections []WikiSection
	var firstErr error
	for _, r := range results {
		if r.err != nil {
			if firstErr == nil {
				firstErr = r.err
			}
			// Keep sections with errors, use error message as content
			r.section.Content = fmt.Sprintf("Error generating documentation: %v", r.err)
		}
		wikiSections = append(wikiSections, r.section)
	}

	result := &WikiResult{
		Sections:      wikiSections,
		TotalSections: len(wikiSections),
	}

	// 6. Render output
	if err := g.render(result); err != nil && firstErr == nil {
		firstErr = err
	}

	return result, firstErr
}

// ============ Data Extraction ============

// communityData represents community data
type communityData struct {
	Name        string
	Description string
	Members     []*graph.Node
}

// moduleData represents module data
type moduleData struct {
	Name    string
	Symbols []SymbolInfo
	Deps    []string
}

// extractCommunities extracts community nodes (uses iterator to avoid full load)
func (g *WikiGenerator) extractCommunities(repo string) []communityData {
	it := g.graph.IterNodes(repo)
	defer it.Close()

	communityMap := make(map[string]*communityData)

	for it.Next() {
		node := it.Node()
		if node.Label != graph.LabelCommunity {
			continue
		}

		cd := &communityData{
			Name:        node.Name,
			Description: node.GetPropString("description"),
		}
		communityMap[node.ID] = cd

		// Find members with MEMBER_OF edges pointing to this community
		inEdges, err := g.graph.GetInEdges(node.ID, string(graph.RelMemberOf))
		if err != nil {
			continue
		}
		for _, edge := range inEdges {
			member, err := g.graph.GetNode(edge.Source)
			if err != nil {
				continue
			}
			cd.Members = append(cd.Members, member)
		}
	}

	result := make([]communityData, 0, len(communityMap))
	for _, cd := range communityMap {
		result = append(result, *cd)
	}
	return result
}

// extractProcesses extracts process nodes (uses iterator)
func (g *WikiGenerator) extractProcesses(repo string) []string {
	it := g.graph.IterNodes(repo)
	defer it.Close()

	var processes []string
	for it.Next() {
		node := it.Node()
		if node.Label != graph.LabelProcess {
			continue
		}
		// Collect STEP_IN_PROCESS steps
		outEdges, err := g.graph.GetOutEdges(node.ID, string(graph.RelStepInProcess))
		if err != nil {
			continue
		}
		steps := make([]string, 0, len(outEdges))
		for _, edge := range outEdges {
			step, err := g.graph.GetNode(edge.Target)
			if err != nil {
				continue
			}
			steps = append(steps, step.Name)
		}
		processes = append(processes, fmt.Sprintf("%s: %s", node.Name, strings.Join(steps, " → ")))
	}
	return processes
}

// extractRoutes extracts route nodes (uses iterator)
func (g *WikiGenerator) extractRoutes(repo string) []RouteInfo {
	it := g.graph.IterNodes(repo)
	defer it.Close()

	var routes []RouteInfo
	for it.Next() {
		node := it.Node()
		if node.Label != graph.LabelRoute {
			continue
		}
		ri := RouteInfo{
			Method:      node.GetPropString("method"),
			Path:        node.GetPropString("path"),
			Handler:     node.GetPropString("handler"),
			Description: node.GetPropString("description"),
		}
		if ri.Path == "" {
			ri.Path = node.Name
		}
		routes = append(routes, ri)
	}
	return routes
}

// extractModules extracts module data from community information
func (g *WikiGenerator) extractModules(repo string, communities []communityData) []moduleData {
	var modules []moduleData

	for _, c := range communities {
		mod := moduleData{
			Name: c.Name,
		}
		for _, member := range c.Members {
			mod.Symbols = append(mod.Symbols, SymbolInfo{
				Name:     member.Name,
				Kind:     string(member.Label),
				FilePath: member.FilePath,
				Doc:      member.GetPropString("doc"),
			})
		}
		// Collect module dependencies (members' IMPORTS out-edges point to nodes in other communities)
		depSet := make(map[string]struct{})
		for _, member := range c.Members {
			outEdges, err := g.graph.GetOutEdges(member.ID, string(graph.RelImports))
			if err != nil {
				continue
			}
			for _, edge := range outEdges {
				target, err := g.graph.GetNode(edge.Target)
				if err != nil {
					continue
				}
				depSet[target.Name] = struct{}{}
			}
		}
		for dep := range depSet {
			mod.Deps = append(mod.Deps, dep)
		}
		modules = append(modules, mod)
	}

	return modules
}

// ============ Formatting Helpers ============

// formatCommunityInfo formats community information as text
func formatCommunityInfo(communities []communityData) string {
	var sb strings.Builder
	for _, c := range communities {
		sb.WriteString(fmt.Sprintf("- **%s**: %s (members: %d)\n", c.Name, c.Description, len(c.Members)))
	}
	return sb.String()
}

// formatProcessInfo formats process information as text
func formatProcessInfo(processes []string) string {
	if len(processes) == 0 {
		return ""
	}
	return strings.Join(processes, "\n")
}

// formatRouteInfo formats route information as text
func formatRouteInfo(routes []RouteInfo) string {
	var sb strings.Builder
	for _, r := range routes {
		sb.WriteString(fmt.Sprintf("- %s %s → %s\n", r.Method, r.Path, r.Handler))
	}
	return sb.String()
}

// communitiesToInfo converts community data to CommunityInfo
func communitiesToInfo(communities []communityData) []CommunityInfo {
	result := make([]CommunityInfo, len(communities))
	for i, c := range communities {
		members := make([]string, len(c.Members))
		for j, m := range c.Members {
			members[j] = m.Name
		}
		result[i] = CommunityInfo{
			Name:        c.Name,
			Description: c.Description,
			Members:     members,
		}
	}
	return result
}

// extractEdges extracts edge information between communities
func extractEdges(gs *graph.GraphStore, repo string) []EdgeInfo {
	it := gs.IterNodes(repo)
	defer it.Close()

	var edges []EdgeInfo
	for it.Next() {
		node := it.Node()
		if node.Label != graph.LabelCommunity {
			continue
		}
		outEdges, err := gs.GetAllOutEdges(node.ID)
		if err != nil {
			continue
		}
		for _, edge := range outEdges {
			target, err := gs.GetNode(edge.Target)
			if err != nil {
				continue
			}
			if target.Label == graph.LabelCommunity {
				edges = append(edges, EdgeInfo{
					Source: node.Name,
					Target: target.Name,
					Type:   string(edge.Type),
				})
			}
		}
	}
	return edges
}

// ============ Render Output ============

// render renders Wiki results to output directory
func (g *WikiGenerator) render(result *WikiResult) error {
	if g.config.OutputDir == "" {
		return nil
	}

	if err := os.MkdirAll(g.config.OutputDir, 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	switch g.config.Format {
	case "html":
		return g.renderHTML(result)
	case "markdown", "":
		return g.renderMarkdown(result)
	default:
		return fmt.Errorf("unsupported format: %s", g.config.Format)
	}
}

// renderMarkdown renders to Markdown file
func (g *WikiGenerator) renderMarkdown(result *WikiResult) error {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	for _, section := range result.Sections {
		buf.WriteString(fmt.Sprintf("# %s\n\n", section.Title))
		buf.WriteString(section.Content)
		buf.WriteString("\n\n---\n\n")
	}

	outPath := filepath.Join(g.config.OutputDir, "wiki.md")
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

// renderHTML renders to HTML file
func (g *WikiGenerator) renderHTML(result *WikiResult) error {
	tmpl, err := template.New("wiki").Parse(wikiHTMLTemplate)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	if err := tmpl.Execute(buf, result); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	outPath := filepath.Join(g.config.OutputDir, "wiki.html")
	return os.WriteFile(outPath, buf.Bytes(), 0o644)
}

// wikiHTMLTemplate is the HTML template
const wikiHTMLTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Wiki</title>
<style>
body{margin:0;font-family:system-ui,-apple-system,sans-serif;display:flex;min-height:100vh}
.sidebar{width:240px;background:#f5f5f5;padding:16px;border-right:1px solid #ddd;flex-shrink:0}
.sidebar h2{margin:0 0 12px;font-size:16px}
.sidebar ul{list-style:none;padding:0;margin:0}
.sidebar li{padding:6px 8px;cursor:pointer;border-radius:4px;font-size:14px}
.sidebar li:hover{background:#e0e0e0}
.sidebar li.active{background:#1976d2;color:#fff}
.main{flex:1;padding:24px;overflow-y:auto}
.section{display:none}
.section.active{display:block}
h1{color:#333;border-bottom:2px solid #1976d2;padding-bottom:8px}
</style>
</head>
<body>
<div class="sidebar">
<h2>Navigation</h2>
<ul>
{{range $i, $s := .Sections}}
<li onclick="show({{$i}})" id="nav-{{$i}}">{{$s.Title}}</li>
{{end}}
</ul>
</div>
<div class="main">
{{range $i, $s := .Sections}}
<div class="section{{if eq $i 0}} active{{end}}" id="section-{{$i}}">
<h1>{{$s.Title}}</h1>
<div>{{$s.Content}}</div>
</div>
{{end}}
</div>
<script>
function show(i){document.querySelectorAll('.section').forEach(function(e){e.classList.remove('active')});document.querySelectorAll('.sidebar li').forEach(function(e){e.classList.remove('active')});document.getElementById('section-'+i).classList.add('active');document.getElementById('nav-'+i).classList.add('active')}
</script>
</body>
</html>`