package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) regenerateIndex() {
	var sb strings.Builder
	sb.WriteString("# Wiki 索引导航\n\n")
	sb.WriteString(fmt.Sprintf("> 最后更新：%s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	type section struct {
		label string
		dir   string
		title string
	}

	sections := []section{
		{"源文档摘要", "sources", "sources"},
		{"实体索引", "entities", "entities"},
		{"概念索引", "concepts", "concepts"},
		{"综合提炼", "syntheses", "syntheses"},
		{"矛盾记录", "contradictions", "contradictions"},
	}

	for _, sec := range sections {
		sb.WriteString(fmt.Sprintf("## %s\n\n", sec.label))
		pages, err := e.Storage.ListPages(sec.dir)
		if err != nil || len(pages) == 0 {
			sb.WriteString("（暂无）\n\n")
			continue
		}
		for _, p := range pages {
			name := strings.TrimSuffix(filepath.Base(p), ".md")
			sb.WriteString(fmt.Sprintf("- [%s](./%s/%s.md)\n", name, sec.title, name))
		}
		sb.WriteString("\n")
	}

	indexPath := filepath.Join(e.Root, "wiki", "index.md")
	os.WriteFile(indexPath, []byte(sb.String()), 0644)
}
