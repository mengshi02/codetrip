package csv

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/mengshi02/codetrip/internal/model"
)

const flushEvery = 500

// nonPrintableRegex matches control characters except \t(0x09), \n(0x0A), \r(0x0D).
var nonPrintableRegex = regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F\x7F]`)

// SanitizeUTF8 cleans a string for safe CSV output.
//  1. \r\n → \n, \r → \n
//  2. Remove control chars [\x00-\x08\x0B\x0C\x0E-\x1F\x7F]
//  3. Remove UTF-16 surrogates [\uD800-\uDFFF]
//     In JS, emoji and other beyond-BMP characters are encoded as
//     surrogate pairs (e.g. 😊 = \uD83D\uDE0A), so they get removed.
//     In Go, strings are UTF-8 and surrogates don't appear. To replicate
//     the JS behavior, we remove runes >= 0x10000 (beyond BMP), which
//     are exactly the codepoints that JS represents as surrogates.
//     Valid UTF-8 surrogates (0xD800-0xDFFF) are also removed as invalid.
//  4. Remove non-characters \uFFFE and \uFFFF.
func SanitizeUTF8(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = nonPrintableRegex.ReplaceAllString(s, "")
	// Remove beyond-BMP runes (>= 0x10000) — these are emoji, CJK extensions,
	// Also remove invalid UTF-8 surrogates (0xD800-0xDFFF) and
	// non-characters U+FFFE, U+FFFF.
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if r >= 0x10000 || (r >= 0xD800 && r <= 0xDFFF) || r == 0xFFFE || r == 0xFFFF {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// EscapeCSVField wraps a value in double quotes, escaping internal quotes.
func EscapeCSVField(value interface{}) string {
	if value == nil {
		return `""`
	}
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case int:
		s = strconv.Itoa(v)
	case int64:
		s = strconv.FormatInt(v, 10)
	case float64:
		s = strconv.FormatFloat(v, 'f', -1, 64)
	case bool:
		s = strconv.FormatBool(v)
	default:
		s = fmt.Sprintf("%v", v)
	}
	s = SanitizeUTF8(s)
	s = strings.ReplaceAll(s, `"`, `""`)
	return `"` + s + `"`
}

// EscapeCSVNumber formats a numeric value for CSV. Returns defaultValue if value is nil.
func EscapeCSVNumber(value interface{}, defaultValue float64) string {
	if value == nil {
		return formatNumber(defaultValue)
	}
	switch v := value.(type) {
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case float64:
		return formatNumber(v)
	case *int:
		if v == nil {
			return formatNumber(defaultValue)
		}
		return strconv.Itoa(*v)
	case *float64:
		if v == nil {
			return formatNumber(defaultValue)
		}
		return formatNumber(*v)
	default:
		return formatNumber(defaultValue)
	}
}

func formatNumber(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// ============================================================================
// Buffered CSV Writer
// ============================================================================

// BufferedCSVWriter buffers CSV rows and flushes to disk periodically.
type BufferedCSVWriter struct {
	ws     *bufio.Writer
	file   *os.File
	buffer []string
	rows   int
}

// NewBufferedCSVWriter creates a new buffered CSV writer with the given header.
func NewBufferedCSVWriter(filePath string, header string) (*BufferedCSVWriter, error) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create directory %s: %w", dir, err)
	}
	f, err := os.Create(filePath)
	if err != nil {
		return nil, fmt.Errorf("create file %s: %w", filePath, err)
	}
	w := &BufferedCSVWriter{
		ws:     bufio.NewWriterSize(f, 64*1024),
		file:   f,
		buffer: []string{header},
	}
	return w, nil
}

// AddRow adds a CSV row to the buffer, flushing if needed.
func (w *BufferedCSVWriter) AddRow(row string) error {
	w.buffer = append(w.buffer, row)
	w.rows++
	if len(w.buffer) >= flushEvery {
		return w.Flush()
	}
	return nil
}

