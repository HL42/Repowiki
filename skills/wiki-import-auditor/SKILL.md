---
name: wiki-import-auditor
description: "审核待导入项目，分析目录结构并筛选出应导入的核心文件清单"
---

# Wiki 导入审核器

当用户说「帮我看看这个项目需要导入什么文件」「准备导入 xxx 项目」「审核导入内容」时触发。

## 前置条件

- 用户提供了待导入项目的本地路径（如 `/Users/hl/xxx-project`）
- 目标是挑出对知识库有价值的文件，跳过噪音

## 工作流程

### 1. 扫描项目全貌

```bash
# 快速了解项目类型和技术栈
ls -la {project_path}/ | head -30

# 识别包管理器和框架
cat {project_path}/package.json 2>/dev/null | head -15    # Node.js
cat {project_path}/go.mod 2>/dev/null | head -10          # Go
cat {project_path}/pom.xml 2>/dev/null | head -10         # Java
cat {project_path}/Cargo.toml 2>/dev/null | head -10       # Rust
cat {project_path}/requirements.txt 2>/dev/null            # Python
cat {project_path}/README.md 2>/dev/null | head -30        # 项目说明

# 完整源文件统计（排除噪音）
find {project_path} -type f \
  \( -name "*.go" -o -name "*.js" -o -name "*.ts" -o -name "*.vue" \
  -o -name "*.py" -o -name "*.java" -o -name "*.rs" -o -name "*.md" \
  -o -name "*.yaml" -o -name "*.yml" -o -name "*.sql" \) \
  ! -path '*/node_modules/*' ! -path '*/.git/*' ! -path '*/dist/*' \
  ! -path '*/vendor/*' ! -path '*/__pycache__/*' ! -path '*/target/*' \
  ! -path '*/build/*' ! -path '*/.next/*' ! -path '*/unpackage/*' \
  | sort
```

### 2. 按模块分层统计

```bash
# 各业务模块的文件数和行数（以 src/ 或各主要目录为单位）
for dir in {project_path}/{main_source_dirs}; do
  name=$(basename "$dir")
  count=$(find "$dir" -type f \( -name "*.go" -o -name "*.js" -o -name "*.ts" -o -name "*.vue" \) 2>/dev/null | wc -l)
  lines=$(find "$dir" -type f \( -name "*.go" -o -name "*.js" -o -name "*.ts" \) 2>/dev/null | xargs wc -l 2>/dev/null | tail -1)
  [ "$count" -gt 0 ] && echo "  $name/: ${count} 文件, ${lines} 行"
done
```

### 3. 分层筛选决策

按以下优先级将文件分为四层：

| 层级 | 导入？ | 判断标准 | 典型例子 |
|------|--------|----------|----------|
| **T1 必须导入** | 是 | 核心业务逻辑：controller/service/model/route/config/入口文件 | `src/controller/*.go`, `App.vue`, `main.js` |
| **T2 建议导入** | 是 | 公共组件、SDK适配层、配置、架构文档 | `components/`, `sdks/*.js`, `etc/*.yaml`, `docs/agents/` |
| **T3 可选导入** | 看需求 | 工具函数库、测试代码、SQL迁移 | `utils/`, `library/`, `test/`, `sql_update/` |
| **T4 跳过** | 否 | 二进制资源、依赖、构建产物、临时文件 | `static/img/`, `node_modules/`, `dist/`, `.git/`, `*.lock`, `*.map`, `.DS_Store` |

### 4. 输出导入方案

输出格式：

```
=== 项目：{项目名} ===
技术栈：{Go 1.22 / go-zero}  |  预估核心文件：{~N} 个

【T1 必须导入】({N} 个文件)
  ├── {dir1}/     : {n} 个 — {一句话说明}
  ├── {dir2}/     : {n} 个 — {一句话说明}
  └── 入口文件      : main.go / App.vue / pages.json

【T2 建议导入】({N} 个文件)
  ├── components/  : {n} 个公共组件
  └── docs/arch/   : {n} 个架构文档

【T3 按需导入】({N} 个文件)
  └── utils/, test/, sql/ — 非核心但可能有用

【T4 跳过】(共 ~N 个)
  node_modules/, static/img/, dist/, .git/, vendor/, *.lock, *.map

建议子项目名：{推荐名称}
建议归属父项目：{通用 / 招财快送 / 新建}
```

## 关键判断规则

### 代码类项目（后端/前端/App）

**必须包含的目录模式：**
- Go: `common/`(模型+枚举), `internal/`(handler+logic+service), `infrastructure/`, `etc/`
- Node.js/Koa: `src/controller/`, `src/service/`, `src/model/`, `src/route/`
- uni-app/Vue: `pages/`(全部页面), `store/`(状态管理), `components/`, `App.vue`, `pages.json`
- React: `src/components/`, `src/pages/`, `src/hooks/`, `src/api/`

**SDK 类目录的处理策略：**
- 如果 SDK 目录下有几十个内部实现文件 → 只取顶层入口文件（客户端初始化、主接口）
- 如果 SDK 只有少量适配器文件 → 全部导入

### 文档类项目（规范/设计文档）

全部导入，但要检查：
- 是否有重复/过时版本
- 是否有纯草稿（status=draft 且无实质内容）

## 注意事项

1. **不要自动执行导入** — 只输出方案，等用户确认后再执行复制+API调用
2. **考虑子项目归属** — 如果是已有项目的关联仓库，建议归入该项目的 sub-projects/
3. **估算 LLM 成本** — 大量小文件比少量大文件更费 API 调用次数（每个文件一次 ExtractMetadata）
4. **保留 hash 机制** — 导入后的增量更新依赖 raw/ 下的 .hash 文件，不要删除它们
5. **代码文件的 entity/concept 提取收益低** — 代码走 ingestOneCode 管线，只生成 sources 页面，不拆实体概念页。这属于正常行为不需要特别说明给用户
