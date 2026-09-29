package wiki

import (
	"strings"
	"testing"
)

func TestExtractKeywords_ChineseBigrams(t *testing.T) {
	keywords := extractKeywords("Golang 分层架构是怎样的")
	set := make(map[string]bool)
	for _, kw := range keywords {
		set[kw] = true
	}

	if !set["Golang"] {
		t.Error("expected token Golang in keywords")
	}
	if !set["分层"] {
		t.Error("expected Chinese bigram 分层 in keywords")
	}
	if !set["架构"] {
		t.Error("expected Chinese bigram 架构 in keywords")
	}
}

func TestExtractKeywords_PunctuationTrim(t *testing.T) {
	keywords := extractKeywords("hello, world!")
	set := make(map[string]bool)
	for _, kw := range keywords {
		set[kw] = true
	}
	if !set["hello"] || !set["world"] {
		t.Errorf("expected hello and world, got %v", keywords)
	}
}

func TestBuildContext_TruncatesAtMaxChars(t *testing.T) {
	chunks := []scoredChunk{
		{source: "a.md", title: "T1", content: strings.Repeat("x", 4000), score: 3},
		{source: "b.md", title: "T2", content: strings.Repeat("y", 4000), score: 2},
	}
	ctx := buildContext(chunks, 6000)
	if len(ctx) > 6100 {
		t.Errorf("context length %d exceeds max ~6000", len(ctx))
	}
	if !strings.Contains(ctx, "a.md") {
		t.Error("expected first chunk in context")
	}
}

func TestBuildCitations_DeduplicatesAndTruncates(t *testing.T) {
	chunks := []scoredChunk{
		{source: "/tmp/a.md", title: "Intro", content: strings.Repeat("订单状态 ", 80), score: 4},
		{source: "/tmp/a.md", title: "Intro", content: "duplicate", score: 3},
		{source: "/tmp/b.md", title: "Callback", content: "配送回调处理", score: 2},
	}

	citations := buildCitations(chunks, 8)
	if len(citations) != 2 {
		t.Fatalf("expected 2 deduplicated citations, got %d", len(citations))
	}
	if citations[0].Page != "a.md" || citations[0].Section != "Intro" || citations[0].Score != 4 {
		t.Fatalf("unexpected first citation: %+v", citations[0])
	}
	if !strings.HasSuffix(citations[0].Excerpt, "...") {
		t.Fatalf("expected long excerpt to be truncated, got %q", citations[0].Excerpt)
	}
}

func TestConfidenceForChunks(t *testing.T) {
	if got := confidenceForChunks(nil); got != "low" {
		t.Fatalf("expected low for no chunks, got %s", got)
	}
	if got := confidenceForChunks([]scoredChunk{{score: 1}}); got != "low" {
		t.Fatalf("expected low for weak match, got %s", got)
	}
	if got := confidenceForChunks([]scoredChunk{{score: 2}}); got != "medium" {
		t.Fatalf("expected medium for score 2, got %s", got)
	}
	if got := confidenceForChunks([]scoredChunk{{score: 4}, {score: 1}, {score: 1}}); got != "high" {
		t.Fatalf("expected high for strong multi-chunk match, got %s", got)
	}
}
