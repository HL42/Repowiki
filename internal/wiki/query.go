package wiki

import (
	"context"
	"fmt"
	"hash/fnv"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"repowiki/internal/llm"
)

const maxQuestionLen = 500 // 用户输入最大字符数，防超长输入撑爆 prompt

type scoredChunk struct {
	source  string
	title   string
	content string
	score   int
}

// Query 查询问答（RAG 流程，含 Query Refiner 多角度检索合并）
// history 为可选的多轮对话历史，用于保持对话上下文连贯性
func (e *Engine) Query(ctx context.Context, question string, history []ChatMessage) (*QueryResponse, error) {
	// 1. Query Refiner：将模糊问题拆解为多个具体子问题（传入历史以理解指代）
	subQuestions, refined := e.refineQuery(ctx, question, history)

	// 2. 对每个子问题分别检索，合并去重
	allChunks := e.multiRetrieve(subQuestions)
	if len(allChunks) == 0 {
		return &QueryResponse{
			Answer:        "知识库中暂无相关信息。",
			Confidence:    "low",
			SourcePages:   []string{},
			Citations:     []SourceCitation{},
			RefinedQueries: refined,
		}, nil
	}

	// 3. 按命中分排序，取 top chunks
	sort.Slice(allChunks, func(i, j int) bool { return allChunks[i].score > allChunks[j].score })

	contextText := buildContext(allChunks, 6000)
	answer, err := e.generateAnswer(ctx, question, contextText, history)
	if err != nil {
		return nil, fmt.Errorf("生成答案失败: %w", err)
	}

	sourceSet := make(map[string]bool)
	for _, ch := range allChunks {
		sourceSet[ch.source] = true
	}
	sources := make([]string, 0, len(sourceSet))
	for s := range sourceSet {
		sources = append(sources, filepath.Base(s))
	}
	sort.Strings(sources)

	return &QueryResponse{
		Answer:         answer,
		SourcePages:    sources,
		Citations:      buildCitations(allChunks, 8),
		Confidence:     confidenceForChunks(allChunks),
		MatchedChunks:  len(allChunks),
		RefinedQueries: refined,
	}, nil
}

// refineQuery 使用 LLM 将模糊问题拆解为多个具体子问题，从不同角度覆盖用户意图。
// history 用于在多轮对话中理解指代（如"它的返回值"中的"它"）。
// 返回子问题列表和用于展示的精简列表（失败时退回原问题）。
func (e *Engine) refineQuery(ctx context.Context, question string, history []ChatMessage) ([]string, []string) {
	// 截断过长输入，防超出 LLM 上下文
	cleaned := question
	runes := []rune(cleaned)
	if len(runes) > maxQuestionLen {
		cleaned = string(runes[:maxQuestionLen])
	}

	// 构建带历史上下文的 prompt
	prompt := ""
	if len(history) > 0 {
		contextParts := make([]string, 0, len(history))
		for _, h := range history {
			roleLabel := "用户"
			if h.Role == "assistant" {
				roleLabel = "助手"
			}
			contextParts = append(contextParts, roleLabel+": "+h.Content)
		}
		recentContext := strings.Join(contextParts, "\n")
		if len(recentContext) > 500 {
			recentContext = recentContext[len(recentContext)-500:]
		}
		prompt = fmt.Sprintf(`你是知识库查询优化助手。以下是对话历史，用户正在进行多轮追问，请结合历史理解指代关系。

对话历史：
%s

当前用户问题：%s

要求：
- 请先理解对话历史中的指代关系，再拆解当前问题
- 拆成 3-5 个更具体的子问题，从不同角度覆盖意图
- 每个子问题具体、可检索
- 覆盖角度：业务流程、技术实现、接口/模块、常见问题、配置/规范
- 直接输出，每行一个，不要编号和解释
- 如果原问题已足够具体，只输出原问题

子问题：`, recentContext, cleaned)
	} else {
		prompt = fmt.Sprintf(`你是知识库查询优化助手。把用户问题拆成 3-5 个更具体的子问题，从不同角度覆盖意图。

用户问题：%s

要求：
- 每个子问题具体、可检索
- 覆盖角度：业务流程、技术实现、接口/模块、常见问题、配置/规范
- 直接输出，每行一个，不要编号和解释
- 如果原问题已足够具体，只输出原问题

子问题：`, cleaned)
	}

	if resp, err := e.LLM.Chat(ctx, []llm.Message{{Role: "user", Content: prompt}}); err == nil {
		lines := strings.Split(strings.TrimSpace(resp), "\n")
		var subQs []string
		for _, line := range lines {
			line = strings.TrimLeft(strings.TrimSpace(line), "0123456789. -、•·()（）①②③④⑤")
			line = strings.TrimSpace(line)
			if line != "" && len([]rune(line)) > 2 {
				subQs = append(subQs, line)
			}
		}
		if len(subQs) > 0 {
			// 去重，原问题排最前；display 排除与原问题重复的项
			seen := map[string]bool{cleaned: true}
			result := []string{cleaned}
			var display []string
			for _, q := range subQs {
				if !seen[q] {
					seen[q] = true
					result = append(result, q)
					if len(display) < 3 {
						display = append(display, q)
					}
				}
			}
			if len(result) > 6 {
				result = result[:6]
			}
			return result, display
		}
	} else {
		fmt.Printf("[warn] refineQuery LLM 调用失败: %v，回退到原问题\n", err)
	}
	// 退回：直接搜原问题
	return []string{question}, nil
}

