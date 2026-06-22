package utils

import (
	"context"
	"fmt"

	"github.com/mengshi02/codetrip"
)

// RouteMap returns API route mapping results.
func RouteMap(ctx context.Context, trip *codetrip.Trip, req *codetrip.RouteMapRequest) (*codetrip.RouteMapResult, error) {
	result, err := trip.RouteMap(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("route map failed: %w", err)
	}
	return result, nil
}

// ToolMap returns MCP/RPC tool mapping results.
func ToolMap(ctx context.Context, trip *codetrip.Trip, req *codetrip.ToolMapRequest) (*codetrip.ToolMapResult, error) {
	result, err := trip.ToolMap(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("tool map failed: %w", err)
	}
	return result, nil
}

// ShapeCheck returns response shape checking results.
func ShapeCheck(ctx context.Context, trip *codetrip.Trip, req *codetrip.ShapeCheckRequest) (*codetrip.ShapeCheckResult, error) {
	result, err := trip.ShapeCheck(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("shape check failed: %w", err)
	}
	return result, nil
}

// ApiImpact returns API impact analysis results.
func ApiImpact(ctx context.Context, trip *codetrip.Trip, req *codetrip.ApiImpactRequest) (*codetrip.ApiImpactResult, error) {
	result, err := trip.ApiImpact(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("API impact analysis failed: %w", err)
	}
	return result, nil
}

// Rename performs multi-file coordinated renaming.
func Rename(ctx context.Context, trip *codetrip.Trip, req *codetrip.RenameRequest) (*codetrip.RenameResult, error) {
	result, err := trip.Rename(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("rename analysis failed: %w", err)
	}
	return result, nil
}

// Stats returns index statistics for a repository.
func Stats(trip *codetrip.Trip, repo string) (*codetrip.IndexStats, error) {
	stats, err := trip.Stats(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}
	return stats, nil
}

// ListRepos returns all indexed repositories.
func ListRepos(trip *codetrip.Trip) ([]codetrip.RepoInfo, error) {
	repos, err := trip.ListRepos()
	if err != nil {
		return nil, fmt.Errorf("failed to list repos: %w", err)
	}
	return repos, nil
}

// RepoStatus returns the status of a repository.
func RepoStatus(trip *codetrip.Trip, repo string) (*codetrip.RepoStatusInfo, error) {
	status, err := trip.RepoStatus(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repo status: %w", err)
	}
	return status, nil
}

// DropIndex removes all index data for a repository.
func DropIndex(trip *codetrip.Trip, repo string) error {
	if err := trip.DropIndex(repo); err != nil {
		return fmt.Errorf("failed to drop index: %w", err)
	}
	return nil
}

// GroupList returns all cross-repo groups.
func GroupList(trip *codetrip.Trip) ([]codetrip.GroupInfo, error) {
	groups, err := trip.GroupList()
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}
	return groups, nil
}

// GroupSync syncs a cross-repo group.
func GroupSync(ctx context.Context, trip *codetrip.Trip, req *codetrip.GroupSyncRequest) (*codetrip.GroupSyncResult, error) {
	result, err := trip.GroupSync(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("group sync failed: %w", err)
	}
	return result, nil
}

// GroupImpact performs cross-repo impact analysis.
func GroupImpact(ctx context.Context, trip *codetrip.Trip, req *codetrip.GroupImpactRequest) (*codetrip.GroupImpactResult, error) {
	result, err := trip.GroupImpact(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("cross-repo impact analysis failed: %w", err)
	}
	return result, nil
}
