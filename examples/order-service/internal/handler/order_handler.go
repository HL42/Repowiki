package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"order-service/internal/model"
	"order-service/internal/repo"
	"order-service/internal/service"
)

// OrderHandler 订单 HTTP 接口层。
//
// 只做三件事：解析请求、调用 service、写响应。
// 所有业务规则（状态机、金额计算）都在 service 层，便于单测时绕开 HTTP。
type OrderHandler struct {
	orderService *service.OrderService
}

// NewOrderHandler 创建订单 Handler
func NewOrderHandler(orderService *service.OrderService) *OrderHandler {
	return &OrderHandler{orderService: orderService}
}

// Register 注册订单相关路由
func (h *OrderHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/order/create", h.Create)
	mux.HandleFunc("/api/order/detail", h.Detail)
	mux.HandleFunc("/api/order/list", h.List)
	mux.HandleFunc("/api/order/cancel", h.Cancel)
}

// Create 创建订单
//
//	POST /api/order/create
//	{"user_id":"U1001","shop_id":"SHOP000001","items":[{"sku_id":"SKU1","name":"招牌炒饭","quantity":2,"price":28.5}],"delivery_fee":5}
func (h *OrderHandler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req service.CreateOrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误: "+err.Error())
		return
	}

	order, err := h.orderService.CreateOrder(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"code": 0,
		"data": order,
	})
}

// Detail 查询订单详情
//
//	GET /api/order/detail?id=ORD20260101123456
func (h *OrderHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少参数 id")
		return
	}

	order, err := h.orderService.GetOrder(r.Context(), id)
	if err != nil {
		// 仓储层的 ErrOrderNotFound 需要映射为 404，而不是笼统的 500
		if errors.Is(err, repo.ErrOrderNotFound) {
			writeError(w, http.StatusNotFound, "订单不存在")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": order})
}

// List 查询用户订单列表
//
//	GET /api/order/list?user_id=U1001
func (h *OrderHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		writeError(w, http.StatusBadRequest, "缺少参数 user_id")
		return
	}

	orders := h.orderService.ListUserOrders(r.Context(), userID)
	if orders == nil {
		orders = []*model.Order{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "data": orders})
}

// Cancel 取消订单
//
//	POST /api/order/cancel  {"order_id":"ORD20260101123456"}
func (h *OrderHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST")
		return
	}

	var req struct {
		OrderID string `json:"order_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求体格式错误: "+err.Error())
		return
	}
	if req.OrderID == "" {
		writeError(w, http.StatusBadRequest, "缺少参数 order_id")
		return
	}

	if err := h.orderService.CancelOrder(r.Context(), req.OrderID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 0, "msg": "订单已取消"})
}

// ---------- 响应工具 ----------

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"code": status, "msg": msg})
}
