package wiki

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// LLMClient is the LLM client interface
type LLMClient interface {
	Complete(ctx context.Context, prompt string, opts ...LLMOption) (string, error)
}

// LLMOption is the LLM call option function type
type LLMOption func(*LLMOptions)

// LLMOptions is the LLM call options
type LLMOptions struct {
	Model       string
	Temperature float64
	MaxTokens   int
}

// WithModel sets the model name
func WithModel(model string) LLMOption {
	return func(o *LLMOptions) {
		o.Model = model
	}
}

// WithTemperature sets the temperature parameter
func WithTemperature(temp float64) LLMOption {
	return func(o *LLMOptions) {
		o.Temperature = temp
	}
}

// WithMaxTokens sets the max token count
func WithMaxTokens(tokens int) LLMOption {
	return func(o *LLMOptions) {
		o.MaxTokens = tokens
	}
}

// applyOptions applies options
func applyOptions(opts ...LLMOption) LLMOptions {
	o := LLMOptions{
		Model:       "",
		Temperature: 0.7,
		MaxTokens:   2048,
	}
	for _, opt := range opts {
		opt(&o)
	}
	return o
}

// ============ NoopLLMClient ============

// NoopLLMClient is a no-op implementation (default, returns templated documentation)
type NoopLLMClient struct{}

// Complete returns templated documentation (does not call any LLM)
func (n *NoopLLMClient) Complete(_ context.Context, prompt string, opts ...LLMOption) (string, error) {
	return generateTemplateResponse(prompt), nil
}

// generateTemplateResponse generates templated documentation based on prompt keywords
func generateTemplateResponse(prompt string) string {
	switch {
	case strings.Contains(prompt, "overview") || strings.Contains(prompt, "概览"):
		return "# Project Overview\n\nThis project is a code analysis engine. The wiki content is generated from the knowledge graph structure.\n\n## Key Components\n\nRefer to the individual module sections for detailed documentation."
	case strings.Contains(prompt, "module") || strings.Contains(prompt, "模块"):
		return "## Module Documentation\n\nThis module provides core functionality. Key symbols and their roles are documented below.\n\n### Symbols\n\nSee the symbol list for detailed API documentation."
	case strings.Contains(prompt, "API") || strings.Contains(prompt, "api") || strings.Contains(prompt, "route") || strings.Contains(prompt, "路由"):
		return "## API Documentation\n\n### Endpoints\n\n| Method | Path | Handler | Description |\n|--------|------|---------|-------------|\n| GET | /api/health | HealthCheck | Service health check |\n\nRefer to the route definitions for the complete API surface."
	case strings.Contains(prompt, "architecture") || strings.Contains(prompt, "架构"):
		return "## Architecture\n\nThe system is organized into several communities (modules) with well-defined dependencies.\n\n### Community Structure\n\nEach community encapsulates related functionality with minimal cross-boundary coupling."
	default:
		return "# Generated Documentation\n\nDocumentation content generated from knowledge graph analysis."
	}
}

// ============ CLILLMClient ============

// CLILLMClient is an LLM client that calls local CLI commands (e.g., ollama run)
type CLILLMClient struct {
	Command string        // CLI command, e.g., "ollama"
	Args    []string      // Command argument prefix, e.g., ["run", "llama3"]
	Timeout time.Duration
}

// NewCLILLMClient creates a CLI LLM client
func NewCLILLMClient(command string, args []string) *CLILLMClient {
	return &CLILLMClient{
		Command: command,
		Args:    args,
		Timeout: 60 * time.Second,
	}
}

// bufferPool reuses bytes.Buffer
var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

// Complete calls local CLI command to complete LLM request
func (c *CLILLMClient) Complete(ctx context.Context, prompt string, opts ...LLMOption) (string, error) {
	options := applyOptions(opts...)

	timeout := c.Timeout
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d < timeout {
			timeout = d
		}
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	args := make([]string, len(c.Args))
	copy(args, c.Args)

	if options.Model != "" {
		args = append(args, options.Model)
	}

	cmd := exec.CommandContext(ctx, c.Command, args...)

	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufferPool.Put(buf)

	buf.WriteString(prompt)
	cmd.Stdin = buf

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cli llm command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// ============ HTTPOpenLLMClient ============

// HTTPOpenLLMClient is an LLM client that HTTP POSTs to OpenAI-compatible APIs
type HTTPOpenLLMClient struct {
	BaseURL    string // e.g., "http://localhost:11434"
	APIKey     string
	Model      string
	HTTPClient *http.Client
}

// NewHTTPOpenLLMClient creates an OpenAI-compatible API client
func NewHTTPOpenLLMClient(baseURL, apiKey, model string) *HTTPOpenLLMClient {
	return &HTTPOpenLLMClient{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		Model:   model,
		HTTPClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// chatRequest is the OpenAI-compatible API request format
type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens"`
}

// chatMessage is a chat message
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// chatResponse is the OpenAI-compatible API response format
type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// Complete calls OpenAI-compatible API to complete LLM request
func (c *HTTPOpenLLMClient) Complete(ctx context.Context, prompt string, opts ...LLMOption) (string, error) {
	options := applyOptions(opts...)

	model := c.Model
	if options.Model != "" {
		model = options.Model
	}

	reqBody := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: "You are a technical documentation writer. Generate clear, well-structured documentation based on the provided code analysis data."},
			{Role: "user", Content: prompt},
		},
		Temperature: options.Temperature,
		MaxTokens:   options.MaxTokens,
	}

	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	url := c.BaseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("api returned status %d: %s", resp.StatusCode, string(body))
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", fmt.Errorf("unmarshal response: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
}