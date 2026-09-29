package wiki

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type LintIssue struct {
	Path       string `json:"path"`
	Category   string `json:"category"`
	Name       string `json:"name"`
	Issue      string `json:"issue"`
	Severity   string `json:"severity"` // "error", "warning", "info"
	Size       int64  `json:"size"`
	Project    string `json:"project,omitempty"`    // 所属项目（空=全局）
	SubProject string `json:"sub_project,omitempty"` // 所属子项目（空=项目级或全局）
}

type LintReport struct {
	TotalPages  int         `json:"total_pages"`
	Issues      []LintIssue `json:"issues"`
	ErrorCount  int         `json:"error_count"`
	WarnCount   int         `json:"warn_count"`
	InfoCount   int         `json:"info_count"`
	CleanCount  int         `json:"clean_count"`
}

// lintAll 扫描所有 wiki 页面（全局 + 所有项目），返回质量问题报告
func (e *Engine) lintAll() *LintReport {
	report := &LintReport{Issues: []LintIssue{}}

	categories := []string{"sources", "entities", "concepts", "syntheses", "contradictions"}
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 扫描函数：对指定 wikiDir + rootDir 做分类扫描
	scanDir := func(wikiDir, rootDir, project, subProject string) {
		var localPages int
		for _, cat := range categories {
			wg.Add(1)
			go func(category string) {
				defer wg.Done()
				pageCount, issues := lintCategory(wikiDir, category, rootDir, project, subProject)
				mu.Lock()
				report.Issues = append(report.Issues, issues...)
				localPages += pageCount
				mu.Unlock()
			}(cat)
		}
		wg.Wait()
		mu.Lock()
		report.TotalPages += localPages
		staleSources := e.lintStaleSources(wikiDir, rootDir, project, subProject)
		staleSyns := e.lintStaleSyntheses(wikiDir, project, subProject)
		report.Issues = append(report.Issues, staleSources...)
		report.Issues = append(report.Issues, staleSyns...)
		report.TotalPages += len(staleSources) + len(staleSyns)
		mu.Unlock()
	}

	// 1. 扫描全局 wiki（project="" 表示全局）
	scanDir(filepath.Join(e.Root, "wiki"), e.Root, "", "")

	// 2. 扫描所有项目的 wiki + sub-projects
	projects, _ := e.Storage.ListProjects()
	for _, proj := range projects {
		projRoot := filepath.Join(e.Root, "projects", proj)

		// 2a. 项目级 wiki（兼容旧数据）
		projWikiDir := filepath.Join(projRoot, "wiki")
		if _, err := os.Stat(projWikiDir); err == nil {
			scanDir(projWikiDir, projRoot, proj, "")
		}

		// 2b. 子项目级 wiki（新数据结构）
		subFS, subErr := e.Storage.Sub(proj)
		if subErr != nil {
			continue
		}
		subNames, _ := subFS.ListSubProjects()
		for _, subName := range subNames {
			subRoot := filepath.Join(projRoot, "sub-projects", subName)
			subWikiDir := filepath.Join(subRoot, "wiki")
			if _, err := os.Stat(subWikiDir); err == nil {
				scanDir(subWikiDir, subRoot, proj, subName)
			}
		}
	}

	// 统计
	for _, issue := range report.Issues {
		switch issue.Severity {
		case "error":
			report.ErrorCount++
		case "warning":
			report.WarnCount++
		case "info":
			report.InfoCount++
		}
	}
	report.CleanCount = report.TotalPages - report.ErrorCount - report.WarnCount - report.InfoCount

	return report
}

