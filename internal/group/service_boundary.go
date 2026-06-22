package group

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mengshi02/codetrip/internal/graph"
)

// ServiceBoundary represents a service boundary
type ServiceBoundary struct {
	Name        string  `json:"name"`
	Path        string  `json:"path"`
	Cohesion    float64 `json:"cohesion"`
	SymbolCount int     `json:"symbolCount"`
}

// ServiceBoundaryDetector detects service boundaries
// Automatically detects microservice boundaries based on directory conventions and contract density
type ServiceBoundaryDetector struct{}

// NewServiceBoundaryDetector creates a service boundary detector
func NewServiceBoundaryDetector() *ServiceBoundaryDetector {
	return &ServiceBoundaryDetector{}
}

// Detect automatically detects microservice boundaries based on directory conventions and contract density
func (d *ServiceBoundaryDetector) Detect(repo string, gs *graph.GraphStore) ([]ServiceBoundary, error) {
	// 1. Collect directory information
	dirStats := make(map[string]*dirStat)

	iter := gs.IterNodes(repo)
	defer iter.Close()

	for iter.Next() {
		node := iter.Node()
		if node.FilePath == "" {
			continue
		}

		// Only count code symbol nodes
		if !node.Label.IsSymbol() && node.Label != graph.LabelRoute {
			continue
		}

		// Extract directory path
		dirPath := directoryOf(node.FilePath)

		if dirStats[dirPath] == nil {
			dirStats[dirPath] = &dirStat{
				symbols:     make(map[string]bool),
				hasRoute:    false,
				hasContract: false,
			}
		}

		stats := dirStats[dirPath]
		stats.symbols[node.ID] = true

		if node.Label == graph.LabelRoute {
			stats.hasRoute = true
		}
		if node.Label == graph.LabelContract {
			stats.hasContract = true
		}

		// Count internal edges (cohesion)
		outEdges, _ := gs.GetAllOutEdges(node.ID)
		for _, edge := range outEdges {
			tgt, e := gs.GetNode(edge.Target)
			if e != nil {
				continue
			}
			tgtDir := directoryOf(tgt.FilePath)
			if tgtDir == dirPath {
				stats.internalEdges++
			} else {
				stats.externalEdges++
			}
		}
	}

	// 2. Calculate cohesion and detect boundaries
	var boundaries []ServiceBoundary
	for dir, stats := range dirStats {
		symbolCount := len(stats.symbols)
		if symbolCount < 3 {
			continue // Too small directory not considered as independent service
		}

		// Cohesion = internal edges / (internal edges + external edges)
		cohesion := 0.0
		total := stats.internalEdges + stats.externalEdges
		if total > 0 {
			cohesion = float64(stats.internalEdges) / float64(total)
		}

		// High cohesion + has Route/Contract → may be service boundary
		name := serviceNameFromPath(dir)
		boundaries = append(boundaries, ServiceBoundary{
			Name:        name,
			Path:        dir,
			Cohesion:    cohesion,
			SymbolCount: symbolCount,
		})
	}

	// Sort by cohesion
	sort.Slice(boundaries, func(i, j int) bool {
		return boundaries[i].Cohesion > boundaries[j].Cohesion
	})

	return boundaries, nil
}

// dirStat is directory statistics
type dirStat struct {
	symbols       map[string]bool
	internalEdges int
	externalEdges int
	hasRoute      bool
	hasContract   bool
}

// directoryOf extracts directory from file path
func directoryOf(filePath string) string {
	parts := strings.Split(filePath, "/")
	if len(parts) <= 1 {
		return filePath
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

// serviceNameFromPath generates service name from directory path
func serviceNameFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return "unknown"
	}

	// Take the last segment as service name
	name := parts[len(parts)-1]

	// Remove common suffixes
	name = strings.TrimSuffix(name, "-service")
	name = strings.TrimSuffix(name, "-svc")
	name = strings.TrimSuffix(name, "-api")
	name = strings.TrimSuffix(name, "-handler")

	if name == "" {
		return fmt.Sprintf("service-%s", path)
	}

	return name
}