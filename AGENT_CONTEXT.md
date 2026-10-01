# RepoWiki Local — AI Agent 上下文指南

> 每次开始工作时先读此文件，快速理解项目全貌和你的职责。
> 目标：用最少 token 获得足够上下文，避免逐个读源文件。

---

## 一、这是什么项目

**RepoWiki Local** — 一个本地运行的、AI 驱动的知识库系统。灵感来自 Karpathy 的 [llm_wiki](https://github.com/karpathy/llm_wiki)。

核心能力：
1. **文档导入** — 扔进去代码/文档/规范，LLM 自动分析生成结构化摘要
2. **智能问答** — RAG 架构（倒排索引检索 → Top10 分块 → LLM 生成答案）
3. **综合提炼** — 输入主题，LLM 跨多篇文档写深度分析报告
4. **质量检查** — 自动检测幻觉产物、空壳页面、过期文档
5. **多项目管理** — 支持项目 → 子项目两级隔离（如招财快送下有4个子仓库）

## 二、技术栈（极简）

| 层 | 技术 | 说明 |
|----|------|------|
| 后端 | Go 1.22+ | 纯标准库，零外部依赖 |
| LLM | DeepSeek API | 兼容 OpenAI 格式，模型 deepseek-chat |
| 存储 | 文件系统 (Markdown) | raw 归档 + wiki 四层结构 |
| 前端 | Vue3 SPA (单 index.html) | 内嵌在 frontend/ 目录 |
| 检索 | 内存倒排索引 (InMemorySearch) | bigram 分词 + posting 计分 |
| 数据库 | 无 | 文件即数据库 |

## 三、目录结构速查

```
repowiki-local/
├── cmd/main.go                    # 入口：读 .env → 初始化 → 启动 HTTP:8080
├── internal/
│   ├── api/                       # HTTP 路由层（server.go 注册所有路由）
│   │   ├── server.go              # 路由表 + CORS + JSON 工具
│   │   ├── query.go               # POST /api/query (问答)
│   │   ├── ingest.go              # POST /api/ingest (导入)
│   │   ├── synthesize.go          # POST /api/synthesize (综合)
│   │   └── stats.go               # GET /api/stats (统计)
│   ├── wiki/                      # ★ 核心业务引擎
│   │   ├── engine.go              # Engine 结构体 + WithProject + WithSubProject
│   │   ├── ingest.go              # 导入管线：Walk → Hash对比 → LLM分析 → 写wiki
│   │   ├── query.go               # 问答管线：关键词→倒排索引→Top10→LLM生成
│   │   ├── synthesize.go          # 综合提炼管线
│   │   ├── lint.go                # 质量检查：扫描所有wiki页+子项目
│   │   ├── searcher.go            # InMemorySearch 倒排索引实现
│   │   ├── accumulate.go          # 实体/概念页增量合并
│   │   └── types.go               # 所有数据结构定义
│   ├── storage/
│   │   ├── filesystem.go          # 文件读写 + Sub() / SubProject() / ListSubProjects()
│   │   └── chunker.go             # Markdown 按 ## 标题分块
│   └── llm/
│       └── deepseek.go            # DeepSeek API 封装 + 所有 prompt 模板
├── frontend/index.html            # 前端单文件（Vue3 + 原生fetch）
├── skills/                      # ★ AI Agent 技能包（团队共享）
│   ├── wiki-import-auditor/     # 导入前审核：筛选核心文件，跳过噪音
│   ├── wiki-quality-detector/   # 质量检测增强：幻觉三级判定+空壳四级分类
│   ├── wiki-query-refiner/      # 模糊问题自动扩展：单问题→多维度并行查询
│   ├── wiki-deduplicator/       # 跨子项目去重：重复实体/概念检测与合并策略
│   └── wiki-sync-guardian/      # 导入守卫：防止重复/错误导入的前置检查
├── docs/技术实现详解.md            # 完整架构文档（13章732行）
├── knowledge-base/                # 运行时数据（git管理，仅此一个子目录）
│   └── projects/                  # ★ 所有数据都在这里
│       ├── 通用/                  # 全局规范/SOP (39 sources / 382 entities / 535 concepts)
│       │   ├── project.md
│       │   ├── raw/              # 原始规范文件归档
│       │   └── wiki/{sources,entities,concepts,syntheses}/
│       └── 招财快送/              # 主项目（4个子项目，~798文档）
│           ├── project.md
│           └── sub-projects/      # ★ 每个子项目独立引擎+搜索索引
│               ├── yl-delivery-service/raw + wiki/    (407 docs)
│               ├── zcks-app/raw + wiki/                (254 docs)
│               ├── zcks-骑手位置反馈/raw + wiki/        (73 docs)
│               └── zcks-server/raw + wiki/             (64 docs)
├── .env                           # DEEPSEEK_API_KEY / DEEPSEEK_MODEL / PORT / WIKI_ROOT
└── go.mod                         # 仅 Go 标准库，无第三方依赖
```

## 四、Wiki 四层知识体系

**所有数据都存储在 `projects/{项目名}/` 下。** 全局 `raw/` 和 `wiki/` 为空（遗留结构，代码启动时自动重建，无实际数据）。

```
projects/{project}/
├── raw/                          # 原始文件归档（导入时从源目录复制过来）
│   └── *.md / *.go / *.js ...    # + 对应的 .hash 文件（SHA256，增量检测用）
└── wiki/
    ├── sources/                  # ★ LLM 生成的源摘要页（每个原始文件对应一个 .md）
    │   └── frontmatter: title/type/status/created/source_file/category/confidence/tags
    ├── entities/                 # ★ 从 sources 中提取的具体事物（LLM ExtractList）
    ├── concepts/                 # ★ 从 sources 中提取的抽象概念（同上）
    ├── syntheses/                # 用户手动触发的跨文件综合分析（POST /api/synthesize）
    ├── contradictions/            # 综合时检测到的信息冲突记录
    └── history/                  # 文件更新时的旧版快照（自动）
```

**shouldSkipEntityConcept 当前规则**：只跳过 SOP 流程文档（路径含 `/sop` 或以 `sop.md` 结尾）。其他所有类型（含 guide/technical）都允许提取实体概念。（2026-06-05 已修复，之前误跳过了全部规范类文档）

## 五、子项目系统（关键设计）

### 存储路径规则

```
knowledge-base/projects/{project}/sub-projects/{sub}/
├── raw/                    ← 该子项目的原始文件归档
└── wiki/
    ├── sources/            ← 子项目的源摘要页
    ├── entities/           ← 子项目的实体页
    ├── concepts/           ← 子项目的概念页
    └── syntheses/          ← 子的综合提炼页
```

### 引擎级联（带缓存）

```
全局 Engine
  └─ WithProject("招财快送")     → 项目引擎A (缓存命中则复用)
      ├─ 不指定 sub_project      → 搜索范围 = A.wiki/sources/ + 所有子项目 sources/
      └─ WithSubProject("zcks-app") → 子项目引擎B (缓存命中则复用)
                                     → 搜索范围 = 仅 zcks-app/wiki/sources/
```

**缓存机制**：`engine.go` 中 `projectEngineCache` 和 `subProjectEngineCache` 两个 map，WithProject/WithSubProject 用 double-check locking 实现懒加载 + 缓存。`invalidateCaches()` 在导入/删除后清空缓存强制重建。

### 当前已导入的项目

**knowledge-base/projects/ 下只有两个项目目录：**

| 项目 | raw 文档 | sources | entities | concepts | syntheses | 说明 |
|------|---------|---------|----------|----------|-----------|------|
| **通用** | 80 (规范文件) | 39 | 382 | 535 | 4 | 团队编码规范/Git规范/数据库规范等 |
| **招财快送** | — | — | — | — | — | 父项目，数据全在子项目中 |

**招财快送 → 4 个子项目：**

| 子项目 | 源文档数 | 技术栈 | 核心内容 |
|--------|---------|--------|----------|
| yl-delivery-service | 407 | Node.js/Koa | controller(44) + service(46) + model(100) + route(29) + sdks(28) |
| zcks-app | 254 | uni-app/Vue | 25个业务模块: order/receipt/shop/stat/delivery/message/login 等 |
| zcks-骑手位置反馈 | 73 | Go/go-zero | 10平台配送SDK: 达达/蜂鸟/顺丰/UU/闪送 等 |
| zcks-server | 64 | Go/gRPC+HTTP | 8个gRPC接口 + HTTP回调 + 好惠送运力对接 |
| **合计** | **~798** | | |

## 六、搜索系统原理

```
InMemorySearch (纯内存倒排索引)

建索引（启动时/导入后）:
  每个 source/*.md → ChunkMarkdown(按##分块) → tokenize(空格token+中文bigram) → 写入 inverted map[string][]posting

查询时:
  用户问题 → extractKeywords(同上分词) → Search(keywords, topN=10)
    → 每个关键词查 inverted 表 O(1) → 合并计分(命中词数) → 排序取Top10

分词策略:
  英文: 按空白切分，去标点，转小写
  中文: 连续中文字符做二元组 bigram ("订单状态" → ["订单","单状","状态"])
  同一文档内同一词去重
```

**SearchEngine 接口**（可替换实现）：
```go
type SearchEngine interface {
    Index(docPath string, chunks []Chunk)    // 新增索引
    Search(keywords []string, topN int) []scoredChunk  // 查询
    Remove(docPath string)                   // 删除索引
    Rebuild(listPages, getChunks) error      // 全量重建
}
```

## 七、API 端点速查

| 方法 | 路径 | 参数 | 说明 |
|------|------|------|------|
| POST | /api/ingest | `{raw_path, project?, sub_project?}` | 导入文档 |
| POST | /api/query | `{question, project?, sub_project?}` | 问答 |
| POST | /api/synthesize | `{topic, project?, sub_project?}` | 综合提炼 |
| GET | /api/stats | `?project=xxx&sub_project=xxx` | 统计 |
| GET | /api/lint | (无) | 质量检查(强制刷新) |
| GET | /api/wiki/list/{category} | `?project=xxx&sub_project=xxx` | 列出页面 |
| GET | /api/wiki/page/{cat}/{name} | 同上 | 读页面 |
| DELETE | /api/wiki/page/{cat}/{name} | 同上 | 删页面 |
| POST | /api/projects | `{name, description, sub_projects}` | 创建项目 |
| GET | /api/projects | (无) | 列出所有项目 |
| GET | /api/project/{name} | (无) | 项目详情(含子项目统计) |
| GET | /api/sub-projects?project=X | (无) | 列出某项目下子项目名 |

**子项目路由规则**：
- **都不传** → 全局引擎（wiki/ 为空，几乎搜不到内容。当前无实际用途）
- **只传 `project=通用`** → 搜规范文档（39 sources + 382 entities + 535 concepts）
- **只传 `project=招财快送`** → 聚合搜索全部 4 个子项目（~798 docs）
- **都传**（`project=招财快送&sub_project=zcks-app`）→ 仅该子项目（254 docs，最快最准）

## 八、关键设计决策（改代码前必读）

1. **零外部依赖** — 不引入任何第三方包。如果觉得需要新依赖（如 bleve、gin），先评估是否可以用标准库替代
2. **文件即数据库** — 所有数据是 Markdown 文件，可用 git 版本管理。不要引入 SQL/Redis
3. **LLM 调用是主要瓶颈** — 单次 Chat ~2-5s。导入时用 goroutine 并发（信号量限 3），问答时尽量减少 LLM 调用次数
4. **代码文件特殊处理** — .go/.js/.vue 等走 `ingestOneCode` 管线（按函数分块+ExtractCodeMeta提取模块说明），不走实体/概念提取（避免产生巨量无意义页面）
5. **增量更新** — SHA256 哈希对比检测文件变化。未变化跳过，已修改走 diff 合并（保留旧实体不删），旧版存 history/
6. **前端是单文件** — `frontend/index.html` 一个 HTML 包含全部 Vue3 组件逻辑。改动前端只动这一个文件

## 九、开发/调试常用命令

```bash
# 编译运行
cd repowiki-local && go build -o repowiki ./cmd/... && ./repowiki &

# 测试问答
curl -s -X POST http://localhost:8080/api/query \
  -H "Content-Type: application/json" \
  -d '{"question":"xxx","project":"招财快送","sub_project":"zcks-app"}'

# 测试导入
curl -s -X POST http://localhost:8080/api/ingest \
  -H "Content-Type: application/json" \
  -d '{"raw_path":"/path/to/files","project":"招财快送","sub_project":"zcks-app"}'

# 查看统计
curl -s http://localhost:8080/api/project/招财快送 | python3 -m json.tool

# 关闭服务
lsof -ti :8080 | xargs kill -TERM

# 提交格式（参考已有 commit）
git commit -m "类型：简短描述\n\n问题：...\n修复：..."
```

## 十、已知限制与待办

- [ ] 搜索精度有限（bigram 分词 vs 专业分词器）。当前够用但复杂查询可能遗漏结果
- [ ] 全局 Engine 启动时不索引 projects/ 下数据（只有不带 project 参数的查询才用全局引擎）
- [ ] syntheses 页面目前需手动触发（POST /api/synthesize），无自动定期综合
- [ ] 前端为单 HTML 文件，随功能增长已显臃肿，未来可拆分为组件化
- [ ] 无用户认证（当前仅本地使用）
- [ ] LLM prompt 在 deepseek.go 中硬编码，抽取为配置项会更灵活

## 十一、Skills 加载规则（重要！）

本项目有 5 个专用 Skill，存放在两个位置：

| 位置 | 路径 | Agent 能否自动发现 |
|------|------|------------------|
| **user 级（已生效）** | `~/.codebuddy/skills/user/wiki-*` | ✅ 自动出现在 available_skills |
| **项目级（源文件）** | `skills/wiki-*` | ❌ 仅供阅读/git管理 |

### 必须加载 Skill 的场景

| 场景 | 加载哪个 Skill | 触发条件 |
|------|--------------|----------|
| 用户要导入新项目 | `wiki-import-auditor` | 出现"导入""审核""看看要导什么文件"等关键词 |
| 准备执行 /api/ingest 前 | `wiki-sync-guardian` | 做前置守卫检查（防重复/防误操作） |
| 执行质量检查时 | `wiki-quality-detector` | 出现"lint""检测幻觉""质量检查"等关键词，且需要比内置 lint 更深度的判断 |
| 用户问题过于模糊 | `wiki-query-refiner` | 问题 < 15字符、或只有单个宽泛关键词如"订单""配送" |
| 发现重复内容/定期维护 | `wiki-deduplicator` | 出现"去重""清理""重复实体"等关键词 |

### 加载方式

当满足上述触发条件时，使用 `use_skill` 工具加载：

```
use_skill("wiki-import-auditor")   → 加载后按其工作流程执行
use_skill("wiki-quality-detector") → 加载后执行深度质量分析
```

**注意**：如果 `use_skill` 调用失败（skill 不在可用列表中），可直接读取 SKILL.md 文件：
```
read_file("skills/wiki-import-auditor/SKILL.md")
```
效果相同，只是不经过 skill 框架的格式校验。

### 团队成员首次使用时的安装

```bash
# 将项目 skills 复制到个人 user 目录（只需执行一次）
cp -r skills/wiki-* ~/.codebuddy/skills/user/

# 验证
ls ~/.codebuddy/skills/user/wiki-*/SKILL.md
# 应看到 5 个文件
```

或者直接运行项目中的安装脚本：
```bash
bash skills/install.sh
```

---

> 最后更新：2026-06-05 | 对应 git commit: 8fc5c99
