package payment

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"

	"order-service/internal/repo"
	"order-service/internal/service"
)

// 回调处理结果，返回给支付平台决定是否重试
const (
	ResultSuccess = "SUCCESS" // 处理成功，平台不再重试
	ResultFail    = "FAIL"    // 处理失败，平台按策略重试
)

var (
	// ErrInvalidSignature 签名校验失败
	ErrInvalidSignature = errors.New("回调签名校验失败")
	// ErrMissingField 回调缺少必填字段
	ErrMissingField = errors.New("回调缺少必填字段")
)

// CallbackRequest 支付平台回调报文
type CallbackRequest struct {
	PaymentID     string `json:"payment_id"`     // 商户支付单号
	OrderID       string `json:"order_id"`       // 商户订单号
	TransactionID string `json:"transaction_id"` // 平台流水号
	Status        string `json:"status"`         // SUCCESS / FAIL
	Amount        string `json:"amount"`         // 金额（分为单位，字符串）
	Sign          string `json:"sign"`           // 签名
}

// Handler 支付回调处理器。
//
// 处理流程：验签 → 校验必填字段 → 幂等更新支付单 → 推进订单状态。
// 任何一步失败都返回 ResultFail，让平台按退避策略重试，而不是吞掉异常。
type Handler struct {
	paymentRepo  *repo.PaymentRepo
	orderService *service.OrderService
	secretKey    string
}

// NewHandler 创建回调处理器
func NewHandler(paymentRepo *repo.PaymentRepo, orderService *service.OrderService, secretKey string) *Handler {
	return &Handler{
		paymentRepo:  paymentRepo,
		orderService: orderService,
		secretKey:    secretKey,
	}
}

// Handle 处理支付结果回调
func (h *Handler) Handle(ctx context.Context, req CallbackRequest) (string, error) {
	if err := h.verify(req); err != nil {
		log.Printf("[payment-callback] 验签失败 order=%s: %v", req.OrderID, err)
		return ResultFail, err
	}
	if req.PaymentID == "" || req.OrderID == "" {
		return ResultFail, ErrMissingField
	}

	switch strings.ToUpper(req.Status) {
	case "SUCCESS":
		if err := h.handleSuccess(ctx, req); err != nil {
			return ResultFail, err
		}
	case "FAIL":
		if err := h.paymentRepo.MarkFailed(req.PaymentID, "平台返回支付失败"); err != nil {
			return ResultFail, err
		}
	default:
		return ResultFail, fmt.Errorf("未知的回调状态: %s", req.Status)
	}

	// 返回 SUCCESS 表示「已收到且处理完成」，平台不再重试
	return ResultSuccess, nil
}

// handleSuccess 支付成功：先结算支付单，再推进订单状态。
// 两步都需要幂等——平台可能重复投递同一条回调。
func (h *Handler) handleSuccess(ctx context.Context, req CallbackRequest) error {
	if err := h.paymentRepo.MarkSuccess(req.PaymentID, req.TransactionID); err != nil {
		return fmt.Errorf("更新支付单失败: %w", err)
	}
	if err := h.orderService.MarkPaid(ctx, req.OrderID); err != nil {
		// 订单推进失败不回滚支付单：钱已到账，由补偿任务重试状态推进
		log.Printf("[payment-callback] 推进订单状态失败 order=%s: %v", req.OrderID, err)
		return err
	}
	return nil
}

// verify 校验回调签名。
// 规则：除 sign 外的所有字段按 key 字典序拼接，与 secretKey 做 HMAC-SHA256。
func (h *Handler) verify(req CallbackRequest) error {
	if req.Sign == "" {
		return ErrInvalidSignature
	}
	expect := signOf(req, h.secretKey)
	if !hmac.Equal([]byte(expect), []byte(req.Sign)) {
		return ErrInvalidSignature
	}
	return nil
}

// signOf 按约定算法计算签名，导出以便测试与联调工具复用
func signOf(req CallbackRequest, secretKey string) string {
	fields := map[string]string{
		"payment_id":     req.PaymentID,
		"order_id":       req.OrderID,
		"transaction_id": req.TransactionID,
		"status":         req.Status,
		"amount":         req.Amount,
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for _, k := range keys {
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(fields[k])
		sb.WriteString("&")
	}
	sb.WriteString("key=")
	sb.WriteString(secretKey)

	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])
}

// Sign 供测试构造合法回调
func Sign(req CallbackRequest, secretKey string) string {
	return signOf(req, secretKey)
}
