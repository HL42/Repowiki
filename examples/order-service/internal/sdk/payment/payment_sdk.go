package payment

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// CreatePrepayRequest 预支付下单请求
type CreatePrepayRequest struct {
	PaymentID string  `json:"payment_id"`
	OrderID   string  `json:"order_id"`
	UserID    string  `json:"user_id"`
	Channel   string  `json:"channel"` // wechat / alipay / wallet
	Amount    float64 `json:"amount"`
	Subject   string  `json:"subject"`
}

// PaymentSDK 支付平台适配层。
//
// 屏蔽各渠道（微信/支付宝/余额）的协议差异，对上暴露统一的
// 「预支付 / 退款 / 查单」三个动作。真实实现里这里会做签名、
// 证书加载与 HTTP 调用；示例项目用内存实现替代。
type PaymentSDK struct {
	mu       sync.Mutex
	refunds  map[string]float64 // 流水号 -> 已退金额，用于防止重复退款
	requests int                // 累计调用次数，便于演示观测
}

// NewPaymentSDK 创建支付 SDK
func NewPaymentSDK() *PaymentSDK {
	return &PaymentSDK{refunds: make(map[string]float64)}
}

// ErrUnsupportedChannel 不支持的支付渠道
var ErrUnsupportedChannel = errors.New("不支持的支付渠道")

// CreatePrepay 调用渠道预支付接口，返回唤起收银台所需参数
func (s *PaymentSDK) CreatePrepay(ctx context.Context, req CreatePrepayRequest) (map[string]string, error) {
	if req.Amount <= 0 {
		return nil, fmt.Errorf("支付金额必须大于 0，当前: %.2f", req.Amount)
	}
	switch req.Channel {
	case "wechat", "alipay", "wallet":
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedChannel, req.Channel)
	}

	s.mu.Lock()
	s.requests++
	s.mu.Unlock()

	// 真实实现：此处会向渠道网关发起 HTTP 请求并解析 prepay_id
	prepayID := fmt.Sprintf("prepay_%s_%d", req.PaymentID, time.Now().Unix())

	switch req.Channel {
	case "wechat":
		return map[string]string{
			"prepay_id": prepayID,
			"nonce_str": "demo-nonce",
			"package":   "Sign=WXPay",
		}, nil
	case "alipay":
		return map[string]string{
			"trade_no":  prepayID,
			"order_str": fmt.Sprintf("alipay_sdk=%s", prepayID),
		}, nil
	default: // wallet
		return map[string]string{
			"wallet_token": prepayID,
		}, nil
	}
}

// Refund 发起退款。同一流水号只允许退一次，重复调用直接返回错误。
func (s *PaymentSDK) Refund(ctx context.Context, transactionID string, amount float64, reason string) error {
	if transactionID == "" {
		return errors.New("缺少第三方流水号，无法退款")
	}
	if amount <= 0 {
		return fmt.Errorf("退款金额必须大于 0，当前: %.2f", amount)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.refunds[transactionID]; exists {
		return fmt.Errorf("流水号 %s 已退款，请勿重复提交", transactionID)
	}
	s.refunds[transactionID] = amount
	return nil
}

// QueryTransaction 查询第三方交易状态，用于对账与补偿任务
func (s *PaymentSDK) QueryTransaction(ctx context.Context, transactionID string) (string, error) {
	if transactionID == "" {
		return "", errors.New("流水号不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, refunded := s.refunds[transactionID]; refunded {
		return "refunded", nil
	}
	return "success", nil
}

// CallCount 返回累计调用次数（供监控面板使用）
func (s *PaymentSDK) CallCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.requests
}
