package delivery

import (
	"context"
	"fmt"
	"log"

	deliverysdk "order-service/internal/sdk/delivery"
	"order-service/internal/service"
)

// 配送平台回调的状态值
const (
	StatusPicking    = "picking"    // 骑手已接单
	StatusDelivering = "delivering" // 配送中
	StatusDelivered  = "delivered"  // 已送达
	StatusCanceled   = "canceled"   // 配送取消
)

// CallbackRequest 配送平台回调报文
type CallbackRequest struct {
	DeliNo   string `json:"deli_no"`   // 配送单号
	OrderID  string `json:"order_id"`  // 关联网格订单号
	Status   string `json:"status"`    // 配送状态
	RiderID  string `json:"rider_id"`  // 骑手 ID
	RiderTel string `json:"rider_tel"` // 骑手电话
	Sign     string `json:"sign"`
}

// Handler 配送回调处理器。
//
// 与支付回调不同：配送状态只影响订单终态（delivered → completed），
// 不涉及资金，因此处理失败只记录日志并交由定时对账补偿，不阻塞回调返回。
type Handler struct {
	orderService *service.OrderService
	deliverySDK  *deliverysdk.DeliverySDK
}

// NewHandler 创建配送回调处理器
func NewHandler(orderService *service.OrderService, deliverySDK *deliverysdk.DeliverySDK) *Handler {
	return &Handler{orderService: orderService, deliverySDK: deliverySDK}
}

// Handle 处理配送状态回调
func (h *Handler) Handle(ctx context.Context, req CallbackRequest) error {
	if req.DeliNo == "" || req.OrderID == "" {
		return fmt.Errorf("配送回调缺少 deli_no 或 order_id")
	}

	// 同步配送单状态，保持 SDK 侧与回调一致
	if err := h.deliverySDK.UpdateStatus(req.DeliNo, req.Status); err != nil {
		log.Printf("[delivery-callback] 更新配送单失败 deli=%s: %v", req.DeliNo, err)
	}

	switch req.Status {
	case StatusDelivered:
		return h.completeOrder(ctx, req)
	case StatusCanceled:
		log.Printf("[delivery-callback] 配送取消 order=%s deli=%s，需人工介入", req.OrderID, req.DeliNo)
		return nil
	case StatusPicking, StatusDelivering:
		// 中间态无需变更订单，仅记录骑手信息供前端展示
		log.Printf("[delivery-callback] order=%s 配送中 rider=%s", req.OrderID, req.RiderID)
		return nil
	default:
		return fmt.Errorf("未知的配送状态: %s", req.Status)
	}
}

// completeOrder 配送送达 → 订单置为已完成
func (h *Handler) completeOrder(ctx context.Context, req CallbackRequest) error {
	order, err := h.orderService.GetOrder(ctx, req.OrderID)
	if err != nil {
		return fmt.Errorf("查询订单失败: %w", err)
	}

	// 幂等：订单已处于终态（完成/取消/退款）则直接跳过
	if order.Status.IsFinal() {
		return nil
	}

	log.Printf("[delivery-callback] order=%s 已送达，推进为 completed", req.OrderID)
	// 配送完成是 deliverying -> completed，由 service 层状态机校验
	return h.orderService.CompleteDelivery(ctx, req.OrderID)
}
