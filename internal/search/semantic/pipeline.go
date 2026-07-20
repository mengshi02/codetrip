package semantic

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/cockroachdb/pebble/v2"
	"github.com/coder/hnsw"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
	"github.com/mengshi02/codetrip/internal/util"
)

// Embedder is the embedding model interface (locally defined, compatible with codetrip.Embedder)
type Embedder interface {
	Dimensions() int
	Embed(ctx context.Context, texts []string) ([][]float32, error)
}

// EmbeddingPipeline is the embedding pipeline
type EmbeddingPipeline struct {
	embedder  Embedder
	graph     *graph.GraphStore
	config    EmbedConfig
	chunker   *Chunker
	store     *store.Store
	dataDir   string // base data directory for vector files
	quantInt8 bool   // enable int8 quantization
}

// NewEmbeddingPipeline creates an embedding pipeline
func NewEmbeddingPipeline(embedder Embedder, graphStore *graph.GraphStore, store *store.Store, config EmbedConfig) *EmbeddingPipeline {
	return &EmbeddingPipeline{
		embedder: embedder,
		graph:    graphStore,
		config:   config,
		chunker:  NewChunker(config),
		store:    store,
	}
}

// NewEmbeddingPipelineWithDir creates an embedding pipeline with data directory for vector files.
func NewEmbeddingPipelineWithDir(embedder Embedder, graphStore *graph.GraphStore, store *store.Store, config EmbedConfig, dataDir string, quantInt8 bool) *EmbeddingPipeline {
	return &EmbeddingPipeline{
		embedder:  embedder,
		graph:     graphStore,
		config:    config,
		chunker:   NewChunker(config),
		store:     store,
		dataDir:   dataDir,
		quantInt8: quantInt8,
	}
}

// PipelineResult represents pipeline run result
type PipelineResult struct {
	NodesEmbedded int
	ChunksCreated int
	DescChunks    int // description modality chunk count
	CodeChunks    int // code modality chunk count
	Errors        int
}

// Embeddable node labels
var embeddableLabels = map[graph.Label]bool{
	graph.LabelFunction:    true,
	graph.LabelMethod:      true,
	graph.LabelConstructor: true,
	graph.LabelClass:       true,
	graph.LabelInterface:   true,
	graph.LabelStruct:      true,
	graph.LabelEnum:        true,
	graph.LabelTrait:       true,
	graph.LabelImpl:        true,
	graph.LabelMacro:       true,
	graph.LabelNamespace:   true,
	graph.LabelTypeAlias:   true,
	graph.LabelTypedef:     true,
	graph.LabelConst:       true,
	graph.LabelProperty:    true,
	graph.LabelRecord:      true,
	graph.LabelUnion:       true,
	graph.LabelStatic:      true,
	graph.LabelVariable:    true,
	graph.LabelTemplate:    true,
	graph.LabelType:        true,
	graph.LabelModule:      true,
	graph.LabelDelegate:    true,
	graph.LabelAnnotation:  true,
	graph.LabelDecorator:   true,
}

type embedNode struct {
	node    *graph.Node
	content string
	chunks  []Chunk
}

// Run executes the dual-modal embedding pipeline.
func (p *EmbeddingPipeline) Run(ctx context.Context, repo string) (*PipelineResult, error) {
	return p.RunDualModal(ctx, repo)
}

// updateModalityIndex updates the modality-specific embedding index.
// modality is "desc" or "code", mapping to embdescidx:{repo} or embcodeidx:{repo}.
func (p *EmbeddingPipeline) updateModalityIndex(repo, modality string, newNodeIDs map[string]bool) error {
	var idxKey string
	switch modality {
	case "desc":
		idxKey = graph.EmbDescIdxKey(repo)
	case "code":
		idxKey = graph.EmbCodeIdxKey(repo)
	default:
		return fmt.Errorf("unknown modality: %s", modality)
	}

	// Read existing index
	var existingIDs []string
	existing, err := p.store.Get([]byte(idxKey))
	if err == nil {
		existingIDs, _ = util.DecodeStringList(existing)
	}

	// Merge new node IDs
	idSet := make(map[string]bool)
	for _, id := range existingIDs {
		idSet[id] = true
	}
	for id := range newNodeIDs {
		idSet[id] = true
	}

	merged := make([]string, 0, len(idSet))
	for id := range idSet {
		merged = append(merged, id)
	}

	data := util.EncodeStringList(merged)
	return p.store.SetNoSync([]byte(idxKey), data)
}

