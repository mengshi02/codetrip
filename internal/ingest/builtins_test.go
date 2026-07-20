package ingest

import "testing"

func TestIsBuiltInOrNoiseReferenceNames(t *testing.T) {
	for _, name := range []string{"set", "setTimeout", "print"} {
		if !IsBuiltInOrNoise(name) {
			t.Fatalf("reference built-in %q was not filtered", name)
		}
	}
	if IsBuiltInOrNoise("businessSetter") {
		t.Fatal("unrelated business method was incorrectly classified as built-in")
	}
}

func TestBuiltinFilteringIsLanguageAware(t *testing.T) {
	for _, name := range []string{"ToString", "find", "run"} {
		if IsBuiltInOrNoiseForLanguage(name, "cpp") {
			t.Fatalf("C++ repository method %s was filtered by another language's built-ins", name)
		}
	}
	if !IsBuiltInOrNoiseForLanguage("ToString", "csharp") {
		t.Fatal("C# ToString should retain its compatibility noise classification")
	}
}
