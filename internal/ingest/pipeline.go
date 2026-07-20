package ingest

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mengshi02/codetrip/internal/ingest/enrich"
	graph "github.com/mengshi02/codetrip/internal/model"
)

// PipelineStage represents a stage in the ingestion pipeline.
type PipelineStage string

const (
	StageScan      PipelineStage = "scan"
	StageStructure PipelineStage = "structure"
	StageParse     PipelineStage = "parse"
	StageResolve   PipelineStage = "resolve"
	StageMRO       PipelineStage = "mro"
	StageCommunity PipelineStage = "community"
	StageProcess   PipelineStage = "process"
)

// PipelineResult holds the result of the full ingestion pipeline.
type PipelineResult struct {
	Graph       *graph.KnowledgeGraph
	Duration    time.Duration
	StageStats  map[PipelineStage]StageStat
	WalkResult  *WalkResult
	LangConfigs *LanguageConfigs
}

// StageStat holds statistics for a pipeline stage.
type StageStat struct {
	Duration   time.Duration
	NodeCount  int
	RelCount   int
	ErrorCount int
}

// Pipeline orchestrates the full ingestion pipeline.
// scan → structure → parse+resolve → MRO → communities → processes
type Pipeline struct {
	RepoPath string
	CSVDir   string
	Verbose  bool
}

// NewPipeline creates a new ingestion pipeline.
func NewPipeline(repoPath string, csvDir string, verbose bool) *Pipeline {
	return &Pipeline{
		RepoPath: repoPath,
		CSVDir:   csvDir,
		Verbose:  verbose,
	}
}