// lintStaleSources 检测归档文件哈希不一致的过期源文档
func (e *Engine) lintStaleSources(wikiDir, rootDir, project, subProject string) []LintIssue {
	var issues []LintIssue
	rawDir := filepath.Join(rootDir, "raw")
	sourcesDir := filepath.Join(wikiDir, "sources") // 对应的 wiki sources 目录

	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return issues
	}

	for _, entry := range entries {
		if entry.IsDir() || strings.HasSuffix(entry.Name(), ".hash") {
			continue
		}
		archivedPath := filepath.Join(rawDir, entry.Name())
		rawName := entry.Name()

		data, err := os.ReadFile(archivedPath)
		if err != nil {
			continue
		}
		currentHash := computeHashString(string(data))

		// 查找 wiki/sources/ 中对应的实际文件名（可能有子项目后缀）
		wikiName := resolveWikiSourceName(sourcesDir, rawName)

		// 从 raw/ 同目录读 .hash 文件
		hashPath := filepath.Join(rawDir, rawName+".hash")
		savedData, err := os.ReadFile(hashPath)
		savedHash := ""
		if err == nil {
			savedHash = strings.TrimSpace(string(savedData))
		}
		if savedHash == "" {
			issues = append(issues, LintIssue{
				Path:       archivedPath,
				Category:   "sources",
				Name:       wikiName, // 使用 wiki 中的实际文件名
				Issue:      "归档文件缺少哈希记录",
				Severity:   "info",
				Size:       int64(len(data)),
				Project:    project,
				SubProject: subProject,
			})
			continue
		}

		if currentHash != savedHash {
			info, _ := entry.Info()
			var size int64
			if info != nil {
				size = info.Size()
			}
			issues = append(issues, LintIssue{
				Path:       archivedPath,
				Category:   "sources",
				Name:       wikiName, // 使用 wiki 中的实际文件名
				Issue:      "源文档已修改但知识库未同步更新",
				Severity:   "info",
				Size:       size,
				Project:    project,
				SubProject: subProject,
			})
		}
	}

	return issues
}

// resolveWikiSourceName 在 wiki/sources/ 目录中查找与 raw 文件对应的 wiki 页面名称
// 子项目场景下，raw 文件名是原始的（如 App.vue），但 wiki 文件名有各种后缀格式：
//   - {原名}-—-{sub}.md        (yl-delivery-service 风格)
//   - code-{原名}---{描述}.md   (含 code- 前缀和中文描述)
// 此函数返回能被 DeletePage 正确处理的文件名（优先返回 .md 文件）
func resolveWikiSourceName(sourcesDir, rawName string) string {
	// 精确匹配：wiki 中存在同名 .md 文件
	if strings.HasSuffix(rawName, ".md") {
		exactPath := filepath.Join(sourcesDir, rawName)
		if info, err := os.Stat(exactPath); err == nil && !info.IsDir() {
			return rawName
		}
	}

	entries, err := os.ReadDir(sourcesDir)
	if err != nil {
		return ensureMdSuffix(rawName)
	}

	// 策略1：前缀匹配 {原名}-—-
	prefixEm := rawName + "-—-"
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		en := entry.Name()
		// 前缀匹配优先
		if strings.HasPrefix(en, prefixEm) {
			return en
		}
		// 包含匹配：文件名中包含 "-{rawName}-" 或 "{rawName}."
		if strings.Contains(en, "-"+rawName+"-") || strings.Contains(en, rawName+".") {
			return en
		}
	}

	return ensureMdSuffix(rawName)
}

// ensureMdSuffix 如果名字不以 .md 结尾则加上
func ensureMdSuffix(name string) string {
	if !strings.HasSuffix(name, ".md") {
		return name + ".md"
	}
	return name
}

// lintStaleSyntheses 检测标记为 stale 的综合提炼页
func (e *Engine) lintStaleSyntheses(wikiDir, project, subProject string) []LintIssue {
	var issues []LintIssue
	synDir := filepath.Join(wikiDir, "syntheses")

	entries, err := os.ReadDir(synDir)
	if err != nil {
		return issues
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		filePath := filepath.Join(synDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		body := string(data)
		if strings.Contains(body, "stale: true") {
			// 尝试提取 stale_since 时间
			since := "未知时间"
			if idx := strings.Index(body, "stale_since:"); idx >= 0 {
				rest := body[idx:]
				end := strings.Index(rest, "\n")
				if end > 0 {
					since = strings.TrimSpace(strings.TrimPrefix(
						strings.Trim(rest[:end], "\"'"), "stale_since:",
					))
					since = strings.Trim(since, " \"'")
				}
			}
			info, _ := entry.Info()
			var size int64
			if info != nil {
				size = info.Size()
			}
			issues = append(issues, LintIssue{
				Path:       filePath,
				Category:   "syntheses",
				Name:       entry.Name(),
				Issue:      "综合提炼引用的来源已更新，建议重新提炼（过期于 " + since + "）",
				Severity:   "info",
				Size:       size,
				Project:    project,
				SubProject: subProject,
			})
		}
	}

	return issues
}

func lintCategory(wikiDir, category, rootDir, project, subProject string) (int, []LintIssue) {
	var issues []LintIssue
	var pageCount int
	dir := filepath.Join(wikiDir, category)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, issues
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			issues = append(issues, LintIssue{
				Path: filePath, Category: category, Name: entry.Name(),
				Issue: fmt.Sprintf("读取失败: %v", err), Severity: "error",
				Project: project, SubProject: subProject,
			})
			continue
		}

		info, _ := entry.Info()
		var size int64
		if info != nil {
			size = info.Size()
		}

		body := string(data)
		pageCount++
		pageIssues := checkPage(body, filePath, category, entry.Name(), size, rootDir, project, subProject)
		issues = append(issues, pageIssues...)
	}

	return pageCount, issues
}

