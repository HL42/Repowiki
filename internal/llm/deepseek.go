package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
)

// Message Part
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// 结构化结果类型
type DocumentMeta struct {
	Title    string   `json:"title"`
	Summary  string   `json:"summary"`
	Keywords []string `json:"keywords"`
	Entities []string `json:"entities"`
	Concepts []string `json:"concepts"`
	KeyFacts []string `json:"key_facts"`
	Category string   `json:"category"`
}

// CodeMeta 代码文件的元数据（用于生成模块说明页）
type CodeMeta struct {
	ModuleName   string   `json:"module_name"`
	Summary      string   `json:"summary"`
	PublicAPIs   []string `json:"public_apis"`
	Dependencies []string `json:"dependencies"`
	Patterns     []string `json:"patterns"`
	Language     string   `json:"language"`
	Layer        string   `json:"-"` // 技术层级，由 classifyCodeFile 规则填充（不由 LLM 返回）
	Domain       string   `json:"-"` // 业务域，由 classifyCodeFile 规则填充（不由 LLM 返回）
	Kind         string   `json:"-"` // 代码类型，由 classifyCodeFile 规则填充（不由 LLM 返回）
}

type SynthesisResult struct {
	Topic       string   `json:"topic"`
	Summary     string   `json:"summary"`
	KeyInsights []string `json:"key_insights"`
	Conflicts   []string `json:"conflicts"`
	Gaps        []string `json:"gaps"`
	Sources     []string `json:"sources"`
}

// 客户端
type DeepSeekClient struct {
	BaseURL string
	APIKey  string
	Model   string
	Client  *http.Client
}

func NewDeepSeekClient(apiKey, model string) *DeepSeekClient {
	if model == "" {

		model = "deepseek-chat"
	}

	return &DeepSeekClient{
		APIKey:  apiKey,
		BaseURL: "https://api.deepseek.com",
		Model:   model,
		Client:  &http.Client{Timeout: 120 * time.Second},
	}
}

// Chat
type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *DeepSeekClient) Chat(ctx context.Context, messages []Message) (string, error) {
	var lastErr error
	for i := 0; i < 2; i++ {

		result, err := c.doChat(ctx, messages)
		if err == nil {

			return result, nil
		}
		lastErr = err
		if i == 0 {

			time.Sleep(2 * time.Second)
		}
	}
	return "", fmt.Errorf("Chat API 调用失败: %w", lastErr)
}

