package source

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/blugelabs/bluge"
	"github.com/go-enry/go-enry/v2"
)

const (
	portableSchemaVersion = 2
	portableBatchSize     = 256
	portableManifestName  = "format.json"
	fieldPath             = "path"
	fieldLanguage         = "language"
	fieldKind             = "kind"
	fieldContent          = "content"
)

type portableManifest struct {
	SchemaVersion int    `json:"schema_version"`
	Backend       string `json:"backend"`
}

type portableIndex struct {
	directory string
	mu        sync.RWMutex
	writer    *bluge.Writer
}

func newPortableIndex(dataDir, snapshot string) *portableIndex {
	return &portableIndex{directory: filepath.Join(dataDir, "content", snapshot)}
}

func (idx *portableIndex) Build(repositoryPath, _ string) error {
	buildDir := idx.directory + ".build"
	_ = os.RemoveAll(buildDir)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	writer, err := bluge.OpenWriter(bluge.DefaultConfig(buildDir))
	if err != nil {
		return fmt.Errorf("create portable content index: %w", err)
	}
	batch := bluge.NewBatch()
	batchCount := 0
	flush := func() error {
		if batchCount == 0 {
			return nil
		}
		if err := writer.Batch(batch); err != nil {
			return err
		}
		batch = bluge.NewBatch()
		batchCount = 0
		return nil
	}
	walkErr := filepath.WalkDir(repositoryPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != repositoryPath && shouldSkipDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if bytes.IndexByte(content, 0) >= 0 || !utf8.Valid(content) {
			return nil
		}
		kind := ClassifyFile(path, content)
		if kind == "" {
			return nil
		}
		relative, err := filepath.Rel(repositoryPath, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		language := strings.ToLower(enry.GetLanguage(path, content))
		document := bluge.NewDocument(relative)
		document.AddField(bluge.NewKeywordField(fieldPath, relative).StoreValue())
		document.AddField(bluge.NewKeywordField(fieldLanguage, language).StoreValue())
		document.AddField(bluge.NewKeywordField(fieldKind, string(kind)).StoreValue())
		document.AddField(bluge.NewTextField(fieldContent, string(content)).StoreValue())
		batch.Update(document.ID(), document)
		batchCount++
		if batchCount >= portableBatchSize {
			return flush()
		}
		return nil
	})
	if walkErr == nil {
		walkErr = flush()
	}
	closeErr := writer.Close()
	if walkErr != nil {
		_ = os.RemoveAll(buildDir)
		return fmt.Errorf("build portable content index: %w", walkErr)
	}
	if closeErr != nil {
		_ = os.RemoveAll(buildDir)
		return fmt.Errorf("close portable content index: %w", closeErr)
	}
	manifest, err := json.Marshal(portableManifest{SchemaVersion: portableSchemaVersion, Backend: "portable"})
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(buildDir, portableManifestName), manifest, 0o644); err != nil {
		_ = os.RemoveAll(buildDir)
		return err
	}
	_ = os.RemoveAll(idx.directory)
	if err := os.Rename(buildDir, idx.directory); err != nil {
		return err
	}
	return idx.Open()
}

func (idx *portableIndex) Open() error {
	if err := idx.Close(); err != nil {
		return err
	}
	manifestBytes, err := os.ReadFile(filepath.Join(idx.directory, portableManifestName))
	if err != nil {
		return fmt.Errorf("portable content index is missing or incompatible; rebuild the repository snapshot: %w", err)
	}
	var manifest portableManifest
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil || manifest.SchemaVersion != portableSchemaVersion || manifest.Backend != "portable" {
		return fmt.Errorf("portable content index format is incompatible; rebuild the repository snapshot")
	}
	writer, err := bluge.OpenWriter(bluge.DefaultConfig(idx.directory))
	if err != nil {
		return fmt.Errorf("open portable content index: %w", err)
	}
	idx.mu.Lock()
	idx.writer = writer
	idx.mu.Unlock()
	return nil
}

type portableQuery struct {
	text          string
	language      string
	filePattern   *regexp.Regexp
	contentRegexp *regexp.Regexp
	caseSensitive bool
}

