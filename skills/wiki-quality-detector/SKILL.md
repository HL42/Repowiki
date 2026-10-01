---
name: wiki-quality-detector
description: "辅助 AI 进行 Wiki 质量检测，精确判断幻觉产物、空壳页面和低质量输出"
---

# Wiki 质量检测增强器

当触发质量检查（lint）流程时自动调用，或用户说「检查质量」「检测幻觉」「清理低质量页面」时使用。

## 前置条件

- 项目已导入完成，`wiki/sources/`, `wiki/entities/`, `wiki/concepts/` 下有数据
- 目标是比基础 lint 规则更精准地识别问题

## 工作流程

### 1. 获取当前 lint 报告作为基准

```bash
curl -s http://localhost:8080/api/lint | python3 -c "
import sys, json
d = json.load(sys.stdin)
print(f'总页:{d[\"total_pages\"]} 严重:{d[\"error_count\"]} 警告:{d[\"warn_count\"]}')
for i in d.get('issues', [])[:20]:
    print(f'  [{i[\"severity\"]:7s}] {i.get(\"category\",\"?\")}/{i.get(\"name\",\"?\")}: {i[\"issue\"][:80]}')
"
```

### 2. 对每个 issue 进行深度分析（超越基础规则）

基础 lint 规则（lint.go 中的 checkPage）只做表面检测。本 skill 增加以下 **深度判断逻辑**：

#### 2.1 幻觉产物三级判定

| 等级 | 判定方法 | 处理建议 |
|------|----------|----------|
| **确定幻觉** | 实体/概念名在 `raw/` 所有源文件全文搜索，完全不存在 | 直接删除 |
| **疑似幻觉** | 名称存在但不完全匹配（如大小写差异、缺少前缀） | 标记 warning + 人工复核 |
| **非幻觉但低质** | 名称存在但页面内容空洞无实质 | 标记 warning + 建议补充 |

```bash
# 幻觉检测操作步骤：
# 1. 取出 entities/*.md 或 concepts/*.md 的文件名（即实体名）
# 2. 解析其 frontmatter 中的 sources 字段 → 得到引用的源文档列表
# 3. 对每个 source_file 找到 raw/ 下对应的归档文件
# 4. 在归档文件原文中 grep 搜索该实体/概念名称
# 5. 未找到 → 幻觉产物

# 具体命令示例：
WIKI="knowledge-base/projects/{project}/sub-projects/{sub}/wiki"
RAW="knowledge-base/projects/{project}/sub-projects/{sub}/raw"

for entity_file in "$WIKI/entities/"*.md; do
  name=$(basename "$entity_file" .md)
  # 在所有原始文件中搜索这个实体名
  found=$(grep -rl "$name" "$RAW"/ 2>/dev/null)
  if [ -z "$found" ]; then
    echo "[幻觉] $name — 在任何源文件中都未找到"
  fi
done
```

#### 2.2 空壳页面四级分类

| 级别 | 特征 | 示例 | 建议 |
|------|------|------|------|
| L1 纯占位 | 正文只有 "待补充" / "(TBD)" / "（待完善)" | 概念页只有定义段 | 删除（浪费检索噪音） |
| L2 极短 | 去除 frontmatter 后正文 < 100 字符 | 只有标题+一行描述 | 标记 error |
| L3 低密度 | 内容存在但信息量极低（重复/废话） | 全篇都是 "该XX用于处理XX" 的车轱辘话 | 标记 warning |
| L4 正常 | 有实质内容、具体数据、代码引用 | 含 API 签名、数据结构、流程说明 | 通过 |

#### 2.3 来源断裂检测

检查 frontmatter 中的 `source_file` 指向的文件是否仍然存在于 `raw/`：

```bash
WIKI=".../wiki/sources"
RAW=".../raw"

for md in "$WIKI"/*.md; do
  src=$(grep "^source_file:" "$md" | sed 's/.*: "//;s/".*//')
  if [ -n "$src" ] && [ ! -f "$RAW/$src" ]; then
    echo "[断链] $(basename $md) → 引用的源文件 $src 不存在于 raw/"
  fi
done
```

### 3. 输出增强版质量报告

```
=== Wiki 质量深度检测报告 ===
项目：{project}/{sub}
扫描时间：{timestamp}

【幻觉产物】({n} 个) — 建议直接删除
  - entities/{name}.md — 名称为 LLM 编造，raw/ 中无匹配
  - concepts/{name}.md — 同上

【空壳页面】({n} 个) — 建议删除或标记
  - L1 纯占位: {n} 个 (只有 "待补充")
  - L2 极短:   {n} 个 (< 100字符正文)
  - L3 低密度: {n} 个 (信息量不足)

【来源断裂】({n} 个)
  - sources/{file}.md → 原始文件已不存在

【正常页面】({n} 个)

建议操作：
  1. 立即删除幻觉产物: DELETE /api/wiki/page/entities/{name}
  2. 审核后删除空壳页面: DELETE /api/wiki/page/concepts/{name}
  3. 断链页面需重新导入源文件
```

## 与 lint.go 内置规则的配合关系

| 检测项 | lint.go 内置 | 本 skill 增强 | 说明 |
|--------|-------------|--------------|------|
| 单字符名称 | 有 | 无需增强 | 已经够用 |
| 幻觉产物 | 有（nameInSources） | **增加三级精度** | 内置只做字符串包含判断；本 skill 加上来源断裂检测+分级 |
| 空壳页面 | 有（<400B=error, <250B=warning） | **增加内容语义判断** | 内置只看字节长度；本 tool 区分纯占位/极短/低密度 |
| 过期检测 | 有（hash对比） | **增加断链检测** | 内置只检测 hash 变化；本 tool 检测 source_file 是否物理存在 |
| 内容质量 | **无** | **新增** | 检测车轱辘话/重复内容/无实质信息的正常长度页面 |

## 注意事项

1. **不要自动删除** — 输出报告和建议操作命令，让用户确认后执行
2. **幻觉检测的误判风险** — 某些实体名可能是 LLM 从多个源文件综合推断出的合理抽象名（如将 "OrderService" 和 "OrderController" 综合为 "订单模块"），这类不算真正幻觉。如果名称在源文件中有**部分词根匹配**，降级为"疑似"而非直接判定
3. **性能考虑** — 大量 entity/concept 文件的全文本 grep 可能较慢。对 >500 个实体的项目，可以抽样检测（如抽检 20%）或并行执行
4. **与 invalidateCaches 配合** — 删除问题页面后调用 `POST /api/lint` 或手动触发 RefreshLint 更新缓存
