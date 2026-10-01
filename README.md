# RepoWiki — Turn your codebase into a conversational AI knowledge base

[English](README.md) | [中文](README.zh-CN.md)

[![Go](https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js)](https://vuejs.org/)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

Import your code and docs → let the LLM read and organize them → ask questions in natural language.

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