func checkPage(body, path, category, name string, size int64, rootDir, project, subProject string) []LintIssue {
	var issues []LintIssue
	nameNoExt := strings.TrimSuffix(name, ".md")

	// 1. 分离 frontmatter 和正文
	fm, rawContent := splitFrontmatter(body)
	content := strings.TrimSpace(rawContent)

	// 2. 正文去除 frontmatter 后，只含标题行也算空
	onlyTitle := false
	lines := strings.Split(content, "\n")
	// 去除空行和 "# xxx" 标题行后还剩多少
	var realLines []string
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "## ") {
			continue
		}
		realLines = append(realLines, t)
	}
	realContent := strings.TrimSpace(strings.Join(realLines, "\n"))

	hasRealContent := len(realContent) > 0 &&
		realContent != "（待补充）"

	if len(realLines) <= 1 && strings.Contains(rawContent, "（待补充）") {
		onlyTitle = true
	}

	// 3. 单字符名（大概率是幻觉）
	isSingleChar := len([]rune(nameNoExt)) <= 1 && category != "sources"

	// 4. Draft + 低置信度
	isDraft := fm != nil && (fm.Status == "draft" || fm.Status == "")
	isLowConf := fm != nil && (fm.Confidence == "low" || fm.Confidence == "medium")

	// 5. 实体/概念页幻觉检测：名字未出现在任何引用源文档中
	isHallucination := false
	if (category == "entities" || category == "concepts") && fm != nil && len(fm.Sources) > 0 {
		isHallucination = !nameInSources(nameNoExt, fm.Sources, rootDir)
	}

	if isSingleChar {
		issues = append(issues, LintIssue{
			Path: path, Category: category, Name: name,
			Issue:      "单字符名称，疑似 LLM 幻觉产物",
			Severity:   "error",
			Size:       size,
			Project:    project,
			SubProject: subProject,
		})
	}

	if isHallucination {
		issues = append(issues, LintIssue{
			Path: path, Category: category, Name: name,
			Issue:      "疑似 LLM 幻觉产物：页面名称未在任何引用源文档中出现",
			Severity:   "error",
			Size:       size,
			Project:    project,
			SubProject: subProject,
		})
	}

	if !hasRealContent || onlyTitle {
		sev := "warning"
		if size < 400 {
			sev = "error"
		}
		issues = append(issues, LintIssue{
			Path: path, Category: category, Name: name,
			Issue:      "页面无实质内容或仅含占位符",
			Severity:   sev,
			Size:       size,
			Project:    project,
			SubProject: subProject,
		})
	} else if size < 250 {
		issues = append(issues, LintIssue{
			Path: path, Category: category, Name: name,
			Issue:      "页面内容过少，信息不完整",
			Severity:   "warning",
			Size:       size,
			Project:    project,
			SubProject: subProject,
		})
	}

	if isDraft && isLowConf && hasRealContent && size > 250 {
		issues = append(issues, LintIssue{
			Path: path, Category: category, Name: name,
			Issue:      "草稿状态且置信度较低，建议审核后发布",
			Severity:   "info",
			Size:       size,
			Project:    project,
			SubProject: subProject,
		})
	}

	return issues
}

// lintFrontmatter 解析简单的 YAML frontmatter（字符串级别，避免引入 yaml 依赖）
type lintFrontmatter struct {
	Status     string
	Confidence string
	Sources    []string
}

