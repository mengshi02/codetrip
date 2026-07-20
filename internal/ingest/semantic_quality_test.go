package ingest

import (
	"testing"

	graph "github.com/mengshi02/codetrip/internal/model"
)

func TestCPPInlineMethodKeepsItsOwnDefinitionRangeAndArity(t *testing.T) {
	source := `class Widget {
public:
    void overload(int) {}
};`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	ProcessParsing(
		g,
		[]FileInput{{Path: "main.cc", Content: source}},
		st,
		NewLanguageRegistry(),
		nil,
	)
	defs := st.LookupFuzzy("overload")
	if len(defs) != 1 {
		t.Fatalf("overload definitions = %d, want 1", len(defs))
	}
	if defs[0].ParameterCount == nil || *defs[0].ParameterCount != 1 {
		t.Fatalf("overload parameter count = %v, want 1", defs[0].ParameterCount)
	}
	if defs[0].OwnerID != "Class:main.cc:Widget" {
		t.Fatalf("overload owner = %q", defs[0].OwnerID)
	}
}

func TestCPPInlineReferenceReturnIsOwnedMethod(t *testing.T) {
	source := `class Rules {
public:
    Rules& methods(int value) { return *this; }
};`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	ProcessParsing(
		g, []FileInput{{Path: "main.cc", Content: source}}, st,
		NewLanguageRegistry(), nil,
	)
	found := false
	for _, def := range st.LookupFuzzy("methods") {
		if def.Type == "Method" && def.OwnerID == "Class:main.cc:Rules" {
			if def.ParameterCount == nil || *def.ParameterCount != 1 {
				t.Fatalf("reference-return method arity = %v", def.ParameterCount)
			}
			found = true
		}
	}
	if !found {
		t.Fatal("inline reference-return method did not retain its class owner")
	}
}

func TestCPPPreprocessorWrappedReferenceMethodIsCaptured(t *testing.T) {
	source := `class Builder { public:
#ifdef ENABLE_FEATURE
Builder& feature(int value) { return *this; }
#endif
};`
	st := NewSymbolTable()
	ProcessParsing(graph.NewKnowledgeGraph(), []FileInput{{Path: "builder.h", Content: source}}, st, NewLanguageRegistry(), nil)
	found := false
	for _, def := range st.LookupFuzzy("feature") {
		if def.Type == "Method" && def.OwnerID == "Class:builder.h:Builder" && def.ReturnType == "Builder" {
			found = true
		}
	}
	if !found {
		t.Fatal("preprocessor-wrapped reference method was not captured with return type")
	}
}

func TestCPPVirtualInterfaceDeclarationsAreOwnedMethods(t *testing.T) {
	source := `class LEVELDB_EXPORT DB {
public:
    virtual Iterator* NewIterator(const Options& options) = 0;
    virtual const Snapshot* GetSnapshot() = 0;
    virtual bool Valid() const = 0;
};`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	ProcessParsing(
		g,
		[]FileInput{{Path: "db.h", Content: source}},
		st,
		NewLanguageRegistry(),
		nil,
	)
	for _, name := range []string{"NewIterator", "GetSnapshot", "Valid"} {
		defs := st.LookupFuzzy(name)
		if len(defs) != 1 {
			t.Fatalf("%s definitions = %d, want 1", name, len(defs))
		}
		if defs[0].Type != "Method" || defs[0].OwnerID != "Class:db.h:DB" {
			t.Fatalf("%s definition = %#v", name, defs[0])
		}
	}
}