// Flush writes all buffered rows to disk.
func (w *BufferedCSVWriter) Flush() error {
	if len(w.buffer) == 0 {
		return nil
	}
	data := strings.Join(w.buffer, "\n") + "\n"
	w.buffer = w.buffer[:0]
	_, err := w.ws.WriteString(data)
	return err
}

// Finish flushes remaining data and closes the file.
func (w *BufferedCSVWriter) Finish() error {
	if err := w.Flush(); err != nil {
		return err
	}
	if err := w.ws.Flush(); err != nil {
		return err
	}
	return w.file.Close()
}

// Rows returns the number of data rows written (excluding header).
func (w *BufferedCSVWriter) Rows() int {
	return w.rows
}

// ============================================================================
// Content Extraction
// ============================================================================

const (
	maxFileContent = 10000
	maxSnippet     = 5000
	contentContext = 2 // lines before/after for snippet
)

// IsBinaryContent checks if content appears to be binary.
func IsBinaryContent(content string) bool {
	if len(content) == 0 {
		return false
	}
	sample := content
	if len(sample) > 1000 {
		sample = sample[:1000]
	}
	nonPrintable := 0
	for _, r := range sample {
		if (r < 9) || (r > 13 && r < 32) || r == 127 {
			nonPrintable++
		}
	}
	return float64(nonPrintable)/float64(len(sample)) > 0.1
}

// utf16Len returns the number of UTF-16 code units in a string,
// matching JavaScript's String.length behavior. Runes in the BMP
// (U+0000–U+FFFF) count as 1; runes beyond BMP (U+10000+) count as 2
// (surrogate pairs in UTF-16).
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// sliceByUTF16 returns the prefix of s containing at most n UTF-16 code units,
// matching JavaScript's String.prototype.slice(0, n) behavior.
func sliceByUTF16(s string, n int) string {
	count := 0
	for i, r := range s {
		inc := 1
		if r >= 0x10000 {
			inc = 2
		}
		if count+inc > n {
			return s[:i]
		}
		count += inc
	}
	return s
}

// ExtractFileContent extracts file-level content for File nodes.
// content.length > MAX_FILE_CONTENT (JS String.length).
func ExtractFileContent(content string) string {
	if IsBinaryContent(content) {
		return "[Binary file - content not stored]"
	}
	if utf16Len(content) > maxFileContent {
		return sliceByUTF16(content, maxFileContent) + "\n... [truncated]"
	}
	return content
}

// ExtractSnippet extracts a code snippet for code element nodes.
// snippet.length > MAX_SNIPPET (JS String.length).
func ExtractSnippet(content string, startLine, endLine int) string {
	if content == "" {
		return ""
	}
	lines := strings.Split(content, "\n")
	start := startLine - contentContext
	if start < 0 {
		start = 0
	}
	end := endLine + contentContext
	if end >= len(lines) {
		end = len(lines) - 1
	}
	snippet := strings.Join(lines[start:end+1], "\n")
	if utf16Len(snippet) > maxSnippet {
		return sliceByUTF16(snippet, maxSnippet) + "\n... [truncated]"
	}
	return snippet
}

