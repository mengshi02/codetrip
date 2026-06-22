package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/mengshi02/codetrip"
)

// CodetripHome returns the codetrip home directory (~/.codetrip by default).
// Can be overridden via CODETRIP_HOME environment variable.
func CodetripHome() string {
	if home := os.Getenv("CODETRIP_HOME"); home != "" {
		return home
	}
	return filepath.Join(os.Getenv("HOME"), ".codetrip")
}

// OpenTrip opens a codetrip engine at the given path.
func OpenTrip(tripPath string) (*codetrip.Trip, error) {
	trip, err := codetrip.Open(tripPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open engine: %w", err)
	}
	return trip, nil
}

// OpenTripWithDir opens a codetrip engine using a trip-dir override or default home.
func OpenTripWithDir(tripDir string) (*codetrip.Trip, error) {
	tripPath := tripDir
	if tripPath == "" {
		tripPath = CodetripHome()
	}
	return OpenTrip(tripPath)
}

// IndexRepo indexes a repository and returns stats.
func IndexRepo(ctx context.Context, trip *codetrip.Trip, repoPath, repoName string, opts []codetrip.IndexOption) (*codetrip.IndexResult, error) {
	stats, err := trip.IndexRepo(ctx, repoPath, opts...)
	if err != nil {
		return nil, fmt.Errorf("indexing failed: %w", err)
	}
	return stats, nil
}

// IndexRepoWithDefaults is a convenience function that opens a trip engine,
// indexes a repo, and closes the engine. Returns stats and elapsed time.
func IndexRepoWithDefaults(ctx context.Context, tripDir, repoPath, repoName string, opts []codetrip.IndexOption) (*codetrip.IndexResult, time.Duration, error) {
	start := time.Now()
	tripPath := tripDir
	if tripPath == "" {
		tripPath = CodetripHome()
	}

	trip, err := OpenTrip(tripPath)
	if err != nil {
		return nil, 0, err
	}
	defer trip.Close()

	stats, err := IndexRepo(ctx, trip, repoPath, repoName, opts)
	if err != nil {
		return nil, 0, err
	}
	return stats, time.Since(start), nil
}

// Query executes a Cypher query and returns the result.
func Query(ctx context.Context, trip *codetrip.Trip, queryStr, repo string) (*codetrip.CypherResult, error) {
	result, err := trip.Cypher(ctx, queryStr,
		codetrip.Param{Key: "repo", Value: repo},
	)
	if err != nil {
		return nil, fmt.Errorf("query error: %w", err)
	}
	return result, nil
}

// Search executes a symbol search and returns the result.
func Search(ctx context.Context, trip *codetrip.Trip, req *codetrip.SearchRequest) (*codetrip.SearchResult, error) {
	result, err := trip.Search(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}
	return result, nil
}

// Impact performs impact analysis and returns the result.
func Impact(ctx context.Context, trip *codetrip.Trip, req *codetrip.ImpactRequest) (*codetrip.ImpactResult, error) {
	result, err := trip.Impact(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("impact analysis failed: %w", err)
	}
	return result, nil
}

// Context returns the 360-degree symbol view.
func Context(ctx context.Context, trip *codetrip.Trip, req *codetrip.ContextRequest) (*codetrip.ContextResult, error) {
	result, err := trip.Context(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to get context: %w", err)
	}
	return result, nil
}

// DetectChanges detects code changes and returns the result.
func DetectChanges(ctx context.Context, trip *codetrip.Trip, req *codetrip.DetectChangesRequest) (*codetrip.DetectChangesResult, error) {
	result, err := trip.DetectChanges(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("change detection failed: %w", err)
	}
	return result, nil
}

// ReIndex performs incremental re-indexing and returns the result.
func ReIndex(ctx context.Context, trip *codetrip.Trip, repoPath string, opts []codetrip.IndexOption) (*codetrip.ReIndexResult, error) {
	result, err := trip.ReIndex(ctx, repoPath, opts...)
	if err != nil {
		return nil, fmt.Errorf("reindex failed: %w", err)
	}
	return result, nil
}

// Check performs structural checks.
func Check(ctx context.Context, trip *codetrip.Trip, req *codetrip.CheckRequest) (*codetrip.CheckResult, error) {
	result, err := trip.Check(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("check failed: %w", err)
	}
	return result, nil
}

// Explain performs taint tracking explanation.
func Explain(ctx context.Context, trip *codetrip.Trip, req *codetrip.ExplainRequest) (*codetrip.ExplainResult, error) {
	result, err := trip.Explain(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("taint explanation failed: %w", err)
	}
	return result, nil
}