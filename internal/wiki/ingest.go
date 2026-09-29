package wiki

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"repowiki/internal/llm"
	"repowiki/internal/storage"
)

// IngestAll 并发导入目录下所有的 .md 文档和代码文件

func (e *Engine) IngestAll(ctx context.Context, dirPath string) (int, []error) {
	var files []string
	walkErr := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		name := info.Name()
		if info.IsDir() {
		// 跳过隐藏目录和噪声目录
		if strings.HasPrefix(name, ".") && name != "." {
			return filepath.SkipDir
		}
		if noiseDirNames[name] {
			return filepath.SkipDir
		}
			return nil
		}

		// 跳过噪声代码文件（测试、生成代码、样式文件等）
		baseName := strings.TrimSuffix(name, filepath.Ext(name))
		if isNoiseCodeFile(name, baseName) {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(name))
		supported := map[string]bool{
			".md": true, ".go": true, ".py": true,
			".js": true, ".ts": true, ".vue": true,
			".java": true, ".rs": true,
		}
		if supported[ext] {
			files = append(files, path)
		}
		return nil
	})

	if walkErr != nil {
		return 0, []error{fmt.Errorf("扫描目录失败: %w", walkErr)}
	}

	if len(files) == 0 {
		return 0, []error{fmt.Errorf("目录 %s 中没有支持的文件类型（.md/.go/.py/.js/.ts/.vue/.java/.rs）", dirPath)}
	}

	fmt.Printf("共发现 %d 个文件，开始导入（最多 3 并发）...\n", len(files))

	e.skipIndexRegen = true
	// 使用独立 defer 确保 skipIndexRegen 始终被复位（即使后续操作 panic）
	defer func() { e.skipIndexRegen = false }()
	defer func() {
		e.regenerateIndex()
		e.generateCodeAggregatePages()
		e.invalidateCaches()
	}()

	// Maximum 3 concurrent requests
	sem := make(chan struct{}, 3)

	var wg sync.WaitGroup
	var mu sync.Mutex
	var allErrs []error
	successCount := 0

	for _, f := range files {
		wg.Add(1)
		go func(filePath string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			var err error
			if storage.IsCodeFile(filePath) {
				err = e.ingestOneCode(ctx, filePath)
			} else {
				err = e.ingestOne(ctx, filePath)
			}

			if err != nil {
				mu.Lock()
				allErrs = append(allErrs, fmt.Errorf("%s: %w", filePath, err))
				mu.Unlock()
			} else {
				mu.Lock()
				successCount++
				mu.Unlock()
			}
		}(f)
	}

	wg.Wait()
	return successCount, allErrs
}