func parsePortableQuery(input string) (portableQuery, error) {
	var query portableQuery
	var terms []string
	regexMode := false
	caseMode := "auto"
	for _, token := range strings.Fields(input) {
		switch {
		case strings.HasPrefix(token, "lang:"):
			query.language = strings.ToLower(strings.TrimPrefix(token, "lang:"))
		case strings.HasPrefix(token, "file:"):
			pattern := strings.TrimPrefix(token, "file:")
			compiled, err := regexp.Compile("(?i)" + pattern)
			if err != nil {
				return query, fmt.Errorf("invalid file filter: %w", err)
			}
			query.filePattern = compiled
		case token == "regex:true" || token == "regex:yes":
			regexMode = true
		case token == "regex:false" || token == "regex:no":
			regexMode = false
		case strings.HasPrefix(token, "case:"):
			caseMode = strings.TrimPrefix(token, "case:")
		default:
			terms = append(terms, token)
		}
	}
	query.text = strings.Join(terms, " ")
	query.caseSensitive = caseMode == "yes" || caseMode == "true" || (caseMode == "auto" && query.text != strings.ToLower(query.text))
	if regexMode && query.text != "" {
		pattern := query.text
		if !query.caseSensitive {
			pattern = "(?i)" + pattern
		}
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return query, fmt.Errorf("invalid content regular expression: %w", err)
		}
		query.contentRegexp = compiled
	}
	if query.text == "" && query.filePattern == nil && query.language == "" {
		return query, fmt.Errorf("empty content query")
	}
	return query, nil
}

func (idx *portableIndex) Search(ctx context.Context, queryText string, scope Scope, limit, contextLines int) ([]Match, error) {
	query, err := parsePortableQuery(queryText)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = 20
	}
	if contextLines < 0 {
		contextLines = 0
	}
	idx.mu.RLock()
	writer := idx.writer
	idx.mu.RUnlock()
	if writer == nil {
		return nil, fmt.Errorf("portable content index is not open")
	}
	reader, err := writer.Reader()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	backendQuery := bluge.Query(bluge.NewMatchAllQuery())
	if query.language != "" {
		booleanQuery := bluge.NewBooleanQuery()
		booleanQuery.AddMust(bluge.NewTermQuery(query.language).SetField(fieldLanguage))
		backendQuery = booleanQuery
	}
	iterator, err := reader.Search(ctx, bluge.NewAllMatches(backendQuery))
	if err != nil {
		return nil, err
	}
	results := make([]Match, 0, limit)
	for len(results) < limit {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		document, err := iterator.Next()
		if err != nil {
			return nil, err
		}
		if document == nil {
			break
		}
		var path, language, content, kind string
		if err := document.VisitStoredFields(func(field string, value []byte) bool {
			switch field {
			case fieldPath:
				path = string(value)
			case fieldLanguage:
				language = string(value)
			case fieldContent:
				content = string(value)
			case fieldKind:
				kind = string(value)
			}
			return true
		}); err != nil {
			return nil, err
		}
		if query.filePattern != nil && !query.filePattern.MatchString(path) {
			continue
		}
		if !kindMatchesScope(FileKind(kind), scope) {
			continue
		}
		results = append(results, matchPortableLines(path, language, content, query, contextLines, limit-len(results))...)
	}
	sort.SliceStable(results, func(i, j int) bool {
		if results[i].FilePath != results[j].FilePath {
			return results[i].FilePath < results[j].FilePath
		}
		return results[i].Line < results[j].Line
	})
	return results, nil
}

func matchPortableLines(path, language, content string, query portableQuery, contextLines, remaining int) []Match {
	if remaining <= 0 {
		return nil
	}
	lines := strings.Split(content, "\n")
	results := make([]Match, 0)
	needle := query.text
	if !query.caseSensitive && query.contentRegexp == nil {
		needle = strings.ToLower(needle)
	}
	for index, line := range lines {
		matched := query.text == ""
		if query.contentRegexp != nil {
			matched = query.contentRegexp.MatchString(line)
		} else if query.text != "" {
			haystack := line
			if !query.caseSensitive {
				haystack = strings.ToLower(haystack)
			}
			matched = strings.Contains(haystack, needle)
		}
		if !matched {
			continue
		}
		beforeStart := index - contextLines
		if beforeStart < 0 {
			beforeStart = 0
		}
		afterEnd := index + contextLines + 1
		if afterEnd > len(lines) {
			afterEnd = len(lines)
		}
		results = append(results, Match{
			FilePath: path, Language: language, Line: index + 1, Content: line,
			Before: strings.Join(lines[beforeStart:index], "\n"),
			After:  strings.Join(lines[index+1:afterEnd], "\n"),
			Score:  1,
		})
		if len(results) >= remaining {
			break
		}
	}
	return results
}

func (idx *portableIndex) Close() error {
	idx.mu.Lock()
	writer := idx.writer
	idx.writer = nil
	idx.mu.Unlock()
	if writer != nil {
		return writer.Close()
	}
	return nil
}
