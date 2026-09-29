package api

import (
	"net/http"

	"repowiki/internal/wiki"
)

type SynthesizeHandler struct {
	Engine *wiki.Engine
}

func NewSynthesizeHandler(engine *wiki.Engine) *SynthesizeHandler {
	return &SynthesizeHandler{Engine: engine}
}

func (h *SynthesizeHandler) HandleSynthesize(w http.ResponseWriter, r *http.Request) {
	var req wiki.SynthesizeRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求格式错误: "+err.Error())
		return
	}

	if req.Topic == "" {
		writeJSONError(w, http.StatusBadRequest, "topic 不能为空")
		return
	}

	engine := h.Engine
	if req.Project != "" {
		engine = h.Engine.WithProject(req.Project)
	}
	if req.SubProject != "" {
		engine = engine.WithSubProject(req.SubProject)
	}

	resp, err := engine.Synthesize(r.Context(), req.Topic)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, resp)
}
