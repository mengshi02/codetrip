//go:build !windows

package source

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	engine "github.com/sourcegraph/zoekt"
	engineindex "github.com/sourcegraph/zoekt/index"
	enginequery "github.com/sourcegraph/zoekt/query"
)

type Index struct {
	directory string
	mu        sync.RWMutex
	searchers []engine.Searcher
}

func New(dataDir, snapshot string) *Index {
	return &Index{directory: filepath.Join(dataDir, "content", snapshot)}
}

func (idx *Index) Build(repositoryPath, snapshot string) error {
	buildDir := idx.directory + ".build"
	_ = os.RemoveAll(buildDir)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return err
	}
	options := engineindex.Options{
		IndexDir: buildDir, ShardPrefixOverride: "content", DisableCTags: true,
		RepositoryDescription: engine.Repository{Name: snapshot, Source: repositoryPath},
	}
	options.SetDefaults()
	builder, err := engineindex.NewBuilder(options)
	if err != nil {
		return fmt.Errorf("create content index: %w", err)
	}
	err = filepath.WalkDir(repositoryPath, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != repositoryPath && (entry.Name() == ".git" || entry.Name() == ".codetrip") {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		relative, err := filepath.Rel(repositoryPath, path)
		if err != nil {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return builder.AddFile(filepath.ToSlash(relative), content)
	})
	if err == nil {
		err = builder.Finish()
	}
	if err != nil {
		_ = os.RemoveAll(buildDir)
		return fmt.Errorf("build content index: %w", err)
	}
	entries, err := os.ReadDir(buildDir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".zoekt") {
			oldPath := filepath.Join(buildDir, entry.Name())
			newName := strings.TrimSuffix(entry.Name(), ".zoekt") + ".content"
			if err := os.Rename(oldPath, filepath.Join(buildDir, newName)); err != nil {
				return err
			}
		}
	}
	_ = os.RemoveAll(idx.directory)
	if err := os.Rename(buildDir, idx.directory); err != nil {
		return err
	}
	return idx.Open()
}

func (idx *Index) Open() error {
	idx.Close()
	paths, err := filepath.Glob(filepath.Join(idx.directory, "*.content"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("content index is missing in %s", idx.directory)
	}
	searchers := make([]engine.Searcher, 0, len(paths))
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			closeSearchers(searchers)
			return err
		}
		indexFile, err := engineindex.NewIndexFile(file)
		if err != nil {
			file.Close()
			closeSearchers(searchers)
			return err
		}
		searcher, err := engineindex.NewSearcher(indexFile)
		if err != nil {
			indexFile.Close()
			closeSearchers(searchers)
			return err
		}
		searchers = append(searchers, searcher)
	}
	idx.mu.Lock()
	idx.searchers = searchers
	idx.mu.Unlock()
	return nil
}

func (idx *Index) Search(ctx context.Context, queryText string, limit, contextLines int) ([]Match, error) {
	query, err := enginequery.Parse(queryText)
	if err != nil {
		return nil, fmt.Errorf("parse content query: %w", err)
	}
	if limit <= 0 {
		limit = 20
	}
	options := &engine.SearchOptions{TotalMaxMatchCount: limit, MaxDocDisplayCount: limit, MaxMatchDisplayCount: limit, NumContextLines: contextLines, MaxWallTime: 30 * time.Second}
	idx.mu.RLock()
	searchers := append([]engine.Searcher(nil), idx.searchers...)
	idx.mu.RUnlock()
	if len(searchers) == 0 {
		return nil, fmt.Errorf("content index is not open")
	}
	result := make([]Match, 0, limit)
	for _, searcher := range searchers {
		found, err := searcher.Search(ctx, query, options)
		if err != nil {
			return nil, err
		}
		for _, file := range found.Files {
			for _, line := range file.LineMatches {
				result = append(result, Match{FilePath: file.FileName, Language: file.Language, Line: line.LineNumber, Content: string(line.Line), Before: string(line.Before), After: string(line.After), Score: file.Score + line.Score})
				if len(result) >= limit {
					break
				}
			}
			if len(result) >= limit {
				break
			}
		}
		if len(result) >= limit {
			break
		}
	}
	sort.SliceStable(result, func(i, j int) bool { return result[i].Score > result[j].Score })
	if len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}

func (idx *Index) Close() {
	idx.mu.Lock()
	current := idx.searchers
	idx.searchers = nil
	idx.mu.Unlock()
	closeSearchers(current)
}

func closeSearchers(searchers []engine.Searcher) {
	for _, searcher := range searchers {
		searcher.Close()
	}
}
