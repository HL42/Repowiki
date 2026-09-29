# RepoWiki — 把代码仓库变成可对话的 AI 知识库

[![Go](https://img.shields.io/badge/Go-1.23+-00ADD8?logo=go)](https://go.dev/)
[![Vue](https://img.shields.io/badge/Vue-3.x-4FC08D?logo=vue.js)](https://vuejs.org/)
[![License](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

导入你的代码仓库 → 自动阅读理解 → 用自然语言提问。

```
"这个项目的订单处理流程是怎样的？"
"SendOrder 接口需要哪些参数？"
"数据库表结构是什么样的？"
```

## 快速开始

### 下载安装

从 [Releases](https://github.com/your/repo/releases) 下载对应平台的安装包：

- **Mac Apple Silicon**: `repowiki-{version}-darwin-arm64.zip`
- **Mac Intel**: `repowiki-{version}-darwin-amd64.zip`  
- **Windows**: `repowiki-{version}-windows-amd64.zip`

解压后双击 `start.command`（Mac）或 `start.bat`（Windows），首次运行输入 [DeepSeek API Key](https://platform.deepseek.com)，浏览器自动打开。

### 从源码

```bash
git clone https://github.com/your/repo.git
cd repowiki
echo 'DEEPSEEK_API_KEY=sk-your-key' > .env
go build -o repowiki ./cmd/... && ./repowiki
# 打开 http://localhost:8080
```

## 它能做什么

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
你的代码/文档
    │
    ▼
  文件扫描 → 跳过测试/mock/生成代码
    │
    ▼
  自动分类 → 识别层级 + 业务域（编辑 classify_rules.json 适配你的领域）
    │
    ▼
  LLM 阅读 → 每份文件生成结构化摘要
    │
    ▼
  倒排索引 → 关键词搜索 毫秒级
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

## License

MIT