func TestCPPQualifiedNamespaceCallIsNotAnObjectMember(t *testing.T) {
	st := NewSymbolTable()
	one := 1
	st.Add("main.cc", "crow", "Namespace:main.cc:crow", "Namespace", nil, "", 0, 100)
	st.Add("main.cc", "method_name", "Function:main.cc:method_name", "Function", &one, "", 10, 20)
	g := graph.NewKnowledgeGraph()
	ProcessCallsFromExtracted(
		g, []ExtractedCall{{
			FilePath: "main.cc", Language: "cpp", SourceID: "Function:main.cc:caller",
			CallName: "method_name", ReceiverName: "crow", CallForm: CallFormMember, ArgCount: 1,
		}}, st, NewImportMap(), NewPackageMap(), make(NamedImportMap), make(ImportOrderMap), map[string]map[string]bool{},
	)
	found := false
	for _, rel := range g.Relationships() {
		if rel.Type == graph.RelCALLS && rel.SourceID == "Function:main.cc:caller" && rel.TargetID == "Function:main.cc:method_name" {
			found = true
		}
	}
	if !found {
		t.Fatal("qualified namespace call did not resolve to free function")
	}
}

func TestKotlinLocalPropertyIsNotAClassMember(t *testing.T) {
	source := `package fixture

class Sample {
    val member = 1
    fun run() {
        val local = 2
        println(local)
    }
}`
	g := graph.NewKnowledgeGraph()
	ProcessParsing(
		g,
		[]FileInput{{Path: "Main.kt", Content: source}},
		NewSymbolTable(),
		NewLanguageRegistry(),
		nil,
	)

	wantMember := false
	foundLocalOwner := false
	for _, rel := range g.Relationships() {
		if rel.Type != graph.RelHAS_METHOD {
			continue
		}
		if rel.SourceID == "Class:Main.kt:Sample" && rel.TargetID == "Property:Main.kt:Sample.member" {
			wantMember = true
		}
		if rel.TargetID == "Property:Main.kt:local" {
			foundLocalOwner = true
		}
	}
	if !wantMember {
		t.Fatal("direct Kotlin property did not receive HAS_METHOD owner")
	}
	if foundLocalOwner {
		t.Fatal("local Kotlin property was incorrectly emitted as a class member")
	}
}

func TestResolveCallCollapsesOverloadsWithSamePhysicalTarget(t *testing.T) {
	st := NewSymbolTable()
	one := 1
	st.Add("main.cc", "overload", "Method:main.cc:overload", "Method", &one,
		"Class:main.cc:Widget", 10, 20)
	st.Add("main.cc", "overload", "Method:main.cc:overload", "Method", &one,
		"Class:main.cc:Widget", 30, 40)
	ctx := &ResolveContext{
		SymbolTable:    st,
		NamedImportMap: make(NamedImportMap),
		ImportMap:      NewImportMap(),
		PackageMap:     NewPackageMap(),
	}

	resolved := resolveCallTarget("overload", "main.cc", "Widget", ctx, CallFormMember, 1)
	if resolved == nil {
		t.Fatal("same-target overload candidates should resolve to their shared graph node")
	}
	if resolved.NodeID != "Method:main.cc:overload" {
		t.Fatalf("resolved target = %q", resolved.NodeID)
	}
}

func TestResolveCallAllowsUniqueExactMemberDefaultArguments(t *testing.T) {
	st := NewSymbolTable()
	one := 1
	st.Add("main.cc", "Fixture", "Class:main.cc:Fixture", "Class", nil, "", 0, 200)
	st.Add("main.cc", "Reopen", "Method:main.cc:Fixture.Reopen", "Method", &one,
		"Class:main.cc:Fixture", 10, 20)
	ctx := &ResolveContext{SymbolTable: st, AssignableOwnerIDs: map[string]map[string]bool{}}

	resolved := resolveCallTarget("Reopen", "main.cc", "Fixture", ctx, CallFormMember, 0)
	if resolved == nil || resolved.NodeID != "Method:main.cc:Fixture.Reopen" {
		t.Fatalf("default-argument call resolved to %#v", resolved)
	}
	if resolved.Reason != "receiver-type-default-args" {
		t.Fatalf("resolution reason = %q", resolved.Reason)
	}
}

