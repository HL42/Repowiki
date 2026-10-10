package model

import (
	"slices"
	"time"
)

// OrderStatus 订单状态机
//
//	created ──支付成功──> paid ──发起配送──> delivering ──送达──> completed
//	   │                    │
//	   └──取消/超时──> canceled  └──退款──> refunded
type OrderStatus string

const (
	OrderStatusCreated    OrderStatus = "created"    // 已创建，待支付
	OrderStatusPaid       OrderStatus = "paid"       // 已支付，待配送
	OrderStatusDelivering OrderStatus = "delivering" // 配送中
	OrderStatusCompleted  OrderStatus = "completed"  // 已完成
	OrderStatusCanceled   OrderStatus = "canceled"   // 已取消
	OrderStatusRefunded   OrderStatus = "refunded"   // 已退款
)

// 允许的状态流转，用于校验非法跃迁（例如从 created 直接跳到 completed）
var allowedTransitions = map[OrderStatus][]OrderStatus{
	OrderStatusCreated:    {OrderStatusPaid, OrderStatusCanceled},
	OrderStatusPaid:       {OrderStatusDelivering, OrderStatusRefunded},
	OrderStatusDelivering: {OrderStatusCompleted},
	OrderStatusCompleted:  {},
	OrderStatusCanceled:   {},
	OrderStatusRefunded:   {},
}

// CanTransitionTo 判断订单能否从当前状态流转到 target
func (s OrderStatus) CanTransitionTo(target OrderStatus) bool {
	return slices.Contains(allowedTransitions[s], target)
}

// IsFinal 判断是否为终态（终态订单不可再修改）
func (s OrderStatus) IsFinal() bool {
	return s == OrderStatusCompleted || s == OrderStatusCanceled || s == OrderStatusRefunded
}

// OrderItem 订单中的单个商品条目
type OrderItem struct {
	SKUID    string  `json:"sku_id"`
	Name     string  `json:"name"`
	Quantity int     `json:"quantity"`
	Price    float64 `json:"price"` // 单价，单位：元
}

// Amount 计算该条目的小计金额
func (i OrderItem) Amount() float64 {
	return i.Price * float64(i.Quantity)
}

// Order 订单主表
type Order struct {
	ID          string      `json:"id"`           // 订单号，格式 ORD + yyyyMMdd + 6 位序号
	UserID      string      `json:"user_id"`      // 下单用户
	ShopID      string      `json:"shop_id"`      // 门店
	Status      OrderStatus `json:"status"`       // 订单状态
	Items       []OrderItem `json:"items"`        // 商品明细
	TotalAmount float64     `json:"total_amount"` // 订单总金额
	DeliveryFee float64     `json:"delivery_fee"` // 配送费
	Remark      string      `json:"remark"`       // 用户备注
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	PaidAt      *time.Time  `json:"paid_at,omitempty"` // 支付时间，未支付为 nil
}

// PayableAmount 应付金额 = 商品总额 + 配送费
func (o *Order) PayableAmount() float64 {
	return o.TotalAmount + o.DeliveryFee
}

// CalcTotal 根据商品明细重算订单总金额
func (o *Order) CalcTotal() {
	var total float64
	for _, item := range o.Items {
		total += item.Amount()
	}
	o.TotalAmount = total
}
