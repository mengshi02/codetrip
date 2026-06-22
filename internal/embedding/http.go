package embedding

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/mengshi02/codetrip/internal/graph"
	"github.com/mengshi02/codetrip/internal/store"
	"github.com/mengshi02/codetrip/internal/util"
)

// HTTPEmbedder is an HTTP API remote embedding service (OpenAI compatible)
type HTTPEmbedder struct {
	endpoint   string // API endpoint (e.g., "http://localhost:11434/v1/embeddings")
	model      string
	apiKey     string
	dimensions int
	httpClient *http.Client
	bufferPool sync.Pool
	store      *store.Store // optional, for EmbedBatch to write vectors
}

// NewHTTPEmbedder creates an HTTP remote embedder
func NewHTTPEmbedder(endpoint, model, apiKey string, dimensions int) *HTTPEmbedder {
	return &HTTPEmbedder{
		endpoint:   endpoint,
		model:      model,
		apiKey:     apiKey,
		dimensions: dimensions,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		bufferPool: sync.Pool{
			New: func() interface{} {
				return new(bytes.Buffer)
			},
		},
	}
}

// WithStore sets Pebble storage (for EmbedBatch)
func (e *HTTPEmbedder) WithStore(store *store.Store) *HTTPEmbedder {
	e.store = store
	return e
}

// Dimensions returns vector dimensions
func (e *HTTPEmbedder) Dimensions() int {
	return e.dimensions
}

// DetectDimensions probes the embedding endpoint with a sample text
// to auto-detect the vector dimensions. Sets e.dimensions if successful.
func (e *HTTPEmbedder) DetectDimensions(ctx context.Context) error {
	if e.dimensions > 0 {
		return nil // already known
	}

	slog.Info("embed: auto-detecting dimensions from endpoint", "endpoint", e.endpoint)

	results, err := e.Embed(ctx, []string{"test"})
	if err != nil {
		return fmt.Errorf("detect dimensions: probe request failed: %w", err)
	}
	if len(results) == 0 || len(results[0]) == 0 {
		return fmt.Errorf("detect dimensions: endpoint returned empty embedding")
	}

	e.dimensions = len(results[0])
	slog.Info("embed: auto-detected dimensions", "dimensions", e.dimensions)
	return nil
}

// embedRequest is an OpenAI compatible embedding request
type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// embedResponse is an OpenAI compatible embedding response
type embedResponse struct {
	Data []embedData `json:"data"`
}

type embedData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

// Embed converts text list to embedding vectors
func (e *HTTPEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	// Group by sub-batch size
	subBatch := e.defaultSubBatchSize()
	var allResults [][]float32

	for i := 0; i < len(texts); i += subBatch {
		end := i + subBatch
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		results, err := e.embedWithRetry(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}
		allResults = append(allResults, results...)
	}

	return allResults, nil
}

// EmbedBatch batch embeds nodes, writes results to Pebble
// nodes parameter uses []*graph.Node to match codetrip.Embedder interface
func (e *HTTPEmbedder) EmbedBatch(ctx context.Context, nodes []*graph.Node, config EmbedConfig) error {
	if e.store == nil {
		return fmt.Errorf("db store not configured for HTTPEmbedder")
	}
	if len(nodes) == 0 {
		return nil
	}

	type nodeInfo struct {
		repo    string
		nodeID  string
		content string
	}

	// Extract node info directly from *graph.Node
	infos := make([]nodeInfo, 0, len(nodes))
	texts := make([]string, 0, len(nodes))

	for _, n := range nodes {
		if n == nil {
			continue
		}
		repo := n.Repo
		nodeID := n.ID
		content := getNodeContent(n)
		if repo == "" || nodeID == "" || content == "" {
			continue
		}

		infos = append(infos, nodeInfo{repo: repo, nodeID: nodeID, content: content})
		texts = append(texts, content)
	}

	if len(texts) == 0 {
		return nil
	}

	// Group by batch size for embedding
	batchSize := config.BatchSize
	if batchSize <= 0 {
		batchSize = 16
	}

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}

		batchTexts := texts[i:end]
		embeddings, err := e.Embed(ctx, batchTexts)
		if err != nil {
			return fmt.Errorf("embed batch [%d:%d]: %w", i, end, err)
		}

		// Write to Pebble with dual-modal keys
		err = e.store.BatchNoSync(func(b *pebble.Batch) error {
			for j, emb := range embeddings {
				if i+j >= len(infos) {
					break
				}
				info := infos[i+j]

				// Code modality vector: embcode:{repo}:{nodeID}
				codeKey := graph.EmbCodeKey(info.repo, info.nodeID)
				vecData := util.EncodeFloat32Vec(emb)
				if err := b.Set([]byte(codeKey), vecData, nil); err != nil {
					return err
				}

				// Description modality vector: embdesc:{repo}:{nodeID}
				// Uses the same embedding since EmbedBatch only has content text.
				descKey := graph.EmbDescKey(info.repo, info.nodeID)
				if err := b.Set([]byte(descKey), vecData, nil); err != nil {
					return err
				}

				// Content hash: embhash:{repo}:{nodeID}
				hashKey := graph.EmbHashKey(info.repo, info.nodeID)
				h := sha1.Sum([]byte(info.content))
				hashVal := fmt.Sprintf("%x", h)
				if err := b.Set([]byte(hashKey), []byte(hashVal), nil); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("write batch to db: %w", err)
		}
	}

	return nil
}

// embedWithRetry performs embedding request with retry
func (e *HTTPEmbedder) embedWithRetry(ctx context.Context, texts []string) ([][]float32, error) {
	var lastErr error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		results, err := e.doEmbed(ctx, texts)
		if err == nil {
			return results, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

// doEmbed performs a single embedding request
func (e *HTTPEmbedder) doEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	// Build request
	reqBody := embedRequest{
		Model: e.model,
		Input: texts,
	}

	bodyData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Get buffer from pool
	buf := e.bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	buf.Write(bodyData)
	defer e.bufferPool.Put(buf)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.endpoint, buf)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if e.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.apiKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return nil, fmt.Errorf("http status %d (failed to read body: %v)", resp.StatusCode, readErr)
		}
		return nil, fmt.Errorf("http status %d: %s", resp.StatusCode, string(body))
	}

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var embedResp embedResponse
	if err := json.Unmarshal(respBody, &embedResp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	// Sort results by index
	results := make([][]float32, len(embedResp.Data))
	for _, d := range embedResp.Data {
		if d.Index < 0 || d.Index >= len(results) {
			continue
		}
		results[d.Index] = d.Embedding
	}

	return results, nil
}

func (e *HTTPEmbedder) defaultSubBatchSize() int {
	if e.dimensions > 0 {
		return 8
	}
	return 16
}