// ingestOne 导入单个文档（含哈希检测和更新逻辑）
func (e *Engine) ingestOne(ctx context.Context, rawPath string) error {
	content, err := os.ReadFile(rawPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	archivedName := archiveSlug(rawPath)
	archivedPath := filepath.Join(e.Root, "raw", archivedName)

	// 去重：如果已归档过则检查内容是否变化
	if _, err := os.ReadFile(archivedPath); err == nil {
		newHash := storage.ComputeHash(string(content))
		oldHash, hashErr := e.Storage.ReadHash(archivedName)

		// 没有旧哈希文件 → 首次迁移，存哈希然后当作已处理跳过
		if hashErr != nil {
			e.Storage.SaveHash(archivedName, newHash)
			fmt.Printf("  [跳过] %s（已有归档，哈希已补录）\n", filepath.Base(rawPath))
			return nil
		}

		if oldHash == newHash {
			fmt.Printf("  [跳过] %s（内容未变）\n", filepath.Base(rawPath))
			return nil
		}

		// 内容变化，走更新流程
		fmt.Printf("  [更新] %s（内容已变化）\n", filepath.Base(rawPath))
		return e.ingestOneUpdate(ctx, rawPath, string(content), archivedName, oldHash)
	}

	// 全新导入
	fmt.Printf("  [处理] %s\n", filepath.Base(rawPath))
	return e.ingestOneNew(ctx, rawPath, string(content), archivedName, archivedPath)
}

// ingestOneNew 全新导入文档
func (e *Engine) ingestOneNew(ctx context.Context, rawPath, content, archivedName, archivedPath string) error {
	// 归档原始文件
	if _, err := e.Storage.ArchiveRawAs(rawPath, archivedName); err != nil {
		return fmt.Errorf("归档失败: %w", err)
	}

	// 计算并保存哈希
	hash := storage.ComputeHash(content)
	e.Storage.SaveHash(archivedName, hash)

	// 提取元数据
	meta, err := e.LLM.ExtractMetadata(ctx, content)
	if err != nil {
		return fmt.Errorf("提取元数据失败: %w", err)
	}

	// 生成并保存源摘要页
	sourceSlug := e.saveSourcePage(rawPath, archivedPath, meta)

	// 规范/指南类文档只保留源摘要页，不拆 entity/concept（避免产生大量空壳页）
	if shouldSkipEntityConcept(rawPath, meta.Category) {
		fmt.Printf("  [文档] %s（跳过 entity/concept，源摘要页已足够）\n", filepath.Base(rawPath))
		e.finalizeIngest(rawPath, "Ingest-Doc", meta.Title, 0, 0)
		return nil
	}

	// 提取并合并实体/概念
	entities, concepts := e.extractAndAccumulate(ctx, content, sourceSlug, meta.KeyFacts)

	// 更新索引和日志
	e.finalizeIngest(rawPath, "Ingest", meta.Title, len(entities), len(concepts))
	return nil
}

// ingestOneUpdate 更新已有文档（快照历史 → 覆盖 → diff合并 → 标记 stale）
func (e *Engine) ingestOneUpdate(ctx context.Context, rawPath, content, archivedName, oldHash string) error {
	ts := time.Now().Format("2006-01-02T15-04-05")

	// 1. 保存历史快照
	oldArchivedContent, _ := e.Storage.ReadArchivedRaw(archivedName)
	if oldArchivedContent != "" {
		e.Storage.SaveHistoryRaw(archivedName, oldArchivedContent, ts)
	}
	e.saveSourceHistory(archivedName, rawPath, ts)

	// 2. 覆盖归档和哈希
	archivedPath := filepath.Join(e.Root, "raw", archivedName)
	os.WriteFile(archivedPath, []byte(content), 0644)
	newHash := storage.ComputeHash(content)
	e.Storage.SaveHash(archivedName, newHash)

	// 3. 提取新版元数据
	newMeta, err := e.LLM.ExtractMetadata(ctx, content)
	if err != nil {
		return fmt.Errorf("提取新版元数据失败: %w", err)
	}

	// 4. 覆盖源摘要页
	sourceSlug := e.saveSourcePage(rawPath, archivedPath, newMeta)

	if shouldSkipEntityConcept(rawPath, newMeta.Category) {
		e.finalizeIngest(rawPath, "Update-Doc", newMeta.Title, 0, 0)
		return nil
	}

	// 5. 提取旧版元数据（用于 diff 对比）
	oldMeta := &llm.DocumentMeta{KeyFacts: []string{}}
	if oldArchivedContent != "" {
		oldMeta, _ = e.LLM.ExtractMetadata(ctx, oldArchivedContent)
	}

	// 6. 提取新版实体/概念
	newEntities, newConcepts := e.extractLists(ctx, content)

	// 7. 提取旧版实体/概念
	var oldEntities, oldConcepts []string
	if oldArchivedContent != "" {
		oldEntities, oldConcepts = e.extractLists(ctx, oldArchivedContent)
	}

	// 8. Diff 合并实体
	entitySet := union(oldEntities, newEntities)
	for _, entity := range entitySet {
		slug := sanitizeSlug(entity)
		if slug == "" {
			continue
		}
		existing, _ := e.Storage.ReadPage("entities", slug)
		oldFacts := factsFor(oldMeta.KeyFacts)
		newFacts := factsFor(newMeta.KeyFacts)
		newContent, err := e.LLM.AccumulateEntityPageDiff(ctx, entity, existing, sourceSlug, oldFacts, newFacts)
		if err != nil {
			fmt.Printf("  ! Diff合并实体 %q 失败: %v\n", entity, err)
			continue
		}
		e.Storage.WritePage("entities", slug, newContent)
	}

	// 9. Diff 合并概念
	conceptSet := union(oldConcepts, newConcepts)
	for _, concept := range conceptSet {
		slug := sanitizeSlug(concept)
		if slug == "" {
			continue
		}
		existing, _ := e.Storage.ReadPage("concepts", slug)
		oldFacts := factsFor(oldMeta.KeyFacts)
		newFacts := factsFor(newMeta.KeyFacts)
		newContent, err := e.LLM.AccumulateConceptPageDiff(ctx, concept, existing, sourceSlug, oldFacts, newFacts)
		if err != nil {
			fmt.Printf("  ! Diff合并概念 %q 失败: %v\n", concept, err)
			continue
		}
		e.Storage.WritePage("concepts", slug, newContent)
	}

	// 10. 标记关联的综合提炼页为 stale
	e.markSynthesesStale(sourceSlug, ts)

	// 11. 索引 + 日志
	e.finalizeIngest(rawPath, "Update", newMeta.Title, len(newEntities), len(newConcepts))
	return nil
}

// ingestOneCode 代码文件专属简化管道：归档 + 生成模块说明页 + 索引 + 日志（不生成实体/概念页）
func (e *Engine) ingestOneCode(ctx context.Context, rawPath string) error {
	content, err := os.ReadFile(rawPath)
	if err != nil {
		return fmt.Errorf("读取文件失败: %w", err)
	}

	archivedName := archiveSlug(rawPath)
	archivedPath := filepath.Join(e.Root, "raw", archivedName)

	// 去重：已归档则检查哈希
	if _, err := os.ReadFile(archivedPath); err == nil {
		newHash := storage.ComputeHash(string(content))
		oldHash, hashErr := e.Storage.ReadHash(archivedName)
		if hashErr == nil && oldHash == newHash {
			fmt.Printf("  [跳过] %s（内容未变）\n", filepath.Base(rawPath))
			return nil
		}
		if hashErr == nil && oldHash != newHash {
			fmt.Printf("  [更新] %s（内容已变化）\n", filepath.Base(rawPath))
			// 更新：覆盖归档和源页
			return e.ingestOneCodeUpdate(ctx, rawPath, string(content), archivedName, archivedPath, oldHash)
		}
		// 无哈希文件 → 补录 + 跳过
		e.Storage.SaveHash(archivedName, newHash)
		fmt.Printf("  [跳过] %s（已有归档，哈希已补录）\n", filepath.Base(rawPath))
		return nil
	}

	fmt.Printf("  [代码] %s\n", filepath.Base(rawPath))

	// 归档
	if _, err := e.Storage.ArchiveRawAs(rawPath, archivedName); err != nil {
		return fmt.Errorf("归档失败: %w", err)
	}

	// 哈希
	hash := storage.ComputeHash(string(content))
	e.Storage.SaveHash(archivedName, hash)

	// 语言检测
	lang := storage.DetectLanguage(filepath.Base(rawPath))

	// 路径规则分类（不用 LLM）
	layer, domain, kind := classifyCodeFile(rawPath)

	// 提取代码元数据
	meta, err := e.LLM.ExtractCodeMeta(ctx, string(content), lang, layer, domain, kind)
	if err != nil {
		return fmt.Errorf("提取代码元数据失败: %w", err)
	}

	// 回填分类到 meta（供模板和聚合页使用）
	meta.Layer = layer
	meta.Domain = domain
	meta.Kind = kind

	// 生成模块说明页
	sourceSlug := e.saveCodeSourcePage(rawPath, archivedPath, meta, string(content), layer, domain, kind)

	// 索引 + 日志
	e.finalizeIngest(rawPath, "Code", meta.ModuleName, 0, 0)

	_ = sourceSlug
	return nil
}

// ingestOneCodeUpdate 代码文件更新
func (e *Engine) ingestOneCodeUpdate(ctx context.Context, rawPath, content, archivedName, archivedPath, oldHash string) error {
	ts := time.Now().Format("2006-01-02T15-04-05")

	// 快照旧版本
	oldContent, _ := e.Storage.ReadArchivedRaw(archivedName)
	if oldContent != "" {
		e.Storage.SaveHistoryRaw(archivedName, oldContent, ts)
	}
	e.saveSourceHistory(archivedName, rawPath, ts)

	// 覆盖
	os.WriteFile(archivedPath, []byte(content), 0644)
	newHash := storage.ComputeHash(content)
	e.Storage.SaveHash(archivedName, newHash)

	// 重新提取
	lang := storage.DetectLanguage(filepath.Base(rawPath))
	layer, domain, kind := classifyCodeFile(rawPath)
	meta, err := e.LLM.ExtractCodeMeta(ctx, content, lang, layer, domain, kind)
	if err != nil {
		return fmt.Errorf("提取代码元数据失败: %w", err)
	}
	meta.Layer = layer
	meta.Domain = domain
	meta.Kind = kind

	sourceSlug := e.saveCodeSourcePage(rawPath, archivedPath, meta, content, layer, domain, kind)
	e.markSynthesesStale(sourceSlug, ts)
	e.finalizeIngest(rawPath, "Code-Update", meta.ModuleName, 0, 0)
	return nil
}

// saveCodeSourcePage 生成并保存代码模块说明页
func (e *Engine) saveCodeSourcePage(rawPath, archivedPath string, meta *llm.CodeMeta, fullContent, layer, domain, kind string) string {
	page := generateCodeSourcePage(rawPath, archivedPath, meta, fullContent, layer, domain, kind)
	sourceSlug := sanitizeSlug(meta.ModuleName)
	if sourceSlug == "" {
		sourceSlug = sanitizeSlug(filepath.Base(rawPath))
	}
	// 加 code- 前缀避免和文档同名冲突
	sourceSlug = "code-" + sourceSlug
	e.Storage.WritePage("sources", sourceSlug, page)
	// 导入后立即索引到搜索引擎
	e.indexSourcePage(sourceSlug)
	return sourceSlug
}

// ---------- 辅助函数 ----------

// extractAndAccumulate 提取实体/概念并累积合并（用于全新导入）
func (e *Engine) extractAndAccumulate(ctx context.Context, content, sourceSlug string, facts []string) (entities, concepts []string) {
	var wg sync.WaitGroup
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
		if err := e.accumulateEntityPage(ctx, entity, sourceSlug, facts); err != nil {
			fmt.Printf("  ! 更新实体页 %q 失败: %v\n", entity, err)
		}
	}
	for _, concept := range concepts {
		if err := e.accumulateConceptPage(ctx, concept, sourceSlug, facts); err != nil {
			fmt.Printf("  ! 更新概念页 %q 失败: %v\n", concept, err)
		}
	}
	return
}

