package wiki

import (
	"os"
	"path/filepath"
	"testing"

	"repowiki/internal/storage"
)

func TestInMemorySearchRebuildReplacesExistingIndex(t *testing.T) {
	search := NewInMemorySearch()
	search.Index("old.md", []storage.Chunk{{
		Source:  "old.md",
		Title:   "Old",
		Content: "旧订单状态",
	}})

	err := search.Rebuild(
		func() ([]string, error) {
			return []string{"new.md"}, nil
		},
		func(page string) ([]storage.Chunk, error) {
			return []storage.Chunk{{
				Source:  page,
				Title:   "New",
				Content: "新配送回调",
			}}, nil
		},
	)
	if err != nil {
		t.Fatalf("rebuild failed: %v", err)
	}

	if got := search.Search([]string{"订单"}, 10); len(got) != 0 {
		t.Fatalf("expected old index to be replaced, got %v", got)
	}

	got := search.Search([]string{"配送"}, 10)
	if len(got) != 1 || got[0].source != "new.md" {
		t.Fatalf("expected rebuilt index to contain new.md, got %v", got)
	}
}

func TestInMemorySearchIndexReplacesDocumentTerms(t *testing.T) {
	search := NewInMemorySearch()
	search.Index("page.md", []storage.Chunk{{
		Source:  "page.md",
		Title:   "Page",
		Content: "订单状态",
	}})
	search.Index("page.md", []storage.Chunk{{
		Source:  "page.md",
		Title:   "Page",
		Content: "配送回调",
	}})

	if got := search.Search([]string{"订单"}, 10); len(got) != 0 {
		t.Fatalf("expected old document terms to be removed, got %v", got)
	}

	got := search.Search([]string{"配送"}, 10)
	if len(got) != 1 || got[0].source != "page.md" {
		t.Fatalf("expected updated document to be searchable, got %v", got)
	}
}

func TestRootEngineInitSearcherIndexesProjectsAndSubProjects(t *testing.T) {
	root := t.TempDir()
	fs, err := storage.NewFileSystem(root)
	if err != nil {
		t.Fatalf("NewFileSystem failed: %v", err)
	}
	engine := NewEngine(nil, root, fs)

	projectSourceDir := filepath.Join(root, "projects", "demo-project", "wiki", "sources")
	subSourceDir := filepath.Join(root, "projects", "demo-project", "sub-projects", "demo-service", "wiki", "sources")
	if err := os.MkdirAll(projectSourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(subSourceDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectSourceDir, "规范.md"), []byte("# 规范\n\n## 分层架构\n\nGolang 分层架构规范"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(subSourceDir, "订单.md"), []byte("# 订单\n\n## 回调\n\n配送订单状态回调"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := engine.InitSearcher(); err != nil {
		t.Fatalf("InitSearcher failed: %v", err)
	}

	if got := engine.Searcher.Search([]string{"分层"}, 10); len(got) == 0 {
		t.Fatal("expected root engine to index project-level source pages")
	}
	if got := engine.Searcher.Search([]string{"配送"}, 10); len(got) == 0 {
		t.Fatal("expected root engine to index sub-project source pages")
	}
}
