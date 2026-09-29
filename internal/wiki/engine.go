package wiki

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"repowiki/internal/llm"
	"repowiki/internal/storage"
)

// resolveSearcher 根据 MEILISEARCH_URL 环境变量选择合适的搜索引擎。
// 如果设置了环境变量 → Meilisearch（持久化、BM25、中文分词）
// 否则 → InMemorySearch（纯内存倒排索引，零依赖）
func resolveSearcher(indexUID string) SearchEngine {
	if os.Getenv("MEILISEARCH_URL") != "" {
		return NewMeilisearchSearch(indexUID)
	}
	return NewInMemorySearch()
}

type chunkCacheEntry struct {
	modTime time.Time
	size    int64
	chunks  []storage.Chunk
}

type Engine struct {
	LLM     *llm.DeepSeekClient
	Storage *storage.FileSystem
	Root    string

	Searcher SearchEngine // 搜索引擎（倒排索引，零依赖，毫秒级查询）

	lintMu        sync.Mutex
	lintCache     *LintReport
	lintCacheTime time.Time

	chunkMu    sync.RWMutex
	chunkCache map[string]chunkCacheEntry

	skipIndexRegen bool

	// 项目/子项目引擎缓存（避免每次请求重复建索引）
	engineMu              sync.RWMutex
	projectEngineCache    map[string]*Engine // project name → cached engine
	subProjectEngineCache map[string]*Engine // "project/sub" → cached sub-engine
}

func NewEngine(llmClient *llm.DeepSeekClient, wikiRoot string, fs *storage.FileSystem) *Engine {
	return &Engine{
		LLM:                   llmClient,
		Storage:               fs,
		Root:                  wikiRoot,
		Searcher:              resolveSearcher("wiki"),
		projectEngineCache:    make(map[string]*Engine),
		subProjectEngineCache: make(map[string]*Engine),
	}
}

// InitSearcher 启动时从存储全量建索引（阻塞，通常在服务启动前调用一次）
// 项目级引擎会聚合该项目下所有子项目的源文档一起建索引
func (e *Engine) InitSearcher() error {
	pages, listErr := e.listAllSourcePages()
	if listErr != nil {
		return listErr
	}
	fmt.Printf("索引构建：发现 %d 个源页面\n", len(pages))
	return e.Searcher.Rebuild(
		func() ([]string, error) { return pages, nil },
		func(path string) ([]storage.Chunk, error) { return e.getChunks(path) },
	)
}

// listAllSourcePages 列出当前引擎范围内的所有源页面。
// 根引擎会聚合 projects/ 下所有项目与子项目；项目引擎会聚合本项目与子项目；子项目引擎只读取自身。
func (e *Engine) listAllSourcePages() ([]string, error) {
	var all []string
	// 1. 当前 Root 下的 sources（兼容旧数据）
	own, _ := e.Storage.ListPages("sources")
	all = append(all, own...)

	// 2. 如果当前是根引擎，扫描所有项目和子项目，保证默认问答不落空
	projectNames, _ := e.Storage.ListProjects()
	for _, projectName := range projectNames {
		projectFS, subErr := e.Storage.Sub(projectName)
		if subErr != nil {
			fmt.Printf("[WARN] listAllSourcePages: 跳过项目 %q: %v\n", projectName, subErr)
			continue
		}
		projectPages, _ := projectFS.ListPages("sources")
		all = append(all, projectPages...)

		subNames, _ := projectFS.ListSubProjects()
		for _, subName := range subNames {
			subFS, spErr := projectFS.SubProject(subName)
			if spErr != nil {
				fmt.Printf("[WARN] listAllSourcePages: 跳过子项目 %q/%q: %v\n", projectName, subName, spErr)
				continue
			}
			subPages, _ := subFS.ListPages("sources")
			all = append(all, subPages...)
		}
	}

	// 3. 如果当前是项目引擎，扫描子项目
	subNames, _ := e.Storage.ListSubProjects()
	for _, subName := range subNames {
		subFS, spErr := e.Storage.SubProject(subName)
		if spErr != nil {
			fmt.Printf("[WARN] listAllSourcePages: 跳过子项目 %q: %v\n", subName, spErr)
			continue
		}
		pages, _ := subFS.ListPages("sources")
		all = append(all, pages...)
	}
	return all, nil
}

// indexSourcePage 索引单个源页面（导入/更新后调用）
func (e *Engine) indexSourcePage(slug string) {
	sourceDir := filepath.Join(e.Root, "wiki", "sources")
	path := filepath.Join(sourceDir, slug+".md")
	chunks, err := e.getChunks(path)
	if err != nil {
		return
	}
	e.Searcher.Index(path, chunks)
}

// LintIfStale 如果缓存超过 maxAge 则重新扫描，返回最新的 lint 报告
func (e *Engine) LintIfStale(maxAge time.Duration) *LintReport {
	e.lintMu.Lock()
	defer e.lintMu.Unlock()

	if e.lintCache != nil && time.Since(e.lintCacheTime) < maxAge {
		return e.lintCache
	}

	e.lintCache = e.lintAll()
	e.lintCacheTime = time.Now()
	return e.lintCache
}

