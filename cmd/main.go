package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"repowiki/internal/api"
	"repowiki/internal/llm"
	"repowiki/internal/storage"
	"repowiki/internal/wiki"
)

func main() {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")

	if apiKey == "" {

		if data, err := os.ReadFile(".env"); err == nil {

			apiKey = parseEnvValue(string(data), "DEEPSEEK_API_KEY")
		}
	}

	if apiKey == "" {

		log.Fatal("请设置环境变量 DEEPSEEK_API_KEY 或在项目根目录创建 .env 文件")
	}

	model := os.Getenv("DEEPSEEK_MODEL")
	if model == "" {
		model = "deepseek-chat"
	}

	wikiRoot := os.Getenv("WIKI_ROOT")
	if wikiRoot == "" {
		wikiRoot = "./knowledge-base"
	}

	// 初始化存储
	fs, err := storage.NewFileSystem(wikiRoot)
	if err != nil {
		log.Fatalf("初始化文件系统失败: %v", err)
	}
	if err := fs.InitWiki(); err != nil {
		log.Fatalf("初始化 Wiki 失败: %v", err)
	}

	// 初始化 LLM 客户端
	llmClient := llm.NewDeepSeekClient(apiKey, model)

	// 初始化引擎（复用已创建的 FileSystem，避免重复示例化）
	engine := wiki.NewEngine(llmClient, wikiRoot, fs)

	// 启动时从磁盘全量构建搜索索引
	fmt.Println("构建搜索索引...")
	if err := engine.InitSearcher(); err != nil {
		log.Printf("索引构建失败（不影响运行）: %v", err)
	}

	// 子命令分发
	args := os.Args[1:]
	if len(args) >= 1 && args[0] == "ingest" {
		dirPath := "."
		if len(args) >= 2 {
			dirPath = args[1]
		}
		fmt.Printf("开始导入: %s\n", dirPath)
		successCount, errs := engine.IngestAll(context.Background(), dirPath)
		fmt.Printf("导入完成: 成功 %d 个\n", successCount)
		for _, e := range errs {
			fmt.Printf("  ! %v\n", e)
		}
		return
	}

	if len(args) >= 1 && args[0] == "cleanup-stubs" {
		dryRun := len(args) >= 2 && args[1] == "--dry-run"
		if dryRun {
			fmt.Println("预览将删除的空壳 entity/concept 页：")
		} else {
			fmt.Println("清理空壳 entity/concept 页...")
		}
		n, err := engine.CleanupStubPages(dryRun)
		if err != nil {
			log.Fatalf("清理失败: %v", err)
		}
		if dryRun {
			fmt.Printf("共 %d 个空壳页可删除（确认后去掉 --dry-run 执行）\n", n)
		} else {
			fmt.Printf("已删除 %d 个空壳页\n", n)
		}
		return
	}

	// 默认：启动 HTTP 服务
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// 启动前检查端口是否被占用
	if conn, err := net.DialTimeout("tcp", ":"+port, 200*time.Millisecond); err == nil {
		conn.Close()
		fmt.Printf("警告：端口 %s 已被占用，请先关闭占用该端口的进程后重试\n", port)
		log.Fatalf("端口 %s 已被占用，无法启动", port)
	}

	handler := api.NewServer(engine)
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// 优雅关闭：监听 SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Println("========================================")
		fmt.Println("  RepoWiki 已启动")
		fmt.Println("========================================")
		fmt.Println()
		fmt.Println("API 端点:")
		fmt.Println("  POST /api/ingest      - 导入文档")
		fmt.Println("  POST /api/query       - 知识问答")
		fmt.Println("  POST /api/synthesize  - 综合提炼")
		fmt.Println("  GET  /api/stats       - 知识库统计")
		fmt.Println()
		fmt.Println("命令行:")
		fmt.Println("  go run cmd/main.go ingest /path/to/docs  - 离线导入")
		fmt.Println()
		fmt.Printf("监听端口: %s\n", port)
		if _, err := os.Stat("./frontend/dist"); err == nil {
			fmt.Println("Web UI: http://localhost:" + port + "/")
		}

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	<-quit
	fmt.Println("\n正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("关闭失败: %v", err)
	}
	fmt.Println("服务已安全关闭")
}

// parseEnvValue 从 .env 文件内容中解析 key 对应的值
func parseEnvValue(content, key string) string {
	prefix := key + "="
	for _, line := range splitLines(content) {
		line = trimLine(line)
		if len(line) >= len(prefix) && line[:len(prefix)] == prefix {
			val := line[len(prefix):]
			// 去掉可能的引号
			if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'')) {
				val = val[1 : len(val)-1]
			}
			return val
		}
	}
	return ""
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func trimLine(s string) string {
	// 去除首尾空白
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\r') {
		end--
	}
	if start > 0 || end < len(s) {
		s = s[start:end]
	}
	// 跳过注释行
	if len(s) > 0 && s[0] == '#' {
		return ""
	}
	return s
}
