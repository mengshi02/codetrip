package lang

import (
	"testing"
)

// ============ lastIndexOf Tests ============

func TestLastIndexOf_Found(t *testing.T) {
	result := lastIndexOf("hello/world", '/')
	if result != 5 {
		t.Errorf("expected 5, got %d", result)
	}
}

func TestLastIndexOf_NotFound(t *testing.T) {
	result := lastIndexOf("hello", '/')
	if result != -1 {
		t.Errorf("expected -1, got %d", result)
	}
}

func TestLastIndexOf_MultipleOccurrences(t *testing.T) {
	result := lastIndexOf("a/b/c/d", '/')
	if result != 5 {
		t.Errorf("expected 5 (last occurrence), got %d", result)
	}
}

func TestLastIndexOf_EmptyString(t *testing.T) {
	result := lastIndexOf("", '/')
	if result != -1 {
		t.Errorf("expected -1 for empty string, got %d", result)
	}
}

func TestLastIndexOf_SingleCharMatch(t *testing.T) {
	result := lastIndexOf("/", '/')
	if result != 0 {
		t.Errorf("expected 0, got %d", result)
	}
}

// ============ mdModuleName Tests ============

func TestMdModuleName_EmptyPath(t *testing.T) {
	result := mdModuleName("")
	if result != "markdown" {
		t.Errorf("expected markdown for empty path, got %s", result)
	}
}

func TestMdModuleName_SimpleFile(t *testing.T) {
	result := mdModuleName("README.md")
	if result != "README" {
		t.Errorf("expected README, got %s", result)
	}
}

func TestMdModuleName_NestedPath(t *testing.T) {
	result := mdModuleName("docs/guide.md")
	if result != "guide" {
		t.Errorf("expected guide, got %s", result)
	}
}

func TestMdModuleName_MultipleDots(t *testing.T) {
	result := mdModuleName("v1.0.0.md")
	if result != "v1.0.0" {
		t.Errorf("expected v1.0.0 (lastIndexOf finds last dot at index 7), got %s", result)
	}
}

func TestMdModuleName_NoExtension(t *testing.T) {
	result := mdModuleName("README")
	if result != "README" {
		t.Errorf("expected README for no extension, got %s", result)
	}
}

func TestMdModuleName_WindowsBackslashPath(t *testing.T) {
	result := mdModuleName("docs\\guide.md")
	if result != "guide" {
		t.Errorf("expected guide for Windows path, got %s", result)
	}
}

func TestMdModuleName_HiddenFile(t *testing.T) {
	result := mdModuleName(".hidden.md")
	if result != ".hidden" {
		t.Errorf("expected .hidden (last dot at index 7 > 0), got %s", result)
	}
}

func TestMdModuleName_DeepNestedPath(t *testing.T) {
	result := mdModuleName("a/b/c/deep/file.md")
	if result != "file" {
		t.Errorf("expected file for deep nested path, got %s", result)
	}
}

func TestMdModuleName_JustExtension(t *testing.T) {
	// ".md" — dot at index 0, dot > 0 check fails, so no stripping
	result := mdModuleName(".md")
	if result != ".md" {
		t.Errorf("expected .md (dot at pos 0, not stripped), got %s", result)
	}
}

// ============ mdExtractHeadingText Tests ============

func TestMdExtractHeadingText_SimpleHeading(t *testing.T) {
	result := mdExtractHeadingText("# Hello World")
	if result != "Hello World" {
		t.Errorf("expected Hello World, got %s", result)
	}
}

func TestMdExtractHeadingText_MultipleLevels(t *testing.T) {
	result := mdExtractHeadingText("### Subsection")
	if result != "Subsection" {
		t.Errorf("expected Subsection, got %s", result)
	}
}

func TestMdExtractHeadingText_TrailingNewline(t *testing.T) {
	result := mdExtractHeadingText("## Title\n")
	if result != "Title" {
		t.Errorf("expected Title, got %s", result)
	}
}

func TestMdExtractHeadingText_TrailingSpaces(t *testing.T) {
	result := mdExtractHeadingText("## Title   ")
	if result != "Title" {
		t.Errorf("expected Title, got %s", result)
	}
}

func TestMdExtractHeadingText_EmptyString(t *testing.T) {
	result := mdExtractHeadingText("")
	if result != "" {
		t.Errorf("expected empty string, got %s", result)
	}
}

func TestMdExtractHeadingText_JustHashes(t *testing.T) {
	result := mdExtractHeadingText("###")
	if result != "" {
		t.Errorf("expected empty for just hashes, got %s", result)
	}
}

func TestMdExtractHeadingText_NoHashes(t *testing.T) {
	result := mdExtractHeadingText("Plain text")
	if result != "Plain text" {
		t.Errorf("expected Plain text, got %s", result)
	}
}

func TestMdExtractHeadingText_TabAfterHash(t *testing.T) {
	result := mdExtractHeadingText("#\tHeading")
	if result != "Heading" {
		t.Errorf("expected Heading, got %s", result)
	}
}

func TestMdExtractHeadingText_TrailingCR(t *testing.T) {
	result := mdExtractHeadingText("# Title\r")
	if result != "Title" {
		t.Errorf("expected Title, got %s", result)
	}
}

func TestMdExtractHeadingText_SpacesAndHashesMixed(t *testing.T) {
	result := mdExtractHeadingText("# # Nested Heading")
	if result != "Nested Heading" {
		t.Errorf("expected Nested Heading, got %s", result)
	}
}