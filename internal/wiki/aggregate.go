package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// generateCodeAggregatePages 扫描所有 code source 页，按业务域和技术层级分组，生成模块聚合索引页
func (e *Engine) generateCodeAggregatePages() {
	pages, err := e.Storage.ListPages("sources")
	if err != nil {
		return
	}

	// 收集所有代码页面的分类信息
	type codePage struct {
		slug    string // 文件名（不含 .md）
		title   string // 来源页标题
		layer   string
		domain  string
		kind    string
	}

	// domain → layer → []page
	byDomain := make(map[string]map[string][]codePage)
	byLayer := make(map[string]map[string][]codePage)
	noDomain := make([]codePage, 0)

	for _, pagePath := range pages {
		// 校验路径在安全范围内（解析符号链接防止绕过）
		safePath, evalErr := filepath.EvalSymlinks(pagePath)
		if evalErr != nil {
			continue
		}
		rootReal, rootErr := filepath.EvalSymlinks(e.Storage.Root)
		if rootErr != nil {
			continue
		}
		if !strings.HasPrefix(filepath.Clean(safePath), rootReal) {
			continue
		}

		base := filepath.Base(safePath)
		slug := strings.TrimSuffix(base, ".md")
		if !strings.HasPrefix(slug, "code-") {
			continue
		}

		data, err := os.ReadFile(safePath)
		if err != nil {
			continue
		}
		body := string(data)

		// 解析 frontmatter 中的分类字段
		layer := extractFMField(body, "layer")
		domain := extractFMField(body, "domain")
		kind := extractFMField(body, "kind")
		title := extractFMField(body, "title")
		if title == "" {
			title = slug
		}

		cp := codePage{slug: slug, title: title, layer: layer, domain: domain, kind: kind}

		if layer != "" {
			if byLayer[layer] == nil {
				byLayer[layer] = make(map[string][]codePage)
			}
			label := codeLayerLabel(layer)
			byLayer[layer][label] = append(byLayer[layer][label], cp)
		}

		if domain != "" {
			label := codeDomainLabel(domain)
			if byDomain[domain] == nil {
				byDomain[domain] = make(map[string][]codePage)
			}
			byDomain[domain][label] = append(byDomain[domain][label], cp)
		} else {
			noDomain = append(noDomain, cp)
		}
	}

	if len(byDomain) == 0 && len(byLayer) == 0 {
		return
	}

	// 生成聚合页
	now := time.Now().Format("2006-01-02")
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("---\ntitle: \"代码模块索引\"\ntype: aggregate\nstatus: published\ncreated: %s\nupdated: %s\nconfidence: high\ntags: [代码, 索引]\n---\n\n", now, now))
	sb.WriteString("# 代码模块索引\n\n")
	sb.WriteString(fmt.Sprintf("> 自动生成于 %s | 按业务域和技术层级组织所有代码文件\n\n", now))

	// 按业务域分组
	sb.WriteString("## 按业务域浏览\n\n")
	domains := sortedKeys(byDomain)
	for _, domain := range domains {
		layers := byDomain[domain]
		sb.WriteString(fmt.Sprintf("### %s\n\n", domain))

		layerKeys := sortedKeys(layers)
		for _, lk := range layerKeys {
			pages := layers[lk]
			sb.WriteString(fmt.Sprintf("**%s**\n\n", lk))
			sort.Slice(pages, func(i, j int) bool { return pages[i].slug < pages[j].slug })
			for _, p := range pages {
				sb.WriteString(fmt.Sprintf("- [%s](%s.md)\n", p.title, p.slug))
			}
			sb.WriteString("\n")
		}
	}

	// 按技术层级分组
	sb.WriteString("## 按技术层级浏览\n\n")
	layerKeys := sortedKeys(byLayer)
	for _, lk := range layerKeys {
		labels := byLayer[lk]
		sb.WriteString(fmt.Sprintf("### %s\n\n", lk))

		labelKeys := sortedKeys(labels)
		for _, label := range labelKeys {
			pages := labels[label]
			sort.Slice(pages, func(i, j int) bool { return pages[i].slug < pages[j].slug })
			for _, p := range pages {
				sb.WriteString(fmt.Sprintf("- [%s](%s.md)  `%s`\n", p.title, p.slug, p.domain))
			}
		}
		sb.WriteString("\n")
	}

	// 未分类文件
	if len(noDomain) > 0 {
		sb.WriteString("## 未分类\n\n")
		sort.Slice(noDomain, func(i, j int) bool { return noDomain[i].slug < noDomain[j].slug })
		for _, p := range noDomain {
			sb.WriteString(fmt.Sprintf("- [%s](%s.md)\n", p.title, p.slug))
		}
		sb.WriteString("\n")
	}

	if err := e.Storage.WritePage("syntheses", "代码模块索引", sb.String()); err != nil {
		fmt.Printf("[WARN] 写入代码模块索引页失败: %v\n", err)
	}
}

// extractFMField 从 Markdown frontmatter 中提取指定字段的值
func extractFMField(body, field string) string {
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "---") {
		return ""
	}
	parts := strings.SplitN(body[3:], "---", 2)
	if len(parts) < 2 {
		return ""
	}
	for _, line := range strings.Split(parts[0], "\n") {
		line = strings.TrimSpace(line)
		kv := strings.SplitN(line, ":", 2)
		if len(kv) == 2 && strings.TrimSpace(kv[0]) == field {
			return strings.Trim(strings.TrimSpace(kv[1]), "\"'")
		}
	}
	return ""
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