func TestResolveCallFiltersReceiverTypesByTransitiveIncludes(t *testing.T) {
	st := NewSymbolTable()
	zero := 0
	st.Add("public/iterator.h", "Iterator", "Class:public/iterator.h:Iterator", "Class", nil, "", 0, 100)
	st.Add("public/iterator.h", "Valid", "Method:public/iterator.h:Iterator.Valid", "Method", &zero,
		"Class:public/iterator.h:Iterator", 10, 20)
	st.Add("internal/skiplist.h", "Iterator", "Class:internal/skiplist.h:Iterator", "Class", nil, "", 0, 100)
	st.Add("internal/skiplist.h", "Valid", "Method:internal/skiplist.h:Iterator.Valid", "Method", &zero,
		"Class:internal/skiplist.h:Iterator", 10, 20)
	imports := NewImportMap()
	imports.AddImport("main.cc", "public/db.h")
	imports.AddImport("public/db.h", "public/iterator.h")
	ctx := &ResolveContext{SymbolTable: st, ImportMap: imports, AssignableOwnerIDs: map[string]map[string]bool{}}
	visible := visibleTypeDefinitions("Iterator", "main.cc", ctx)
	if len(visible) != 1 || visible[0].NodeID != "Class:public/iterator.h:Iterator" {
		t.Fatalf("visible Iterator definitions = %#v", visible)
	}

	resolved := resolveCallTarget("Valid", "main.cc", "Iterator", ctx, CallFormMember, 0)
	if resolved == nil || resolved.NodeID != "Method:public/iterator.h:Iterator.Valid" {
		t.Fatalf("visible Iterator.Valid resolved to %#v", resolved)
	}
}

func TestResolveCallExpandsVisibleCppTypeAlias(t *testing.T) {
	st := NewSymbolTable()
	zero := 0
	st.Add("engine.h", "Engine", "Class:engine.h:Engine", "Class", nil, "", 0, 100)
	st.Add("engine.h", "run", "Method:engine.h:Engine.run", "Method", &zero, "Class:engine.h:Engine", 10, 20)
	st.AddTypeAlias("engine.h", "SimpleEngine", "Engine")
	imports := NewImportMap()
	imports.AddImport("main.cc", "engine.h")
	ctx := &ResolveContext{SymbolTable: st, ImportMap: imports, AssignableOwnerIDs: map[string]map[string]bool{}}

	resolved := resolveCallTarget("run", "main.cc", "SimpleEngine", ctx, CallFormMember, 0)
	if resolved == nil || resolved.NodeID != "Method:engine.h:Engine.run" {
		t.Fatalf("type-alias receiver resolved to %#v", resolved)
	}
}

func TestCppFluentOverloadUsesIntermediateArityForReturnType(t *testing.T) {
	st := NewSymbolTable()
	zero, one := 0, 1
	st.Add("app.h", "Crow", "Class:app.h:Crow", "Class", nil, "", 0, 200)
	st.Add("app.h", "port", "Method:app.h:Crow.port", "Method", &zero, "Class:app.h:Crow", 10, 20)
	st.SetReturnType("Method:app.h:Crow.port", 10, "uint16_t")
	st.Add("app.h", "port", "Method:app.h:Crow.port", "Method", &one, "Class:app.h:Crow", 30, 40)
	st.SetReturnType("Method:app.h:Crow.port", 30, "self_t")
	st.Add("app.h", "run", "Method:app.h:Crow.run", "Method", &zero, "Class:app.h:Crow", 50, 60)
	st.AddTypeAlias("app.h", "App", "Crow")
	st.AddTypeAlias("app.h", "self_t", "Crow")
	imports := NewImportMap()
	imports.AddImport("main.cc", "app.h")
	g := graph.NewKnowledgeGraph()
	ProcessCallsFromExtracted(g, []ExtractedCall{{
		FilePath: "main.cc", Language: "cpp", SourceID: "Function:main.cc:main",
		CallName: "run", ReceiverName: "app", ReceiverTypeName: "App",
		ReceiverChain: []string{"port"}, ReceiverChainArgCounts: []int{1},
		CallForm: CallFormMember, ArgCount: 0,
	}}, st, imports, NewPackageMap(), make(NamedImportMap), make(ImportOrderMap), map[string]map[string]bool{})
	for _, rel := range g.Relationships() {
		if rel.Type == graph.RelCALLS && rel.TargetID == "Method:app.h:Crow.run" {
			return
		}
	}
	t.Fatal("fluent setter overload did not propagate Crow return type to run")
}