// multiRetrieve 对多个子问题分别检索，合并去重（并发搜索提升性能）
func (e *Engine) multiRetrieve(questions []string) []scoredChunk {
	perQuery := 8
	if len(questions) > 1 {
		perQuery = 6
	}

	var mu sync.Mutex
	seen := make(map[string]bool)
	var merged []scoredChunk
	var wg sync.WaitGroup

	for _, q := range questions {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()
			chunks, _ := e.retrieveRelevantChunksWithLimit(query, perQuery)
			mu.Lock()
			for _, ch := range chunks {
				// 去重键 = source + title + content 前 64 字符哈希，区分同文件不同段落
				key := ch.source + "\x00" + ch.title + "\x00" + hashContent(ch.content)
				if !seen[key] {
					seen[key] = true
					merged = append(merged, ch)
				}
			}
			mu.Unlock()
		}(q)
	}
	wg.Wait()
	return merged
}

func hashContent(s string) string {
	h := fnv.New64a()
	h.Write([]byte(s))
	return fmt.Sprintf("%x", h.Sum64())
}

func (e *Engine) retrieveRelevantChunks(question string) ([]scoredChunk, error) {
	return e.retrieveRelevantChunksWithLimit(question, 10)
}

func (e *Engine) retrieveRelevantChunksWithLimit(question string, limit int) ([]scoredChunk, error) {
	keywords := extractKeywords(question)
	if len(keywords) == 0 {
		keywords = strings.Fields(question)
	}
	return e.Searcher.Search(keywords, limit), nil
}

func extractKeywords(text string) []string {
	set := make(map[string]bool)
	for _, token := range strings.Fields(text) {
		token = strings.Trim(token, `，。！？、；：""''（）【】《》,.!?;:()[]{}`)
		if token != "" {
			set[token] = true
		}
	}

	runes := []rune(text)
	var buf []rune
	for _, r := range runes {
		if r >= 0x4e00 && r <= 0x9fff {
			buf = append(buf, r)
		} else if len(buf) > 0 {
			for i := 0; i < len(buf)-1; i++ {
				set[string(buf[i:i+2])] = true
			}
			buf = nil
		}
	}
	if len(buf) > 1 {
		for i := 0; i < len(buf)-1; i++ {
			set[string(buf[i:i+2])] = true
		}
	}

	keywords := make([]string, 0, len(set))
	for k := range set {
		keywords = append(keywords, k)
	}
	return keywords
}

func (e *Engine) generateAnswer(ctx context.Context, question string, contextText string, history []ChatMessage) (string, error) {
	var messages []llm.Message

	// 有对话历史时，加 system prompt 告知 LLM 这是多轮对话
	if len(history) > 0 {
		messages = append(messages, llm.Message{
			Role:    "system",
			Content: "你是知识库助手，正在与用户进行多轮对话。重要规则：\n- 如果用户问的是关于对话本身的元问题（如\"我刚刚问了什么\"\"刚才的回答是什么\"\"重复一遍\"\"总结一下之前说的\"），请直接基于上面的对话历史回答，不需要依赖知识库内容。\n- 如果用户的追问包含代词指代（如\"它的返回值\"\"第一个方案\"），请结合对话历史理解指代对象，再基于知识库内容回答。\n- 如果用户的新问题与历史完全无关，则独立基于知识库回答。",
		})
		for _, h := range history {
			// 仅允许 user 和 assistant 角色，防止前端传入 system 等角色混淆
			if h.Role != "user" && h.Role != "assistant" {
				continue
			}
			messages = append(messages, llm.Message{Role: h.Role, Content: h.Content})
		}
	}

	prompt := fmt.Sprintf("基于以下知识库内容，回答用户问题。\n\n知识库内容：\n%s\n\n用户问题：%s\n\n要求：\n1. 优先基于知识库回答，不要编造知识库中没有的细节\n2. 如果知识库内容不足，明确说明\"当前知识库中暂无充分信息\"\n3. 对关键结论标注来源文件名，例如：[来源: xxx.md]\n4. 如果涉及排查或代码定位，优先列出相关模块、接口、文件或字段\n\n回答：", contextText, question)
	messages = append(messages, llm.Message{Role: "user", Content: prompt})

	return e.LLM.Chat(ctx, messages)
}

func buildContext(chunks []scoredChunk, maxChars int) string {
	var builder strings.Builder
	total := 0
	for _, ch := range chunks {
		addition := fmt.Sprintf("\n---\n来源: %s\n章节: %s\n\n%s\n", ch.source, ch.title, ch.content)
		if total+len(addition) > maxChars {
			break
		}
		builder.WriteString(addition)
		total += len(addition)
	}
	return builder.String()
}

func buildCitations(chunks []scoredChunk, max int) []SourceCitation {
	if len(chunks) == 0 {
		return []SourceCitation{}
	}
	seen := make(map[string]bool)
	citations := make([]SourceCitation, 0, minInt(len(chunks), max))
	for _, ch := range chunks {
		key := ch.source + "\n" + ch.title
		if seen[key] {
			continue
		}
		seen[key] = true
		citations = append(citations, SourceCitation{
			Page:    filepath.Base(ch.source),
			Section: ch.title,
			Score:   ch.score,
			Excerpt: excerpt(ch.content, 220),
		})
		if len(citations) >= max {
			break
		}
	}
	return citations
}

func confidenceForChunks(chunks []scoredChunk) string {
	if len(chunks) == 0 {
		return "low"
	}
	if chunks[0].score >= 4 && len(chunks) >= 3 {
		return "high"
	}
	if chunks[0].score >= 2 {
		return "medium"
	}
	return "low"
}

func excerpt(content string, maxRunes int) string {
	fields := strings.Fields(content)
	text := strings.Join(fields, " ")
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[:maxRunes]) + "..."
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
