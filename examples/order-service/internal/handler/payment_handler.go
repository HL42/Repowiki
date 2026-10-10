package handler

import (
	"encoding/json"
	"net/http"
	"strings"

	"order-service/internal/model"
	"order-service/internal/service"
)

// PaymentHandler 支付 HTTP 接口层
type PaymentHandler struct {
	paymentService *service.PaymentService
}

// NewPaymentHandler 创建支付 Handler
func NewPaymentHandler(paymentService *service.PaymentService) *PaymentHandler {
	return &PaymentHandler{paymentService: paymentService}
}

// Register 注册支付相关路由
func (h *PaymentHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/payment/pay", h.Pay)
	mux.HandleFunc("/api/payment/refund", h.Refund)
	mux.HandleFunc("/api/payment/query", h.Query)
}

// Pay 发起支付，返回唤起收银台所需参数
//
//	POST /api/payment/pay
//	{"order_id":"ORD20260101123456","channel":"wechat"}
func (h *PaymentHandler) Pay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
		Channel string `json:"channel"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "缺少参数 order_id")
		return
	}

	channel := model.PaymentChannel(req.Channel)
	if channel == "" {
		channel = model.ChannelWechat // 默认微信支付
	}

	params, err := h.paymentService.PayOrder(r.Context(), req.OrderID, channel)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": params})
}

// Refund 发起退款
//
//	POST /api/payment/refund  {"order_id":"ORD...","reason":"用户申请取消"}
func (h *PaymentHandler) Refund(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
		Reason  string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "缺少参数 order_id")
		return
	}
	if strings.TrimSpace(req.Reason) == "" {
		req.Reason = "用户申请退款"
	}

	if err := h.paymentService.Refund(r.Context(), req.OrderID, req.Reason); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "退款已受理"})
}

// Query 查询订单对应的支付单
//
//	GET /api/payment/query?order_id=ORD20260101123456
func (h *PaymentHandler) Query(w http.ResponseWriter, r *http.Request) {
	orderID := strings.TrimSpace(r.URL.Query().Get("order_id"))
	if orderID == "" {
		writeError(w, http.StatusBadRequest, "缺少参数 order_id")
		return
	}

	pay, err := h.paymentService.GetPaymentByOrder(r.Context(), orderID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": pay})
}