// FormatStringArray formats a string slice as ['x','y'].
// Escapes: backslash -> \\, single quote -> ”, comma -> \,
func FormatStringArray(items []string) string {
	escaped := make([]string, len(items))
	for i, item := range items {
		s := strings.ReplaceAll(item, `\`, `\\`)
		s = strings.ReplaceAll(s, `'`, `''`)
		s = strings.ReplaceAll(s, `,`, `\,`)
		escaped[i] = `'` + s + `'`
	}
	return "[" + strings.Join(escaped, ",") + "]"
}

// ============================================================================
// Streaming CSV Generation
// ============================================================================

// StreamedCSVResult holds the result of streaming CSV generation.
type StreamedCSVResult struct {
	NodeFiles  map[NodeTableName]CSVFileInfo
	RelCSVPath string
	RelRows    int
}

// CSVFileInfo holds info about a generated CSV file.
type CSVFileInfo struct {
	CSVPath string
	Rows    int
}

// StreamAllCSVsToDisk writes all graph data as CSV files to the given directory.
func StreamAllCSVsToDisk(g *model.KnowledgeGraph, repoPath string, csvDir string) (*StreamedCSVResult, error) {
	// Clean and recreate output directory
	os.RemoveAll(csvDir)
	if err := os.MkdirAll(csvDir, 0o755); err != nil {
		return nil, fmt.Errorf("create csv dir: %w", err)
	}

	// Create writers for core node types
	fileWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "file.csv"), FileHeader)
	if err != nil {
		return nil, err
	}
	folderWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "folder.csv"), FolderHeader)
	if err != nil {
		return nil, err
	}
	functionWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "function.csv"), CodeElementHeader)
	if err != nil {
		return nil, err
	}
	classWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "class.csv"), CodeElementHeader)
	if err != nil {
		return nil, err
	}
	interfaceWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "interface.csv"), CodeElementHeader)
	if err != nil {
		return nil, err
	}
	methodWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "method.csv"), MethodHeader)
	if err != nil {
		return nil, err
	}
	codeElemWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "codeelement.csv"), CodeElementHeader)
	if err != nil {
		return nil, err
	}
	communityWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "community.csv"), CommunityHeader)
	if err != nil {
		return nil, err
	}
	processWriter, err := NewBufferedCSVWriter(filepath.Join(csvDir, "process.csv"), ProcessHeader)
	if err != nil {
		return nil, err
	}

	// Multi-language node type writers
	multiLangWriters := make(map[string]*BufferedCSVWriter)
	for _, t := range MultiLangTypes {
		w, err := NewBufferedCSVWriter(filepath.Join(csvDir, strings.ToLower(string(t))+".csv"), MultiLangHeader)
		if err != nil {
			return nil, err
		}
		multiLangWriters[string(t)] = w
	}

	// Content cache for lazy file reading
	contentCache := NewFileContentCache(repoPath)

	// Single pass over all nodes
	seenFileIDs := make(map[string]bool)
	g.ForEachNode(func(node *model.GraphNode) {
		switch node.Label {
		case model.LabelFile:
			if seenFileIDs[node.ID] {
				return
			}
			seenFileIDs[node.ID] = true
			content, _ := contentCache.Get(node.Properties.FilePath)
			content = ExtractFileContent(content)
			row := strings.Join([]string{
				EscapeCSVField(node.ID),
				EscapeCSVField(node.Properties.Name),
				EscapeCSVField(node.Properties.FilePath),
				EscapeCSVField(content),
			}, ",")
			fileWriter.AddRow(row)

		case model.LabelFolder:
			row := strings.Join([]string{
				EscapeCSVField(node.ID),
				EscapeCSVField(node.Properties.Name),
				EscapeCSVField(node.Properties.FilePath),
			}, ",")
			folderWriter.AddRow(row)

		case model.LabelCommunity:
			keywords := FormatStringArray(node.Properties.Keywords)
			row := strings.Join([]string{
				EscapeCSVField(node.ID),
				EscapeCSVField(node.Properties.Name),
				EscapeCSVField(node.Properties.HeuristicLabel),
				keywords,
				EscapeCSVField(node.Properties.Description),
				EscapeCSVField(node.Properties.EnrichedBy),
				EscapeCSVNumber(node.Properties.Cohesion, 0),
				EscapeCSVNumber(node.Properties.SymbolCount, 0),
			}, ",")
			communityWriter.AddRow(row)

		case model.LabelProcess:
			communities := FormatStringArray(node.Properties.Communities)
			row := strings.Join([]string{
				EscapeCSVField(node.ID),
				EscapeCSVField(node.Properties.Name),
				EscapeCSVField(node.Properties.HeuristicLabel),
				EscapeCSVField(node.Properties.ProcessType),
				EscapeCSVNumber(node.Properties.StepCount, 0),
				communities,
				EscapeCSVField(node.Properties.EntryPointID),
				EscapeCSVField(node.Properties.TerminalID),
			}, ",")
			processWriter.AddRow(row)

		case model.LabelMethod:
			content, _ := contentCache.Get(node.Properties.FilePath)
			snippet := extractSnippetFromContent(content, node.Properties.StartLine, node.Properties.EndLine)
			row := strings.Join([]string{
				EscapeCSVField(node.ID),
				EscapeCSVField(node.Properties.Name),
				EscapeCSVField(node.Properties.FilePath),
				EscapeCSVNumber(node.Properties.StartLine, -1),
				EscapeCSVNumber(node.Properties.EndLine, -1),
				formatBool(node.Properties.IsExported),
				EscapeCSVField(snippet),
				EscapeCSVField(node.Properties.Description),
				EscapeCSVNumber(node.Properties.ParameterCount, 0),
				EscapeCSVField(node.Properties.ReturnType),
			}, ",")
			methodWriter.AddRow(row)

		default:
			// Code element types (Function, Class, Interface, CodeElement)
			if IsCodeElementType(string(node.Label)) {
				content, _ := contentCache.Get(node.Properties.FilePath)
				snippet := extractSnippetFromContent(content, node.Properties.StartLine, node.Properties.EndLine)
				row := strings.Join([]string{
					EscapeCSVField(node.ID),
					EscapeCSVField(node.Properties.Name),
					EscapeCSVField(node.Properties.FilePath),
					EscapeCSVNumber(node.Properties.StartLine, -1),
					EscapeCSVNumber(node.Properties.EndLine, -1),
					formatBool(node.Properties.IsExported),
					EscapeCSVField(snippet),
					EscapeCSVField(node.Properties.Description),
				}, ",")
				getCodeElementWriter(string(node.Label), functionWriter, classWriter, interfaceWriter, codeElemWriter).AddRow(row)
			} else if w, ok := multiLangWriters[string(node.Label)]; ok {
				content, _ := contentCache.Get(node.Properties.FilePath)
				snippet := extractSnippetFromContent(content, node.Properties.StartLine, node.Properties.EndLine)
				row := strings.Join([]string{
					EscapeCSVField(node.ID),
					EscapeCSVField(node.Properties.Name),
					EscapeCSVField(node.Properties.FilePath),
					EscapeCSVNumber(node.Properties.StartLine, -1),
					EscapeCSVNumber(node.Properties.EndLine, -1),
					EscapeCSVField(snippet),
					EscapeCSVField(node.Properties.Description),
				}, ",")
				w.AddRow(row)
			}
		}
	})

	// Finish all node writers
	allWriters := []*BufferedCSVWriter{
		fileWriter, folderWriter, functionWriter, classWriter,
		interfaceWriter, methodWriter, codeElemWriter,
		communityWriter, processWriter,
	}
	for _, w := range multiLangWriters {
		allWriters = append(allWriters, w)
	}
	for _, w := range allWriters {
		w.Finish()
	}

	// Stream relationship CSV
	relCSVPath := filepath.Join(csvDir, "relations.csv")
	relWriter, err := NewBufferedCSVWriter(relCSVPath, RelationHeader)
	if err != nil {
		return nil, err
	}
	g.ForEachRelationship(func(rel *model.GraphRelationship) {
		row := strings.Join([]string{
			EscapeCSVField(rel.SourceID),
			EscapeCSVField(rel.TargetID),
			EscapeCSVField(string(rel.Type)),
			EscapeCSVNumber(rel.Confidence, 1.0),
			EscapeCSVField(rel.Reason),
			EscapeCSVNumber(rel.Step, 0),
		}, ",")
		relWriter.AddRow(row)
	})
	relWriter.Finish()

	// Build result map
	result := &StreamedCSVResult{
		NodeFiles:  make(map[NodeTableName]CSVFileInfo),
		RelCSVPath: relCSVPath,
		RelRows:    relWriter.Rows(),
	}

	type tableWriter struct {
		name   NodeTableName
		writer *BufferedCSVWriter
		path   string
	}
	tw := []tableWriter{
		{"File", fileWriter, filepath.Join(csvDir, "file.csv")},
		{"Folder", folderWriter, filepath.Join(csvDir, "folder.csv")},
		{"Function", functionWriter, filepath.Join(csvDir, "function.csv")},
		{"Class", classWriter, filepath.Join(csvDir, "class.csv")},
		{"Interface", interfaceWriter, filepath.Join(csvDir, "interface.csv")},
		{"Method", methodWriter, filepath.Join(csvDir, "method.csv")},
		{"CodeElement", codeElemWriter, filepath.Join(csvDir, "codeelement.csv")},
		{"Community", communityWriter, filepath.Join(csvDir, "community.csv")},
		{"Process", processWriter, filepath.Join(csvDir, "process.csv")},
	}
	for _, t := range MultiLangTypes {
		tw = append(tw, tableWriter{
			name:   t,
			writer: multiLangWriters[string(t)],
			path:   filepath.Join(csvDir, strings.ToLower(string(t))+".csv"),
		})
	}
	for _, t := range tw {
		if t.writer.Rows() > 0 {
			result.NodeFiles[t.name] = CSVFileInfo{CSVPath: t.path, Rows: t.writer.Rows()}
		}
	}

	return result, nil
}

