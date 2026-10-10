# RepoWiki — 把代码仓库变成可对话的 AI 知识库

[English](README.md) | [中文](README.zh-CN.md)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js)](https://vuejs.org/)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

导入你的代码和文档 → 让 LLM 自动阅读整理 → 用自然语言提问。

```
"这个项目的订单处理流程是怎样的？"
"SendOrder 接口需要哪些参数？"
"数据库表结构是什么样的？"
```

## 快速开始

### 下载安装包

从 [Releases](https://github.com/HL42/Repowiki/releases) 下载对应平台的安装包（`v0.1.0` 为例）：

- **macOS Apple Silicon**：`RepoWiki-v0.1.0-arm64.dmg`
- **macOS Intel**：`RepoWiki-v0.1.0-amd64.dmg`
- **Windows x64**：`repowiki-v0.1.0-windows-amd64.zip`

**安装步骤**

- **macOS**：双击 `.dmg` 打开安装盘 → 把 `RepoWiki.app` 拖进「应用程序」→ 双击打开，出现**独立的软件窗口**（不借道浏览器）；
- **Windows**：解压 `zip` → 双击 `repowiki.exe`（无控制台窗口），自动用默认浏览器打开。

打开后左侧「快速开始」有分步引导；**第一次使用时直接在界面里粘贴 [DeepSeek API Key](https://platform.deepseek.com/api_keys) 并保存**，即填即校验、无需碰终端。端口被占用时会自动改用空闲端口，地址始终正确。

想先试手感，可在「导入文档」里直接导入包内自带的示例项目 `examples/order-service`。

> **macOS 提示「无法验证开发者」？** 安装包未做代码签名（Apple 签名需付费账号）。首次打开 `RepoWiki.app` 时：**右键 → 打开 → 确认**即可。
>
> **Windows 提示 SmartScreen 已阻止？** 点「更多信息」→「仍要运行」。

程序**只监听 `127.0.0.1`**，不会暴露到局域网或公网；同时校验请求的 Host 与 Origin，拒绝非本机来源的跨站请求（否则浏览器里任意网页都能读到本机磁盘）。API Key 与知识库数据保存在本机用户数据目录（macOS `~/Library/Application Support/RepoWiki`、Windows `%APPDATA%\RepoWiki`）。

### 从源码

```bash
git clone https://github.com/HL42/Repowiki.git
cd Repowiki
echo 'DEEPSEEK_API_KEY=sk-your-key' > .env   # 或留空，启动后在浏览器引导页填写
go build -o repowiki ./cmd/... && ./repowiki
# 自动打开 http://127.0.0.1:8080
```

自带的前端产物若不存在，先构建一次：`cd frontend && npm install && npm run build`。
`./repowiki --version` 可查看版本，`./repowiki --help` 查看命令行用法，`./repowiki --no-open` 启动但不自动开浏览器。

## 功能

| 功能 | 说明 |
|---|---|
| **导入代码** | 支持 Go / Python / JS / TS / Vue / Java / Rust / Markdown |
| **自动分类** | 按技术层级（handler/service/model）和业务域（订单/支付/用户）归类 |
| **代码地图** | 导入完成自动生成「代码模块索引」，一眼看清项目结构 |
| **智能问答** | RAG 检索增强生成，支持多轮对话 |
| **增量更新** | SHA256 哈希对比，只处理变化的文件 |
| **质量检测** | 自动检测 LLM 幻觉、空壳页面、过期文档 |

## 工作原理

```
你的代码 / 文档
    │
    ▼
  扫描 → 跳过测试 / mock / 生成代码
    │
    ▼
  分类 → 识别层级 + 业务域（编辑 classify_rules.json 适配你的领域）
    │
    ▼
  LLM 阅读 → 每份文件生成结构化摘要
    │
    ▼
  倒排索引 → 毫秒级关键词搜索
    │
    ▼
  RAG 问答 ← 用户提问
```

## 适配你的业务

编辑 `classify_rules.json`，添加你自己的业务关键词：

```json
{
  "domains": [
    {"keywords": ["inventory", "库存"], "domain": "库存"},
    {"keywords": ["billing", "计费"], "domain": "计费"}
  ]
}
```

重启生效，不用改代码。

## 切换搜索引擎

默认内存倒排索引（零依赖）。数据多了可切 Meilisearch：

```bash
export MEILISEARCH_URL=http://localhost:7700
```

提供中文分词、BM25 排序、持久化。

## 技术栈

| 层 | 技术 |
|---|---|
| 后端 | Go 标准库（零外部依赖） |
| LLM | DeepSeek API（OpenAI 兼容） |
| 前端 | Vue 3 + Vite |
| 存储 | 文件系统（Markdown + YAML） |
| 搜索 | 内存倒排索引 / Meilisearch（可选） |

## 详细文档

- [用户使用教程](docs/用户使用教程.md)
- [技术实现详解](docs/技术实现详解.md)
- [公司级部署方案](docs/公司级部署方案.md)

灵感来自 [Karpathy 的 llm_wiki](https://github.com/karpathy/llm_wiki)。

## License

[MIT](LICENSE)
