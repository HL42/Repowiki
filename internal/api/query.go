package api

import (
	"net/http"

	"repowiki/internal/wiki"
)

type QueryHandler struct {
	Engine *wiki.Engine
}

func NewQueryHandler(engine *wiki.Engine) *QueryHandler {
	return &QueryHandler{Engine: engine}
}

func (h *QueryHandler) HandleQuery(w http.ResponseWriter, r *http.Request) {
	var req wiki.QueryRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	if req.Question == "" {
		writeJSONError(w, http.StatusBadRequest, "question 不能为空")
		return
	}

	engine := h.Engine
	if req.Project != "" {
		engine = h.Engine.WithProject(req.Project)
	}
	if req.SubProject != "" {
		engine = engine.WithSubProject(req.SubProject)
	}

	resp, err := engine.Query(r.Context(), req.Question, req.History)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp.SearchScope = queryScopeLabel(req.Project, req.SubProject)

	writeJSON(w, resp)
}

func queryScopeLabel(project, subProject string) string {
	if project == "" {
		return "全部项目"
	}
	if subProject == "" {
		return project
	}
	return project + " / " + subProject
}
