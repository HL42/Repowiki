package storage

import (
	"regexp"
	"strings"
)

// 文档分块
type Chunk struct {
	Title   string
	Content string
	Source  string
}

// ChunkMarkdown 按 ##/### 标题分块 每个分块不能超过MaxChars 字符
func ChunkMarkdown(content string, source string, maxChars int) []Chunk {
	if maxChars <= 0 {
		maxChars = 3000
	}

	var chunks []Chunk
	lines := strings.Split(content, "\n")

	var currentTitle string
	var currentLines []string
	currentLen := 0

	flush := func() {
		if len(currentLines) == 0 {

			return
		}
		chunks = append(chunks, Chunk{
			Title:   currentTitle,
			Content: strings.Join(currentLines, "\n"),
			Source:  source,
		})
		currentLines = nil
		currentLen = 0
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "#") {

			flush()
			currentTitle = strings.TrimLeft(line, "# ")
			currentLines = append(currentLines, line)
			currentLen += len(line)
			continue
		}

		if currentLen+len(line) > maxChars {

			flush()
			currentTitle = currentTitle + "（续）"
		}

		currentLines = append(currentLines, line)
		currentLen += len(line)
	}
	flush()

	return chunks
}

// ---------- 代码文件分块 ----------

// ChunkCodeFile 按函数/类型/类定义边界分块代码文件
// 每种语言用各自的正则匹配定义行作为分块边界
func ChunkCodeFile(content string, source string, maxChars int, language string) []Chunk {
	if maxChars <= 0 {
		maxChars = 3000
	}

	pattern := definitionPattern(language)
	if pattern == nil {
		// 不支持的语言，整块返回
		return []Chunk{{Title: source, Content: content, Source: source}}
	}

	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var currentTitle string
	var currentLines []string
	currentLen := 0

	flush := func() {
		if len(currentLines) == 0 {
			return
		}
		chunks = append(chunks, Chunk{
			Title:   currentTitle,
			Content: strings.Join(currentLines, "\n"),
			Source:  source,
		})
		currentLines = nil
		currentLen = 0
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		isDefinition := pattern.MatchString(trimmed)
		// 跳过空行和注释行
		isComment := strings.HasPrefix(trimmed, "//") ||
			strings.HasPrefix(trimmed, "#") ||
			strings.HasPrefix(trimmed, "/*") ||
			strings.HasPrefix(trimmed, "*")

		if isDefinition {
			flush()
			currentTitle = extractDefName(trimmed, language)
			currentLines = append(currentLines, line)
			currentLen = len(line)
			continue
		}

		if isComment && currentLen == 0 {
			continue // 块开头跳过纯注释
		}

		if currentLen+len(line) > maxChars && currentLen > 0 {
			flush()
			currentTitle = currentTitle + "（续）"
		}

		currentLines = append(currentLines, line)
		currentLen += len(line)
	}
	flush()

	return chunks
}

// definitionPattern 返回匹配函数/类型/类定义的正则
func definitionPattern(language string) *regexp.Regexp {
	switch language {
	case "go":
		return regexp.MustCompile(`^(func |type |var |const |func \()`)
	case "python":
		return regexp.MustCompile(`^(def |class |async def )`)
	case "javascript", "typescript", "js", "ts":
		return regexp.MustCompile(`^(function |class |export |const |let |var |async function |interface |type |enum )`)
	case "vue":
		return regexp.MustCompile(`^(export |function |class |const |let |var |<script>|<template>|<style)`)
	case "java":
		return regexp.MustCompile(`^\s*(public |private |protected |class |interface |enum |@)`)
	case "rust":
		return regexp.MustCompile(`^(fn |pub |struct |enum |trait |impl |mod |const |static |type )`)
	default:
		return nil
	}
}

// extractDefName 从定义行中提取函数/类型名称
func extractDefName(line, language string) string {
	// 去掉常见修饰符和注释
	line = strings.TrimSpace(line)
	line = regexp.MustCompile(`//.*$|#.*$`).ReplaceAllString(line, "")

	// 通用：取第二个"单词"（func Foo → Foo, class Foo → Foo）
	words := strings.Fields(line)
	if len(words) >= 2 {
		// 跳过修饰符
		for i, w := range words {
			switch w {
			case "func", "def", "class", "function", "export", "public",
				"private", "protected", "pub", "fn", "struct", "enum",
				"trait", "impl", "mod", "interface", "type", "const",
				"let", "var", "async", "static":
				continue
			default:
				if i+1 < len(words) && words[i] == "func" {
					continue
				}
				name := w
				// 去掉括号和泛型
				name = strings.SplitN(name, "(", 2)[0]
				name = strings.SplitN(name, "<", 2)[0]
				name = strings.SplitN(name, "{", 2)[0]
				name = strings.SplitN(name, ":", 2)[0]
				return strings.TrimSpace(name)
			}
		}
	}
	return line[:min(len(line), 40)]
}

// DetectLanguage 根据文件扩展名推断语言
func DetectLanguage(filename string) string {
	ext := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(ext, ".go"):
		return "go"
	case strings.HasSuffix(ext, ".py"):
		return "python"
	case strings.HasSuffix(ext, ".js"):
		return "javascript"
	case strings.HasSuffix(ext, ".ts"):
		return "typescript"
	case strings.HasSuffix(ext, ".vue"):
		return "vue"
	case strings.HasSuffix(ext, ".java"):
		return "java"
	case strings.HasSuffix(ext, ".rs"):
		return "rust"
	case strings.HasSuffix(ext, ".md"):
		return "markdown"
	default:
		return ""
	}
}

// IsCodeFile 判断是否为支持的代码文件
func IsCodeFile(filename string) bool {
	lang := DetectLanguage(filename)
	return lang != "" && lang != "markdown"
}
