package repo

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"order-service/internal/model"
)

// ErrPaymentNotFound 支付单不存在
var ErrPaymentNotFound = errors.New("支付单不存在")

// PaymentRepo 支付单数据访问层。
//
// 支付单与订单是一对多：用户取消支付后重新发起会产生新的支付单，
// 因此这里额外维护 orderIndex 支持「按订单号查所有支付尝试」。
type PaymentRepo struct {
	mu         sync.RWMutex
	payments   map[string]*model.Payment // 主键：支付单号
	orderIndex map[string][]string       // 订单号 -> 支付单号列表
	seq        int64
}

// NewPaymentRepo 创建支付单仓储
func NewPaymentRepo() *PaymentRepo {
	return &PaymentRepo{
		payments:   make(map[string]*model.Payment),
		orderIndex: make(map[string][]string),
	}
}

func (r *PaymentRepo) nextID(now time.Time) string {
	r.seq++
	return fmt.Sprintf("PAY%s%06d", now.Format("20060102"), r.seq)
}

// Create 创建支付单
func (r *PaymentRepo) Create(payment *model.Payment) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	payment.ID = r.nextID(time.Now())
	payment.CreatedAt = time.Now()
	payment.Status = model.PaymentStatusPending

	cp := *payment
	r.payments[payment.ID] = &cp
	r.orderIndex[payment.OrderID] = append(r.orderIndex[payment.OrderID], payment.ID)

	return cp.ID, nil
}

// GetByID 按支付单号查询
func (r *PaymentRepo) GetByID(id string) (*model.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	payment, ok := r.payments[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrPaymentNotFound, id)
	}
	cp := *payment
	return &cp, nil
}

// GetByOrderID 查询某订单下最近一次支付尝试，用于回调时定位支付单
func (r *PaymentRepo) GetByOrderID(orderID string) (*model.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.orderIndex[orderID]
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: order=%s", ErrPaymentNotFound, orderID)
	}
	// 取最后一次尝试
	cp := *r.payments[ids[len(ids)-1]]
	return &cp, nil
}

// MarkSuccess 标记支付成功并记录第三方流水号。
// 回调可能重复投递，因此这里做了幂等处理：已是终态则直接返回。
func (r *PaymentRepo) MarkSuccess(id, transactionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	payment, ok := r.payments[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrPaymentNotFound, id)
	}
	if payment.IsSettled() {
		return nil // 幂等：重复回调不改变已有终态
	}

	now := time.Now()
	payment.Status = model.PaymentStatusSuccess
	payment.TransactionID = transactionID
	payment.PaidAt = &now
	return nil
}

// MarkFailed 标记支付失败并记录原因
func (r *PaymentRepo) MarkFailed(id, reason string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	payment, ok := r.payments[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrPaymentNotFound, id)
	}
	if payment.IsSettled() {
		return nil
	}

	payment.Status = model.PaymentStatusFailed
	payment.FailReason = reason
	return nil
}

// MarkRefunded 标记已退款
func (r *PaymentRepo) MarkRefunded(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	payment, ok := r.payments[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrPaymentNotFound, id)
	}
	if payment.Status != model.PaymentStatusSuccess {
		return fmt.Errorf("只有支付成功的单才能退款，当前状态: %s", payment.Status)
	}

	now := time.Now()
	payment.Status = model.PaymentStatusRefunded
	payment.RefundedAt = &now
	return nil
}
