package ingest

import "testing"

func TestSupportedLanguagesMatchesV140JavaScriptExtensions(t *testing.T) {
	if _, ok := SupportedLanguages[".cjs"]; ok {
		t.Fatal(".cjs must remain unsupported to match")
	}
	if _, ok := SupportedLanguages[".mjs"]; ok {
		t.Fatal(".mjs must remain unsupported to match")
	}
	for _, ext := range []string{".js", ".jsx"} {
		if SupportedLanguages[ext] != "javascript" {
			t.Fatalf("%s language = %q, want javascript", ext, SupportedLanguages[ext])
		}
	}
	if SupportedLanguages[".tsx"] != "tsx" {
		t.Fatalf(".tsx language = %q, want tsx", SupportedLanguages[".tsx"])
	}
}
