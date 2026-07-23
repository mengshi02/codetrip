package ingest

import (
	"strings"
	"testing"

	graph "github.com/mengshi02/codetrip/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestSwiftInitializerAndCallArity(t *testing.T) {
	source := []byte(`
class Service {
    init(repository: Repository) {}
}
func main() {
    let service: Service = Service(repository: repository)
}
`)
	lang, err := NewLanguageRegistry().GetLanguage("swift")
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
	var initializer, call *sitter.Node
	var walk func(*sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}
		if node.Kind() == "init_declaration" {
			initializer = node
		}
		if node.Kind() == "call_expression" && strings.HasPrefix(node.Utf8Text(source), "Service(") {
			call = node
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			walk(node.NamedChild(i))
		}
	}
	walk(tree.RootNode())
	if initializer == nil || call == nil {
		t.Fatalf("initializer or call missing: %s", tree.RootNode().ToSexp())
	}
	signature := ExtractMethodSignature(initializer, source)
	if signature.ParameterCount == nil || *signature.ParameterCount != 1 {
		t.Fatalf("initializer arity = %v, want 1; tree=%s", signature.ParameterCount, tree.RootNode().ToSexp())
	}
	if got := CountCallArguments(call); got != 1 {
		t.Fatalf("call arity = %d, want 1; tree=%s", got, tree.RootNode().ToSexp())
	}
}

func TestSwiftImplicitStructConstructorPrefersDeclarationOverExtension(t *testing.T) {
	source := "struct Complex {}\nextension Complex {}\nfunc make() { _ = Complex() }\n"
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	extracted := ProcessParsing(
		g, []FileInput{{Path: "Complex.swift", Content: source}}, st,
		NewLanguageRegistry(), nil,
	)
	st.FinalizeRangeIndex()
	ProcessCallsFromExtracted(
		g, extracted.Calls, st, NewImportMap(), NewPackageMap(), make(NamedImportMap), make(ImportOrderMap), map[string]map[string]bool{},
	)
	for _, relation := range g.Relationships() {
		if relation.Type == graph.RelCALLS && relation.TargetID == "Struct:Complex.swift:Complex" {
			return
		}
	}
	t.Fatal("implicit Swift struct construction did not target the concrete declaration")
}

func TestSwiftSelfReceiverUsesEnclosingProtocolType(t *testing.T) {
	source := `
protocol Real {
    static func pow(_ x: Self, _ n: Int) -> Self
}
extension Real where Self: FixedWidthFloatingPoint {
    static func check(_ x: Self) {
        _ = Self.pow(x, 2)
    }
}
`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	extracted := ProcessParsing(
		g, []FileInput{{Path: "Real.swift", Content: source}}, st,
		NewLanguageRegistry(), nil,
	)
	for _, call := range extracted.Calls {
		if call.CallName == "pow" {
			if call.ReceiverName != "Self" || call.ReceiverTypeName != "Real" {
				t.Fatalf("Self.pow receiver=(%q, %q), want (Self, Real)", call.ReceiverName, call.ReceiverTypeName)
			}
			return
		}
	}
	t.Fatal("Self.pow call was not extracted")
}

func TestSwiftGenericReceiverUsesLeadingConstraint(t *testing.T) {
	source := `
protocol Real { static func log(_ x: Self) -> Self }
func check<T: Real & FixedWidthFloatingPoint>(_ type: T.Type) {
    _ = T.log(1)
}
`
	g := graph.NewKnowledgeGraph()
	st := NewSymbolTable()
	extracted := ProcessParsing(
		g, []FileInput{{Path: "Generic.swift", Content: source}}, st,
		NewLanguageRegistry(), nil,
	)
	for _, call := range extracted.Calls {
		if call.CallName == "log" {
			if call.ReceiverName != "T" || call.ReceiverTypeName != "Real" {
				t.Fatalf("T.log receiver=(%q, %q), want (T, Real)", call.ReceiverName, call.ReceiverTypeName)
			}
			return
		}
	}
	t.Fatal("T.log call was not extracted")
}
