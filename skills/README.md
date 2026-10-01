# RepoWiki Skills — AI Agent 技能包

> 专为本项目定制的 Skill，让每次接入的 AI Agent 都能以高标准执行操作。

## 使用方式

将 `skills/` 目录下的内容复制到你的 CodeBuddy skills 加载路径：

```bash
# 方式1：放入用户级目录（仅自己可用）
cp -r skills/wiki-* ~/.codebuddy/skills/user/

# 方式2：放入项目级目录（团队共享，推荐）
# 当前已在此项目中，Agent 可直接读取 skills/
```

## Skills 总览

| Skill | 触发词 | 行数 | 解决什么问题 |
|-------|--------|------|-------------|
| **wiki-import-auditor** | "审核导入""看看要导什么文件" | 118 | 用户想导入项目时，自动分析目录结构，按 T1-T4 分层筛选出核心文件 |
| **wiki-quality-detector** | "检测幻觉""质量检查""深度 lint" | 130 | 质量检测时超越基础 lint 规则，三级幻觉判定 + 四级空壳分类 |
| **wiki-query-refiner** | 模糊/宽泛问题的自动扩展 | 147 | 用户问"配送怎么回事"这种模糊问题时，拆解为多维度并行查询再综合 |
| **wiki-deduplicator** | "去重""清理重复" | 155 | 跨子项目的重复实体/概念检测，三种合并策略（保留最佳/聚合页/主从标记） |
| **wiki-sync-guardian** | 导入前置检查 | 138 | 执行 ingest 前的三重防护（hash重复/跨项目重叠/目标合理性） |

## 调用链路

```
用户: "帮我把 /path/to/project 导入进来"

  ↓ 触发 wiki-import-auditor
分析项目 → 输出 T1-T4 筛选方案 → 用户确认

  ↓ 触发 wiki-sync-guardian (前置守卫)
检查是否重复 → 检查 hash 状态 → 检查跨项目冲突 → 放行

  ↓ 执行 /api/ingest
导入完成

  ↓ 触发 wiki-quality-detector (事后质检)
三级幻觉检测 → 四级空壳分类 → 来源断裂检测 → 输出增强报告

  ↓ 定期触发 wiki-deduplicator (维护时)
跨子项目去重 → 输出去重报告 → 用户确认后清理

  ↓ 日常问答中触发 wiki-query-refiner (按需)
模糊问题 → 多维扩展查询 → 综合输出
```

## 各 Skill 详细说明

### 1. wiki-import-auditor

**核心能力**：
- 自动识别技术栈和框架模式（Go/go-zero、Node.js/Koa、uni-app/Vue 等）
- T1(必须) → T2(建议) → T3(可选) → T4(跳过) 四层筛选
- SDK 目录智能精简策略（只取入口文件）
- 输出含子项目命名和归属建议的完整导入方案

**不做什么**：不自动执行导入，只出方案等确认。

### 2. wiki-quality-detector

**与 lint.go 内置规则的对比**：

| 维度 | lint.go 内置 | 本 skill 增强 |
|------|-------------|--------------|
| 幻觉检测 | nameInSources 字符串包含判断 | 三级精度：确定/疑似/非幻觉低质 |
| 空壳页面 | <400B=error, <250B=warning | 四级：纯占位/极短/低密度/正常 |
| 断链检测 | 无 | source_file 物理存在性验证 |
| 内容语义 | 无 | 车轱辘话/信息密度判断 |

### 3. wiki-query-refiner

**三种扩展策略**：
- **A 关键词拆分**："配送" → 4 个子查询并行（订单分配+骑手位置+管理+SDK对接）
- **B 上下文补全**：只给 project 不给 sub_project → 自动拆到各子项目分别查询
- **C 技术栈转换**："前端怎么提交订单" → 自动翻译为 API名/组件名等技术术语

**内置「招财快送」扩展模板**：订单→5维度、配送→4维度、店铺→2维度等。

### 4. wiki-deduplicator

**三种去重策略**：
- **A 保留最丰富版**（推荐）：比较字符数/sources数/confidence，留最好的
- **B 合并为聚合页**：跨项目来源标注，一份文档整合多视角
- **C 主从标记**：保留全部但轻量版指向完整版

**智能识别层级关系 vs 真重复**：
- "配送" 和 "达达配送" → 层级关系，**不合并**
- "OrderStatus" 在两个子项目各有一份 → **真重复，需处理**

### 5. wiki-sync-guardian

**三重防护**：
- A: Hash 完全匹配检测（所有文件都没变？→ 阻止无效导入）
- B: 跨子项目特征文件碰撞（同一份代码用不同名字导入了两次？）
- C: 目标合理性（是不是误选了 node_modules/.git/dist？）

**定位**：只读 gate，不修改任何数据。放在 ingest 之前执行。

## 给新 Agent 的快速指引

> 在开始工作前，请先读取 `AGENT_CONTEXT.md` 了解项目全貌。
> 
> 以下情况请加载对应 Skill：
> - 用户要导入项目 → 先加载 `skills/wiki-import-auditor/SKILL.md`
> - 要做质量检查 → 加载 `skills/wiki-quality-detector/SKILL.md`
> - 问题太模糊不好答 → 加载 `skills/wiki-query-refiner/SKILL.md`
> - 发现结果有重复 → 加载 `skills/wiki-deduplicator/SKILL.md`
> - 准备执行导入前 → 加载 `skills/wiki-sync-guardian/SKILL.md`

> 最后更新：2026-06-05 | 共 5 个 Skill，总计 ~688 行
