package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"repowiki/internal/wiki"
)

type Server struct {
	Engine      *wiki.Engine
	Handlers    map[string]http.HandlerFunc
	FrontendDir string
}

func NewServer(engine *wiki.Engine) *Server {
	s := &Server{Engine: engine}
	s.Handlers = map[string]http.HandlerFunc{
		"POST /api/ingest":           cors(NewIngestHandler(engine).HandleIngest),
		"POST /api/ingest/validate": cors(NewIngestHandler(engine).HandleValidate),
		"POST /api/query":       cors(NewQueryHandler(engine).HandleQuery),
		"POST /api/synthesize":  cors(NewSynthesizeHandler(engine).HandleSynthesize),
		"GET /api/stats":        cors(NewStatsHandler(engine).HandleStats),
		"GET /api/lint":         cors(s.handleLint),
		"POST /api/projects":    cors(s.handleCreateProject),
		"GET /api/projects":     cors(s.handleListProjects),
		"GET /api/sub-projects":  cors(s.handleListSubProjects),
		"GET /api/project/":     cors(s.handleGetProject),
	}

	for _, dir := range []string{"./frontend/dist", "frontend/dist"} {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			s.FrontendDir = dir
			break
		}
	}
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	key := r.Method + " " + r.URL.Path
	if handler, ok := s.Handlers[key]; ok {
		handler(w, r)
		return
	}

	// /api/wiki/list/:category  -> 列出某类页面
	if strings.HasPrefix(r.URL.Path, "/api/wiki/list/") {
		cors(s.handleWikiList)(w, r)
		return
	}

	// /api/wiki/page/:category/:name  -> 返回页面内容
	if strings.HasPrefix(r.URL.Path, "/api/wiki/page/") {
		if r.Method == "DELETE" {
			cors(s.handleWikiPageDelete)(w, r)
			return
		}
		cors(s.handleWikiPage)(w, r)
		return
	}

	// /api/project/:name  -> 获取项目详情（支持子路径 /api/project/:name/...）
	if strings.HasPrefix(r.URL.Path, "/api/project/") && r.Method == http.MethodGet {
		cors(s.handleGetProject)(w, r)
		return
	}

	// 未知 API 路径：返回 JSON 404（不返回 HTML，避免误导 API 调用方）
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeJSONError(w, http.StatusNotFound, "接口不存在")
		return
	}

	// 非 API 路由：提供前端静态资源（SPA fallback）
	if s.FrontendDir != "" && r.Method == http.MethodGet {
		s.serveFrontend(w, r)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) serveFrontend(w http.ResponseWriter, r *http.Request) {
	cleanPath := filepath.Clean(r.URL.Path)
	if strings.Contains(cleanPath, "..") {
		http.NotFound(w, r)
		return
	}

	filePath := filepath.Join(s.FrontendDir, strings.TrimPrefix(cleanPath, "/"))
	if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
		http.ServeFile(w, r, filePath)
		return
	}

	http.ServeFile(w, r, filepath.Join(s.FrontendDir, "index.html"))
}

// handleWikiList 列出某个子目录下的所有页面（支持 ?project=xxx）
func (s *Server) handleWikiList(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/api/wiki/list/"), "/", 2)
	if len(parts) < 1 {
		writeJSON(w, map[string]string{"error": "category required"})
		return
	}
	category := parts[0]
	if !allowedWikiCategories[category] {
		writeJSONError(w, http.StatusBadRequest, "invalid category")
		return
	}

	engine := s.Engine
	project := r.URL.Query().Get("project")
	if project != "" {
		engine = s.Engine.WithProject(project)
	}
	subProject := r.URL.Query().Get("sub_project")
	if subProject != "" {
		engine = engine.WithSubProject(subProject)
	}
	dir := filepath.Join(engine.Storage.Root, "wiki", category)
	entries, err := os.ReadDir(dir)
	if err != nil {
		writeJSON(w, map[string][]string{"pages": {}})
		return
	}

	var pages []map[string]string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			pages = append(pages, map[string]string{"name": e.Name()})
		}
	}

	writeJSON(w, map[string]interface{}{"pages": pages})
}

// handleWikiPage 返回页面 Markdown 内容（支持 ?project=xxx）
func (s *Server) handleWikiPage(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/wiki/page/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	category, name := parts[0], parts[1]

	root := s.Engine.Storage.Root
	project := r.URL.Query().Get("project")
	if project != "" {
		engineForPath := s.Engine.WithProject(project)
		subProject := r.URL.Query().Get("sub_project")
		if subProject != "" {
			root = engineForPath.WithSubProject(subProject).Storage.Root
		} else {
			root = engineForPath.Storage.Root
		}
	}

	filePath, err := safeWikiPagePath(root, category, name)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	http.ServeFile(w, r, filePath)
}