// extractLists 仅提取实体/概念列表（不累积）
func (e *Engine) extractLists(ctx context.Context, content string) (entities, concepts []string) {
	var wg sync.WaitGroup
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
	return
}

// saveSourcePage 保存源摘要页，返回 slug
func (e *Engine) saveSourcePage(rawPath, archivedPath string, meta *llm.DocumentMeta) string {
	sourcePage := generateSourcePage(rawPath, archivedPath, meta)
	sourceSlug := sanitizeSlug(meta.Title)
	if sourceSlug == "" {
		sourceSlug = sanitizeSlug(filepath.Base(rawPath))
	}
	e.Storage.WritePage("sources", sourceSlug, sourcePage)
	// 导入后立即索引到搜索引擎
	e.indexSourcePage(sourceSlug)
	return sourceSlug
}

// saveSourceHistory 备份旧源摘要页到 history
func (e *Engine) saveSourceHistory(archivedName, rawPath, ts string) {
	// 在所有 sources 中查找引用了该原始路径的页面
	sourceFiles, err := e.Storage.ListPages("sources")
	if err != nil {
		return
	}
	for _, sf := range sourceFiles {
		data, err := os.ReadFile(sf)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), rawPath) {
			e.Storage.SaveHistoryRaw("sources/"+filepath.Base(sf), string(data), ts)
			return
		}
	}
}

