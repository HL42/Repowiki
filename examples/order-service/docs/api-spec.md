# 订单服务接口文档

- **版本**：v1.2
- **Base URL**：`http://order-service.internal/api`
- **协议**：HTTP + JSON
- **字符集**：UTF-8

所有接口统一返回以下外层结构，业务数据放在 `data` 中：

```json
{
  "code": 0,
  "msg": "ok",
  "data": {}
}
```

`code` 为 `0` 表示成功，非 `0` 时等于 HTTP 状态码，`msg` 为可直接展示给用户的错误原因。

---

## 1. 订单接口

### 1.1 创建订单

`POST /api/order/create`

**请求体**

| 字段 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| user_id | string | 是 | 下单用户 ID |
| shop_id | string | 是 | 门店 ID，格式 `SHOP` + 6 位数字 |
| items | array | 是 | 商品列表，至少一项 |
| items[].sku_id | string | 是 | 商品 SKU |
| items[].name | string | 是 | 商品名称 |
| items[].quantity | int | 是 | 数量，必须为正整数 |
| items[].price | float | 是 | 单价（元），不可为负 |
| delivery_fee | float | 否 | 配送费（元），默认 0 |
| remark | string | 否 | 备注，超过 200 字符会被截断 |

```json
{
  "user_id": "U1001",
  "shop_id": "SHOP000001",
  "items": [
    { "sku_id": "SKU1001", "name": "招牌炒饭", "quantity": 2, "price": 28.5 }
  ],
  "delivery_fee": 5,
  "remark": "不要葱"
}
```

**响应**：返回完整订单对象，`status` 初始为 `created`，`total_amount = 商品小计 + 配送费`。

**错误**

| 场景 | 提示 |
| --- | --- |
| `user_id` 为空 | user_id 不能为空 |
| `shop_id` 格式非法 | shop_id 格式非法: xxx |
| `items` 为空 | 订单至少需要一个商品 |
| 数量非正 | 商品 SKU1001 数量必须为正数 |

---

### 1.2 查询订单详情

`GET /api/order/detail?id=ORD20260101123456`

订单不存在时返回 **404**（而非 500），前端据此展示「订单不存在」的空态。

---

### 1.3 查询用户订单列表

`GET /api/order/list?user_id=U1001`

按创建时间倒序返回该用户的全部订单。用户无订单时 `data` 为空数组，不返回 `null`。

---

### 1.4 取消订单

`POST /api/order/cancel` — `{"order_id":"ORD20260101123456"}`

**只有 `created`（待支付）状态的订单可以直接取消**；已支付订单必须先走退款流程，否则返回 **409**：

```
当前订单状态不可取消: 当前状态 paid，已支付订单请走退款流程
```

---

## 2. 支付接口

### 2.1 发起支付

`POST /api/payment/pay` — `{"order_id":"ORD...","channel":"wechat"}`

`channel` 取值：`wechat`（微信）、`alipay`（支付宝）、`wallet`（余额）。缺省为 `wechat`。

**响应 `data`** 为唤起收银台所需参数：

| 渠道 | 关键字段 |
| --- | --- |
| wechat | prepay_id, nonce_str, time_stamp |
| alipay | trade_no, order_str |
| wallet | balance_after |

订单不存在、或订单处于终态时返回 **400**。

### 2.2 发起退款

`POST /api/payment/refund` — `{"order_id":"ORD...","reason":"用户申请取消"}`

`reason` 缺省为「用户申请退款」。退款成功后订单状态变为 `refunded`。
重复退款返回 **409**，由 SDK 层的幂等校验拦截。

### 2.3 查询支付单

`GET /api/payment/query?order_id=ORD...` — 返回该订单关联的支付单，含 `transaction_id` 与结算状态。

---

## 3. 回调接口

回调接口**不对外暴露**，仅接受第三方平台的服务器 IP，通过 HMAC-SHA256 验签。

### 3.1 支付结果回调

`POST /callback/payment`

```json
{
  "payment_id": "PAY20260101123456",
  "order_id": "ORD20260101123456",
  "transaction_id": "420000123420260101",
  "status": "SUCCESS",
  "amount": "5900",
  "sign": "9f86d081..."
}
```

签名算法：除 `sign` 外的字段按 key 字典序拼成 `k1=v1&k2=v2&...&key=<secret>`，取 SHA256 十六进制小写。

**应答**：纯文本 `SUCCESS` 表示已处理完成、平台不再重试；`FAIL` 表示平台需按退避策略重投。
**幂等**：同一 `payment_id` 重复投递不会重复入账。

### 3.2 配送状态回调

`POST /callback/delivery`

| status | 处理 |
| --- | --- |
| `picking` | 骑手已接单，仅记录骑手信息 |
| `delivering` | 配送中，仅记录 |
| `delivered` | 订单推进为 `completed` |
| `canceled` | 记录日志，需人工介入，订单状态不变 |

配送回调不涉及资金，处理失败只记日志、由定时对账任务补偿，不阻塞应答。

---

## 4. 订单状态机

```
created ──支付成功──> paid ──呼叫运力──> delivering ──送达──> completed
   │                    │
   │ 取消                │ 退款
   v                    v
canceled             refunded
```

非法流转（例如 `completed` → `paid`）统一返回 `订单状态流转非法: completed -> paid`。
`completed` / `canceled` / `refunded` 为终态，不允许再流转。

---

## 5. 通用错误码

| HTTP | 含义 |
| --- | --- |
| 400 | 参数缺失或格式错误 |
| 404 | 资源不存在（订单/支付单） |
| 405 | 请求方法不支持 |
| 409 | 状态冲突（重复退款、非法状态流转） |
| 500 | 服务内部错误 |
