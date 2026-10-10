package api

import (
	"net/http"
	"strings"

	"repowiki/internal/llm"
)

// ConfigHandler 提供运行期读写配置（主要是 DeepSeek API Key）的接口，
// 支撑「首次启动在网页里填 Key，而不是在终端里填」的流程。
type ConfigHandler struct {
	LLM  *llm.DeepSeekClient
	Save func(apiKey, model string) error
}

// NewConfigHandler 创建配置处理器。
// save 负责把新配置持久化到 .env（由 cmd/main.go 注入具体实现）。
func NewConfigHandler(llmClient *llm.DeepSeekClient, save func(apiKey, model string) error) *ConfigHandler {
	return &ConfigHandler{LLM: llmClient, Save: save}
}

type configStatus struct {
	APIKeySet    bool   `json:"api_key_set"`
	APIKeyMasked string `json:"api_key_masked"`
	Model        string `json:"model"`
}

// HandleGet 返回当前配置状态（绝不返回完整密钥）。
func (h *ConfigHandler) HandleGet(w http.ResponseWriter, r *http.Request) {
	set, masked, model := h.LLM.Config()
	writeJSON(w, configStatus{APIKeySet: set, APIKeyMasked: masked, Model: model})
}

type setConfigRequest struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
}

// HandleSet 校验并保存 API Key / 模型，随后立即对客户端生效。
func (h *ConfigHandler) HandleSet(w http.ResponseWriter, r *http.Request) {
	var req setConfigRequest
	if err := readJSON(r, &req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	apiKey := strings.TrimSpace(req.APIKey)
	if apiKey == "" {
		writeJSONError(w, http.StatusBadRequest, "api_key 不能为空")
		return
	}
	if !strings.HasPrefix(apiKey, "sk-") {
		writeJSONError(w, http.StatusBadRequest, "API Key 格式不正确（DeepSeek 密钥以 sk- 开头）")
		return
	}

	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = "deepseek-chat"
	}

	// 真实校验一次：无效 key 直接拒绝，不落盘
	if err := h.LLM.ValidateKey(r.Context(), apiKey); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	// 先持久化再改内存：落盘失败时不至于出现「内存已改、下次启动又丢」的不一致
	if h.Save != nil {
		if err := h.Save(apiKey, model); err != nil {
			writeJSONError(w, http.StatusInternalServerError, "保存配置失败: "+err.Error())
			return
		}
	}
	h.LLM.Configure(apiKey, model)

	_, masked, m := h.LLM.Config()
	writeJSON(w, configStatus{APIKeySet: true, APIKeyMasked: masked, Model: m})
}
