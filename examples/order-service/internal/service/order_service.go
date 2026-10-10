package service

import (
	"context"
	"errors"
	"fmt"

	"order-service/internal/model"
	"order-service/internal/repo"
	"order-service/internal/sdk/delivery"
	"order-service/internal/util"
)

var (
	// ErrEmptyItems 订单没有商品
	ErrEmptyItems = errors.New("订单至少需要一个商品")
	// ErrInvalidTransition 非法状态流转
	ErrInvalidTransition = errors.New("订单状态流转非法")
	// ErrOrderNotCancelable 订单当前状态不可取消
	ErrOrderNotCancelable = errors.New("当前订单状态不可取消")
)

// OrderService 订单业务逻辑层。
//
// 职责边界：校验入参 → 驱动状态机 → 编排仓储与外部 SDK。
// HTTP 相关逻辑（解码、状态码）一律留在 handler 层，保持本层可被
// 定时任务、消息消费者等非 HTTP 入口复用。
type OrderService struct {
	orderRepo   *repo.OrderRepo
	paymentRepo *repo.PaymentRepo
	deliverySDK *delivery.DeliverySDK
}

// NewOrderService 装配订单服务
func NewOrderService(orderRepo *repo.OrderRepo, paymentRepo *repo.PaymentRepo, deliverySDK *delivery.DeliverySDK) *OrderService {
	return &OrderService{
		orderRepo:   orderRepo,
		paymentRepo: paymentRepo,
		deliverySDK: deliverySDK,
	}
}

// CreateOrderRequest 创建订单入参
type CreateOrderRequest struct {
	UserID      string            `json:"user_id"`
	ShopID      string            `json:"shop_id"`
	Items       []model.OrderItem `json:"items"`
	DeliveryFee float64           `json:"delivery_fee"`
	Remark      string            `json:"remark"`
}

// CreateOrder 创建订单。流程：校验入参 → 计算金额 → 落库。
func (s *OrderService) CreateOrder(ctx context.Context, req CreateOrderRequest) (*model.Order, error) {
	if req.UserID == "" {
		return nil, errors.New("user_id 不能为空")
	}
	if !util.IsValidShopID(req.ShopID) {
		return nil, fmt.Errorf("shop_id 格式非法: %s", req.ShopID)
	}
	if len(req.Items) == 0 {
		return nil, ErrEmptyItems
	}
	for _, item := range req.Items {
		if item.Quantity <= 0 {
			return nil, fmt.Errorf("商品 %s 数量必须为正数", item.SKUID)
		}
		if item.Price < 0 {
			return nil, fmt.Errorf("商品 %s 单价不能为负", item.SKUID)
		}
	}
	if req.DeliveryFee < 0 {
		return nil, errors.New("配送费不能为负")
	}

	order := &model.Order{
		UserID:      req.UserID,
		ShopID:      req.ShopID,
		Items:       req.Items,
		DeliveryFee: req.DeliveryFee,
		Remark:      util.Truncate(req.Remark, 200),
	}

	id, err := s.orderRepo.Create(order)
	if err != nil {
		return nil, fmt.Errorf("保存订单失败: %w", err)
	}

	return s.orderRepo.GetByID(id)
}

// GetOrder 查询订单详情
func (s *OrderService) GetOrder(ctx context.Context, orderID string) (*model.Order, error) {
	return s.orderRepo.GetByID(orderID)
}

// ListUserOrders 查询用户订单列表
func (s *OrderService) ListUserOrders(ctx context.Context, userID string) []*model.Order {
	return s.orderRepo.ListByUser(userID)
}

// MarkPaid 支付成功后推进订单状态。由支付回调触发。
func (s *OrderService) MarkPaid(ctx context.Context, orderID string) error {
	return s.transition(orderID, model.OrderStatusPaid)
}

// CancelOrder 取消订单。
// 只有未支付（created）的订单可以直接取消；已支付的订单必须先退款。
func (s *OrderService) CancelOrder(ctx context.Context, orderID string) error {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return err
	}
	if order.Status != model.OrderStatusCreated {
		return fmt.Errorf("%w: 当前状态 %s，已支付订单请走退款流程", ErrOrderNotCancelable, order.Status)
	}
	return s.transition(orderID, model.OrderStatusCanceled)
}

// Dispatch 呼叫运力发起配送。只有已支付订单可以发起。
func (s *OrderService) Dispatch(ctx context.Context, orderID string) (string, error) {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return "", err
	}
	if order.Status != model.OrderStatusPaid {
		return "", fmt.Errorf("只有已支付订单可以发起配送，当前状态: %s", order.Status)
	}

	deliNo, err := s.deliverySDK.Dispatch(ctx, delivery.DispatchRequest{
		OrderID: order.ID,
		ShopID:  order.ShopID,
		UserID:  order.UserID,
	})
	if err != nil {
		return "", fmt.Errorf("呼叫运力失败: %w", err)
	}

	if err := s.transition(orderID, model.OrderStatusDelivering); err != nil {
		return "", err
	}
	return deliNo, nil
}

// CompleteDelivery 配送送达后置订单为已完成。由配送回调触发。
func (s *OrderService) CompleteDelivery(ctx context.Context, orderID string) error {
	return s.transition(orderID, model.OrderStatusCompleted)
}

// MarkRefunded 退款完成后置订单为已退款
func (s *OrderService) MarkRefunded(ctx context.Context, orderID string) error {
	return s.transition(orderID, model.OrderStatusRefunded)
}

// CountByStatus 统计各状态订单数，供看板与对账任务使用
func (s *OrderService) CountByStatus(status model.OrderStatus) int {
	return len(s.orderRepo.ListByStatus(status))
}

// transition 统一的状态流转入口：先校验状态机，再落库
func (s *OrderService) transition(orderID string, target model.OrderStatus) error {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return err
	}
	if order.Status == target {
		return nil // 幂等
	}
	if !order.Status.CanTransitionTo(target) {
		return fmt.Errorf("%w: %s -> %s", ErrInvalidTransition, order.Status, target)
	}
	return s.orderRepo.UpdateStatus(orderID, target)
}
