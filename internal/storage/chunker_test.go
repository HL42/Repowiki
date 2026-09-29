package storage

import (
	"strings"
	"testing"
)

func TestChunkMarkdown_ByHeading(t *testing.T) {
	content := "# Title\n\nintro\n\n## Section A\n\ncontent A\n\n## Section B\n\ncontent B"
	chunks := ChunkMarkdown(content, "test.md", 3000)

	if len(chunks) != 3 {
		t.Fatalf("expected 3 chunks, got %d", len(chunks))
	}
	if chunks[1].Title != "Section A" {
		t.Errorf("chunk 1 title = %q, want Section A", chunks[1].Title)
	}
}

func TestChunkMarkdown_SplitOnMaxChars(t *testing.T) {
	longLine := strings.Repeat("x", 2000)
	content := "## Big\n\n" + longLine + "\n" + longLine
	chunks := ChunkMarkdown(content, "big.md", 3000)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks for oversized section, got %d", len(chunks))
	}
}

func TestDetectLanguage(t *testing.T) {
	cases := map[string]string{
		"main.go":    "go",
		"app.py":     "python",
		"index.js":   "javascript",
		"App.vue":    "vue",
		"readme.md":  "markdown",
		"unknown.txt": "",
	}
	for file, want := range cases {
		if got := DetectLanguage(file); got != want {
			t.Errorf("DetectLanguage(%q) = %q, want %q", file, got, want)
		}
	}
}

func TestIsCodeFile(t *testing.T) {
	if !IsCodeFile("main.go") {
		t.Error("main.go should be a code file")
	}
	if IsCodeFile("readme.md") {
		t.Error("readme.md should not be a code file")
	}
}
