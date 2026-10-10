package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"repowiki/internal/api"
	"repowiki/internal/config"
	"repowiki/internal/llm"
	"repowiki/internal/storage"
	"repowiki/internal/wiki"
)

// Version 由构建时注入（-X main.Version=v1.2.3），未注入时为 dev
var Version = "dev"

func main() {
	args := os.Args[1:]

	// 信息类参数最先处理：不读 .env、不加载配置，输出保持干净
	noOpen := false
	var portFile string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--version", "-v":
			fmt.Printf("repowiki %s\n", Version)
			return
		case "--help", "-h":
			printUsage()
			return
		case "--no-open", "--no-browser":
			noOpen = true
		case "--port-file":
			// 供 macOS 原生壳使用：绑定成功后把实际端口写进该文件，壳据此拼 URL
			if i+1 < len(args) {
				portFile = args[i+1]
				i++
			}
		}
	}

	// 读取配置（环境变量 > .env 文件 > 默认值），并定位打包资源目录
	settings := config.Load()
	resourceDir := config.ResourceDir()

	// 未配置 Key 不再致命退出：首次启动在浏览器里引导用户填写（见 /api/config）
	if settings.APIKey == "" {
		log.Printf("未配置 DeepSeek API Key，启动后请在浏览器引导页中填写（或创建 .env 文件）")
	}

	// 初始化存储
	fs, err := storage.NewFileSystem(settings.WikiRoot)
	if err != nil {
		log.Fatalf("初始化文件系统失败: %v", err)
	}
	if err := fs.InitWiki(); err != nil {
		log.Fatalf("初始化 Wiki 失败: %v", err)
	}

	// 初始化 LLM 客户端（可能暂为空 Key，运行期可通过 /api/config 更新）
	llmClient := llm.NewDeepSeekClient(settings.APIKey, settings.Model)

	// 初始化引擎（复用已创建的 FileSystem，避免重复实例化）
	engine := wiki.NewEngine(llmClient, settings.WikiRoot, fs)

	// 启动时从磁盘全量构建搜索索引
	fmt.Println("构建搜索索引...")
	if err := engine.InitSearcher(); err != nil {
		log.Printf("索引构建失败（不影响运行）: %v", err)
	}

	// 子命令分发（ingest / cleanup-stubs 仍保留 CLI 用法）
	if len(args) >= 1 {
		switch args[0] {
		case "ingest":
			if settings.APIKey == "" {
				log.Printf("警告：未配置 API Key，ingest 会失败；请先设置 DEEPSEEK_API_KEY")
			}
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

		case "cleanup-stubs":
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

		default:
			if !strings.HasPrefix(args[0], "-") {
				fmt.Fprintf(os.Stderr, "未知参数: %s\n\n", args[0])
				printUsage()
				os.Exit(2)
			}
		}
	}

	// 默认：启动 HTTP 服务
	// 仅监听本机回环地址：本工具定位为「下载到本地个人使用」，
	// 不暴露到局域网（目录浏览接口会列出本机磁盘，必须限制访问来源）
	const host = "127.0.0.1"

	// 用真实的监听来绑定端口：被占用时自动换一个空闲端口，
	// 而不是像旧版那样报错退出（用户浏览器里则可能打开到占用端口的其它服务）
	ln, err := net.Listen("tcp", net.JoinHostPort(host, settings.Port))
	if err != nil {
		log.Printf("端口 %s 不可用，自动改用随机空闲端口", settings.Port)
		ln, err = net.Listen("tcp", net.JoinHostPort(host, "0"))
		if err != nil {
			log.Fatalf("无法监听回环地址: %v", err)
		}
	}
	settings.Port = strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)

	// 把实际端口写给 macOS 原生壳（--port-file 指定）
	if portFile != "" {
		if err := os.WriteFile(portFile, []byte(settings.Port+"\n"), 0600); err != nil {
			log.Printf("写入端口文件失败（忽略）: %v", err)
		}
	}

	// 运行期改 Key 时的落盘实现：把当前生效配置（含最终端口）写回 .env
	saveConfig := func(apiKey, model string) error {
		s := settings
		s.APIKey = apiKey
		s.Model = model
		return config.SaveEnv(s)
	}

	handler := api.NewServer(engine, llmClient, saveConfig, resourceDir)
	srv := &http.Server{
		Addr:    net.JoinHostPort(host, settings.Port),
		Handler: handler,
	}

	// 优雅关闭：监听 SIGINT/SIGTERM
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		fmt.Println("========================================")
		fmt.Printf("  RepoWiki %s 已启动\n", Version)
		fmt.Println("========================================")
		fmt.Println()
		fmt.Printf("监听地址: http://%s\n", srv.Addr)
		if resourceDir != "" {
			fmt.Println("资源目录: " + resourceDir)
		}
		if settings.WikiRoot != "" {
			fmt.Println("数据目录: " + settings.WikiRoot)
		}
		if settings.APIKey == "" {
			fmt.Println("提示：尚未配置 API Key，浏览器会打开引导页引导填写。")
		}
		fmt.Println()

		if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	// 服务真正监听后（ln 已就绪）再打开浏览器，URL 用实际绑定地址
	if !noOpen {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser("http://" + srv.Addr + "/")
		}()
	}

	<-quit
	fmt.Println("\n正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("关闭失败: %v", err)
	}
	fmt.Println("服务已安全关闭")
}

// openBrowser 用系统默认浏览器打开 URL。
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}

// printUsage 打印命令行用法
func printUsage() {
	fmt.Print(`RepoWiki - 代码库知识库

用法:
  repowiki [子命令] [参数]

子命令:
  （无）                    启动 Web 服务并自动打开浏览器
  ingest <目录>             离线导入指定目录的代码/文档
  cleanup-stubs [--dry-run] 清理空壳 entity/concept 页

选项:
  -h, --help                显示本帮助
  -v, --version             显示版本号
  --no-open                 启动服务但不自动打开浏览器

配置（环境变量 > .env 文件 > 默认值）:
  DEEPSEEK_API_KEY  可选，DeepSeek API Key（也可在网页引导页填写）
  DEEPSEEK_MODEL    可选，默认 deepseek-chat
  PORT              可选，默认 8080（被占用时自动换空闲端口）
  WIKI_ROOT         可选，默认 ./knowledge-base（打包后为系统用户数据目录）
`)
}
