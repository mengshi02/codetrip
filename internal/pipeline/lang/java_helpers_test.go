package lang

import (
	"testing"
)

// ============ javaParseTypeName Tests ============

func TestJavaParseTypeName_Class(t *testing.T) {
	result := javaParseTypeName("class Foo", "class ")
	if result != "Foo" {
		t.Errorf("expected Foo, got %s", result)
	}
}

func TestJavaParseTypeName_InterfaceWithGeneric(t *testing.T) {
	result := javaParseTypeName("interface Bar<T>", "interface ")
	if result != "Bar" {
		t.Errorf("expected Bar, got %s", result)
	}
}

func TestJavaParseTypeName_Enum(t *testing.T) {
	result := javaParseTypeName("enum Baz", "enum ")
	if result != "Baz" {
		t.Errorf("expected Baz, got %s", result)
	}
}

func TestJavaParseTypeName_Record(t *testing.T) {
	result := javaParseTypeName("record Point", "record ")
	if result != "Point" {
		t.Errorf("expected Point, got %s", result)
	}
}

func TestJavaParseTypeName_ClassWithBrace(t *testing.T) {
	result := javaParseTypeName("class Foo {", "class ")
	if result != "Foo" {
		t.Errorf("expected Foo, got %s", result)
	}
}

func TestJavaParseTypeName_EmptyRest(t *testing.T) {
	result := javaParseTypeName("class ", "class ")
	if result != "" {
		t.Errorf("expected empty for empty rest after prefix, got %s", result)
	}
}

func TestJavaParseTypeName_InterfaceWithExtends(t *testing.T) {
	result := javaParseTypeName("interface Serializable extends Base", "interface ")
	if result != "Serializable" {
		t.Errorf("expected Serializable, got %s", result)
	}
}

func TestJavaParseTypeName_Annotation(t *testing.T) {
	result := javaParseTypeName("@interface MyAnnotation", "@interface ")
	if result != "MyAnnotation" {
		t.Errorf("expected MyAnnotation, got %s", result)
	}
}

// ============ javaParseFuncName Tests ============

func TestJavaParseFuncName_PublicVoid(t *testing.T) {
	result := javaParseFuncName("public void foo(")
	if result != "foo" {
		t.Errorf("expected foo, got %s", result)
	}
}

func TestJavaParseFuncName_StringReturn(t *testing.T) {
	result := javaParseFuncName("String bar(")
	if result != "bar" {
		t.Errorf("expected bar, got %s", result)
	}
}

func TestJavaParseFuncName_Constructor(t *testing.T) {
	result := javaParseFuncName("MyClass(")
	if result != "MyClass" {
		t.Errorf("expected MyClass, got %s", result)
	}
}

func TestJavaParseFuncName_NoParenthesis(t *testing.T) {
	result := javaParseFuncName("no parenthesis here")
	if result != "" {
		t.Errorf("expected empty for no parenthesis, got %s", result)
	}
}

func TestJavaParseFuncName_GenericMethod(t *testing.T) {
	result := javaParseFuncName("public <T> T genericMethod(")
	if result != "genericMethod" {
		t.Errorf("expected genericMethod, got %s", result)
	}
}

func TestJavaParseFuncName_PrivateMethod(t *testing.T) {
	result := javaParseFuncName("private static void helper(")
	if result != "helper" {
		t.Errorf("expected helper, got %s", result)
	}
}

// ============ javaModuleName Tests ============

func TestJavaModuleName_FullPath(t *testing.T) {
	result := javaModuleName("/path/to/MyClass.java")
	if result != "MyClass" {
		t.Errorf("expected MyClass, got %s", result)
	}
}

func TestJavaModuleName_SimpleFile(t *testing.T) {
	result := javaModuleName("Simple.java")
	if result != "Simple" {
		t.Errorf("expected Simple, got %s", result)
	}
}

func TestJavaModuleName_NestedPath(t *testing.T) {
	result := javaModuleName("src/main/java/com/example/App.java")
	if result != "App" {
		t.Errorf("expected App, got %s", result)
	}
}

// ============ javaExtractImportPath Tests ============

func TestJavaExtractImportPath_StandardImport(t *testing.T) {
	result := javaExtractImportPath("import com.example.Foo;")
	if result != "com.example.Foo" {
		t.Errorf("expected com.example.Foo, got %s", result)
	}
}

func TestJavaExtractImportPath_StaticImport(t *testing.T) {
	result := javaExtractImportPath("import static com.example.Bar.method;")
	if result != "com.example.Bar.method" {
		t.Errorf("expected com.example.Bar.method, got %s", result)
	}
}

func TestJavaExtractImportPath_WildcardImport(t *testing.T) {
	result := javaExtractImportPath("import com.example.*;")
	if result != "com.example.*" {
		t.Errorf("expected com.example.*, got %s", result)
	}
}

func TestJavaExtractImportPath_JavaUtil(t *testing.T) {
	result := javaExtractImportPath("import java.util.List;")
	if result != "java.util.List" {
		t.Errorf("expected java.util.List, got %s", result)
	}
}

func TestJavaExtractImportPath_NoTrailingSemicolon(t *testing.T) {
	result := javaExtractImportPath("import com.example.NoSemi")
	if result != "com.example.NoSemi" {
		t.Errorf("expected com.example.NoSemi, got %s", result)
	}
}