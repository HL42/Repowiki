package service

import (
	"context"
	"errors"
	"fmt"

	"order-service/internal/model"
	"order-service/internal/repo"
	"order-service/internal/sdk/payment"
)

var (
	// ErrOrderNotPayable 订单当前状态不可支付
	ErrOrderNotPayable = errors.New("当前订单状态不可支付")
	// ErrPaymentNotRefundable 支付单不可退款
	ErrPaymentNotRefundable = errors.New("当前支付单不可退款")
)

// PaymentService 支付业务逻辑层。
//
// 与 OrderService 的分工：本层只关心「钱」的流转（预支付单、退款），
// 订单状态推进通过 OrderService.MarkPaid 回调，避免两个 service 互相依赖。
type PaymentService struct {
	paymentRepo *repo.PaymentRepo
	orderRepo   *repo.OrderRepo
	paymentSDK  *payment.PaymentSDK
}

// NewPaymentService 装配支付服务
func NewPaymentService(paymentRepo *repo.PaymentRepo, orderRepo *repo.OrderRepo, paymentSDK *payment.PaymentSDK) *PaymentService {
	return &PaymentService{
		paymentRepo: paymentRepo,
		orderRepo:   orderRepo,
		paymentSDK:  paymentSDK,
	}
}

// PayOrder 发起支付：校验订单 → 创建支付单 → 调起第三方预支付。
// 返回的 prepayParams 由前端交给渠道 SDK 唤起收银台。
func (s *PaymentService) PayOrder(ctx context.Context, orderID string, channel model.PaymentChannel) (map[string]string, error) {
	order, err := s.orderRepo.GetByID(orderID)
	if err != nil {
		return nil, err
	}
	if order.Status != model.OrderStatusCreated {
		return nil, fmt.Errorf("%w: 订单状态 %s", ErrOrderNotPayable, order.Status)
	}

	pay := &model.Payment{
		OrderID: order.ID,
		UserID:  order.UserID,
		Channel: channel,
		Amount:  order.PayableAmount(),
	}
	payID, err := s.paymentRepo.Create(pay)
	if err != nil {
		return nil, fmt.Errorf("创建支付单失败: %w", err)
	}

	prepayParams, err := s.paymentSDK.CreatePrepay(ctx, payment.CreatePrepayRequest{
		PaymentID: payID,
		OrderID:   order.ID,
		UserID:    order.UserID,
		Channel:   string(channel),
		Amount:    order.PayableAmount(),
		Subject:   fmt.Sprintf("订单 %s", order.ID),
	})
	if err != nil {
		// 预支付失败不删除支付单，保留现场便于排查
		_ = s.paymentRepo.MarkFailed(payID, err.Error())
		return nil, fmt.Errorf("调起支付失败: %w", err)
	}
	return prepayParams, nil
}

// Refund 退款：校验支付单 → 调第三方退款 → 更新本地状态。
func (s *PaymentService) Refund(ctx context.Context, orderID string, reason string) error {
	pay, err := s.paymentRepo.GetByOrderID(orderID)
	if err != nil {
		return err
	}
	if pay.Status != model.PaymentStatusSuccess {
		return fmt.Errorf("%w: 支付单状态 %s", ErrPaymentNotRefundable, pay.Status)
	}

	if err := s.paymentSDK.Refund(ctx, pay.TransactionID, pay.Amount, reason); err != nil {
		return fmt.Errorf("第三方退款失败: %w", err)
	}

	// 先调第三方成功、再改本地状态；若本地失败，对账任务会依据流水号补偿
	if err := s.paymentRepo.MarkRefunded(pay.ID); err != nil {
		return fmt.Errorf("更新退款状态失败: %w", err)
	}
	return nil
}

// GetPaymentByOrder 查询订单对应的支付单
func (s *PaymentService) GetPaymentByOrder(ctx context.Context, orderID string) (*model.Payment, error) {
	return s.paymentRepo.GetByOrderID(orderID)
}
