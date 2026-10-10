# order-service

一个用于演示 RepoWiki 导入能力的示例订单服务。

> 这个项目是 **RepoWiki 自带的示例数据**，代码为教学用途的简化实现，不依赖任何外部中间件，
> 可以直接导入 RepoWiki 体验「导入代码 → 自动分类 → 问答」的完整流程。

## 它演示了什么

- **技术分层识别**：`handler` / `service` / `repo` / `model` / `callback` / `sdk` 六层结构，
  可被 RepoWiki 的路径规则自动归类；
- **业务域识别**：订单（order）、支付（payment）、配送（delivery）、用户（user）四个业务域；
- **回调与外部适配**：支付回调、配送回调、第三方 SDK 适配层；
- **文档与代码混合**：`docs/` 下同时包含接口文档与需求文档。

## 目录结构

```
order-service/
├── cmd/main.go                          应用入口：装配依赖并注册路由
├── internal/
│   ├── handler/                          HTTP 接口层
│   │   ├── order_handler.go              订单相关接口
│   │   └── payment_handler.go            支付相关接口
│   ├── service/                          业务逻辑层
│   │   ├── order_service.go              订单创建、取消、状态流转
│   │   └── payment_service.go            支付、退款
│   ├── repo/                             数据访问层
│   │   ├── order_repo.go
│   │   └── payment_repo.go
│   ├── model/                            数据模型
│   │   ├── order.go
│   │   └── payment.go
│   ├── callback/                         外部平台回调
│   │   ├── payment/payment_callback.go   支付结果回调
│   │   └── delivery/delivery_callback.go 配送状态回调
│   ├── sdk/                              外部平台适配
│   │   ├── payment/payment_sdk.go
│   │   └── delivery/delivery_sdk.go
│   └── util/validator.go                 通用校验工具
└── docs/
    ├── api-spec.md                       接口文档
    └── prd-order-flow.md                 需求文档
```

## 核心业务流程

1. 用户下单 → `OrderService.CreateOrder` 校验参数、生成订单号、落库；
2. 发起支付 → `PaymentService.PayOrder` 调用支付平台 SDK 创建预支付单；
3. 支付结果 → 支付平台异步回调 `PaymentCallback.Handle`，更新订单状态为「待配送」；
4. 发起配送 → `DeliverySDK.Dispatch` 呼叫运力；
5. 配送状态 → 配送平台回调 `DeliveryCallback.Handle`，推进订单至「已完成」。

## 运行

```bash
go run ./cmd/main.go
```

服务监听 `:8080`，提供 `/api/order/*` 与 `/api/payment/*` 两组接口。
