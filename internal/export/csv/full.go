package csv

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mengshi02/codetrip/internal/graph"
)

type FullManifest struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Repository    string              `json:"repository"`
	NodeCount     int                 `json:"nodeCount"`
	EdgeCount     int                 `json:"edgeCount"`
	Files         map[string]FileHash `json:"files"`
}

type FileHash struct {
	SHA256 string `json:"sha256"`
	Rows   int    `json:"rows"`
}

// ExportFull writes the complete persisted graph rather than the compact
// validation tables. It is intended for storage inspection and round-trip
// verification.
func ExportFull(graphStore *graph.GraphStore, directory string) (*FullManifest, error) {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, err
	}
	manifest := &FullManifest{
		SchemaVersion: 2,
		Repository:    graphStore.Repo(),
		Files:         make(map[string]FileHash),
	}

	nodePath := filepath.Join(directory, "nodes.csv")
	nodeRows, err := writeCSV(nodePath,
		[]string{"id", "label", "name", "language", "filePath", "startLine", "endLine", "content", "description", "properties"},
		func(writer *csv.Writer) error {
			return graphStore.ForEachNode(func(node *graph.Node) error {
				properties, err := json.Marshal(node.Props)
				if err != nil {
					return err
				}
				return writer.Write([]string{
					node.ID, string(node.Label), node.Name, node.GetPropString("language"), node.FilePath,
					strconv.Itoa(node.GetPropInt("startLine")), strconv.Itoa(node.GetPropInt("endLine")),
					node.GetPropString("content"), node.GetPropString("description"), string(properties),
				})
			})
		})
	if err != nil {
		return nil, err
	}
	manifest.NodeCount = nodeRows
	manifest.Files["nodes.csv"] = FileHash{SHA256: fileSHA256(nodePath), Rows: nodeRows}

	edgePath := filepath.Join(directory, "edges.csv")
	edgeRows, err := writeCSV(edgePath,
		[]string{"id", "from", "to", "type", "confidence", "reason", "step", "properties"},
		func(writer *csv.Writer) error {
			return graphStore.ForEachEdge(func(edge *graph.Edge) error {
				properties, err := json.Marshal(edge.Props)
				if err != nil {
					return err
				}
				return writer.Write([]string{
					edge.ID, edge.Source, edge.Target, string(edge.Type),
					strconv.FormatFloat(edge.Confidence(), 'g', -1, 64), edge.GetPropString("reason"),
					strconv.Itoa(edge.GetPropInt("step")), string(properties),
				})
			})
		})
	if err != nil {
		return nil, err
	}
	manifest.EdgeCount = edgeRows
	manifest.Files["edges.csv"] = FileHash{SHA256: fileSHA256(edgePath), Rows: edgeRows}

	encoded, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(directory, "manifest.json"), append(encoded, '\n'), 0o644); err != nil {
		return nil, err
	}
	return manifest, nil
}

func writeCSV(path string, header []string, rows func(*csv.Writer) error) (int, error) {
	file, err := os.Create(path)
	if err != nil {
		return 0, err
	}
	writer := csv.NewWriter(file)
	if err := writer.Write(header); err != nil {
		return 0, err
	}
	if err := rows(writer); err != nil {
		file.Close()
		return 0, err
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		file.Close()
		return 0, err
	}
	if err := file.Close(); err != nil {
		return 0, err
	}
	// Count lines after writing so quoted multi-line content remains correct.
	file, err = os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	records := 0
	for {
		_, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, err
		}
		records++
	}
	return records - 1, nil
}

func fileSHA256(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return ""
	}
	return hex.EncodeToString(hash.Sum(nil))
}
