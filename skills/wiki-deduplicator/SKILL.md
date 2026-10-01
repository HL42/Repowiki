---
name: wiki-deduplicator
description: "跨项目/子项目检测和合并重复的 Wiki 实体、概念与源文档"
---

# Wiki 去重器

当需要清理知识库中的重复内容时触发。建议在每次批量导入新子项目后执行一次。

## 前置条件

- 多个子项目已导入，可能存在交叉重复的实体/概念
- 目标是减少检索噪音，提高问答精准度

## 工作流程

### 1. 检测重复实体（跨子项目）

```bash
# 收集所有子项目的 entities 文件名
KB="knowledge-base/projects"

# 找出所有 entities 中同名或高度相似的条目
for project in $(ls "$KB"); do
  for sub in $(ls "$KB/$project/sub-projects/" 2>/dev/null); do
    ent_dir="$KB/$project/sub-projects/$sub/wiki/entities"
    [ -d "$ent_dir" ] || continue
    ls "$ent_dir/"*.md 2>/dev/null | while read f; do
      name=$(basename "$f" .md)
      echo "$project/$sub|$name|$f"
    done
  done
done | sort -t'|' -k2 | awk -F'|' '
  '{
    key = $2
    if (prev_key == key) {
      print "  [重复实体] \""key"\" 出现在:\n    - "prev_path"\n    - "$3
    }
    prev_key = key
    prev_path = $3
  }'
```

### 2. 检测相似但不完全同名的实体

对以下模式进行模糊匹配：

| 模式 | 示例 | 处理 |
|------|------|------|
| 大小写差异 | `OrderStatus` vs `orderstatus` | 合并到标准名 |
| 中英文同一概念 | `订单状态` vs `OrderStatus` | 保留中文版，英文版加 redirect |
| 全称/缩写 | `UserDeliveryService` vs `UDS` | 保留全称，缩写版指向全称 |
| 细分粒度不同 | `配送` vs `骑手位置反馈` vs `达达配送` | **不合并** — 这是合理的层级关系 |
| 同一文件被导入两次 | `admin_app` 在两个 sub-project 都出现 | 保留内容更丰富的版本 |

### 3. 检测重复源文档

```bash
# 基于 source_file frontmatter 字段检测：同一个原始文件是否被两个子项目都导入了
for project in $(ls "$KB"); do
  for sub in $(ls "$KB/$project/sub-projects/" 2>/dev/null); do
    src_dir="$KB/$project/sub-projects/$sub/wiki/sources"
    [ -d "$src_dir" ] || continue
    grep -h "^source_file:" "$src_dir/"*.md 2>/dev/null | sed 's/.*: "//;s/".*//' | sort | \
      uniq -c | awk '$1 > 1 {print "[重复源] 出现 "$1" 次: "$0}'
  done
done
```

### 4. 三种去重策略

#### 策略 A：保留最丰富的版本（推荐）

比较重复页面的：
- 正文长度（字符数）
- sources 引用数量
- confidence 等级

保留信息量最大的，删除其他。

```bash
# 比较两个同名实体的质量分数
score_file() {
  local f=$1
  chars=$(wc -c < "$f")
  sources=$(grep -c "^source:" "$f" 2>/dev/null || echo 0)
  conf=$(grep "confidence:" "$f" | head -1)
  # high=3, medium=2, low=1, 其他=0
  case "$conf" in *high*) c=3;; *medium*) c=2;; *low*) c=1;; *) c=0;; esac
  echo $((chars / 100 + sources * 10 + c))
}
```

#### 策略 B：合并为跨项目聚合页

将多个子项目中关于同一实体的描述合并到一个页面中，按来源标注：

```markdown
---
title: "OrderStatus"
type: entity
status: published
sources:
  - yl-delivery-service: OrderService.go
  - zcks-app: pages/order/detail.vue
---

# OrderStatus

## 后端视角 (来自 yl-delivery-service)
{后端定义的内容...}

## APP 端视角 (来自 zcks-app)  
{APP端定义的内容...}
```

#### 策略 C：标记为主从关系

保留所有页面，但在内容较少的页面头部添加引用标记：

```markdown
> ⚠️ 此页面为简要版本。完整定义见「招财快送/yl-delivery-service」中的 {canonical_name} 页面。
```

### 5. 输出去重报告

```
=== Wiki 去重报告 ===
扫描范围: {N} 个子项目, 共 {M} 个实体/概念

【完全重复】({n} 组) — 建议策略 A（保留最佳）
  组1: "OrderStatus"
    → yl-delivery-service/wiki/entities/OrderStatus.md (2456字节, 5 sources, confidence:high) ✅ 保留
    → zcks-app/wiki/entities/OrderStatus.md (890字节, 1 source, confidence:low)   ❌ 删除

【高度相似】({n} 组) — 建议策略 B（合并）或人工确认
  组1: "配送管理" ~ "骑手配送" (名称不同但内容重叠度 78%)

【合理重复】({n} 组) — 不处理（属于正常层级关系）
  - "配送"(通用概念) / "骑手位置反馈"(具体实现) / "达达配送"(平台适配)

【重复源文档】({n} 个) — 可能是误操作导致的双重导入

预计可释放空间:
  - 可删除实体/概念: {n} 个 (~{size} KB)
  - 减少检索噪音: 预计搜索结果精密度提升 {X}%
```

## 注意事项

1. **不要自动执行删除** — 只输出报告和推荐命令。去重是不可逆操作，必须人工确认
2. **层级关系不是重复** — 如 "配送"(父概念) 和各平台配送SDK(子概念) 是合理的知识层级，不要合并
3. **代码文件的重复是正常的** — 同一个模块名出现在前端和后端（如 order.vue 和 OrderController.go）是预期行为，它们的 sources 内容不同
4. **先备份再操作** — 删除前建议用户 commit 当前状态：`git add -A && git commit -m "pre-dedup snapshot"`
5. **删除后需重建索引** — 删除页面后调用 `/api/lint` 或重启服务让搜索引擎刷新
