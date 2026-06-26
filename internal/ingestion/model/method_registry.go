package model

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ---------------------------------------------------------------------------
// MethodRegistry — 纯查询接口
// ---------------------------------------------------------------------------

// MethodRegistry — owner-scoped 方法索引。
type MethodRegistry interface {
	LookupMethodByOwner(ownerNodeID string, methodName string, argCount *int) *shared.SymbolDefinition
	LookupMethodByName(name string) []*shared.SymbolDefinition
	LookupAllByOwner(ownerNodeID string, methodName string) []*shared.SymbolDefinition
	HasFunctionMethods() bool
}

// ---------------------------------------------------------------------------
// MutableMethodRegistry — 含注册和清除
// ---------------------------------------------------------------------------

type MutableMethodRegistry interface {
	MethodRegistry
	Register(ownerNodeID string, methodName string, def *shared.SymbolDefinition)
	Clear()
}

// ---------------------------------------------------------------------------
// 工厂：CreateMethodRegistry
// ---------------------------------------------------------------------------

func CreateMethodRegistry() MutableMethodRegistry {
	return &methodRegistryImpl{
		methodByOwner:   make(map[string][]*shared.SymbolDefinition),
		methodsByName:   make(map[string][]*shared.SymbolDefinition),
		hasFunctionFlag: false,
	}
}

type methodRegistryImpl struct {
	methodByOwner   map[string][]*shared.SymbolDefinition
	methodsByName   map[string][]*shared.SymbolDefinition
	hasFunctionFlag bool
}

// LookupMethodByOwner — arity 过滤 + returnType 消歧。
func (mr *methodRegistryImpl) LookupMethodByOwner(
	ownerNodeID string,
	methodName string,
	argCount *int,
) *shared.SymbolDefinition {
	key := ownerNodeID + "\x00" + methodName
	defs, ok := mr.methodByOwner[key]
	if !ok || len(defs) == 0 {
		return nil
	}

	// Arity narrowing
	pool := defs
	if argCount != nil && len(defs) > 1 {
		matchedCount := 0
		rejectedCount := 0
		ac := *argCount

		for _, d := range defs {
			if d.ParameterCount == nil {
				matchedCount++
				continue
			}
			min := d.ParameterCount
			if d.RequiredParameterCount != nil {
				min = d.RequiredParameterCount
			}
			if ac >= *min && ac <= *d.ParameterCount {
				matchedCount++
			} else {
				rejectedCount++
			}
		}

		if matchedCount > 0 && rejectedCount > 0 {
			arityMatched := make([]*shared.SymbolDefinition, 0, matchedCount)
			for _, d := range defs {
				if d.ParameterCount == nil {
					arityMatched = append(arityMatched, d)
					continue
				}
				min := d.ParameterCount
				if d.RequiredParameterCount != nil {
					min = d.RequiredParameterCount
				}
				if ac >= *min && ac <= *d.ParameterCount {
					arityMatched = append(arityMatched, d)
				}
			}
			pool = arityMatched
		}
	}

	// 单候选直接返回
	if len(pool) == 1 {
		return pool[0]
	}

	// 多候选 returnType 消歧
	firstReturnType := pool[0].ReturnType
	if firstReturnType == nil {
		return nil
	}
	for i := 1; i < len(pool); i++ {
		if pool[i].ReturnType == nil || *pool[i].ReturnType != *firstReturnType {
			return nil
		}
	}
	return pool[0]
}

func (mr *methodRegistryImpl) LookupMethodByName(name string) []*shared.SymbolDefinition {
	defs, ok := mr.methodsByName[name]
	if !ok {
		return []*shared.SymbolDefinition{}
	}
	return defs
}

func (mr *methodRegistryImpl) LookupAllByOwner(ownerNodeID string, methodName string) []*shared.SymbolDefinition {
	key := ownerNodeID + "\x00" + methodName
	defs, ok := mr.methodByOwner[key]
	if !ok {
		return []*shared.SymbolDefinition{}
	}
	return defs
}

func (mr *methodRegistryImpl) HasFunctionMethods() bool {
	return mr.hasFunctionFlag
}

// Register — 双索引写入
func (mr *methodRegistryImpl) Register(ownerNodeID string, methodName string, def *shared.SymbolDefinition) {
	key := ownerNodeID + "\x00" + methodName
	mr.methodByOwner[key] = append(mr.methodByOwner[key], def)
	mr.methodsByName[methodName] = append(mr.methodsByName[methodName], def)

	if !mr.hasFunctionFlag && def.Type == shared.LabelFunction {
		mr.hasFunctionFlag = true
	}
}

func (mr *methodRegistryImpl) Clear() {
	for k := range mr.methodByOwner {
		delete(mr.methodByOwner, k)
	}
	for k := range mr.methodsByName {
		delete(mr.methodsByName, k)
	}
	mr.hasFunctionFlag = false
}
