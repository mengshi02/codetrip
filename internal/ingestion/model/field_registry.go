package model

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// FieldRegistry — 纯查询接口
// ---------------------------------------------------------------------------

// FieldRegistry — owner-scoped 字段索引。
type FieldRegistry interface {
	LookupFieldByOwner(ownerNodeID string, fieldName string) *shared.SymbolDefinition
	LookupAllByOwner(ownerNodeID string, fieldName string) []*shared.SymbolDefinition
}

// ---------------------------------------------------------------------------
// MutableFieldRegistry — 含注册和清除
// ---------------------------------------------------------------------------

type MutableFieldRegistry interface {
	FieldRegistry
	Register(ownerNodeID string, fieldName string, def *shared.SymbolDefinition)
	Clear()
}

// ---------------------------------------------------------------------------
// 工厂：CreateFieldRegistry
// ---------------------------------------------------------------------------

func CreateFieldRegistry() MutableFieldRegistry {
	return &fieldRegistryImpl{
		fieldByOwner: make(map[string][]*shared.SymbolDefinition),
	}
}

type fieldRegistryImpl struct {
	fieldByOwner map[string][]*shared.SymbolDefinition
}

func (fr *fieldRegistryImpl) LookupAllByOwner(ownerNodeID string, fieldName string) []*shared.SymbolDefinition {
	key := ownerNodeID + "\x00" + fieldName
	defs, ok := fr.fieldByOwner[key]
	if !ok {
		return []*shared.SymbolDefinition{}
	}
	return defs
}

func (fr *fieldRegistryImpl) LookupFieldByOwner(ownerNodeID string, fieldName string) *shared.SymbolDefinition {
	pool := fr.LookupAllByOwner(ownerNodeID, fieldName)
	if len(pool) == 0 {
		return nil
	}
	return pool[0]
}

func (fr *fieldRegistryImpl) Register(ownerNodeID string, fieldName string, def *shared.SymbolDefinition) {
	key := ownerNodeID + "\x00" + fieldName
	fr.fieldByOwner[key] = append(fr.fieldByOwner[key], def)
}

func (fr *fieldRegistryImpl) Clear() {
	for k := range fr.fieldByOwner {
		delete(fr.fieldByOwner, k)
	}
}
