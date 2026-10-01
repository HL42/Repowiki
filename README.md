# RepoWiki — Turn your codebase into a conversational AI knowledge base

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js)](https://vuejs.org/)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Import your code and docs → let the LLM read and organize them → ask questions in natural language.

> 把代码仓库变成可对话的 AI 知识库。中文文档见 [docs/用户使用教程.md](docs/用户使用教程.md)。

```
"How does the order flow work in this project?"
"What parameters does the SendOrder API take?"
"What does the database schema look like?"
```

## Quick Start

### Download (release binary)

Download the installer for your platform from [Releases](https://github.com/HL42/Repowiki/releases):

- **macOS Apple Silicon**: `repowiki-{version}-darwin-arm64.zip`
- **macOS Intel**: `repowiki-{version}-darwin-amd64.zip`
- **Windows**: `repowiki-{version}-windows-amd64.zip`

Unzip, then double-click `start.command` (macOS) or `start.bat` (Windows). On first run, enter your [DeepSeek API Key](https://platform.deepseek.com) — the browser opens automatically.

### From source

```bash
git clone https://github.com/HL42/Repowiki.git
cd Repowiki
echo 'DEEPSEEK_API_KEY=sk-your-key' > .env
go build -o repowiki ./cmd/... && ./repowiki
# open http://localhost:8080
```

## What it does

| Feature | Description |
|---|---|
| **Import code** | Supports Go / Python / JS / TS / Vue / Java / Rust / Markdown |
| **Auto-classify** | Organizes by tech layer (handler/service/model) and business domain (order/payment/user) |
| **Code map** | Generates a "module index" so you can see the project structure at a glance |
| **Q&A** | RAG answer generation with multi-turn conversation |
| **Incremental updates** | SHA256 diff — only changed files are reprocessed |
| **Quality checks** | Detects LLM hallucinations, stub pages, and stale docs |

## How it works

```
your code / docs
    │
    ▼
  scan → skip tests / mocks / generated code
    │
    ▼
  classify → detect layer + domain (edit classify_rules.json to fit your domain)
    │
    ▼
  LLM reads → generate a structured summary per file
    │
    ▼
  inverted index → millisecond keyword search
    │
    ▼
  RAG Q&A ← user question
```

## Customize for your domain

Edit `classify_rules.json` and add your own business keywords:

```json
{
  "domains": [
    {"keywords": ["inventory", "库存"], "domain": "库存"},
    {"keywords": ["billing", "计费"], "domain": "计费"}
  ]
}
```

Restart to apply — no code changes needed.

## Switch search engine

The default is an in-memory inverted index (zero dependencies). For larger corpora, switch to Meilisearch:

```bash
export MEILISEARCH_URL=http://localhost:7700
```

This adds Chinese tokenization, BM25 ranking, and persistence.

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go standard library (zero external dependencies) |
| LLM | DeepSeek API (OpenAI-compatible) |
| Frontend | Vue 3 + Vite |
| Storage | Filesystem (Markdown + YAML) |
| Search | In-memory inverted index / Meilisearch (optional) |

## License

[MIT](LICENSE)

---

## 中文说明

RepoWiki 是一个本地运行的 AI 知识库工具：把代码/文档导入后，LLM 自动阅读、分类并生成结构化摘要，之后可以用自然语言问答。

- **快速开始**：从 [Releases](https://github.com/HL42/Repowiki/releases) 下载对应平台安装包，双击 `start.command`（Mac）或 `start.bat`（Windows），首次运行输入 DeepSeek API Key。
- **从源码**：`git clone https://github.com/HL42/Repowiki.git && cd Repowiki && go build -o repowiki ./cmd/... && ./repowiki`
- **技术栈**：Go 纯标准库（零外部依赖）+ DeepSeek + Vue 3 + 文件存储 + 倒排索引 / Meilisearch（可选）
- **详细文档**：[用户使用教程](docs/用户使用教程.md) · [技术实现详解](docs/技术实现详解.md) · [公司级部署方案](docs/公司级部署方案.md)

灵感来自 [Karpathy 的 llm_wiki](https://github.com/karpathy/llm_wiki)。
