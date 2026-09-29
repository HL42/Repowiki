package wiki

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func (e *Engine) Synthesize(ctx context.Context, topic string) (*SynthesizeResponse, error) {
	sourceFiles, err := e.Storage.ListPages("sources")
	if err != nil {
		return nil, fmt.Errorf("列出源页面失败: %w", err)
	}

	var relevantContents []string
	var relevantPaths []string
	for _, sf := range sourceFiles {
		content, err := os.ReadFile(sf)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(string(content)), strings.ToLower(topic)) {
			relevantContents = append(relevantContents, string(content))
			relevantPaths = append(relevantPaths, filepath.Base(sf))
		}
	}

	if len(relevantContents) == 0 {
		return &SynthesizeResponse{
			Result: fmt.Sprintf("未找到与 %q 相关的页面", topic),
		}, nil
	}

	result, err := e.LLM.Synthesize(ctx, topic, relevantContents)
	if err != nil {
		return nil, fmt.Errorf("综合提炼失败: %w", err)
	}

	now := time.Now().Format("2006-01-02")
	pageContent := fmt.Sprintf(`---
title: "%s 综合提炼"
type: synthesis
status: published
created: %s
updated: %s
sources:
%s
confidence: medium
tags: [综合]
---

# %s 综合提炼

## 综合摘要

%s

## 关键洞察

%s

## 已发现矛盾

%s

## 知识空白

%s

## 来源页面

%s
`,
		topic, now, now,
		formatSources(result.Sources),
		topic,
		result.Summary,
		formatList(result.KeyInsights),
		formatList(result.Conflicts),
		formatList(result.Gaps),
		formatSources(relevantPaths),
	)

	// 如果有矛盾，额外生成矛盾记录页
	if len(result.Conflicts) > 0 {
		conflictContent := fmt.Sprintf(`---
title: "%s 矛盾记录"
type: contradiction
status: draft
created: %s
updated: %s
confidence: low
tags: [矛盾, %s]
---

# %s 矛盾记录

## 矛盾点

%s

## 相关来源

%s

## 待解决

（需要人工确认）
`, topic, now, now, topic, topic,
			formatList(result.Conflicts),
			formatSources(relevantPaths),
		)
		e.Storage.WritePage("contradictions", sanitizeSlug(topic), conflictContent)
	}

	slug := sanitizeSlug(topic + "-综合")
	if err := e.Storage.WritePage("syntheses", slug, pageContent); err != nil {
		return nil, fmt.Errorf("保存综合页失败: %w", err)
	}

	e.regenerateIndex()
	e.invalidateCaches()

	e.Storage.AppendLog(fmt.Sprintf("## Synthesize - %s\n- 时间: %s\n- 相关页: %d\n- 矛盾: %d\n- 空白: %d",
		topic, now, len(relevantPaths), len(result.Conflicts), len(result.Gaps)))

	return &SynthesizeResponse{
		PagePath: filepath.Join("wiki", "syntheses", slug+".md"),
		Result:   fmt.Sprintf("综合完成。发现 %d 个矛盾，%d 个知识空白。", len(result.Conflicts), len(result.Gaps)),
	}, nil
}

// ---------- 辅助 ----------

func formatSources(sources []string) string {
	var lines []string
	for _, s := range sources {
		lines = append(lines, fmt.Sprintf("  - \"%s\"", s))
	}
	return strings.Join(lines, "\n")
}

func formatList(items []string) string {
	if len(items) == 0 {
		return "（暂无）"
	}
	var lines []string
	for _, item := range items {
		lines = append(lines, fmt.Sprintf("- %s", item))
	}
	return strings.Join(lines, "\n")
}
