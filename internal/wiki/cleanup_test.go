package wiki

import "testing"

func TestShouldSkipEntityConcept(t *testing.T) {
	cases := []struct {
		path     string
		category string
		want     bool
	}{
		{"/Users/hl/ai_wiki/docs/team-standards/git-branch-management/index.md", "guide", true},
		{"/Users/hl/ai_wiki/sop.md", "guide", true},
		{"/Users/hl/ai_wiki/docs/team-standards/core-development-standards.md", "technical", true},
		{"/Users/hl/project/docs/prd/feat-1001.md", "report", true},
		{"/Users/hl/project/src/service/order.js", "", false},
	}
	for _, c := range cases {
		if got := shouldSkipEntityConcept(c.path, c.category); got != c.want {
			t.Errorf("shouldSkipEntityConcept(%q, %q) = %v, want %v", c.path, c.category, got, c.want)
		}
	}
}

func TestArchiveSlug(t *testing.T) {
	got := archiveSlug("/Users/hl/ai_wiki/docs/team-standards/git-branch-management/index.md")
	want := "git-branch-management-index"
	if got != want {
		t.Errorf("archiveSlug = %q, want %q", got, want)
	}
}

func TestIsStubPage(t *testing.T) {
	stub := `---
title: "分支命名规范"
type: concept
status: draft
---

# 分支命名规范

## 定义

（待补充）
`
	if !isStubPage(stub) {
		t.Error("expected stub page to be detected")
	}
}