// splitFrontmatter 分离 YAML frontmatter 和正文
func splitFrontmatter(body string) (*lintFrontmatter, string) {
	body = strings.TrimSpace(body)
	if !strings.HasPrefix(body, "---") {
		return nil, body
	}

	parts := strings.SplitN(body[3:], "---", 2)
	if len(parts) < 2 {
		return nil, body
	}

	fm := &lintFrontmatter{}
	inSources := false
	for _, line := range strings.Split(parts[0], "\n") {
		line = strings.TrimSpace(line)
		if kv := strings.SplitN(line, ":", 2); len(kv) == 2 {
			key := strings.TrimSpace(kv[0])
			val := strings.Trim(strings.TrimSpace(kv[1]), "\"'")
			switch key {
			case "status":
				fm.Status = val
			case "confidence":
				fm.Confidence = val
			case "sources":
				inSources = true
				// sources 可能是数组第一项或单值
				if val != "" && val != "[" {
					fm.Sources = append(fm.Sources, val)
				}
			}
		} else if inSources && strings.HasPrefix(line, "- ") {
			val := strings.TrimPrefix(line, "- ")
			val = strings.Trim(val, "\"'")
			if val != "" {
				fm.Sources = append(fm.Sources, val)
			}
		} else if line == "" {
			inSources = false
		}
	}

	return fm, strings.TrimSpace(parts[1])
}

// DeletePage 删除指定页面
// 支持模糊匹配：当精确文件名找不到时，自动查找以该名字为前缀的 .md 文件
// 这解决了子项目场景下 lint 报告中的 name（原始文件名）与 wiki 中实际文件名（带子项目后缀）不一致的问题
func (e *Engine) DeletePage(category, name string) error {
	dir := filepath.Join(e.Root, "wiki", category)
	filePath := filepath.Join(dir, name)

	// 精确匹配
	removeErr := os.Remove(filePath)
	if removeErr == nil {
		// 删除成功，清理索引
		e.cleanupAfterDelete(category, filePath)
		return nil
	}

	// 精确匹配失败 → 尝试前缀匹配（处理子项目后缀场景）
	if !os.IsNotExist(removeErr) {
		return fmt.Errorf("删除失败: %w", removeErr)
	}

	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return fmt.Errorf("删除失败（精确匹配未找到且无法读取目录）: %w", readErr)
	}

	prefix := name + "-—-"
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		en := entry.Name()
		// 前缀匹配（标准格式: {name}-—-{sub}.md）
		if strings.HasPrefix(en, prefix) {
			matchedPath := filepath.Join(dir, en)
			if removeErr := os.Remove(matchedPath); removeErr != nil {
				return fmt.Errorf("删除匹配文件失败: %w", removeErr)
			}
			e.cleanupAfterDelete(category, matchedPath)
			return nil
		}
		// 包含匹配（处理 code-{name}---{描述}.md 等非标准格式）
		if strings.Contains(en, "-"+name+"-") || strings.Contains(en, name+".") {
			matchedPath := filepath.Join(dir, en)
			if removeErr := os.Remove(matchedPath); removeErr != nil {
				return fmt.Errorf("删除匹配文件失败: %w", removeErr)
			}
			e.cleanupAfterDelete(category, matchedPath)
			return nil
		}
	}

	return fmt.Errorf("删除失败: 文件不存在 (%s)", filePath)
}

// cleanupAfterDelete 删除后的清理工作：重建索引、清除缓存、移除搜索条目
func (e *Engine) cleanupAfterDelete(category string, filePath string) {
	e.regenerateIndex()
	e.invalidateCaches()
	if category == "sources" {
		e.Searcher.Remove(filePath)
	}
}

// ReingestOne 重新导入单个已归档文档
func (e *Engine) ReingestOne(archivedName string) error {
	rawDir := filepath.Join(e.Root, "raw")
	entries, err := os.ReadDir(rawDir)
	if err != nil {
		return fmt.Errorf("读取归档目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.Name() == archivedName {
			return e.ingestOneFromArchived(filepath.Join(rawDir, entry.Name()))
		}
	}
	return fmt.Errorf("归档文件中未找到: %s", archivedName)
}

func (e *Engine) ingestOneFromArchived(archivedPath string) error {
	content, err := os.ReadFile(archivedPath)
	if err != nil {
		return fmt.Errorf("读取归档文件失败: %w", err)
	}
	return e.ingestOneFromContent(filepath.Base(archivedPath), string(content))
}