// getNodeContent gets the code content of a node
func getNodeContent(node *graph.Node) string {
	// Prefer content property
	if v, ok := node.Props.GetProp("content"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	// Then use snippet property
	if v, ok := node.Props.GetProp("snippet"); ok {
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}

// buildSynthesizedContent creates a code-like text representation from node
// properties when no actual source content is available. This allows embedding
// even when the indexing pipeline did not store source code content.
func buildSynthesizedContent(node *graph.Node) string {
	var b strings.Builder

	// Signature line (e.g. "func processOrder(order Order) error")
	b.WriteString(buildNodeSignature(node))
	b.WriteByte('\n')

	// File location
	if node.FilePath != "" {
		startLine := node.GetPropInt("startLine")
		endLine := node.GetPropInt("endLine")
		if startLine > 0 && endLine > 0 {
			fmt.Fprintf(&b, "// File: %s:%d-%d\n", node.FilePath, startLine, endLine)
		} else if startLine > 0 {
			fmt.Fprintf(&b, "// File: %s:%d\n", node.FilePath, startLine)
		} else {
			fmt.Fprintf(&b, "// File: %s\n", node.FilePath)
		}
	}

	// Visibility
	if v, ok := node.Props.GetProp("visibility"); ok {
		fmt.Fprintf(&b, "// Visibility: %v\n", v)
	}

	// Qualified name
	if v, ok := node.Props.GetProp("qualifiedName"); ok {
		fmt.Fprintf(&b, "// Qualified: %v\n", v)
	}

	// Description (doc comments)
	if v, ok := node.Props.GetProp("description"); ok {
		if s, ok := v.(string); ok && s != "" {
			fmt.Fprintf(&b, "// %s\n", s)
		}
	}

	// Return type
	if v, ok := node.Props.GetProp("returnType"); ok {
		fmt.Fprintf(&b, "// Returns: %v\n", v)
	}

	// Base types (for classes/interfaces)
	if v, ok := node.Props.GetProp("baseTypes"); ok {
		if sl, ok := v.([]string); ok && len(sl) > 0 {
			fmt.Fprintf(&b, "// Extends/Implements: %s\n", strings.Join(sl, ", "))
		}
	}

	// Receiver (for methods)
	if v, ok := node.Props.GetProp("receiver"); ok {
		fmt.Fprintf(&b, "// Receiver: %v\n", v)
	}

	// Annotations/decorators
	if v, ok := node.Props.GetProp("annotations"); ok {
		if sl, ok := v.([]string); ok && len(sl) > 0 {
			for _, a := range sl {
				fmt.Fprintf(&b, "@%s\n", a)
			}
		}
	}

	result := b.String()
	if len(result) <= 1 { // just a newline
		return ""
	}
	return result
}

// RunDualModal executes the dual-modal embedding pipeline:
// iterate nodes → generate description + code text → embed → store vectors.
// This is the primary embedding method, replacing the old single-modal Run().
func (p *EmbeddingPipeline) RunDualModal(ctx context.Context, repo string) (*PipelineResult, error) {
	result := &PipelineResult{}

	// Check if embedder is available — return empty result silently (no error)
	if p.embedder == nil {
		slog.Warn("embed: embedder is nil, skipping embedding")
		return result, nil
	}
	if p.embedder.Dimensions() == 0 {
		slog.Warn("embed: embedder dimensions is 0, skipping embedding (specify --dimensions or use an endpoint that supports auto-detect)")
		return result, nil
	}

	// Build embed context (pre-load adjacency data to avoid N+1 queries)
	ec, err := buildEmbedContext(p.graph)
	if err != nil {
		return nil, fmt.Errorf("build embed context: %w", err)
	}

	// Iterate all nodes in graph, collect embeddable nodes
	var nodes []embedNode
	var totalIterated int
	var labelSkipped int
	var contentSkipped int
	skippedLabels := make(map[graph.Label]int)
	iter := p.graph.IterNodes(repo)
	defer iter.Close()

	for iter.Next() {
		totalIterated++
		node := iter.Node()
		if !embeddableLabels[node.Label] {
			labelSkipped++
			skippedLabels[node.Label]++
			continue
		}

		// Get code content: prefer actual source content, fallback to signature
		content := getNodeContent(node)
		if content == "" {
			// No source content stored — synthesize from node properties
			content = buildSynthesizedContent(node)
		}
		if content == "" {
			contentSkipped++
			continue
		}

		// Chunking (code modality)
		chunks := p.chunker.Chunk(content, string(node.Label))
		if len(chunks) == 0 {
			continue
		}

		nodes = append(nodes, embedNode{
			node:    node,
			content: content,
			chunks:  chunks,
		})
		result.ChunksCreated += len(chunks)
	}

	if len(nodes) == 0 {
		// Log skipped label details
		topSkipped := make([]string, 0, 5)
		for lbl, cnt := range skippedLabels {
			topSkipped = append(topSkipped, fmt.Sprintf("%s=%d", lbl, cnt))
		}
		slog.Info("embed: no embeddable nodes found",
			"repo", repo,
			"total_iterated", totalIterated,
			"label_skipped", labelSkipped,
			"content_skipped", contentSkipped,
			"top_skipped_labels", topSkipped,
		)
		return result, nil
	}

	// Process in batch size groups using dual-modal combined batching
	batchSize := p.config.BatchSize
	if batchSize <= 0 {
		batchSize = 16
	}

	for i := 0; i < len(nodes); i += batchSize {
		if ctx.Err() != nil {
			return result, ctx.Err()
		}

		end := i + batchSize
		if end > len(nodes) {
			end = len(nodes)
		}

		batch := nodes[i:end]
		descCount, codeCount, err := p.processDualModalBatch(ctx, repo, batch, ec)
		if err != nil {
			result.Errors++
			continue
		}
		result.NodesEmbedded += len(batch)
		result.DescChunks += descCount
		result.CodeChunks += codeCount
	}

	// Build quantized vector file if quantization is enabled
	if p.quantInt8 && p.dataDir != "" && result.NodesEmbedded > 0 {
		if err := p.buildQuantizedVectorFile(ctx, repo); err != nil {
			slog.Warn("embedding: failed to build quantized vector file", "repo", repo, "error", err)
		}
	}

	return result, nil
}

// processDualModalBatch processes a batch of nodes with dual-modal generation.
// It combines description + code texts into a single Embed() call to reduce
// HTTP overhead by ~50% (description texts are short, typically < 200 tokens).
func (p *EmbeddingPipeline) processDualModalBatch(ctx context.Context, repo string, batch []embedNode, ec *embedContext) (descChunks, codeChunks int, err error) {
	// Pre-allocate slices: each node has 1 desc text + N code chunks
	totalTexts := len(batch) // at least 1 desc per node
	for _, en := range batch {
		totalTexts += len(en.chunks)
	}
	allTexts := make([]string, 0, totalTexts)
	refs := make([]modalityRef, 0, totalTexts)

	for ni, en := range batch {
		// Description modality text
		descText := buildDescriptionText(en.node, ec)
		if p.config.MaxSnippetLength > 0 && len(descText) > p.config.MaxSnippetLength {
			descText = descText[:p.config.MaxSnippetLength]
		}
		allTexts = append(allTexts, descText)
		refs = append(refs, modalityRef{nodeIdx: ni, modality: "desc", chunkIdx: 0})
		descChunks++

		// Code modality chunks
		for ci, chunk := range en.chunks {
			text := chunk.Content
			if p.config.MaxSnippetLength > 0 && len(text) > p.config.MaxSnippetLength {
				text = text[:p.config.MaxSnippetLength]
			}
			allTexts = append(allTexts, text)
			refs = append(refs, modalityRef{nodeIdx: ni, modality: "code", chunkIdx: ci})
			codeChunks++
		}
	}

	if len(allTexts) == 0 {
		return 0, 0, nil
	}

	// Single Embed call: desc + code combined batch
	embeddings, err := p.embedder.Embed(ctx, allTexts)
	if err != nil {
		return descChunks, codeChunks, fmt.Errorf("embed texts: %w", err)
	}

	if len(embeddings) != len(allTexts) {
		return descChunks, codeChunks, fmt.Errorf("embedding count mismatch: got %d, want %d", len(embeddings), len(allTexts))
	}

	// Write to Pebble with dual-modal key prefixes
	descNodeIDs := make(map[string]bool)
	codeNodeIDs := make(map[string]bool)

	err = p.store.BatchNoSync(func(b *pebble.Batch) error {
		for ri, emb := range embeddings {
			if ri >= len(refs) {
				break
			}
			ref := refs[ri]
			node := batch[ref.nodeIdx].node

			vecData := util.EncodeFloat32Vec(emb)

			switch ref.modality {
			case "desc":
				// Description modality: embdesc:{repo}:{nodeID}
				descNodeIDs[node.ID] = true
				descKey := graph.EmbDescKey(repo, node.ID)
				if err := b.Set([]byte(descKey), vecData, nil); err != nil {
					return err
				}

			case "code":
				if ref.chunkIdx == 0 {
					// Code modality node-level vector: embcode:{repo}:{nodeID}
					codeNodeIDs[node.ID] = true
					codeKey := graph.EmbCodeKey(repo, node.ID)
					if err := b.Set([]byte(codeKey), vecData, nil); err != nil {
						return err
					}

				}

				// Code chunk vector: embcode:{repo}:{nodeID}:{chunkIdx}
				codeChunkKey := graph.EmbCodeChunkKey(repo, node.ID, ref.chunkIdx)
				if err := b.Set([]byte(codeChunkKey), vecData, nil); err != nil {
					return err
				}
			}
		}
		return nil
	})

	if err != nil {
		return descChunks, codeChunks, fmt.Errorf("write embeddings to db: %w", err)
	}

	// Update dual-modal embedding indices separately
	if err := p.updateModalityIndex(repo, "desc", descNodeIDs); err != nil {
		return descChunks, codeChunks, fmt.Errorf("update desc embedding index: %w", err)
	}
	if err := p.updateModalityIndex(repo, "code", codeNodeIDs); err != nil {
		return descChunks, codeChunks, fmt.Errorf("update code embedding index: %w", err)
	}

	return descChunks, codeChunks, nil
}

// buildQuantizedVectorFile builds the quantized vector file for a repo.
// It reads all float32 vectors from dual-modal keys (embdesc:/embcode:),
// trains quantization parameters, quantizes to int8, and writes the vector file.
func (p *EmbeddingPipeline) buildQuantizedVectorFile(ctx context.Context, repo string) error {
	slog.Info("embedding: building quantized vector file", "repo", repo)

	type vecEntry struct {
		nodeID string
		vec    []float32
		chunk  bool // true if this is a chunk-level vector
	}

	// Helper: read vectors for a modality
	readModalityVecs := func(modality string) ([]vecEntry, error) {
		var idxKey string
		var keyFn func(string, string) string
		var chunkKeyFn func(string, string, int) string
		switch modality {
		case "desc":
			idxKey = graph.EmbDescIdxKey(repo)
			keyFn = graph.EmbDescKey
			chunkKeyFn = graph.EmbDescChunkKey
		case "code":
			idxKey = graph.EmbCodeIdxKey(repo)
			keyFn = graph.EmbCodeKey
			chunkKeyFn = graph.EmbCodeChunkKey
		default:
			return nil, fmt.Errorf("unknown modality: %s", modality)
		}

		idxData, err := p.store.Get([]byte(idxKey))
		if err != nil {
			return nil, nil // no data for this modality
		}
		nodeIDs, _ := util.DecodeStringList(idxData)

		var entries []vecEntry
		for _, nodeID := range nodeIDs {
			vecKey := keyFn(repo, nodeID)
			vecData, err := p.store.Get([]byte(vecKey))
			if err != nil {
				continue
			}
			vec := util.DecodeFloat32Vec(vecData)
			entries = append(entries, vecEntry{nodeID: nodeID, vec: vec})

			// Read chunk-level vectors
			for ci := 1; ; ci++ {
				chunkKey := chunkKeyFn(repo, nodeID, ci)
				chunkData, err := p.store.Get([]byte(chunkKey))
				if err != nil {
					break
				}
				chunkVec := util.DecodeFloat32Vec(chunkData)
				entries = append(entries, vecEntry{nodeID: nodeID, vec: chunkVec, chunk: true})
			}
		}
		return entries, nil
	}

	// Read both modalities
	descEntries, _ := readModalityVecs("desc")
	codeEntries, _ := readModalityVecs("code")

	var allVecs [][]float32
	var nodeVecs []vecEntry
	var chunkVecs []vecEntry

	for _, e := range descEntries {
		allVecs = append(allVecs, e.vec)
		if e.chunk {
			chunkVecs = append(chunkVecs, e)
		} else {
			nodeVecs = append(nodeVecs, e)
		}
	}
	for _, e := range codeEntries {
		allVecs = append(allVecs, e.vec)
		if e.chunk {
			chunkVecs = append(chunkVecs, e)
		} else {
			nodeVecs = append(nodeVecs, e)
		}
	}

	if len(allVecs) == 0 {
		return nil
	}

	// Train quantization parameters from all vectors
	dim := len(allVecs[0])
	params := hnsw.TrainQuantParams(allVecs)

	// Build vector file
	writer := NewVectorFileWriter(dim, params.Scale, params.Offset)
	for _, entry := range nodeVecs {
		qvec := hnsw.Quantize(entry.vec, params)
		writer.AddNodeVector(qvec)
	}
	for _, entry := range chunkVecs {
		qvec := hnsw.Quantize(entry.vec, params)
		writer.AddChunkVector(qvec)
	}

	vecPath := VectorFilePath(p.dataDir, repo)
	if err := writer.Write(vecPath); err != nil {
		return fmt.Errorf("write quantized vector file: %w", err)
	}

	slog.Info("embedding: quantized vector file built",
		"repo", repo,
		"nodes", len(nodeVecs),
		"chunks", len(chunkVecs),
		"dim", dim,
		"path", vecPath,
	)

	return nil
}