func getCodeElementWriter(label string, fn, cls, iface, ce *BufferedCSVWriter) *BufferedCSVWriter {
	switch label {
	case "Function":
		return fn
	case "Class":
		return cls
	case "Interface":
		return iface
	case "CodeElement":
		return ce
	default:
		return ce
	}
}

func formatBool(v *bool) string {
	if v != nil && *v {
		return "true"
	}
	return "false"
}

func extractSnippetFromContent(content string, startLine, endLine *int) string {
	if content == "" {
		return ""
	}
	if startLine == nil || endLine == nil {
		return ""
	}
	return ExtractSnippet(content, *startLine, *endLine)
}

// ============================================================================
// File Content Cache (LRU, lazy disk read)
// ============================================================================

// FileContentCache provides LRU-cached lazy file reading.
type FileContentCache struct {
	cache       map[string]string
	accessOrder []string
	maxSize     int
	repoPath    string
}

// NewFileContentCache creates a new LRU content cache.
func NewFileContentCache(repoPath string) *FileContentCache {
	return &FileContentCache{
		cache:    make(map[string]string),
		maxSize:  3000,
		repoPath: repoPath,
	}
}

// Get reads file content from cache or disk.
func (c *FileContentCache) Get(relativePath string) (string, error) {
	if relativePath == "" {
		return "", nil
	}
	if cached, ok := c.cache[relativePath]; ok {
		// LRU promotion
		for i, p := range c.accessOrder {
			if p == relativePath {
				c.accessOrder = append(c.accessOrder[:i], c.accessOrder[i+1:]...)
				c.accessOrder = append(c.accessOrder, relativePath)
				break
			}
		}
		return cached, nil
	}
	fullPath := filepath.Join(c.repoPath, relativePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		c.set(relativePath, "")
		return "", nil
	}
	content := string(data)
	c.set(relativePath, content)
	return content, nil
}

func (c *FileContentCache) set(key, value string) {
	if len(c.cache) >= c.maxSize {
		oldest := c.accessOrder[0]
		c.accessOrder = c.accessOrder[1:]
		delete(c.cache, oldest)
	}
	c.cache[key] = value
	c.accessOrder = append(c.accessOrder, key)
}

// Verify BufferedCSVWriter has Flush+Close semantics (not io.Writer — rows are appended via AddRow)