func (e *Engine) ingestOneFromContent(name, content string) error {
	ctx := context.Background()
	meta, err := e.LLM.ExtractMetadata(ctx, content)
	if err != nil {
		return fmt.Errorf("提取元数据失败: %w", err)
	}

	sourceSlug := sanitizeSlug(meta.Title)
	if sourceSlug == "" {
		sourceSlug = sanitizeSlug(name)
	}

	sourcePage := generateSourcePage(name, filepath.Join(e.Root, "raw", name), meta)
	if err := e.Storage.WritePage("sources", sourceSlug, sourcePage); err != nil {
		return fmt.Errorf("保存源摘要页失败: %w", err)
	}

	var wg sync.WaitGroup
	var entities, concepts []string
	var emu, cmu sync.Mutex

	wg.Add(2)
	go func() {
		defer wg.Done()
		ents, _ := e.LLM.ExtractList(ctx, content, "实体（人物、项目、组织等具体事物）")
		emu.Lock()
		entities = ents
		emu.Unlock()
	}()
	go func() {
		defer wg.Done()
		concs, _ := e.LLM.ExtractList(ctx, content, "概念（技术术语、方法论等抽象概念）")
		cmu.Lock()
		concepts = concs
		cmu.Unlock()
	}()
	wg.Wait()

	for _, entity := range entities {
		_ = e.accumulateEntityPage(ctx, entity, sourceSlug, meta.KeyFacts)
	}
	for _, concept := range concepts {
		_ = e.accumulateConceptPage(ctx, concept, sourceSlug, meta.KeyFacts)
	}
	e.regenerateIndex()
	e.invalidateCaches()
	return nil
}

// nameInSources 检查实体/概念名是否出现在引用源文档的归档原文中
func nameInSources(name string, sourcePages []string, rootDir string) bool {
	rawDir := filepath.Join(rootDir, "raw")
	sourcesDir := filepath.Join(rootDir, "wiki", "sources")

	for _, src := range sourcePages {
		src = strings.Trim(src, "\"'")
		if src == "" {
			continue
		}

		// 源页面引用可能是 slug 格式，尝试直接读归档文件
		sourcePath := filepath.Join(sourcesDir, sanitizeSlugLint(src)+".md")
		sourceData, err := os.ReadFile(sourcePath)
		if err != nil {
			continue
		}

		// 解析 frontmatter 中的 source_file 字段找到原始归档名
		sourceBody := string(sourceData)
		rawName := ""
		if idx := strings.Index(sourceBody, "source_file:"); idx >= 0 {
			rest := sourceBody[idx:]
			end := strings.Index(rest, "\n")
			if end > 0 {
				rawPath := strings.TrimSpace(rest[len("source_file:"):end])
				rawPath = strings.Trim(rawPath, "\"'")
				rawName = filepath.Base(rawPath)
			}
		}

		if rawName != "" {
			rawContent, err := os.ReadFile(filepath.Join(rawDir, rawName))
			if err == nil && strings.Contains(string(rawContent), name) {
				return true
			}
		}

		// 兜底：直接在 sources 页面内容中搜索
		if strings.Contains(sourceBody, name) {
			return true
		}
	}

	// 也检查 raw 目录下所有归档文件
	entries, err := os.ReadDir(rawDir)
	if err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), ".hash") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(rawDir, entry.Name()))
			if err == nil && nameMatchesContent(name, string(data)) {
				return true
			}
		}
	}

	return false
}

// nameMatchesContent 概念名与原文匹配（支持 slug 归一化）
func nameMatchesContent(name, content string) bool {
	if strings.Contains(content, name) {
		return true
	}
	// slug 形式互转：master-main-分支 ↔ master/main 分支
	normalized := strings.NewReplacer("-", "", " ", "", "/", "").Replace(strings.ToLower(name))
	contentNorm := strings.NewReplacer("-", "", " ", "", "/", "").Replace(strings.ToLower(content))
	return strings.Contains(contentNorm, normalized)
}

// sanitizeSlugLint 简版 slug 清理（lint 专用，避免循环依赖）
func sanitizeSlugLint(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer(
		"\"", "", "\\", "-", "/", "-",
		":", "-", "*", "-", "?", "",
		"<", "-", ">", "-", "|", "-",
		" ", "-",
	).Replace(s)
	runes := []rune(s)
	if len(runes) > 80 {
		runes = runes[:80]
	}
	return strings.TrimRight(string(runes), "-")
}

// computeHashString 计算字符串的 SHA256
func computeHashString(s string) string {
	h := sha256.New()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum(nil))
}
