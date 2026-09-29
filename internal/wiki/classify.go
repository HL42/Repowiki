package wiki

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// classifyCodeFile 按文件路径规则自动分类代码文件（不用 LLM）
// 返回 layer（技术层级）、domain（业务域）、kind（代码类型）
func classifyCodeFile(rawPath string) (layer, domain, kind string) {
	lower := strings.ToLower(rawPath)

	// 1. 技术层级：按目录路径模式
	layer = matchLayer(lower)

	// 2. 业务域：按路径 + 文件名关键词
	domain = matchDomain(lower)

	// 3. 代码类型：由层级映射
	kind = layerToKind(layer)

	return
}

// ---------- 规则类型 ----------

type classifyDomainRule struct {
	Keywords []string `json:"keywords"`
	Domain   string   `json:"domain"`
}

type classifyLayerRule struct {
	Keywords []string `json:"keywords"`
	Layer    string   `json:"layer"`
}

type classifyRulesConfig struct {
	Domains []classifyDomainRule `json:"domains"`
	Layers  []classifyLayerRule  `json:"layers"`
}

// ---------- 默认规则（classify_rules.json 不存在时使用） ----------

var defaultDomainRules = []classifyDomainRule{
	{[]string{"order", "订单"}, "订单"},
	{[]string{"delivery", "distribution", "配送", "运力", "dispatch"}, "配送"},
	{[]string{"shop", "store", "门店", "商家", "merchant"}, "门店"},
	{[]string{"receipt", "print", "小票", "打印", "printer"}, "收据"},
	{[]string{"payment", "pay", "refund", "支付", "退款", "tip", "小费"}, "支付"},
	{[]string{"user", "login", "auth", "用户", "登录", "注册", "signup", "verify"}, "用户"},
	{[]string{"stat", "report", "统计", "报表", "看板", "dashboard", "analytics"}, "统计"},
	{[]string{"message", "push", "消息", "通知", "推送", "sms", "email"}, "消息"},
	{[]string{"callback", "notify", "webhook", "回调", "通知"}, "回调"},
	{[]string{"rider", "骑手", "位置", "location", "position"}, "骑手"},
	{[]string{"config", "配置", "setting", "环境", "env"}, "配置"},
	{[]string{"admin", "管理", "后台"}, "管理"},
	{[]string{"marketing", "营销", "campaign", "promotion"}, "营销"},
	{[]string{"review", "评价", "reply", "回复"}, "评价"},
}

var defaultLayerRules = []classifyLayerRule{
	{[]string{"handler", "controller", "api"}, "handler"},
	{[]string{"service", "logic", "biz", "usecase"}, "service"},
	{[]string{"model", "entity", "schema", "domain"}, "model"},
	{[]string{"repo", "repository", "dao", "mapper"}, "repository"},
	{[]string{"route", "router"}, "route"},
	{[]string{"sdk", "client", "adapter", "platform"}, "sdk"},
	{[]string{"callback", "notify", "webhook"}, "callback"},
	{[]string{"job", "cron", "task", "schedule"}, "job"},
	{[]string{"config", "cfg", "setting", "etc"}, "config"},
	{[]string{"util", "helper", "common", "lib"}, "util"},
	{[]string{"page", "pages", "view", "views"}, "page"},
	{[]string{"component", "components", "widget"}, "component"},
	{[]string{"store", "state", "vuex", "pinia"}, "store"},
	{[]string{"rpc", "proto", "grpc"}, "api"},
	{[]string{"middleware", "interceptor", "filter"}, "middleware"},
	{[]string{"docs", "doc", "readme"}, "documentation"},
}

// 运行时规则（init 时从 classify_rules.json 加载，失败则用默认）
var domainRules []classifyDomainRule
var layerRules []classifyLayerRule

func init() {
	domainRules, layerRules = loadClassifyRules()
}