func TestCppLowercaseAndBaseInitializerCallsResolveConstructors(t *testing.T) {
	source := `
struct returnable { returnable(const char* value) {} };
struct response { response() {} response(int code) {} };
struct wvalue : returnable { wvalue(): returnable("json") {} };
int main() { auto value = response(100); }
`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	extracted := ProcessParsing(
		g, []FileInput{{Path: "main.cc", Content: source}}, st,
		NewLanguageRegistry(), nil,
	)
	assignable := BuildAssignableOwnerIDs(
		extracted.Heritage, st, NewImportMap(), NewPackageMap(), make(NamedImportMap), make(ImportOrderMap),
	)
	ProcessCallsFromExtracted(
		g, extracted.Calls, st, NewImportMap(), NewPackageMap(), make(NamedImportMap), make(ImportOrderMap), assignable,
	)
	want := map[string]bool{
		"Constructor:main.cc:returnable.returnable": false,
		"Constructor:main.cc:response.response":     false,
	}
	baseInitializerOwnedByConstructor := false
	for _, rel := range g.Relationships() {
		if rel.Type == graph.RelCALLS {
			if _, ok := want[rel.TargetID]; ok {
				want[rel.TargetID] = true
			}
			if rel.SourceID == "Constructor:main.cc:wvalue.wvalue" && rel.TargetID == "Constructor:main.cc:returnable.returnable" {
				baseInitializerOwnedByConstructor = true
			}
		}
	}
	for target, found := range want {
		if !found {
			t.Fatalf("constructor call target %s was not emitted", target)
		}
	}
	constructor, ok := g.GetNode("Constructor:main.cc:response.response")
	if !ok || constructor.Properties.StartLine == nil || constructor.Properties.EndLine == nil ||
		*constructor.Properties.StartLine > 2 || *constructor.Properties.EndLine < 2 {
		t.Fatalf("collapsed constructor range = %#v", constructor)
	}
	if !baseInitializerOwnedByConstructor {
		t.Fatal("base initializer call was not owned by the derived constructor")
	}
}

func TestCppVisibleConstructorOutranksImportedForwardDeclaration(t *testing.T) {
	st := NewSymbolTable()
	one := 1
	st.Add("env.h", "Slice", "Class:env.h:Slice", "Class", nil, "", 0, 10)
	st.Add("slice.h", "Slice", "Class:slice.h:Slice", "Class", nil, "", 0, 100)
	st.Add("slice.h", "Slice", "Constructor:slice.h:Slice.Slice", "Constructor", &one, "Class:slice.h:Slice", 10, 20)
	imports := NewImportMap()
	imports.AddImport("reader.cc", "env.h")
	imports.AddImport("env.h", "slice.h")
	ctx := &ResolveContext{SymbolTable: st, ImportMap: imports, AssignableOwnerIDs: map[string]map[string]bool{}}
	resolved := resolveCallTarget("Slice", "reader.cc", "", ctx, CallFormConstructor, 1)
	if resolved == nil || resolved.NodeID != "Constructor:slice.h:Slice.Slice" {
		t.Fatalf("Slice constructor resolved to %#v", resolved)
	}
}

