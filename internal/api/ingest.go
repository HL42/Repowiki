package api

import (
	"net/http"

	"repowiki/internal/wiki"
)

type IngestHandler struct {
	Engine *wiki.Engine
}

func NewIngestHandler(engine *wiki.Engine) *IngestHandler {
	return &IngestHandler{Engine: engine}
}

// buildEngine 根据请求参数构造带 project/sub_project 的引擎实例，消除 HandleIngest 和 HandleValidate 的重复代码。
func (h *IngestHandler) buildEngine(req *wiki.IngestRequest) *wiki.Engine {
	engine := h.Engine
	project := req.Project
	if project == "" {
		project = "通用"
	}
	engine = h.Engine.WithProject(project)
	if req.SubProject != "" {
		engine = engine.WithSubProject(req.SubProject)
	}
	return engine
}

func (h *IngestHandler) HandleIngest(w http.ResponseWriter, r *http.Request) {
	var req wiki.IngestRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	if req.RawPath == "" {
		writeJSONError(w, http.StatusBadRequest, "raw_path 不能为空")
		return
	}

	engine := h.buildEngine(&req)

	// 前置校验（不阻塞导入，仅返回警告）
	validation := engine.ValidateImport(req.RawPath)

	successCount, errs := engine.IngestAll(r.Context(), req.RawPath)

	resp := map[string]interface{}{
		"success": successCount,
	}
	if len(errs) > 0 {
		var msgs []string
		for _, e := range errs {
			msgs = append(msgs, e.Error())
		}
		resp["errors"] = msgs
	}
	if len(validation.Warnings) > 0 || len(validation.Errors) > 0 {
		resp["validation"] = validation
	}

	writeJSON(w, resp)
}

// HandleValidate 导入前置检查（只读，不执行实际导入）
func (h *IngestHandler) HandleValidate(w http.ResponseWriter, r *http.Request) {
	var req wiki.IngestRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	if req.RawPath == "" {
		writeJSONError(w, http.StatusBadRequest, "raw_path 不能为空")
		return
	}

	engine := h.buildEngine(&req)
	report := engine.ValidateImport(req.RawPath)
	writeJSON(w, report)
}
