---
name: wiki-sync-guardian
description: "导入前检查防止重复/错误目标，守护知识库数据一致性"
---

# Wiki 导入守卫

在用户准备执行 `/api/ingest` 导入操作之前触发，检查是否会产生重复或错误。

## 前置条件

- 用户准备导入新项目或重新导入已有项目
- 服务已启动：`http://localhost:8080`

## 工作流程

### 1. 检查项目是否已存在

```bash
# 查看已有哪些项目和子项目
curl -s http://localhost:8080/api/projects | python3 -c "
import sys, json
d = json.load(sys.stdin)
for p in d:
    print(f'  项目: {p[\"name\"]}')
    for sp in p.get('sub_project_list', []):
        print(f'    子项目: {sp[\"name\"]} ({sp[\"source_pages\"]} docs)')
"
```

### 2. 检查目标路径的 raw 目录

```bash
TARGET_RAW="{user_specified_raw_path}"

# 目标 raw 是否有内容
echo "=== 目标目录文件统计 ==="
find "$TARGET_RAW" -type f ! -name '*.hash' | wc -l
echo "个文件 (不含 hash)"

# 目标 raw 的 hash 记录（检测之前是否导入过）
echo "=== 已存在的 hash 记录 ==="
ls "$TARGET_RAW/"*.hash 2>/dev/null | wc -l
echo "个 hash 文件（= 之前已导入过的文件数）"

# 如果 hash 数 > 0 且接近总文件数 → 可能是重复导入
```

### 3. 三重防护检查

#### 检查 A：完全重复检测

```bash
# 场景：用户对同一目录再次执行 ingest
# 判断：目标 raw/ 下的所有 .hash 值是否与实际文件内容一致

HASH_MATCH=0
HASH_TOTAL=0

for f in $(find "$TARGET_RAW" -name "*.hash"); do
  HASH_TOTAL=$((HASH_TOTAL + 1))
  expected=$(cat "$f")
  src_file="${f%.hash}"
  if [ -f "$src_file" ]; then
    actual=$(shasum -a 256 "$src_file" | awk '{print $1}')
    [ "$expected" = "$actual" ] && HASH_MATCH=$((HASH_MATCH + 1))
  fi
done

echo "Hash 完全匹配: $HASH_MATCH / $HASH_TOTAL"
if [ "$HASH_MATCH" -eq "$HASH_TOTAL" ] && [ "$HASH_TOTAL" -gt 0 ]; then
  echo "⚠️ 警告: 所有文件都未变化！重复导入不会产生任何新数据。"
fi
```

#### 检查 B：跨子项目重复内容检测

```bash
# 场景：同一份代码用不同子项目名导入了两次
# 判断：目标目录的主要文件是否在其他 sub-project/raw/ 下已存在

# 取目标目录的特征文件（入口文件、配置文件）
SIGNATURE_FILES=$(ls "$TARGET_RAW"/*.go "$TARGET_RAW"/*.js "$TARGET_RAW"/package.json \
  "$TARGET_RAW"/*.vue "$TARGET_RAW"/go.mod 2>/dev/null | head -5)

for sig in $SIGNATURE_FILES; do
  name=$(basename "$sig")
  # 在所有其他 sub-project 中搜索同名文件
  FOUND_IN=$(find "knowledge-base/projects/" -path "*/raw/$name" ! -path "*/$TARGET_RAW/*" 2>/dev/null)
  if [ -n "$FOUND_IN" ]; then
    echo "[可能重复] $name 也存在于:"
    echo "$FOUND_IN"
  fi
done
```

#### 检查 C：目标合理性验证

| 检查项 | 验证方法 | 不通过时的处理 |
|--------|----------|--------------|
| 路径存在且可读 | `test -d` + `test -r` | 报错让用户提供正确路径 |
| 不是 node_modules/dist/vendor | `basename` 检查 | 自动跳过这些目录 |
| 不是 .git 目录本身 | 检查是否有 `.git` 子目录 | 进入内部而不是导出 .git |
| 文件数量合理 (<5000) | `find \| wc -l` | 过多时提示用户筛选核心文件 |
| 有可识别的项目结构 | 检查 package.json/go.mod/pom.xml 等 | 无法识别时提示用户使用 wiki-import-auditor |

### 4. 输出守卫报告

```
=== 导入前置检查报告 ===

目标路径: {raw_path}
建议项目名: {project}
建议子项目名: {sub_project}

【检查结果】
✅ 路径有效: {是/否}
✅ 可读权限: {是/否}
⚠️ 已有 hash: {n} 个 (其中 {m} 个匹配 = 未变化的文件)
⚠️ 可能跨项目重复: {n} 个特征文件在其他位置也发现
📁 预估导入量: ~{n} 个源文件

【结论】
情况A: 全新导入 ✅ 可以继续执行 /api/ingest
情况B: 重复导入 ⚠️ 所有文件未变化，无需操作  
情况C: 部分更新 ✅ 有 {n} 个文件发生变化，将走增量更新流程
情况D: 跨项目风险 ⚠️ 发现与其他子项目重叠，请确认不是误操作

【推荐操作】
{具体的 curl 命令或建议}
```

## 注意事项

1. **此 skill 是只读检查** — 不修改任何文件，不调用任何写入 API
2. **放在 ingest 之前执行** — 作为前置 gate，不是事后清理
3. **hash 匹配 ≠ 一定不要导入** — 如果用户明确知道改了文件但 hash 显示匹配，可能是文件被外部工具格式化（如 gofmt/linter）导致微小变化。此时可以提醒用户而非阻止
4. **与 wiki-import-auditor 配合** — 先用 import-auditor 筛选要导入的文件，再用 sync-guardian 检查是否该导入，最后才执行 ingest
