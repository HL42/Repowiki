package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	deliverycb "order-service/internal/callback/delivery"
	paymentcb "order-service/internal/callback/payment"
	"order-service/internal/handler"
	"order-service/internal/repo"
	deliverysdk "order-service/internal/sdk/delivery"
	paymentsdk "order-service/internal/sdk/payment"
	"order-service/internal/service"
)

// 演示用的回调签名密钥。真实项目应通过环境变量或配置中心注入。
const callbackSecret = "demo-secret-key"

func main() {
	// ---------- 依赖装配 ----------
	// 自下而上：仓储 → SDK → 服务 → 处理器。
	// 示例项目不引入 DI 框架，手工装配反而更直观。
	orderRepo := repo.NewOrderRepo()
	paymentRepo := repo.NewPaymentRepo()

	paySDK := paymentsdk.NewPaymentSDK()
	deliSDK := deliverysdk.NewDeliverySDK()

	orderService := service.NewOrderService(orderRepo, paymentRepo, deliSDK)
	paymentService := service.NewPaymentService(paymentRepo, orderRepo, paySDK)

	orderHandler := handler.NewOrderHandler(orderService)
	paymentHandler := handler.NewPaymentHandler(paymentService)

	payCallback := paymentcb.NewHandler(paymentRepo, orderService, callbackSecret)
	deliCallback := deliverycb.NewHandler(orderService, deliSDK)

	// ---------- 路由 ----------
	mux := http.NewServeMux()
	orderHandler.Register(mux)
	paymentHandler.Register(mux)

	// 第三方平台回调入口，不对外暴露文档
	mux.HandleFunc("/callback/payment", func(w http.ResponseWriter, r *http.Request) {
		handleCallback(w, r, func(ctx context.Context, req paymentcb.CallbackRequest) error {
			// Handle 返回平台约定的应答字符串，非 SUCCESS 一律按失败处理，让平台重投
			result, err := payCallback.Handle(ctx, req)
			if err != nil {
				return err
			}
			if result != paymentcb.ResultSuccess {
				return fmt.Errorf("支付回调处理返回 %s", result)
			}
			return nil
		})
	})
	mux.HandleFunc("/callback/delivery", func(w http.ResponseWriter, r *http.Request) {
		handleCallback(w, r, func(ctx context.Context, req deliverycb.CallbackRequest) error {
			return deliCallback.Handle(ctx, req)
		})
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// ---------- 启动 ----------
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	srv := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("order-service 启动于 :%s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 优雅关闭
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("关闭失败: %v", err)
	}
	log.Println("服务已安全关闭")
}

// handleCallback 统一处理回调入口：解析报文 → 调处理器 → 返回平台约定的应答。
//
// 支付与配送的应答体格式相同（纯文本 SUCCESS/FAIL），
// 因此用泛型函数收敛这两个入口的重复代码。
func handleCallback[T any](w http.ResponseWriter, r *http.Request, process func(context.Context, T) error) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req T
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "FAIL", http.StatusBadRequest)
		return
	}

	if err := process(r.Context(), req); err != nil {
		log.Printf("[callback] 处理失败: %v", err)
		w.WriteHeader(http.StatusOK) // 平台约定：处理失败也用 200 + FAIL
		_, _ = w.Write([]byte("FAIL"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("SUCCESS"))
}
