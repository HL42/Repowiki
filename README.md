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

Download the installer for your platform from [Releases](https://github.com/HL42/Repowiki/releases) (example version: `v0.1.0`):

- **macOS Apple Silicon**: `RepoWiki-v0.1.0-arm64.dmg`
- **macOS Intel**: `RepoWiki-v0.1.0-amd64.dmg`
- **Windows x64**: `repowiki-v0.1.0-windows-amd64.zip`

**Install**

- **macOS**: open the `.dmg` → drag `RepoWiki.app` into **Applications** → double-click it — it opens in its **own native window** (no browser);
- **Windows**: unzip the archive → double-click `repowiki.exe` (no console window) — it opens in your default browser.

The **Quick Start** tab walks you through the rest. **On first run, paste your [DeepSeek API Key](https://platform.deepseek.com/api_keys) right in the page** — it's validated and saved instantly, no terminal needed. If the default port is busy the app picks a free one automatically, so the URL is always correct.

To try it out, import the bundled sample project `examples/order-service` from the **Import** tab.

> **macOS says "cannot verify the developer"?** The app isn't code-signed (an Apple certificate requires a paid account). First launch: **right-click `RepoWiki.app` → Open → confirm**.
>
> **Windows SmartScreen blocked it?** Click **More info** → **Run anyway**.

The server listens on `127.0.0.1` **only** — never on your LAN or the internet. It also validates the `Host` and `Origin` headers and rejects non-local origins, so an arbitrary web page you happen to visit can't reach the API (which can list local directories and read files). Your API key and knowledge-base data live in the user data directory (macOS `~/Library/Application Support/RepoWiki`, Windows `%APPDATA%\RepoWiki`).

### From source

```bash
git clone https://github.com/HL42/Repowiki.git
cd Repowiki
echo 'DEEPSEEK_API_KEY=sk-your-key' > .env   # or leave empty and fill it in the browser on first run
go build -o repowiki ./cmd/... && ./repowiki
# opens http://127.0.0.1:8080 automatically
```

If `frontend/dist` is missing, build it once: `cd frontend && npm install && npm run build`.
Use `./repowiki --version` for the version, `./repowiki --help` for CLI usage, and `./repowiki --no-open` to start without opening a browser.

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
