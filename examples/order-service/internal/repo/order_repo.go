package repo

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"order-service/internal/model"
)

// ErrOrderNotFound 订单不存在
var ErrOrderNotFound = errors.New("订单不存在")

// OrderRepo 订单数据访问层。
//
// 示例项目用内存 map 代替真实数据库，但保留了仓储层应有的并发安全与
// 查询语义（按用户、按状态、按时间范围），方便对照真实实现。
type OrderRepo struct {
	mu     sync.RWMutex
	orders map[string]*model.Order // 主键：订单号
	seq    int64                   // 订单号自增序号
}

// NewOrderRepo 创建订单仓储
func NewOrderRepo() *OrderRepo {
	return &OrderRepo{orders: make(map[string]*model.Order)}
}

// nextID 生成订单号，格式：ORD + yyyyMMdd + 6 位序号
func (r *OrderRepo) nextID(now time.Time) string {
	r.seq++
	return fmt.Sprintf("ORD%s%06d", now.Format("20060102"), r.seq)
}

// Create 保存新订单，返回生成的订单号
func (r *OrderRepo) Create(order *model.Order) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	order.ID = r.nextID(now)
	order.CreatedAt = now
	order.UpdatedAt = now
	order.Status = model.OrderStatusCreated
	order.CalcTotal()

	// 存副本，避免调用方后续修改切片影响仓储内数据
	cp := *order
	cp.Items = append([]model.OrderItem(nil), order.Items...)
	r.orders[order.ID] = &cp

	return cp.ID, nil
}

// GetByID 按订单号查询
func (r *OrderRepo) GetByID(id string) (*model.Order, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	order, ok := r.orders[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrOrderNotFound, id)
	}
	cp := *order
	return &cp, nil
}

// UpdateStatus 更新订单状态，返回是否成功。
// 状态机的合法性由 service 层校验，这里只负责持久化。
func (r *OrderRepo) UpdateStatus(id string, status model.OrderStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	order, ok := r.orders[id]
	if !ok {
		return fmt.Errorf("%w: %s", ErrOrderNotFound, id)
	}
	order.Status = status
	order.UpdatedAt = time.Now()
	if status == model.OrderStatusPaid {
		paidAt := time.Now()
		order.PaidAt = &paidAt
	}
	return nil
}

// ListByUser 查询某用户的订单，按创建时间倒序
func (r *OrderRepo) ListByUser(userID string) []*model.Order {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*model.Order
	for _, order := range r.orders {
		if order.UserID == userID {
			cp := *order
			result = append(result, &cp)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result
}

// ListByStatus 查询处于指定状态的订单，用于对账与补偿任务
func (r *OrderRepo) ListByStatus(status model.OrderStatus) []*model.Order {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []*model.Order
	for _, order := range r.orders {
		if order.Status == status {
			cp := *order
			result = append(result, &cp)
		}
	}
	return result
}

// Count 返回订单总数
func (r *OrderRepo) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.orders)
}
