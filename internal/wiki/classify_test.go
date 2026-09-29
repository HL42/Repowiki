package wiki

import "testing"

func TestClassifyCodeFile(t *testing.T) {
	tests := []struct {
		path       string
		wantLayer  string
		wantDomain string
		wantKind   string
	}{
		// Go 项目
		{"internal/handler/orderhandler.go", "handler", "订单", "API入口"},
		{"internal/logic/sendorderlogic.go", "service", "订单", "业务逻辑"},
		{"internal/model/ordermodel.go", "model", "订单", "数据模型"},
		{"internal/repo/ordermodelrepo.go", "repository", "订单", "数据访问"},
		{"callback/payment/callback.go", "callback", "支付", "回调处理"},
		{"sdks/payment/paymentsdk.go", "sdk", "支付", "外部平台适配"},
		{"rpc/delivery/delivery.proto", "api", "配送", "API入口"},
		{"main.go", "entry", "", "应用入口"},

		// Node.js 项目
		{"src/controller/OrderController.js", "handler", "订单", "API入口"},
		{"src/service/OrderService.js", "service", "订单", "业务逻辑"},
		{"src/model/Order.js", "model", "订单", "数据模型"},

		// Vue 项目
		{"pages/order/detail.vue", "page", "订单", "前端页面"},
		{"pages/delivery/list.vue", "page", "配送", "前端页面"},
		{"components/OrderCard.vue", "component", "订单", "前端组件"},
		{"App.vue", "entry", "", "应用入口"},

		// 无匹配
		{"some/random/file.go", "", "", ""},
	}

	for _, tt := range tests {
		layer, domain, kind := classifyCodeFile(tt.path)
		if layer != tt.wantLayer {
			t.Errorf("%s: layer = %q, want %q", tt.path, layer, tt.wantLayer)
		}
		if domain != tt.wantDomain {
			t.Errorf("%s: domain = %q, want %q", tt.path, domain, tt.wantDomain)
		}
		if kind != tt.wantKind {
			t.Errorf("%s: kind = %q, want %q", tt.path, kind, tt.wantKind)
		}
	}
}

func TestClassifyCodeFileLabels(t *testing.T) {
	if codeDomainLabel("") != "未分类" {
		t.Error("empty domain should show 未分类")
	}
	if codeDomainLabel("订单") != "订单" {
		t.Error("known domain should pass through")
	}
	if codeKindLabel("") != "未分类" {
		t.Error("empty kind should show 未分类")
	}
	if codeLayerLabel("handler") != "API 入口" {
		t.Errorf("handler label = %q, want API 入口", codeLayerLabel("handler"))
	}
}
