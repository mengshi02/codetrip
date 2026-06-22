package embedding

import (
	"strings"
	"sync"
)

// Chunk represents chunking result
type Chunk struct {
	Content   string
	StartLine int
	EndLine   int
	Mode      string // "ast-function" | "ast-declaration" | "character"
}

// Chunker is the code chunker
type Chunker struct {
	config EmbedConfig
}

// NewChunker creates a code chunker
func NewChunker(config EmbedConfig) *Chunker {
	return &Chunker{config: config}
}

var builderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

func getBuilder() *strings.Builder {
	b := builderPool.Get().(*strings.Builder)
	b.Reset()
	return b
}

func putBuilder(b *strings.Builder) {
	builderPool.Put(b)
}

// AST-aware node label classification
var (
	// function/method/constructor → ast-function chunk
	functionLabels = map[string]bool{
		"Function":    true,
		"Method":      true,
		"Constructor": true,
	}

	// declaration types → ast-declaration chunk
	declarationLabels = map[string]bool{
		"Class":     true,
		"Interface": true,
		"Struct":    true,
		"Enum":      true,
		"Trait":     true,
		"Impl":      true,
		"Macro":     true,
		"Namespace": true,
		"TypeAlias": true,
		"Typedef":   true,
		"Record":    true,
		"Union":     true,
		"Template":  true,
		"Type":      true,
		"Module":    true,
	}
)

// Chunk chunks the code text
// nodeLabel is used to determine chunking strategy
func (c *Chunker) Chunk(content string, nodeLabel string) []Chunk {
	if content == "" {
		return nil
	}

	mode := c.chunkMode(nodeLabel)

	switch mode {
	case "ast-function":
		return c.chunkByFunction(content)
	case "ast-declaration":
		return c.chunkByDeclaration(content)
	default:
		return c.chunkByCharacter(content)
	}
}

// chunkMode determines chunking mode based on node label
func (c *Chunker) chunkMode(nodeLabel string) string {
	if functionLabels[nodeLabel] {
		return "ast-function"
	}
	if declarationLabels[nodeLabel] {
		return "ast-declaration"
	}
	return "character"
}

// chunkByFunction performs function-level chunking
// Function bodies are typically small, use as a single chunk
func (c *Chunker) chunkByFunction(content string) []Chunk {
	if len(content) <= c.config.ChunkSize {
		lines := strings.Split(content, "\n")
		return []Chunk{{
			Content:   content,
			StartLine: 1,
			EndLine:   len(lines),
			Mode:      "ast-function",
		}}
	}

	// If function body is too large, fall back to character-level chunking
	chunks := c.chunkByCharacter(content)
	for i := range chunks {
		chunks[i].Mode = "ast-function"
	}
	return chunks
}

// chunkByDeclaration performs declaration-level chunking
// Class/interface/struct declarations, grouped by members
func (c *Chunker) chunkByDeclaration(content string) []Chunk {
	if len(content) <= c.config.ChunkSize {
		lines := strings.Split(content, "\n")
		return []Chunk{{
			Content:   content,
			StartLine: 1,
			EndLine:   len(lines),
			Mode:      "ast-declaration",
		}}
	}

	// Large declarations: chunk by indentation-level top-level members
	chunks := c.chunkByMember(content)
	if len(chunks) > 0 {
		return chunks
	}

	// Fall back to character-level chunking
	chunks = c.chunkByCharacter(content)
	for i := range chunks {
		chunks[i].Mode = "ast-declaration"
	}
	return chunks
}

