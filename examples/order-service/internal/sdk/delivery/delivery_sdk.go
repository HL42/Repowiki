package delivery

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// DispatchRequest 呼叫运力请求
type DispatchRequest struct {
	OrderID string `json:"order_id"`
	ShopID  string `json:"shop_id"`
	UserID  string `json:"user_id"`
}

// DeliverySDK 配送平台适配层。
//
// 真实实现里对接的是第三方运力平台（蜂鸟/达达/顺丰同城等），
// 这里用一个内存实现演示接口契约：派单、取消、查状态。
type DeliverySDK struct {
	mu       sync.Mutex
	orders   map[string]string // 配送单号 -> 状态
	dispatch int
}

// NewDeliverySDK 创建配送 SDK
func NewDeliverySDK() *DeliverySDK {
	return &DeliverySDK{orders: make(map[string]string)}
}

// ErrNoCapacity 当前无可用运力
var ErrNoCapacity = errors.New("当前无可用运力，请稍后重试")

// Dispatch 呼叫运力，返回配送单号
func (s *DeliverySDK) Dispatch(ctx context.Context, req DispatchRequest) (string, error) {
	if req.OrderID == "" {
		return "", errors.New("订单号不能为空")
	}
	if req.ShopID == "" {
		return "", errors.New("门店号不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.dispatch++
	deliNo := fmt.Sprintf("DELI%s%04d", time.Now().Format("20060102"), s.dispatch)
	s.orders[deliNo] = "dispatched"
	return deliNo, nil
}

// Cancel 取消配送单。已签收的配送单不可取消。
func (s *DeliverySDK) Cancel(ctx context.Context, deliNo string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.orders[deliNo]
	if !ok {
		return fmt.Errorf("配送单不存在: %s", deliNo)
	}
	if status == "delivered" {
		return fmt.Errorf("配送单 %s 已签收，不可取消", deliNo)
	}
	s.orders[deliNo] = "canceled"
	return nil
}

// QueryStatus 查询配送状态，返回 dispatched / picking / delivering / delivered / canceled
func (s *DeliverySDK) QueryStatus(ctx context.Context, deliNo string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	status, ok := s.orders[deliNo]
	if !ok {
		return "", fmt.Errorf("配送单不存在: %s", deliNo)
	}
	return status, nil
}

// UpdateStatus 由配送回调驱动，推进配送单状态
func (s *DeliverySDK) UpdateStatus(deliNo, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.orders[deliNo]; !ok {
		return fmt.Errorf("配送单不存在: %s", deliNo)
	}
	s.orders[deliNo] = status
	return nil
}

// EstimateFee 预估配送费，按距离阶梯计价
func (s *DeliverySDK) EstimateFee(distanceKm float64) float64 {
	switch {
	case distanceKm <= 3:
		return 5.0
	case distanceKm <= 5:
		return 7.0
	default:
		return 7.0 + (distanceKm-5)*1.5
	}
}