// markSynthesesStale 标记引用了该源文档的综合提炼页为过期
func (e *Engine) markSynthesesStale(sourceSlug, ts string) {
	synFiles, err := e.Storage.ListPages("syntheses")
	if err != nil {
		return
	}
	for _, sf := range synFiles {
		data, err := os.ReadFile(sf)
		if err != nil {
			continue
		}
		body := string(data)
		// 检查 sources 中是否引用了该源文档
		if strings.Contains(body, sourceSlug) {
			// 在 frontmatter 或末尾追加 stale 标记
			if !strings.Contains(body, "stale:") {
				updated := strings.Replace(body,
					"---\n\n#",
					"stale: true\nstale_since: \""+ts+"\"\n---\n\n#",
					1,
				)
				os.WriteFile(sf, []byte(updated), 0644)
				fmt.Printf("  [stale] 综合提炼页 %s 已标记为过期\n", filepath.Base(sf))
			}
		}
	}
}

// finalizeIngest 重建索引 + 写日志
func (e *Engine) finalizeIngest(rawPath, action, title string, entityCount, conceptCount int) {
	fmt.Printf("  [完成] %s（实体: %d, 概念: %d）\n", filepath.Base(rawPath), entityCount, conceptCount)
	if !e.skipIndexRegen {
		e.regenerateIndex()
	}

	logEntry := fmt.Sprintf(
		"## %s - %s\n\n- 文件: %s\n- 时间: %s\n- 标题: %s\n- 实体: %d\n- 概念: %d",
		action, filepath.Base(rawPath),
		rawPath,
		time.Now().Format("2006-01-02 15:04:05"),
		title, entityCount, conceptCount,
	)
	e.Storage.AppendLog(logEntry)
}

