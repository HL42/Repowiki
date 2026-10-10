package model

import "time"

// PaymentStatus 支付单状态
type PaymentStatus string

const (
	PaymentStatusPending  PaymentStatus = "pending"  // 已创建，等待用户支付
	PaymentStatusSuccess  PaymentStatus = "success"  // 支付成功
	PaymentStatusFailed   PaymentStatus = "failed"   // 支付失败
	PaymentStatusRefunded PaymentStatus = "refunded" // 已退款
)

// PaymentChannel 支付渠道
type PaymentChannel string

const (
	ChannelWechat PaymentChannel = "wechat" // 微信支付
	ChannelAlipay PaymentChannel = "alipay" // 支付宝
	ChannelWallet PaymentChannel = "wallet" // 平台余额
)

// Payment 支付单。一个订单可能对应多次支付尝试（用户取消后重新支付），
// 因此订单号与支付单号是一对多关系。
type Payment struct {
	ID            string         `json:"id"`             // 支付单号，格式 PAY + yyyyMMdd + 6 位序号
	OrderID       string         `json:"order_id"`       // 关联订单号
	UserID        string         `json:"user_id"`        // 付款人
	Channel       PaymentChannel `json:"channel"`        // 支付渠道
	Status        PaymentStatus  `json:"status"`         // 支付状态
	Amount        float64        `json:"amount"`         // 支付金额
	TransactionID string         `json:"transaction_id"` // 第三方平台流水号
	FailReason    string         `json:"fail_reason,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	PaidAt        *time.Time     `json:"paid_at,omitempty"`
	RefundedAt    *time.Time     `json:"refunded_at,omitempty"`
}

// IsSettled 判断支付单是否已终结（成功或已退款），终结后不再接受状态变更
func (p *Payment) IsSettled() bool {
	return p.Status == PaymentStatusSuccess || p.Status == PaymentStatusRefunded
}
