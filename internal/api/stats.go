package api

import (
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"repowiki/internal/storage"
	"repowiki/internal/wiki"
)

type StatsHandler struct {
	Engine *wiki.Engine
}

func NewStatsHandler(engine *wiki.Engine) *StatsHandler {
	return &StatsHandler{Engine: engine}
}

func (h *StatsHandler) HandleStats(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project != "" {
		// 单项目统计
		engine := h.Engine.WithProject(project)
		h.writeStats(w, engine)
		return
	}
	// 全局：聚合所有项目
	h.writeGlobalStats(w)
}

func (h *StatsHandler) writeStats(w http.ResponseWriter, engine *wiki.Engine) {
	sourcePages, _ := engine.Storage.ListPages("sources")
	entityPages, _ := engine.Storage.ListPages("entities")
	conceptPages, _ := engine.Storage.ListPages("concepts")
	synthesisPages, _ := engine.Storage.ListPages("syntheses")

	// 聚合子项目页面
	subNames, _ := engine.Storage.ListSubProjects()
	for _, subName := range subNames {
		subFS, spErr := engine.Storage.SubProject(subName)
		if spErr != nil {
			continue
		}
		sSrc, _ := subFS.ListPages("sources")
		sEnt, _ := subFS.ListPages("entities")
		sCon, _ := subFS.ListPages("concepts")
		sSyn, _ := subFS.ListPages("syntheses")
		sourcePages = append(sourcePages, sSrc...)
		entityPages = append(entityPages, sEnt...)
		conceptPages = append(conceptPages, sCon...)
		synthesisPages = append(synthesisPages, sSyn...)
	}

	total := len(sourcePages) + len(entityPages) + len(conceptPages) + len(synthesisPages)

	lintReport := engine.LintIfStale(24 * time.Hour)
	resp := wiki.StatsResponse{
		TotalPages:     total,
		SourcePages:    len(sourcePages),
		EntityPages:    len(entityPages),
		ConceptPages:   len(conceptPages),
		SynthesisPages: len(synthesisPages),
		StaleDocs:      countStaleSyntheses(engine),
	}
	if lintReport != nil {
		resp.LintErrors = lintReport.ErrorCount
		resp.LintWarnings = lintReport.WarnCount
		resp.LintInfo = lintReport.InfoCount
		resp.LintTime = time.Now().Format("01-02 15:04")
	}
	writeJSON(w, resp)
}

// countStalePages 统计指定存储中标记为 stale 的综合提炼页数量
func countStalePages(fs *storage.FileSystem) int {
	pages, err := fs.ListPages("syntheses")
	if err != nil {
		return 0
	}
	count := 0
	for _, pagePath := range pages {
		if !strings.HasSuffix(pagePath, ".md") {
			continue
		}
		slug := strings.TrimSuffix(filepath.Base(pagePath), ".md")
		content, readErr := fs.ReadPage("syntheses", slug)
		if readErr != nil {
			continue
		}
		// 只检查前 4096 字节（Front Matter 通常在前部）
		if len(content) > 4096 {
			content = content[:4096]
		}
		if strings.Contains(content, "stale: true") {
			count++
		}
	}
	return count
}

// countStaleSyntheses 统计引擎范围内（含子项目）标记为 stale 的综合提炼页数量
func countStaleSyntheses(engine *wiki.Engine) int {
	count := countStalePages(engine.Storage)
	subNames, _ := engine.Storage.ListSubProjects()
	for _, subName := range subNames {
		subFS, spErr := engine.Storage.SubProject(subName)
		if spErr != nil {
			continue
		}
		count += countStalePages(subFS)
	}
	return count
}

func (h *StatsHandler) writeGlobalStats(w http.ResponseWriter) {
	projects, _ := h.Engine.ListProjects()
	var totalSrc, totalEnt, totalCon, totalSyn int
	var totalLintErrors, totalLintWarnings, totalLintInfo int

	var totalStale int

	for _, proj := range projects {
		engine := h.Engine.WithProject(proj.Name)
		totalStale += countStaleSyntheses(engine)

		// 聚合项目级页面
		src, _ := engine.Storage.ListPages("sources")
		ent, _ := engine.Storage.ListPages("entities")
		con, _ := engine.Storage.ListPages("concepts")
		syn, _ := engine.Storage.ListPages("syntheses")

		// 聚合所有子项目的页面
		subNames, _ := engine.Storage.ListSubProjects()
		for _, subName := range subNames {
			subFS, spErr := engine.Storage.SubProject(subName)
			if spErr != nil {
				continue
			}
			sSrc, _ := subFS.ListPages("sources")
			sEnt, _ := subFS.ListPages("entities")
			sCon, _ := subFS.ListPages("concepts")
			sSyn, _ := subFS.ListPages("syntheses")
			src = append(src, sSrc...)
			ent = append(ent, sEnt...)
			con = append(con, sCon...)
			syn = append(syn, sSyn...)
		}

		totalSrc += len(src)
		totalEnt += len(ent)
		totalCon += len(con)
		totalSyn += len(syn)

		// 聚合 lint 统计数据
		lintReport := engine.LintIfStale(24 * time.Hour)
		if lintReport != nil {
			totalLintErrors += lintReport.ErrorCount
			totalLintWarnings += lintReport.WarnCount
			totalLintInfo += lintReport.InfoCount
		}
	}
	resp := wiki.StatsResponse{
		TotalPages:     totalSrc + totalEnt + totalCon + totalSyn,
		SourcePages:    totalSrc,
		EntityPages:    totalEnt,
		ConceptPages:   totalCon,
		SynthesisPages: totalSyn,
		StaleDocs:      totalStale,
		LintErrors:     totalLintErrors,
		LintWarnings:   totalLintWarnings,
		LintInfo:       totalLintInfo,
		LintTime:       time.Now().Format("01-02 15:04"),
	}
	writeJSON(w, resp)
}