// loadClassifyRules 尝试从 classify_rules.json 加载分类规则。
// 查找顺序：当前目录 → 上级目录 → 默认规则
func loadClassifyRules() ([]classifyDomainRule, []classifyLayerRule) {
	paths := []string{
		"classify_rules.json",
		"../classify_rules.json",
		"../../classify_rules.json",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			data, err := os.ReadFile(p)
			if err != nil {
				continue
			}
			var cfg classifyRulesConfig
			if err := json.Unmarshal(data, &cfg); err != nil {
				fmt.Printf("[WARN] classify_rules.json 解析失败: %v，使用默认规则\n", err)
				return defaultDomainRules, defaultLayerRules
			}
			if len(cfg.Domains) > 0 || len(cfg.Layers) > 0 {
				fmt.Printf("[INFO] 已加载分类规则: classify_rules.json (%d 域, %d 层)\n",
					len(cfg.Domains), len(cfg.Layers))
				if len(cfg.Domains) == 0 {
					cfg.Domains = defaultDomainRules
				}
				if len(cfg.Layers) == 0 {
					cfg.Layers = defaultLayerRules
				}
				return cfg.Domains, cfg.Layers
			}
		}
	}

	return defaultDomainRules, defaultLayerRules
}

// ---------- 层级匹配 ----------

func matchLayer(pathLower string) string {
	for _, rule := range layerRules {
		for _, kw := range rule.Keywords {
			if strings.Contains(pathLower, "/"+kw+"/") || strings.HasPrefix(pathLower, kw+"/") ||
				strings.Contains(pathLower, "/"+kw+"_") || strings.Contains(pathLower, "/"+kw+"s/") ||
				strings.HasPrefix(pathLower, kw+"s/") {
				return rule.Layer
			}
		}
	}
	base := filepath.Base(pathLower)
	switch base {
	case "main.go", "main.js", "main.ts", "index.js", "index.ts", "app.vue", "app.js":
		return "entry"
	}
	return ""
}

// ---------- 业务域匹配 ----------

func matchDomain(pathLower string) string {
	parts := strings.Split(pathLower, "/")
	base := filepath.Base(pathLower)

	for _, rule := range domainRules {
		for _, kw := range rule.Keywords {
			for _, part := range parts {
				if part == kw {
					return rule.Domain
				}
				if len(kw) > 3 && strings.Contains(part, kw) {
					return rule.Domain
				}
			}
			if strings.Contains(base, kw) {
				return rule.Domain
			}
		}
	}
	return ""
}

// ---------- 类型映射 ----------

func layerToKind(layer string) string {
	switch layer {
	case "handler", "controller", "route", "api":
		return "API入口"
	case "service", "logic", "biz", "usecase":
		return "业务逻辑"
	case "model", "entity", "schema":
		return "数据模型"
	case "repository", "dao", "mapper":
		return "数据访问"
	case "sdk", "client", "adapter":
		return "外部平台适配"
	case "callback":
		return "回调处理"
	case "job", "cron", "task":
		return "定时任务"
	case "config", "setting":
		return "配置"
	case "util", "helper":
		return "工具函数"
	case "page", "view":
		return "前端页面"
	case "component":
		return "前端组件"
	case "store":
		return "状态管理"
	case "middleware":
		return "中间件"
	case "entry":
		return "应用入口"
	case "documentation":
		return "文档"
	default:
		return ""
	}
}

// ---------- 显示标签 ----------

func codeDomainLabel(domain string) string {
	if domain == "" {
		return "未分类"
	}
	return domain
}

func codeKindLabel(kind string) string {
	if kind == "" {
		return "未分类"
	}
	return kind
}

func codeLayerLabel(layer string) string {
	switch layer {
	case "handler":
		return "API 入口"
	case "service":
		return "业务逻辑"
	case "model":
		return "数据模型"
	case "repository":
		return "数据访问"
	case "route":
		return "路由"
	case "sdk":
		return "平台适配"
	case "callback":
		return "回调处理"
	case "job":
		return "定时任务"
	case "config":
		return "配置"
	case "util":
		return "工具"
	case "page":
		return "页面"
	case "component":
		return "组件"
	case "store":
		return "状态管理"
	case "api":
		return "API 入口"
	case "middleware":
		return "中间件"
	case "entry":
		return "入口"
	case "documentation":
		return "文档"
	default:
		return layer
	}
}
