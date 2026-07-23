package ingest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	graph "github.com/mengshi02/codetrip/internal/model"
)

func TestPipelineResolvesGoInterfaceDispatchAndCrossFileCalls(t *testing.T) {
	repository := t.TempDir()
	files := map[string]string{
		"go.mod":           "module example.com/semanticfixture\n\ngo 1.26\n",
		"helper/helper.go": "package helper\n",
		"contract.go": `package fixture
type Repo interface { Create() error }
`,
		"repository.go": `package fixture
type repo struct{}
func (*repo) Create() error { return nil }
`,
		"service.go": `package fixture
type Service struct { Repo Repo }
func (service *Service) Run() error { return service.Repo.Create() }
`,
		"wire.go": `package fixture
import _ "example.com/semanticfixture/helper"
func Build() error {
	repository := &repo{}
	service := &Service{Repo: repository}
	return service.Run()
}
`,
	}
	for name, content := range files {
		path := filepath.Join(repository, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	result, err := NewPipeline(repository, "", false).Run()
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string]bool{
		"IMPLEMENTS:Struct:repository.go:repo->Interface:contract.go:Repo":                        false,
		"METHOD_IMPLEMENTS:Method:repository.go:Create->Method:contract.go:Repo.Create":           false,
		"DISPATCHES_TO:Method:contract.go:Repo.Create->Method:repository.go:Create":               false,
		"CALLS:Method:service.go:Run->Method:contract.go:Repo.Create":                             false,
		"CALLS:Function:wire.go:Build->Method:service.go:Run":                                     false,
		"IMPORTS:Package:example.com/semanticfixture->Package:example.com/semanticfixture/helper": false,
	}
	result.Graph.ForEachRelationship(func(relationship *graph.GraphRelationship) {
		key := string(relationship.Type) + ":" + relationship.SourceID + "->" + relationship.TargetID
		if _, ok := expected[key]; ok {
			expected[key] = true
		}
	})
	for relationship, found := range expected {
		if !found {
			t.Errorf("missing semantic relationship %s", relationship)
		}
	}
	result.Graph.ForEachRelationship(func(relationship *graph.GraphRelationship) {
		if relationship.Type == graph.RelIMPORTS && strings.HasPrefix(relationship.SourceID, "File:") {
			t.Errorf("Go file import was not compacted: %s", relationship.ID)
		}
	})
}
