package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CleanupStubPages 删除空壳 entity/concept 页（定义区含「待补充」或内容过少）
func (e *Engine) CleanupStubPages(dryRun bool) (int, error) {
	var deleted int
	for _, cat := range []string{"entities", "concepts"} {
		dir := filepath.Join(e.Root, "wiki", cat)
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
				continue
			}
			path := filepath.Join(dir, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if !isStubPage(string(data)) {
				continue
			}
			if dryRun {
				fmt.Printf("  [待删] %s/%s\n", cat, entry.Name())
			} else {
				if err := os.Remove(path); err != nil {
					return deleted, err
				}
				fmt.Printf("  [已删] %s/%s\n", cat, entry.Name())
			}
			deleted++
		}
	}
	if !dryRun && deleted > 0 {
		e.regenerateIndex()
		e.invalidateCaches()
	}
	return deleted, nil
}

func isStubPage(body string) bool {
	fm, content := splitFrontmatter(body)
	if fm != nil && fm.Status == "published" && len(content) > 800 {
		return false
	}
	if strings.Contains(body, "（待补充）") {
		return true
	}
	// 空壳页：去掉 frontmatter 和标题后几乎无内容
	lines := strings.Split(strings.TrimSpace(content), "\n")
	var realLines int
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		realLines++
	}
	return realLines <= 3 && len(body) < 900
}
