package ingest

import (
	"strings"
	"testing"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

func findNodeKind(node *sitter.Node, kind string) *sitter.Node {
	if node.Kind() == kind {
		return node
	}
	for i := uint(0); i < node.NamedChildCount(); i++ {
		if found := findNodeKind(node.NamedChild(i), kind); found != nil {
			return found
		}
	}
	return nil
}

func TestExtractReceiverNameFromCppSubscript(t *testing.T) {
	source := []byte(`class Slice { public: const char* data() const; }; void f(const Slice* keys, int i) { keys[i].data(); }`)
	lang, err := NewLanguageRegistry().GetLanguage("cpp")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()
	call := findNodeKind(tree.RootNode(), "call_expression")
	if call == nil {
		t.Fatal("call_expression not found")
	}
	field := call.ChildByFieldName("function")
	name := field.ChildByFieldName("field")
	if got := ExtractReceiverName(name, source); got != "keys" {
		t.Fatalf("ExtractReceiverName() = %q, want keys", got)
	}
	env := BuildTypeEnv(tree.RootNode(), "cpp", source)
	if got := LookupTypeEnv(env, "keys", call, source); got != "Slice" {
		t.Fatalf("LookupTypeEnv(keys) = %q, want Slice", got)
	}
}

func TestCppFieldReceiverCarriesExplicitType(t *testing.T) {
	source := []byte(`class Holder { const DB* const db_; void run() { db_->NewIterator(); } };`)
	lang, err := NewLanguageRegistry().GetLanguage("cpp")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()
	call := findNodeKind(tree.RootNode(), "call_expression")
	if call == nil {
		t.Fatal("call_expression not found")
	}
	env := BuildTypeEnv(tree.RootNode(), "cpp", source)
	if got := LookupTypeEnv(env, "db_", call, source); got != "DB" {
		t.Fatalf("LookupTypeEnv(db_) = %q, want DB", got)
	}
}

func TestCppRangeForReceiverCarriesExplicitType(t *testing.T) {
	source := []byte(`void run(Container values) { for (Blueprint* bp : values) { bp->static_dir(); } }`)
	lang, err := NewLanguageRegistry().GetLanguage("cpp")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()
	call := findNodeKind(tree.RootNode(), "call_expression")
	if call == nil {
		t.Fatal("call_expression not found")
	}
	env := BuildTypeEnv(tree.RootNode(), "cpp", source)
	if got := LookupTypeEnv(env, "bp", call, source); got != "Blueprint" {
		t.Fatalf("LookupTypeEnv(bp) = %q, want Blueprint", got)
	}
}

func TestCppUnnamedOperatorScopeDoesNotLeakParameterTypes(t *testing.T) {
	source := []byte(`struct Box { bool end(); };
bool operator<(const std::string& l, const std::string& r) { return l.end(); }
void later(Box l) { l.end(); }`)
	lang, err := NewLanguageRegistry().GetLanguage("cpp")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()
	call := findNodeKind(tree.RootNode(), "call_expression")
	if call == nil {
		t.Fatal("call_expression not found")
	}
	env := BuildTypeEnv(tree.RootNode(), "cpp", source)
	if got := LookupTypeEnv(env, "l", call, source); got != "string" {
		t.Fatalf("operator parameter l type = %q, want string", got)
	}
}

func TestCppDirectInitializationBindsLocalReceiverType(t *testing.T) {
	source := []byte(`void decode() { rvalue ret(Type::List); ret.set_error(); }`)
	lang, err := NewLanguageRegistry().GetLanguage("cpp")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()
	call := findNodeKind(tree.RootNode(), "call_expression")
	if call == nil {
		t.Fatal("call_expression not found")
	}
	env := BuildTypeEnv(tree.RootNode(), "cpp", source)
	if got := LookupTypeEnv(env, "ret", call, source); got != "rvalue" {
		t.Fatalf("direct-initialized ret type = %q, want rvalue", got)
	}
}

func TestExtractCppFluentReceiverChain(t *testing.T) {
	source := []byte(`void f(App app) { app.bind().port(80).run(); }`)
	lang, _ := NewLanguageRegistry().GetLanguage("cpp")
	parser := sitter.NewParser()
	defer parser.Close()
	_ = parser.SetLanguage(lang)
	tree := parser.Parse(source, nil)
	defer tree.Close()
	var terminal *sitter.Node
	var findTerminal func(*sitter.Node)
	findTerminal = func(node *sitter.Node) {
		if terminal != nil {
			return
		}
		if node.Kind() == "field_identifier" && node.Utf8Text(source) == "run" {
			terminal = node
			return
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			findTerminal(node.NamedChild(i))
		}
	}
	findTerminal(tree.RootNode())
	base, chain, argCounts := ExtractCppReceiverChain(terminal, source)
	if base != "app" || len(chain) != 2 || chain[0] != "bind" || chain[1] != "port" || len(argCounts) != 2 || argCounts[0] != 0 || argCounts[1] != 1 {
		t.Fatalf("fluent receiver = %q %#v %#v", base, chain, argCounts)
	}
}

func TestCppTemplateReceiverCarriesExplicitType(t *testing.T) {
	source := []byte(`class Other { public: void begin(); }; class R { template<typename T, typename U> static bool equals(const T& l, const U& r) { return l.begin() == r.begin(); } };`)
	lang, err := NewLanguageRegistry().GetLanguage("cpp")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()
	query, err := sitter.NewQuery(lang, LanguageQueries("cpp"))
	if query == nil {
		t.Fatal(err)
	}
	defer query.Close()
	cursor := sitter.NewQueryCursor()
	defer cursor.Close()
	matches := cursor.Matches(query, tree.RootNode(), source)
	names := query.CaptureNames()
	env := BuildTypeEnv(tree.RootNode(), "cpp", source)
	seen := 0
	for {
		match := matches.Next()
		if match == nil {
			break
		}
		cm := buildCaptureMap(match, names)
		call, ok := cm["call"]
		if !ok {
			continue
		}
		name := cm["call.name"]
		receiver := ExtractReceiverName(name, source)
		if InferCallForm(call, name, source) != CallFormMember {
			t.Fatalf("%s was not classified as a member call", call.Utf8Text(source))
		}
		gotType := LookupTypeEnv(env, receiver, call, source)
		if (receiver == "l" && gotType != "T") || (receiver == "r" && gotType != "U") {
			t.Fatalf("receiver %q type = %q", receiver, gotType)
		}
		seen++
	}
	if seen != 2 {
		t.Fatalf("saw %d member calls, want 2", seen)
	}
}

func TestCppNamespaceQualifiedTemplateReceiverCarriesExplicitType(t *testing.T) {
	source := []byte(`int main() { crow::App<crow::CORSHandler> app; app.port(18080).run(); }`)
	lang, err := NewLanguageRegistry().GetLanguage("cpp")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()

	var runCall *sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil || runCall != nil {
			return
		}
		if node.Kind() == "call_expression" && strings.HasSuffix(node.Utf8Text(source), ".run()") {
			runCall = node
			return
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	if runCall == nil {
		t.Fatal("run call not found")
	}
	if got := LookupTypeEnv(BuildTypeEnv(tree.RootNode(), "cpp", source), "app", runCall, source); got != "App" {
		t.Fatalf("LookupTypeEnv(app) = %q, want App", got)
	}
}

func TestCppSmartPointerReceiverCarriesPointeeType(t *testing.T) {
	source := []byte(`class Server { public: void stop(); }; class App { std::unique_ptr<Server> server_; void halt() { server_->stop(); } };`)
	lang, err := NewLanguageRegistry().GetLanguage("cpp")
	if err != nil {
		t.Fatal(err)
	}
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()
	var stopCall *sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "call_expression" && strings.HasSuffix(node.Utf8Text(source), "stop()") {
			stopCall = node
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	if stopCall == nil {
		t.Fatal("stop call not found")
	}
	if got := LookupTypeEnv(BuildTypeEnv(tree.RootNode(), "cpp", source), "server_", stopCall, source); got != "Server" {
		t.Fatalf("LookupTypeEnv(server_) = %q, want Server", got)
	}
}