// union 求两个字符串切片的并集
func union(a, b []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range append(a, b...) {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// factsFor 格式化事实列表
func factsFor(facts []string) string {
	if len(facts) == 0 {
		return "（无）"
	}
	return strings.Join(facts, "\n")
}

// ---------- 辅助 ----------

func generateSourcePage(rawPath, archivedPath string, meta *llm.DocumentMeta) string {
	now := time.Now().Format("2006-01-02")
	keywords := strings.Join(meta.Keywords, ", ")
	facts := ""
	for _, f := range meta.KeyFacts {
		facts += "- " + f + "\n"
	}

	return fmt.Sprintf(`---
title: "%s"
type: source
status: published
created: %s
updated: %s
source_file: "%s"
category: "%s"
confidence: high
tags: [%s]
---

# %s

## 摘要

%s

## 关键事实

%s

## 文件

- 原始路径: %s
- 归档路径: %s
`,
		meta.Title, now, now,
		rawPath, meta.Category, keywords,
		meta.Title, meta.Summary,
		facts,
		rawPath, archivedPath,
	)
}

func generateCodeSourcePage(rawPath, archivedPath string, meta *llm.CodeMeta, fullContent, layer, domain, kind string) string {
	now := time.Now().Format("2006-01-02")
	apis := ""
	for _, a := range meta.PublicAPIs {
		apis += "- `" + a + "`\n"
	}
	if apis == "" {
		apis = "（未提取到公开 API）"
	}
	deps := strings.Join(meta.Dependencies, ", ")
	if deps == "" {
		deps = "（未检测到内部依赖）"
	}
	patterns := strings.Join(meta.Patterns, "、")
	if patterns == "" {
		patterns = "（未识别到显著模式）"
	}

	// 取代码前 6000 字符作为参考片段（确保核心函数逻辑被包含）
	codePreview := fullContent
	runes := []rune(codePreview)
	if len(runes) > 6000 {
		codePreview = string(runes[:6000]) + "\n\n...（内容过长，已截断）"
	}

	// 分类标签行：仅当三字段均非空时才写入
	layerLine := ""
	if layer != "" && domain != "" && kind != "" {
		layerLine = fmt.Sprintf("layer: \"%s\"\ndomain: \"%s\"\nkind: \"%s\"\n", layer, domain, kind)
	}

	return fmt.Sprintf(`---
title: "%s（代码）"
type: source
subtype: code
language: "%s"
status: published
created: %s
updated: %s
source_file: "%s"
confidence: medium
tags: [代码, %s]
%s---

# %s

## 模块说明

%s

## 公开 API

%s

## 内部依赖

%s

## 设计模式

%s

## 源代码参考

`+"```"+`%s
%s
`+"```"+`

## 文件信息

- 原始路径: %s
- 归档路径: %s
`,
		meta.ModuleName, meta.Language, now, now,
		rawPath, meta.Language,
		layerLine,
		meta.ModuleName,
		meta.Summary,
		apis,
		deps,
		patterns,
		meta.Language, codePreview,
		rawPath, archivedPath,
	)
}

// archiveSlug 生成归档文件名；index.md/README.md 等同名文件加父目录前缀避免覆盖
func archiveSlug(rawPath string) string {
	base := filepath.Base(rawPath)
	name := strings.TrimSuffix(base, filepath.Ext(base))
	switch strings.ToLower(name) {
	case "index", "readme", "readme.en":
		parent := filepath.Base(filepath.Dir(rawPath))
		if parent != "" && parent != "." {
			name = parent + "-" + name
		}
	}
	return sanitizeSlug(name)
}

// shouldSkipEntityConcept 跳过规范/流程/PRD 类文档的 entity/concept 提取。
// 这类文档通常用源摘要页即可承载，继续拆实体/概念容易制造大量空壳页面。
func shouldSkipEntityConcept(rawPath, category string) bool {
	lower := strings.ToLower(rawPath)

	switch strings.ToLower(strings.TrimSpace(category)) {
	case "guide", "technical", "report", "sop", "standard", "spec", "prd":
		return true
	}

	skipPathParts := []string{
		"/sop",
		"/prd/",
		"/spec/",
		"/standards/",
		"/team-standards/",
		"-standards",
		"standard.md",
	}
	for _, part := range skipPathParts {
		if strings.Contains(lower, part) {
			return true
		}
	}

	return false
}

func sanitizeSlug(s string) string {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer(
		`"`, "", `\`, "-", `/`, "-",
		`:`, "-", `*`, "-", `?`, "",
		`<`, "-", `>`, "-", `|`, "-",
		` `, "-",
	).Replace(s)
	// 限制长度，避免文件名超长
	runes := []rune(s)
	if len(runes) > 80 {
		runes = runes[:80]
	}
	s = string(runes)
	s = strings.TrimRight(s, "-")
	return s
}

// isNoiseCodeFile 跳过测试、生成代码、mock、样式等低价值文件
func isNoiseCodeFile(name, baseName string) bool {
	lowerName := strings.ToLower(name)
	// 测试文件（大小写不敏感）
	if strings.HasSuffix(lowerName, "_test.go") || strings.HasSuffix(lowerName, ".spec.ts") ||
		strings.HasSuffix(lowerName, ".test.js") || strings.HasSuffix(lowerName, ".test.ts") ||
		strings.HasSuffix(lowerName, "_test.py") || strings.HasPrefix(baseName, "test_") {
		return true
	}
	// protobuf 等自动生成文件
	if strings.HasSuffix(name, ".pb.go") || strings.HasSuffix(name, ".pb.gw.go") ||
		strings.HasSuffix(name, ".gen.go") || strings.HasSuffix(name, ".generated.go") {
		return true
	}
	// 样式文件（非 Vue SFC）
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".css", ".scss", ".less", ".sass":
		return true
	}
	// mock 文件
	if strings.HasPrefix(baseName, "mock_") || strings.HasSuffix(baseName, "_mock") ||
		strings.Contains(name, "mock_") || strings.Contains(name, "_mock.") {
		return true
	}
	return false
}
