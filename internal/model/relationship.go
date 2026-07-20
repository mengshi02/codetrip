package model

// RelationshipType represents the type of a graph relationship.
type RelationshipType string

const (
	RelCONTAINS        RelationshipType = "CONTAINS"
	RelCALLS           RelationshipType = "CALLS"
	RelINHERITS        RelationshipType = "INHERITS"
	RelOVERRIDES       RelationshipType = "OVERRIDES"
	RelIMPORTS         RelationshipType = "IMPORTS"
	RelUSES            RelationshipType = "USES"
	RelDEFINES         RelationshipType = "DEFINES"
	RelDECORATES       RelationshipType = "DECORATES"
	RelIMPLEMENTS      RelationshipType = "IMPLEMENTS"
	RelEXTENDS         RelationshipType = "EXTENDS"
	RelHAS_METHOD      RelationshipType = "HAS_METHOD"
	RelMEMBER_OF       RelationshipType = "MEMBER_OF"
	RelSTEP_IN_PROCESS RelationshipType = "STEP_IN_PROCESS"
)

// GraphRelationship represents a relationship in the knowledge graph.
type GraphRelationship struct {
	ID         string           `json:"id"`
	SourceID   string           `json:"sourceId"`
	TargetID   string           `json:"targetId"`
	Type       RelationshipType `json:"type"`
	Confidence float64          `json:"confidence"`
	Reason     string           `json:"reason"`
	Step       *int             `json:"step,omitempty"`
}
