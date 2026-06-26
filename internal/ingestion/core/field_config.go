package core

import "github.com/odvcencio/gotreesitter"

// FieldExtractionConfig defines per-language field extraction configuration.
type FieldExtractionConfig struct {
	Language              SupportedLanguage
	TypeDeclarationNodes  []string
	FieldNodeTypes        []string
	BodyNodeTypes         []string
	DefaultVisibility     FieldVisibility
	QualifiedNodeID       bool // when true, OwnerFQN uses fully-qualified name

	// Function hooks (optional — nil = not set)
	ExtractOwnerName      func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string
	FindBodyNodes         func(node *gotreesitter.Node, lang *gotreesitter.Language) []*gotreesitter.Node
	ExtractName           func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string
	ExtractNames          func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string // multi-name (e.g. Ruby attr_accessor)
	ExtractType           func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string
	ExtractVisibility     func(node *gotreesitter.Node, lang *gotreesitter.Language) FieldVisibility
	ExtractVisibilityForName func(node *gotreesitter.Node, name string, lang *gotreesitter.Language) FieldVisibility
	IsStatic              func(node *gotreesitter.Node, lang *gotreesitter.Language) bool
	IsReadonly            func(node *gotreesitter.Node, lang *gotreesitter.Language) bool
	ExtractPrimaryFields  func(ownerNode *gotreesitter.Node, ctx *FieldExtractorContext, source []byte, lang *gotreesitter.Language) []FieldInfo
}