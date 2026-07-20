package graph

import ingestgraph "github.com/mengshi02/codetrip/internal/model"

// ImportKnowledgeGraph persists the validated ingestion graph without making
// the parser depend on Pebble. Nodes are written before edges so every
// adjacency entry references an existing node.
func (s *GraphStore) ImportKnowledgeGraph(src *ingestgraph.KnowledgeGraph) error {
	for _, source := range src.Nodes() {
		node := NewNode(s.repo, Label(source.Label), source.Properties.Name).
			WithID(source.ID).
			WithFile(source.Properties.FilePath)
		copyNodeProperties(node, &source.Properties)
		if err := s.AddNode(node); err != nil {
			return err
		}
	}
	for _, source := range src.Relationships() {
		edge := NewEdge(RelType(source.Type), source.SourceID, source.TargetID).
			WithID(source.ID).
			WithProp("confidence", source.Confidence)
		if source.Reason != "" {
			edge.WithProp("reason", source.Reason)
		}
		if source.Step != nil {
			edge.WithProp("step", *source.Step)
		}
		if err := s.AddEdge(edge); err != nil {
			return err
		}
	}
	return nil
}

func copyNodeProperties(node *Node, source *ingestgraph.NodeProperties) {
	props := map[string]any{
		"language":       source.Language,
		"content":        source.Content,
		"description":    source.Description,
		"returnType":     source.ReturnType,
		"visibility":     source.Visibility,
		"modifiers":      source.Modifiers,
		"heuristicLabel": source.HeuristicLabel,
		"keywords":       source.Keywords,
		"enrichedBy":     source.EnrichedBy,
		"processType":    source.ProcessType,
		"communities":    source.Communities,
		"entryPointID":   source.EntryPointID,
		"terminalID":     source.TerminalID,
		"fileSize":       source.FileSize,
		"languageID":     source.LanguageID,
		"packageName":    source.PackageName,
		"version":        source.Version,
	}
	for key, value := range props {
		switch typed := value.(type) {
		case string:
			if typed != "" {
				node.WithProp(key, typed)
			}
		case []string:
			if len(typed) > 0 {
				node.WithProp(key, typed)
			}
		case int64:
			if typed != 0 {
				node.WithProp(key, typed)
			}
		}
	}
	copyOptionalNodeProperty(node, "startLine", source.StartLine)
	copyOptionalNodeProperty(node, "endLine", source.EndLine)
	copyOptionalNodeProperty(node, "parameterCount", source.ParameterCount)
	copyOptionalNodeProperty(node, "isExported", source.IsExported)
	copyOptionalNodeProperty(node, "isAbstract", source.IsAbstract)
	copyOptionalNodeProperty(node, "isStatic", source.IsStatic)
	copyOptionalNodeProperty(node, "isAsync", source.IsAsync)
	copyOptionalNodeProperty(node, "isTest", source.IsTest)
	copyOptionalNodeProperty(node, "isBinary", source.IsBinary)
	copyOptionalNodeProperty(node, "cohesion", source.Cohesion)
	copyOptionalNodeProperty(node, "symbolCount", source.SymbolCount)
	copyOptionalNodeProperty(node, "stepCount", source.StepCount)
}

func copyOptionalNodeProperty[T any](node *Node, key string, value *T) {
	if value != nil {
		node.WithProp(key, *value)
	}
}