func TestCppHeritageNeverUsesSameNamedConstructors(t *testing.T) {
	source := `class Base { public: Base() {} }; class Child : public Base { public: Child() {} };`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	extracted := ProcessParsing(
		g, []FileInput{{Path: "main.cc", Content: source}}, st,
		NewLanguageRegistry(), nil,
	)
	ProcessHeritageFromExtracted(
		g, extracted.Heritage, st, NewImportMap(), NewPackageMap(), make(NamedImportMap), make(ImportOrderMap),
	)
	for _, relation := range g.Relationships() {
		if relation.Type == graph.RelEXTENDS {
			if relation.SourceID != "Class:main.cc:Child" || relation.TargetID != "Class:main.cc:Base" {
				t.Fatalf("inheritance resolved through non-type nodes: %#v", relation)
			}
			return
		}
	}
	t.Fatal("Child -> Base EXTENDS relation not emitted")
}

func TestCppPreprocessorWrappedOrdinaryMethodOwnsCalls(t *testing.T) {
	source := `
class Response { public: void end() {} };
class Rule {
#ifdef ENABLE_SSL
  void handle(Response& response) { response.end(); }
#endif
};`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	extracted := ProcessParsing(
		g, []FileInput{{Path: "main.cc", Content: source}}, st,
		NewLanguageRegistry(), nil,
	)
	ProcessCallsFromExtracted(
		g, extracted.Calls, st, NewImportMap(), NewPackageMap(), make(NamedImportMap), make(ImportOrderMap), map[string]map[string]bool{},
	)
	for _, relation := range g.Relationships() {
		if relation.Type == graph.RelCALLS && relation.TargetID == "Method:main.cc:Response.end" {
			if relation.SourceID != "Method:main.cc:Rule.handle" {
				t.Fatalf("preprocessor-wrapped call owner = %s", relation.SourceID)
			}
			return
		}
	}
	t.Fatal("preprocessor-wrapped method call not emitted")
}

func TestCppMemberInitializerDoesNotResolveToSameNamedFreeFunction(t *testing.T) {
	source := `void env() {} class Options { int env; public: Options(): env(1) {} };`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	extracted := ProcessParsing(
		g, []FileInput{{Path: "main.cc", Content: source}}, st,
		NewLanguageRegistry(), nil,
	)
	ProcessCallsFromExtracted(
		g, extracted.Calls, st, NewImportMap(), NewPackageMap(), make(NamedImportMap), make(ImportOrderMap), map[string]map[string]bool{},
	)
	for _, relation := range g.Relationships() {
		if relation.Type == graph.RelCALLS && relation.TargetID == "Function:main.cc:env" {
			t.Fatalf("data-member initializer produced a fake constructor call: %#v", relation)
		}
	}
}

func TestCPPUsingAliasCapturesTemplateBaseType(t *testing.T) {
	st := NewSymbolTable()
	ProcessParsing(
		graph.NewKnowledgeGraph(),
		[]FileInput{{Path: "engine.h", Content: `template<class T> class Engine {}; using SimpleEngine = Engine<int>;`}},
		st, NewLanguageRegistry(), nil,
	)
	aliases := st.LookupTypeAliases("SimpleEngine")
	if len(aliases) != 1 || aliases[0].TargetName != "Engine" {
		t.Fatalf("SimpleEngine aliases = %#v", aliases)
	}
}

func TestMemberIdentityIncludesDirectOwner(t *testing.T) {
	source := `class Left { public: void run() {} };
class Right { public: void run() {} };`
	g := graph.NewKnowledgeGraph()
	ProcessParsing(
		g,
		[]FileInput{{Path: "main.cc", Content: source}},
		NewSymbolTable(), NewLanguageRegistry(), nil,
	)
	ids := make(map[string]bool)
	for _, node := range g.Nodes() {
		if node.Label == graph.LabelMethod && node.Properties.Name == "run" {
			ids[node.ID] = true
		}
	}
	if len(ids) != 2 || !ids["Method:main.cc:Left.run"] || !ids["Method:main.cc:Right.run"] {
		t.Fatalf("method IDs = %v", ids)
	}
}