// chunkByMember chunks by top-level members (for large class/struct)
func (c *Chunker) chunkByMember(content string) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk

	var currentB *strings.Builder = getBuilder()
	defer putBuilder(currentB)
	currentB.Reset()

	startLine := 1
	currentSize := 0

	// Find first non-empty line to determine base indentation
	baseIndent := -1
	for _, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")
		if trimmed == "" {
			continue
		}
		baseIndent = len(line) - len(trimmed)
		break
	}

	for i, line := range lines {
		trimmed := strings.TrimLeft(line, " \t")

		// Detect member boundary: indentation returns to base indent and not empty line
		if i > 0 && trimmed != "" && baseIndent >= 0 {
			indent := len(line) - len(trimmed)
			if indent == baseIndent && currentB.Len() > 0 {
				// Current chunk ends
				if currentSize > c.config.ChunkSize {
					// Chunk too large, character-level split
					subChunks := c.chunkByCharacter(currentB.String())
					for _, sc := range subChunks {
						sc.Mode = "ast-declaration"
						sc.StartLine += startLine - 1
						sc.EndLine += startLine - 1
						chunks = append(chunks, sc)
					}
				} else {
					chunks = append(chunks, Chunk{
						Content:   currentB.String(),
						StartLine: startLine,
						EndLine:   i,
						Mode:      "ast-declaration",
					})
				}
				currentB.Reset()
				currentSize = 0
				startLine = i + 1
			}
		}

		currentB.WriteString(line)
		currentB.WriteString("\n")
		currentSize += len(line) + 1
	}

	// Process last chunk
	if currentB.Len() > 0 {
		if currentSize > c.config.ChunkSize {
			subChunks := c.chunkByCharacter(currentB.String())
			for _, sc := range subChunks {
				sc.Mode = "ast-declaration"
				sc.StartLine += startLine - 1
				sc.EndLine += startLine - 1
				chunks = append(chunks, sc)
			}
		} else {
			chunks = append(chunks, Chunk{
				Content:   currentB.String(),
				StartLine: startLine,
				EndLine:   len(lines),
				Mode:      "ast-declaration",
			})
		}
	}

	return chunks
}

// chunkByCharacter performs character-level chunking (fallback strategy)
// Splits by line boundaries, supports overlap
func (c *Chunker) chunkByCharacter(content string) []Chunk {
	if content == "" {
		return nil
	}

	lines := strings.Split(content, "\n")
	var chunks []Chunk

	var currentB *strings.Builder = getBuilder()
	defer putBuilder(currentB)
	currentB.Reset()

	startLine := 1
	currentSize := 0
	chunkSize := c.config.ChunkSize
	overlap := c.config.Overlap

	if chunkSize <= 0 {
		chunkSize = 1200
	}
	if overlap < 0 {
		overlap = 0
	}
	if overlap >= chunkSize {
		overlap = chunkSize / 10
	}

	// For tracking overlap line index range
	overlapLineStart := 0

	for i, line := range lines {
		lineLen := len(line) + 1 // +1 for \n

		if currentSize+lineLen > chunkSize && currentB.Len() > 0 {
			// Current chunk complete
			chunks = append(chunks, Chunk{
				Content:   currentB.String(),
				StartLine: startLine,
				EndLine:   i,
				Mode:      "character",
			})

			// Roll back overlap lines
			currentB.Reset()
			currentSize = 0
			if overlap > 0 && overlapLineStart < i {
				// Calculate rollback line count
				overlapSize := 0
				overlapStart := i - 1
				for overlapStart >= overlapLineStart && overlapSize < overlap {
					overlapSize += len(lines[overlapStart]) + 1
					if overlapSize > overlap {
						overlapStart++
						break
					}
					if overlapStart <= overlapLineStart {
						break
					}
					overlapStart--
				}
				startLine = overlapStart + 1
				for j := overlapStart; j < i; j++ {
					currentB.WriteString(lines[j])
					currentB.WriteString("\n")
					currentSize += len(lines[j]) + 1
				}
			} else {
				startLine = i + 1
			}
			overlapLineStart = startLine - 1
		}

		currentB.WriteString(line)
		currentB.WriteString("\n")
		currentSize += lineLen
	}

	// Last chunk
	if currentB.Len() > 0 {
		chunks = append(chunks, Chunk{
			Content:   currentB.String(),
			StartLine: startLine,
			EndLine:   len(lines),
			Mode:      "character",
		})
	}

	return chunks
}
