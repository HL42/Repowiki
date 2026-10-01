#!/bin/bash
# Wiki Skills 安装脚本
# 将项目专用的 5 个 Skill 安装到 CodeBuddy 的 user 级目录
# 用法: bash skills/install.sh

set -e

SKILL_SRC="$(cd "$(dirname "$0")" && pwd)"
SKILL_DST="$HOME/.codebuddy/skills/user"

SKILLS=(
  "wiki-import-auditor"
  "wiki-quality-detector"
  "wiki-query-refiner"
  "wiki-deduplicator"
  "wiki-sync-guardian"
)

echo "=== RepoWiki Skills 安装器 ==="
echo "源目录: $SKILL_SRC"
echo "目标: $SKILL_DST"
echo ""

# 确保目标目录存在
mkdir -p "$SKILL_DST"

INSTALLED=0
SKIPPED=0

for skill in "${SKILLS[@]}"; do
  SRC="$SKILL_SRC/$skill/SKILL.md"
  DST="$SKILL_DST/$skill/SKILL.md"

  if [ ! -f "$SRC" ]; then
    echo "⚠️  跳过 $skill (源文件不存在: $SRC)"
    SKIPPED=$((SKIPPED + 1))
    continue
  fi

  if [ -f "$DST" ]; then
    # 检查是否相同
    if diff -q "$SRC" "$DST" >/dev/null 2>&1; then
      echo "✅ $skill (已是最新)"
      SKIPPED=$((SKIPPED + 1))
    else
      cp -r "$SRC" "$DST"
      echo "🔄 $skill (已更新)"
      INSTALLED=$((INSTALLED + 1))
    fi
  else
    mkdir -p "$(dirname "$DST")"
    cp -r "$SKILL_SRC/$skill" "$DST"
    echo "✨ $skill (新安装)"
    INSTALLED=$((INSTALLED + 1))
  fi
done

echo ""
echo "=== 安装完成 ==="
echo "新安装/更新: $INSTALLED 个"
echo "跳过(已最新): $SKIPPED 个"
echo ""
echo "当前已安装的 wiki skills:"
ls "$SKILL_DST"/wiki-*/SKILL.md 2>/dev/null | while read f; do
  name=$(dirname "$f" | xargs basename)
  lines=$(wc -l < "$f")
  echo "  ✅ $name ($lines 行)"
done
echo ""
echo "重启 Agent 后即可使用这些 skills"