// handleLint 质量检查（强制刷新）
func (s *Server) handleLint(w http.ResponseWriter, r *http.Request) {
	report := s.Engine.RefreshLint()
	writeJSON(w, report)
}

// handleWikiPageDelete 删除页面
func (s *Server) handleWikiPageDelete(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/wiki/page/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) < 2 {
		writeJSONError(w, http.StatusBadRequest, "invalid path")
		return
	}
	category, name := parts[0], parts[1]

	project := r.URL.Query().Get("project")
	subProject := r.URL.Query().Get("sub_project")

	// 关键修复：使用带 project/sub_project 的引擎实例来执行删除
	// 这样 DeletePage 内部拼出的路径才是正确的（指向项目的 wiki/ 而非全局）
	var targetEngine *wiki.Engine
	if project != "" && subProject != "" {
		targetEngine = s.Engine.WithProject(project).WithSubProject(subProject)
	} else if project != "" {
		targetEngine = s.Engine.WithProject(project)
	} else {
		targetEngine = s.Engine
	}

	// 用目标引擎的 Root 来校验路径安全性
	deleteRoot := targetEngine.Storage.Root
	if _, err := safeWikiPagePath(deleteRoot, category, name); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 用目标引擎执行删除（其 Root 已正确指向项目/子项目目录）
	if err := targetEngine.DeletePage(category, filepath.Base(name)); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"deleted": filepath.Base(name)})
}

// handleCreateProject 创建项目
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req wiki.ProjectCreateRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if req.Name == "" {
		writeJSONError(w, http.StatusBadRequest, "项目名不能为空")
		return
	}
	if err := s.Engine.CreateProject(req.Name, req.Description, req.SubProjects); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, map[string]string{"created": req.Name})
}

// handleListProjects 列出所有项目
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := s.Engine.ListProjects()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []*wiki.ProjectInfo{}
	}
	writeJSON(w, map[string]interface{}{"projects": projects})
}

// handleListSubProjects 列出指定项目下的所有子项目
func (s *Server) handleListSubProjects(w http.ResponseWriter, r *http.Request) {
	project := r.URL.Query().Get("project")
	if project == "" {
		writeJSONError(w, http.StatusBadRequest, "缺少 project 参数")
		return
	}
	fs, subErr := s.Engine.Storage.Sub(project)
	if subErr != nil {
		writeJSONError(w, http.StatusInternalServerError, subErr.Error())
		return
	}
	names, err := fs.ListSubProjects()
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if names == nil {
		names = []string{}
	}
	writeJSON(w, map[string]interface{}{"sub_projects": names})
}

// handleGetProject 获取项目详情
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/project/")
	name = strings.TrimSuffix(name, "/")
	if name == "" {
		writeJSONError(w, http.StatusBadRequest, "缺少项目名")
		return
	}
	info, err := s.Engine.GetProjectInfo(name)
	if err != nil {
		writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, info)
}

var allowedWikiCategories = map[string]bool{
	"sources": true, "entities": true, "concepts": true,
	"syntheses": true, "contradictions": true,
}

// safeWikiPagePath 校验 category/name 并返回安全的绝对路径，防止路径穿越
func safeWikiPagePath(root, category, name string) (string, error) {
	if !allowedWikiCategories[category] {
		return "", fmt.Errorf("invalid category")
	}
	baseName := filepath.Base(name)
	if baseName != name || baseName == "." || baseName == ".." {
		return "", fmt.Errorf("invalid page name")
	}
	if !strings.HasSuffix(baseName, ".md") {
		return "", fmt.Errorf("page must be a .md file")
	}

	wikiRoot, err := filepath.Abs(filepath.Join(root, "wiki", category))
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	fullPath, err := filepath.Abs(filepath.Join(wikiRoot, baseName))
	if err != nil {
		return "", fmt.Errorf("invalid path")
	}
	if !strings.HasPrefix(fullPath, wikiRoot+string(os.PathSeparator)) && fullPath != wikiRoot {
		return "", fmt.Errorf("path traversal detected")
	}
	return fullPath, nil
}

// cors 包装器
func cors(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next(w, r)
	}
}

// JSON
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func readJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

func extractPathParam(path, prefix string) string {
	return strings.TrimPrefix(path, prefix)
}