// RefreshLint 强制重新扫描
func (e *Engine) RefreshLint() *LintReport {
	e.lintMu.Lock()
	defer e.lintMu.Unlock()

	e.lintCache = e.lintAll()
	e.lintCacheTime = time.Now()
	return e.lintCache
}

// ---------- 项目管理 ----------

// WithProject 返回项目级引擎（带缓存，避免每次请求重复建索引）
// 继承 LLM，使用项目级存储和独立搜索引擎
func (e *Engine) WithProject(project string) *Engine {
	// 1. 先查缓存（读锁，无阻塞）
	e.engineMu.RLock()
	cached, ok := e.projectEngineCache[project]
	e.engineMu.RUnlock()
	if ok {
		return cached
	}

	// 2. 缓存未命中 → 创建引擎（写锁）
	e.engineMu.Lock()
	defer e.engineMu.Unlock()

	// 双重检查：等待锁期间可能已被其他 goroutine 创建
	if cached, ok = e.projectEngineCache[project]; ok {
		return cached
	}

	projectRoot := filepath.Join(e.Root, "projects", project)
	fs, fsErr := e.Storage.Sub(project)
	if fsErr != nil {
		// 创建目录失败时降级为只读模式：尝试不创建目录直接构造 FileSystem
		fs = &storage.FileSystem{Root: projectRoot}
	}
	projEngine := &Engine{
		LLM:                   e.LLM,
		Storage:               fs,
		Root:                  projectRoot,
		Searcher:              resolveSearcher("wiki_prj_" + sanitizeIndexUID(project)),
		projectEngineCache:    make(map[string]*Engine),
		subProjectEngineCache: make(map[string]*Engine),
	}
	projEngine.InitSearcher() // 项目级引擎启动时建索引（聚合所有子项目）

	// 存入缓存
	e.projectEngineCache[project] = projEngine
	return projEngine
}

// WithSubProject 返回子项目级引擎（带缓存）
// 用于同一大项目下隔离不同子项目的 raw/wiki 数据
// 注意：此方法必须在 WithProject() 之后调用（因为子项目路径是相对项目的）
func (e *Engine) WithSubProject(subProject string) *Engine {
	cacheKey := subProject // 在项目引擎内部，key 就是子项目名（已限定在项目范围内）

	// 1. 查缓存（读锁）
	e.engineMu.RLock()
	cached, ok := e.subProjectEngineCache[cacheKey]
	e.engineMu.RUnlock()
	if ok {
		return cached
	}

	// 2. 缓存未命中 → 创建
	e.engineMu.Lock()
	defer e.engineMu.Unlock()

	if cached, ok = e.subProjectEngineCache[cacheKey]; ok {
		return cached
	}

	subRoot := filepath.Join(e.Root, "sub-projects", subProject)
	fs, fsErr := e.Storage.SubProject(subProject)
	if fsErr != nil {
		fs = &storage.FileSystem{Root: subRoot}
	}
	// 从 Root 提取父项目名，构造 Meilisearch 索引名
	parentProject := filepath.Base(e.Root)
	if parentProject == "." || parentProject == "" || parentProject == "/" {
		parentProject = "unknown"
	}
	indexName := "wiki_prj_" + sanitizeIndexUID(parentProject) + "_sub_" + sanitizeIndexUID(subProject)
	subEngine := &Engine{
		LLM:                   e.LLM,
		Storage:               fs,
		Root:                  subRoot,
		Searcher:              resolveSearcher(indexName),
		projectEngineCache:    make(map[string]*Engine),
		subProjectEngineCache: make(map[string]*Engine),
	}
	subEngine.InitSearcher()

	e.subProjectEngineCache[cacheKey] = subEngine
	return subEngine
}

// CreateProject 创建新项目（生成目录结构和 project.md）
func (e *Engine) CreateProject(name, description, subProjects string) error {
	fs, err := e.Storage.Sub(name)
	if err != nil {
		return fmt.Errorf("创建项目目录失败: %w", err)
	}
	fs.InitWiki()
	// 生成项目简介页
	page := fmt.Sprintf(`---
title: "%s"
type: project
status: published
created: %s
---
# %s

## 项目简介

%s

## 子项目结构

%s
`, name, time.Now().Format("2006-01-02"), name, description, subProjects)
	os.MkdirAll(filepath.Join(e.Root, "projects", name), 0755)
	return os.WriteFile(
		filepath.Join(e.Root, "projects", name, "project.md"),
		[]byte(page), 0644,
	)
}