func (c *DeepSeekClient) doChat(ctx context.Context, messages []Message) (string, error) {
	reqbody := chatRequest{
		Model:       c.Model,
		Messages:    messages,
		Temperature: 0.3,
	}

	jsonData, err := json.Marshal(reqbody)
	if err != nil {
		return "", fmt.Errorf("序列化请求体失败: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/chat/completions", bytes.NewReader(jsonData))
	if err != nil {

		return "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.APIKey)

	resp, err := c.Client.Do(req)
	if err != nil {

		return "", fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 返回 HTTP %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {

		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	var cr chatResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	if cr.Error != nil {
		return "", fmt.Errorf("API 错误: %s", cr.Error.Message)
	}

	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("API 返回结果为空")
	}

	return cr.Choices[0].Message.Content, nil
}

// ========== 结构化提取 ==========

// ExtractMetadata 提取文档元数据（标题、摘要、关键词等）
func (c *DeepSeekClient) ExtractMetadata(ctx context.Context, content string) (*DocumentMeta, error) {
	// 截取前 8000 字符
	truncated := truncateUTF8(content, 8000)

	prompt := `请分析以下文档，提取元数据。

要求：entities 和 concepts 只填原文中明确写出的内容，不要联想、不要补充无关内容。如果原文没有，就返回空数组 []。

只返回 JSON，不要任何额外文字。

{
  "title": "文档标题",
  "summary": "摘要（200字以内）",
  "keywords": ["关键词1", "关键词2"],
  "entities": ["原文提到的具体实体"],
  "concepts": ["原文提到的技术术语/方法论"],
  "key_facts": ["原文中的关键事实"],
  "category": "文档类型（technical/report/meeting/guide/其他）"
}

文档内容：
` + truncated

	msg := []Message{{Role: "user", Content: prompt}}
	result, err := c.Chat(ctx, msg)
	if err != nil {
		return nil, err
	}

	return parseDocumentMeta(result)
}

// ExtractList 提取实体或概念列表
func (c *DeepSeekClient) ExtractList(ctx context.Context, content string, label string) ([]string, error) {
	truncated := truncateUTF8(content, 6000)

	prompt := fmt.Sprintf(
		`从以下文档中，提取所有%s。

要求：只提取原文中明确写出的，不要联想、不要补充无关内容。如果原文没有提到任何%s，返回空数组 []。

返回 JSON 字符串数组，格式如 ["a", "b"]。只返回 JSON。

文档：
%s`, label, label, truncated)

	msg := []Message{{Role: "user", Content: prompt}}
	result, err := c.Chat(ctx, msg)
	if err != nil {
		return nil, err
	}

	return parseStringArray(result)
}

// ExtractCodeMeta 从代码文件中提取模块说明元数据
// layer/domain/kind 由路径规则预分类（不用 LLM），作为上下文帮助 LLM 写更准确的摘要
func (c *DeepSeekClient) ExtractCodeMeta(ctx context.Context, content string, language string, layer, domain, kind string) (*CodeMeta, error) {
	truncated := truncateUTF8(content, 8000)

	classCtx := ""
	if layer != "" || domain != "" {
		classCtx = fmt.Sprintf("\n此文件的自动分类：层级=%s，业务域=%s，类型=%s。请基于此分类写出更准确的摘要。\n", layer, domain, kind)
	}

	prompt := fmt.Sprintf(`分析以下%s代码，提取模块说明信息。只返回 JSON，不要任何额外文字。%s
{
  "module_name": "此文件/模块的核心功能名称",
  "summary": "功能概述（200字以内，说明这个文件做什么、怎么做的）",
  "public_apis": ["公开函数/方法签名", "对外暴露的类型"],
  "dependencies": ["此文件依赖的内部模块/包"],
  "patterns": ["使用的设计模式或架构约定"],
  "language": "%s"
}

要求：只描述代码中实际存在的内容，不要联想、不要补充不存在的东西。

代码内容：
%s`, language, classCtx, language, truncated)

	msg := []Message{{Role: "user", Content: prompt}}
	result, err := c.Chat(ctx, msg)
	if err != nil {
		return nil, err
	}

	return parseCodeMeta(result, language)
}

// AccumulateEntityPage 增量合并实体页
func (c *DeepSeekClient) AccumulateEntityPage(ctx context.Context, entityName, existingContent, newSource, newFacts string) (string, error) {
	if existingContent == "" {
		now := time.Now().Format("2006-01-02")
		return fmt.Sprintf(`---
title: "%s"
type: entity
status: draft
created: %s
updated: %s
sources:
  - "%s"
confidence: medium
---

# %s

## 来源

- %s

## 关键信息

%s
`, entityName, now, now, newSource, entityName, newSource, newFacts), nil
	}

	prompt := "合并新信息到实体\"" + entityName + "\"的现有页面。\n\n" +
		"要求：\n" +
		"1. 保留原有 frontmatter，更新 updated 为 " + time.Now().Format("2006-01-02") + "，追加新来源\n" +
		"2. 不删除已有内容，只追加新事实\n" +
		"3. 如果有冲突信息，标注\"待确认\"\n" +
		"4. 返回完整 Markdown 页面\n\n" +
		"现有页面：\n" + existingContent + "\n\n" +
		"新来源：" + newSource + "\n新信息：\n" + newFacts + "\n\n返回完整页面："

	msg := []Message{{Role: "user", Content: prompt}}
	return c.Chat(ctx, msg)
}

// AccumulateConceptPage 增量合并概念页
func (c *DeepSeekClient) AccumulateConceptPage(ctx context.Context, conceptName, existingContent, newSource, newFacts string) (string, error) {
	if existingContent == "" {
		now := time.Now().Format("2006-01-02")
		return fmt.Sprintf(`---
title: "%s"
type: concept
status: draft
created: %s
updated: %s
sources:
  - "%s"
confidence: medium
---

# %s

## 定义

（待补充）

## 来源

- %s

## 相关信息

%s
`, conceptName, now, now, newSource, conceptName, newSource, newFacts), nil
	}

	prompt := "合并新信息到概念\"" + conceptName + "\"的现有页面。\n\n" +
		"要求：\n" +
		"1. 保留原有 frontmatter，更新 updated 为 " + time.Now().Format("2006-01-02") + "，追加新来源\n" +
		"2. 不删除已有内容，只追加新理解\n" +
		"3. 如果有冲突，标注\"待确认\"\n" +
		"4. 返回完整 Markdown 页面\n\n" +
		"现有页面：\n" + existingContent + "\n\n" +
		"新来源：" + newSource + "\n新信息：\n" + newFacts + "\n\n返回完整页面："

	msg := []Message{{Role: "user", Content: prompt}}
	return c.Chat(ctx, msg)
}

// AccumulateEntityPageDiff 文档更新时的实体页 diff 合并
// oldFacts 是旧版本文档的事实，newFacts 是新版本文档的事实
func (c *DeepSeekClient) AccumulateEntityPageDiff(ctx context.Context, entityName, existingContent, sourcePage, oldFacts, newFacts string) (string, error) {
	if existingContent == "" {
		// 全新实体，走原来的创建逻辑
		return c.AccumulateEntityPage(ctx, entityName, "", sourcePage, newFacts)
	}

	now := time.Now().Format("2006-01-02")
	prompt := fmt.Sprintf(`来源文档已更新，请整合实体"%s"的页面内容。

要求：
1. 保留原有 frontmatter（更新时间 updated 为 %s）
2. 对比旧信息和新信息：
   - 如果旧信息在新版本中被更正或移除，标注"（已更新）"并替换为新信息
   - 如果新信息是新增的，追加到对应章节
   - 如果新旧信息一致，保留原有描述
3. 在页面末尾添加"## 变更记录"章节，简要说明本次更新了什么
4. 返回完整的 Markdown 页面

现有页面：
%s

旧版本文档中的事实：
%s

新版本文档中的事实：
%s

返回完整页面：`, entityName, now, existingContent, oldFacts, newFacts)

	msg := []Message{{Role: "user", Content: prompt}}
	return c.Chat(ctx, msg)
}

// AccumulateConceptPageDiff 文档更新时的概念页 diff 合并
func (c *DeepSeekClient) AccumulateConceptPageDiff(ctx context.Context, conceptName, existingContent, sourcePage, oldFacts, newFacts string) (string, error) {
	if existingContent == "" {
		return c.AccumulateConceptPage(ctx, conceptName, "", sourcePage, newFacts)
	}

	now := time.Now().Format("2006-01-02")
	prompt := fmt.Sprintf(`来源文档已更新，请整合概念"%s"的页面内容。

要求：
1. 保留原有 frontmatter（更新时间 updated 为 %s）
2. 对比旧理解和新理解：
   - 如果旧理解被新版本更正，标注"（已更新）"并替换为新理解
   - 如果新资料提供了新的视角，追加到对应章节
   - 如果新旧理解一致，保留原有描述
3. 在页面末尾添加"## 变更记录"章节，简要说明本次更新了什么
4. 返回完整的 Markdown 页面

现有页面：
%s

旧版本文档中的资料：
%s

新版本文档中的资料：
%s

返回完整页面：`, conceptName, now, existingContent, oldFacts, newFacts)

	msg := []Message{{Role: "user", Content: prompt}}
	return c.Chat(ctx, msg)
}

// Synthesize 综合跨文档信息
func (c *DeepSeekClient) Synthesize(ctx context.Context, topic string, sourceContents []string) (*SynthesisResult, error) {
	var sourcesText string
	for i, sc := range sourceContents {
		sourcesText += fmt.Sprintf("\n--- 来源 %d ---\n%s\n", i+1, sc)
	}

	prompt := "综合以下来源，完成一次知识提炼。\n\n" +
		"主题：" + topic + "\n" +
		"各来源信息：\n" + sourcesText + "\n\n" +
		`返回 JSON：
{
  "topic": "提炼主题",
  "summary": "综合摘要（500字内）",
  "key_insights": ["洞察1", "洞察2"],
  "conflicts": ["矛盾点1"],
  "gaps": ["知识空白1"],
  "sources": ["来源1", "来源2"]
}` + "\n\n只返回 JSON。"

	msg := []Message{{Role: "user", Content: prompt}}
	result, err := c.Chat(ctx, msg)
	if err != nil {
		return nil, err
	}

	return parseSynthesisResult(result)
}

// ========== 解析工具 ==========

func truncateUTF8(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	var count int
	for i := range s {
		if count >= maxRunes {
			return s[:i]
		}
		count++
	}
	return s
}

func cleanJSON(raw string) string {
	raw = strings.TrimSpace(raw)
	// 去掉 ```json ... ``` 或 ``` ... ``` 包裹
	for _, prefix := range []string{"```json\n", "```json ", "```\n", "``` "} {
		if strings.HasPrefix(raw, prefix) && strings.HasSuffix(raw, "```") {
			return strings.TrimSpace(raw[len(prefix) : len(raw)-3])
		}
	}
	return raw
}

func parseDocumentMeta(raw string) (*DocumentMeta, error) {
	cleaned := cleanJSON(raw)
	var meta DocumentMeta
	if err := json.Unmarshal([]byte(cleaned), &meta); err != nil {
		return nil, fmt.Errorf("解析文档元数据 JSON 失败: %w\n原始输出（前 200 字符）: %s", err, truncateForError(raw))
	}
	return &meta, nil
}

func truncateForError(s string) string {
	runes := []rune(s)
	if len(runes) > 200 {
		return string(runes[:200]) + "..."
	}
	return s
}

func parseStringArray(raw string) ([]string, error) {
	cleaned := cleanJSON(raw)
	var arr []string
	if err := json.Unmarshal([]byte(cleaned), &arr); err != nil {
		// 降级1：按逗号分割（LLM 经常返回格式不标准的长列表）
		cleaned = strings.Trim(cleaned, `[]`)
		var fallback []string
		for _, item := range strings.Split(cleaned, ",") {
			item = strings.TrimSpace(item)
			item = strings.Trim(item, `"`)
			if item != "" {
				fallback = append(fallback, item)
			}
		}
		if len(fallback) > 0 {
			return fallback, nil
		}
		// 降级2：逐行拆
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			line = strings.Trim(line, `[],"`)
			if line != "" {
				fallback = append(fallback, line)
			}
		}
		return fallback, nil
	}
	return arr, nil
}

func parseCodeMeta(raw string, language string) (*CodeMeta, error) {
	cleaned := cleanJSON(raw)
	var cm CodeMeta
	if err := json.Unmarshal([]byte(cleaned), &cm); err != nil {
		return &CodeMeta{
			ModuleName: "未命名模块",
			Summary:    raw,
			Language:   language,
		}, nil
	}
	if cm.Language == "" {
		cm.Language = language
	}
	return &cm, nil
}

func parseSynthesisResult(raw string) (*SynthesisResult, error) {
	cleaned := cleanJSON(raw)
	var sr SynthesisResult
	if err := json.Unmarshal([]byte(cleaned), &sr); err != nil {
		return nil, fmt.Errorf("解析综合结果失败: %w", err)
	}
	return &sr, nil
}
