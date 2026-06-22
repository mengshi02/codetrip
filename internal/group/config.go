package group

import (
	"encoding/json"
	"fmt"
	"strings"
)

// GroupConfig is the group configuration (corresponds to group.yaml)
type GroupConfig struct {
	Version     int                          `json:"version"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Repos       map[string]string            `json:"repos"`    // repo name → path
	Links       []GroupManifestLink          `json:"links"`    // manual contract links
	Packages    map[string]map[string]string `json:"packages"` // repo → package → version
	Detect      DetectConfig                 `json:"detect"`
	Matching    MatchingConfig               `json:"matching"`
}

// GroupManifestLink represents a manual contract link
type GroupManifestLink struct {
	SourceRepo   string `json:"sourceRepo"`
	SourceSymbol string `json:"sourceSymbol"`
	TargetRepo   string `json:"targetRepo"`
	TargetSymbol string `json:"targetSymbol"`
	Type         string `json:"type"` // http/grpc/thrift/topic/lib/include
}

// DetectConfig is the contract detection configuration
type DetectConfig struct {
	EnabledTypes []string `json:"enabledTypes"` // default ["http","grpc","topic","lib"]
	Languages    []string `json:"languages"`
}

// MatchingConfig is the matching configuration
type MatchingConfig struct {
	BM25Threshold      float64 `json:"bm25Threshold"`      // default 0.6
	EmbeddingThreshold float64 `json:"embeddingThreshold"` // default 0.85
	MaxMatches         int     `json:"maxMatches"`         // default 10
}

// DefaultMatchingConfig returns the default matching configuration
func DefaultMatchingConfig() MatchingConfig {
	return MatchingConfig{
		BM25Threshold:      0.6,
		EmbeddingThreshold: 0.85,
		MaxMatches:         10,
	}
}

// DefaultDetectConfig returns the default detection configuration
func DefaultDetectConfig() DetectConfig {
	return DetectConfig{
		EnabledTypes: []string{"http", "grpc", "topic", "lib"},
		Languages:    []string{},
	}
}

// ParseGroupConfig parses GroupConfig from YAML/JSON
// Uses simple string processing to convert YAML to JSON, then deserializes
func ParseGroupConfig(data []byte) (*GroupConfig, error) {
	jsonData, err := yamlToJSON(data)
	if err != nil {
		return nil, fmt.Errorf("yaml to json: %w", err)
	}

	config := &GroupConfig{
		Detect:   DefaultDetectConfig(),
		Matching: DefaultMatchingConfig(),
		Repos:    make(map[string]string),
		Packages: make(map[string]map[string]string),
	}

	if err := json.Unmarshal(jsonData, config); err != nil {
		return nil, fmt.Errorf("unmarshal group config: %w", err)
	}

	if config.Name == "" {
		return nil, fmt.Errorf("group config: name is required")
	}

	// Fill default values
	if len(config.Detect.EnabledTypes) == 0 {
		config.Detect = DefaultDetectConfig()
	}
	if config.Matching.BM25Threshold == 0 {
		config.Matching.BM25Threshold = 0.6
	}
	if config.Matching.EmbeddingThreshold == 0 {
		config.Matching.EmbeddingThreshold = 0.85
	}
	if config.Matching.MaxMatches == 0 {
		config.Matching.MaxMatches = 10
	}

	return config, nil
}

// yamlToJSON is a simple YAML to JSON converter
// Does not introduce external dependencies, uses line-based parsing to handle simple YAML structures
func yamlToJSON(data []byte) ([]byte, error) {
	lines := strings.Split(string(data), "\n")
	var obj map[string]any = make(map[string]any)

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Simple key: value parsing
		idx := strings.Index(trimmed, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(trimmed[:idx])
		val := strings.TrimSpace(trimmed[idx+1:])

		if val == "" {
			// May be a nested structure, skip
			continue
		}

		// Remove quotes
		val = strings.Trim(val, "\"")
		val = strings.Trim(val, "'")

		obj[key] = parseYAMLValue(val)
	}

	return json.Marshal(obj)
}

// parseYAMLValue parses a YAML scalar value
func parseYAMLValue(val string) any {
	// Number
	if strings.Contains(val, ".") {
		var f float64
		if _, err := fmt.Sscanf(val, "%f", &f); err == nil {
			return f
		}
	}
	var i int
	if _, err := fmt.Sscanf(val, "%d", &i); err == nil {
		return i
	}
	// Boolean
	if val == "true" {
		return true
	}
	if val == "false" {
		return false
	}
	// List
	if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
		inner := strings.Trim(val, "[]")
		items := strings.Split(inner, ",")
		result := make([]any, 0, len(items))
		for _, item := range items {
			trimmed := strings.TrimSpace(strings.Trim(item, "\"'"))
			if trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result
	}
	return val
}