// GetProjectInfo 获取项目信息及统计（聚合所有子项目）
func (e *Engine) GetProjectInfo(name string) (*ProjectInfo, error) {
	projectFile := filepath.Join(e.Root, "projects", name, "project.md")
	data, err := os.ReadFile(projectFile)
	if err != nil {
		return nil, fmt.Errorf("项目不存在: %s", name)
	}
	body := string(data)

	// 简单解析 project.md（仅解析描述，子项目列表由文件系统动态生成）
	desc := ""
	if idx := strings.Index(body, "## 项目简介"); idx >= 0 {
		rest := body[idx:]
		if nextIdx := strings.Index(rest[len("## 项目简介\n"):], "\n## "); nextIdx >= 0 {
			desc = strings.TrimSpace(rest[len("## 项目简介\n"):][:nextIdx])
		} else {
			desc = strings.TrimSpace(rest[len("## 项目简介\n"):])
		}
	}

	fs, err := e.Storage.Sub(name)
	if err != nil {
		return nil, fmt.Errorf("读取项目目录失败: %w", err)
	}

	// 聚合项目级（旧数据兼容，直接在 wiki/ 下的文件）
	srcPages, _ := fs.ListPages("sources")
	entPages, _ := fs.ListPages("entities")
	conPages, _ := fs.ListPages("concepts")
	synPages, _ := fs.ListPages("syntheses")

	// 聚合所有子项目的数据
	subNames, _ := fs.ListSubProjects()
	var subInfos []SubProjectInfo
	for _, subName := range subNames {
		subFS, spErr := fs.SubProject(subName)
		if spErr != nil {
			continue
		}
		sSrc, _ := subFS.ListPages("sources")
		sEnt, _ := subFS.ListPages("entities")
		sCon, _ := subFS.ListPages("concepts")
		sSyn, _ := subFS.ListPages("syntheses")
		subInfos = append(subInfos, SubProjectInfo{
			Name:           subName,
			SourcePages:    len(sSrc),
			EntityPages:    len(sEnt),
			ConceptPages:   len(sCon),
			SynthesisPages: len(sSyn),
		})
		srcPages = append(srcPages, sSrc...)
		entPages = append(entPages, sEnt...)
		conPages = append(conPages, sCon...)
		synPages = append(synPages, sSyn...)
	}

	return &ProjectInfo{
		Name:           name,
		Description:    desc,
		SubProjects:    buildSubProjectSummary(subInfos),
		SubProjectList: subInfos,
		SourcePages:    len(srcPages),
		EntityPages:    len(entPages),
		ConceptPages:   len(conPages),
		SynthesisPages: len(synPages),
		TotalPages:     len(srcPages) + len(entPages) + len(conPages) + len(synPages),
	}, nil
}

// ListProjects 列出所有项目
func (e *Engine) ListProjects() ([]*ProjectInfo, error) {
	names, err := e.Storage.ListProjects()
	if err != nil {
		return nil, err
	}
	var projects []*ProjectInfo
	for _, name := range names {
		info, err := e.GetProjectInfo(name)
		if err != nil {
			info = &ProjectInfo{Name: name}
		}
		projects = append(projects, info)
	}
	return projects, nil
}

// getChunks 读取并缓存 Markdown 分块，按文件修改时间失效
func (e *Engine) getChunks(path string) ([]storage.Chunk, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	e.chunkMu.RLock()
	if entry, ok := e.chunkCache[path]; ok &&
		entry.modTime.Equal(info.ModTime()) && entry.size == info.Size() {
		chunks := entry.chunks
		e.chunkMu.RUnlock()
		return chunks, nil
	}
	e.chunkMu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	chunks := storage.ChunkMarkdown(string(data), path, 3000)

	e.chunkMu.Lock()
	if e.chunkCache == nil {
		e.chunkCache = make(map[string]chunkCacheEntry)
	}
	e.chunkCache[path] = chunkCacheEntry{
		modTime: info.ModTime(),
		size:    info.Size(),
		chunks:  chunks,
	}
	e.chunkMu.Unlock()
	return chunks, nil
}

// buildSubProjectSummary 根据实际子项目列表动态生成描述文本，替代静态 project.md
func buildSubProjectSummary(subInfos []SubProjectInfo) string {
	if len(subInfos) == 0 {
		return ""
	}
	var parts []string
	for _, sub := range subInfos {
		parts = append(parts, fmt.Sprintf("%s（%d 源文档）", sub.Name, sub.SourcePages))
	}
	return strings.Join(parts, "\n")
}

// invalidateCaches 导入/删除后使检索与 lint 缓存失效
// 同时清空项目/子项目引擎缓存，确保下次请求使用最新数据重建索引
func (e *Engine) invalidateCaches() {
	e.chunkMu.Lock()
	e.chunkCache = nil
	e.chunkMu.Unlock()

	e.lintMu.Lock()
	e.lintCache = nil
	e.lintMu.Unlock()

	// 清理引擎缓存：导入新文档后，缓存的引擎搜索器已过时，需要重建
	e.engineMu.Lock()
	e.projectEngineCache = make(map[string]*Engine)
	e.subProjectEngineCache = make(map[string]*Engine)
	e.engineMu.Unlock()
}