// Run executes the full ingestion pipeline and returns the result.
func (p *Pipeline) Run() (*PipelineResult, error) {
	result := &PipelineResult{
		StageStats: make(map[PipelineStage]StageStat),
	}
	totalStart := time.Now()

	// Stage 1: Scan (walk repository)
	if p.Verbose {
		log.Println("[pipeline] Stage 1: Scanning repository...")
	}
	stageStart := time.Now()
	walkResult, err := WalkRepository(p.RepoPath)
	if err != nil {
		return nil, fmt.Errorf("stage scan: %w", err)
	}
	result.WalkResult = walkResult
	result.StageStats[StageScan] = StageStat{
		Duration:  time.Since(stageStart),
		NodeCount: 0,
		RelCount:  0,
	}
	if p.Verbose {
		log.Printf("[pipeline] Scan complete: %d files, %d bytes\n",
			walkResult.TotalFiles, walkResult.TotalSize)
	}

	// Load project configs
	result.LangConfigs = LoadAllLanguageConfigs(p.RepoPath)

	// Create knowledge graph
	kg := graph.NewKnowledgeGraph()
	result.Graph = kg

	// Stage 2: Structure (File/Folder nodes + CONTAINS)
	if p.Verbose {
		log.Println("[pipeline] Stage 2: Processing structure...")
	}
	stageStart = time.Now()
	ProcessStructure(kg, walkResult)
	result.StageStats[StageStructure] = StageStat{
		Duration:  time.Since(stageStart),
		NodeCount: kg.NodeCount(),
		RelCount:  kg.RelationshipCount(),
	}
	if p.Verbose {
		log.Printf("[pipeline] Structure complete: %d nodes, %d relationships\n",
			kg.NodeCount(), kg.RelationshipCount())
	}

	// Stage 3: Parse + Resolve (tree-sitter parsing + import/call/heritage resolution)
	if p.Verbose {
		log.Println("[pipeline] Stage 3: Parse + Resolve...")
	}
	stageStart = time.Now()

	symbolTable := NewSymbolTable()
	registry := NewLanguageRegistry()

	// Build file inputs from walk result
	var fileInputs []FileInput
	var allPaths []string
	for _, f := range walkResult.Files {
		allPaths = append(allPaths, f.RelativePath)
		if f.LanguageID == "" {
			continue
		}
		if !registry.HasBinding(f.LanguageID) {
			continue
		}
		content, err := ReadFileContents(f.AbsolutePath, f.Size)
		if err != nil {
			continue // skip unreadable / too-large files
		}
		fileInputs = append(fileInputs, FileInput{
			Path:    f.RelativePath,
			Content: content,
		})
	}

	// Parse all files — definitions go into graph + symbolTable; ExtractedData for fast-path
	extracted := ProcessParsing(kg, fileInputs, symbolTable, registry, nil)
	if extracted != nil {
		log.Printf("[pipeline] ExtractedData: %d imports, %d calls, %d heritage, %d routes",
			len(extracted.Imports), len(extracted.Calls), len(extracted.Heritage), len(extracted.Routes))
	} else {
		log.Printf("[pipeline] ExtractedData is nil!")
	}

	// Build import resolution context (suffix index + file lists)
	normalizedFileList := make([]string, len(allPaths))
	copy(normalizedFileList, allPaths)
	resolveCtx := BuildImportResolutionContext(normalizedFileList, allPaths)

	importMap := NewImportMap()
	packageMap := NewPackageMap()
	namedImportMap := make(NamedImportMap)
	importOrderMap := make(ImportOrderMap)

	if extracted != nil && len(extracted.Imports) > 0 {
		// Fast path: convert ExtractedImport → ImportQueryCapture
		captures := make([]ImportQueryCapture, len(extracted.Imports))
		for i, imp := range extracted.Imports {
			importPath := cleanImportPath(imp.ImportPath)
			namedBinding := ""
			exportedName := ""
			if imp.Language == "kotlin" && !strings.HasSuffix(importPath, ".*") {
				namedBinding = importPath
				if idx := strings.LastIndex(namedBinding, "."); idx >= 0 {
					namedBinding = namedBinding[idx+1:]
				}
				exportedName = namedBinding
			}
			captures[i] = ImportQueryCapture{
				FilePath: imp.FilePath, ImportPath: importPath, Language: imp.Language,
				NamedBinding: namedBinding, ExportedName: exportedName,
			}
		}
		ProcessImportsFromExtracted(kg, captures, importMap, packageMap, namedImportMap, result.LangConfigs, resolveCtx, importOrderMap)
	} else {
		// Sequential fallback: re-parse for imports
		langMap := make(map[string]string)
		for _, fi := range fileInputs {
			langMap[fi.Path] = GetLanguageFromFilename(fi.Path)
		}
		ProcessImports(kg, registry, pathsFromInputs(fileInputs), langMap,
			nil, nil, nil, importMap, packageMap, namedImportMap, result.LangConfigs, resolveCtx)
	}

	// Go same-package implicit visibility: files in the same directory are mutually
	// visible without explicit import. Add them to packageMap so Tier 2b can resolve.
	// Go same-package visibility is handled by Tier 2b (importMap from import resolution)
	// and Tier 3 (unique-global fallback).

	// Finalize range index for enclosing function lookup (must be after all Add() calls)
	symbolTable.FinalizeRangeIndex()

	// Resolve calls from extracted data
	if extracted != nil {
		assignableOwners := BuildAssignableOwnerIDs(
			extracted.Heritage, symbolTable, importMap, packageMap,
			namedImportMap, importOrderMap,
		)
		ProcessCallsFromExtracted(kg, extracted.Calls, symbolTable, importMap, packageMap, namedImportMap, importOrderMap, assignableOwners)
		ProcessHeritageFromExtracted(kg, extracted.Heritage, symbolTable, importMap, packageMap, namedImportMap, importOrderMap)
		ProcessRoutesFromExtracted(kg, extracted.Routes, symbolTable, importMap, packageMap)
	}

	result.StageStats[StageParse] = StageStat{
		Duration:  time.Since(stageStart),
		NodeCount: kg.NodeCount(),
		RelCount:  kg.RelationshipCount(),
	}
	if p.Verbose {
		log.Printf("[pipeline] Parse+Resolve complete: %d nodes, %d relationships\n",
			kg.NodeCount(), kg.RelationshipCount())
	}

	// Stage 4: MRO (Method Resolution Order)
	if p.Verbose {
		log.Println("[pipeline] Stage 4: MRO...")
	}
	stageStart = time.Now()
	mroResult := enrich.ComputeMRO(kg)
	result.StageStats[StageMRO] = StageStat{
		Duration:  time.Since(stageStart),
		NodeCount: kg.NodeCount(),
		RelCount:  kg.RelationshipCount(),
	}
	if p.Verbose {
		log.Printf("[pipeline] MRO complete: %d classes, %d OVERRIDES edges, %d ambiguities\n",
			len(mroResult.Entries), mroResult.OverrideEdges, mroResult.AmbiguityCount)
	}

	// Stage 5: Communities (Leiden algorithm)
	if p.Verbose {
		log.Println("[pipeline] Stage 5: Communities...")
	}
	stageStart = time.Now()
	communityResult := enrich.ProcessCommunities(kg)
	enrich.ApplyCommunitiesToGraph(kg, communityResult)
	result.StageStats[StageCommunity] = StageStat{
		Duration:  time.Since(stageStart),
		NodeCount: kg.NodeCount(),
		RelCount:  kg.RelationshipCount(),
	}
	if p.Verbose {
		log.Printf("[pipeline] Communities complete: %d communities (modularity %.3f)\n",
			communityResult.Stats.TotalCommunities, communityResult.Stats.Modularity)
	}

	// Stage 6: Processes (BFS trace detection)
	if p.Verbose {
		log.Println("[pipeline] Stage 6: Processes...")
	}
	stageStart = time.Now()

	// Dynamic maxProcesses: scale with symbol count
	symbolCount := 0
	kg.ForEachNode(func(n *graph.GraphNode) {
		if n.Label != graph.LabelFile {
			symbolCount++
		}
	})
	dynamicMaxProcesses := 20 + symbolCount/10
	if dynamicMaxProcesses > 300 {
		dynamicMaxProcesses = 300
	}

	processResult := enrich.ProcessProcesses(kg, communityResult.Memberships, enrich.ProcessDetectionConfig{
		MaxTraceDepth: 10,
		MaxBranching:  4,
		MaxProcesses:  dynamicMaxProcesses,
		MinSteps:      3,
	})
	enrich.ApplyProcessesToGraph(kg, processResult)
	result.StageStats[StageProcess] = StageStat{
		Duration:  time.Since(stageStart),
		NodeCount: kg.NodeCount(),
		RelCount:  kg.RelationshipCount(),
	}
	if p.Verbose {
		log.Printf("[pipeline] Processes complete: %d processes (%d cross-community)\n",
			processResult.Stats.TotalProcesses, processResult.Stats.CrossCommunityCount)
	}

	result.Duration = time.Since(totalStart)
	if p.Verbose {
		log.Printf("[pipeline] Pipeline complete in %v: %d nodes, %d relationships\n",
			result.Duration, kg.NodeCount(), kg.RelationshipCount())
	}

	return result, nil
}

// pathsFromInputs extracts file paths from FileInput slice.
func pathsFromInputs(inputs []FileInput) []string {
	paths := make([]string, len(inputs))
	for i, fi := range inputs {
		paths[i] = fi.Path
	}
	return paths
}
