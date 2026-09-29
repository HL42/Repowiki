package wiki

import (
	"sort"
	"strings"
	"sync"

	"repowiki/internal/storage"
)

// SearchEngine 搜索接口：后期切 ES 只需实现此接口，query/ingest 不用改
type SearchEngine interface {
	// Index 索引一个文档的所有分块（导入/更新时调用）
	Index(docPath string, chunks []storage.Chunk)
	// Search 按关键词搜索，返回评分最高的 topN 个块
	Search(keywords []string, topN int) []scoredChunk
	// Remove 移除某个文档的索引（删除页时调用）
	Remove(docPath string)
	// Rebuild 从存储重建全部索引（启动时调用）
	Rebuild(listPages func() ([]string, error), getChunks func(string) ([]storage.Chunk, error)) error
}

// InMemorySearch 纯内存倒排索引（零外部依赖，毫秒级查询）
type InMemorySearch struct {
	mu       sync.RWMutex
	inverted map[string][]posting // 关键词 → 文档块列表
	docMap   map[string][]storage.Chunk
}

type posting struct {
	Source  string
	Title   string
	Content string
}

func NewInMemorySearch() *InMemorySearch {
	return &InMemorySearch{
		inverted: make(map[string][]posting),
		docMap:   make(map[string][]storage.Chunk),
	}
}

// Index 对文档分块建倒排索引
func (s *InMemorySearch) Index(docPath string, chunks []storage.Chunk) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 先移除旧索引
	s.remove(docPath)
	// 存原始块（用于 Rebuild 时重新分词）
	s.docMap[docPath] = chunks

	indexChunks(s.inverted, chunks)
}

// Search O(K) 查倒排索引，按命中词数计分，取 topN
func (s *InMemorySearch) Search(keywords []string, topN int) []scoredChunk {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 统计每个块命中多少关键词
	scores := make(map[postingKey]int)
	contents := make(map[postingKey]scoredChunk)
	for _, kw := range keywords {
		kw = strings.ToLower(kw)
		for _, p := range s.inverted[kw] {
			k := postingKey{Source: p.Source, Title: p.Title}
			scores[k]++
			if _, ok := contents[k]; !ok {
				contents[k] = scoredChunk{
					source:  p.Source,
					title:   p.Title,
					content: p.Content,
				}
			}
		}
	}

	// 转为列表并按分数降序排序
	var result []scoredChunk
	for k, s := range scores {
		sc := contents[k]
		sc.score = s
		result = append(result, sc)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].score > result[j].score
	})

	if len(result) > topN {
		result = result[:topN]
	}
	return result
}

// Remove 移除文档索引
func (s *InMemorySearch) Remove(docPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.remove(docPath)
}

func (s *InMemorySearch) remove(docPath string) {
	chunks, ok := s.docMap[docPath]
	if !ok {
		return
	}
	for _, ch := range chunks {
		terms := tokenize(ch.Content)
		for _, t := range terms {
			list := s.inverted[t]
			filtered := list[:0]
			for _, p := range list {
				if p.Source != ch.Source || p.Title != ch.Title {
					filtered = append(filtered, p)
				}
			}
			if len(filtered) == 0 {
				delete(s.inverted, t)
			} else {
				s.inverted[t] = filtered
			}
		}
	}
	delete(s.docMap, docPath)
}

// Rebuild 从存储全量重建索引（启动时调用）
func (s *InMemorySearch) Rebuild(
	listPages func() ([]string, error),
	getChunks func(string) ([]storage.Chunk, error),
) error {
	pages, err := listPages()
	if err != nil {
		return err
	}

	inverted := make(map[string][]posting)
	docMap := make(map[string][]storage.Chunk)
	for _, page := range pages {
		chunks, err := getChunks(page)
		if err != nil {
			continue
		}
		docMap[page] = chunks
		indexChunks(inverted, chunks)
	}

	s.mu.Lock()
	s.inverted = inverted
	s.docMap = docMap
	s.mu.Unlock()
	return nil
}

func indexChunks(inverted map[string][]posting, chunks []storage.Chunk) {
	for _, ch := range chunks {
		terms := tokenize(ch.Content)
		seen := make(map[string]bool)
		for _, t := range terms {
			if seen[t] {
				continue
			}
			seen[t] = true
			inverted[t] = append(inverted[t], posting{
				Source:  ch.Source,
				Title:   ch.Title,
				Content: ch.Content,
			})
		}
	}
}

// ---------- 分词 ----------

type postingKey struct {
	Source string
	Title  string
}

// tokenize 与 extractKeywords 逻辑一致：单词 token + 中文 bigram
func tokenize(text string) []string {
	set := make(map[string]bool)
	// 1. 按空白分词
	for _, token := range strings.Fields(text) {
		token = strings.ToLower(token)
		token = strings.Trim(token, `，。！？、；：""''（）【】《》,.!?;:()[]{}`)
		if token != "" {
			set[token] = true
		}
	}
	// 2. 中文 bigram
	runes := []rune(text)
	var buf []rune
	for _, r := range runes {
		if r >= 0x4e00 && r <= 0x9fff {
			buf = append(buf, r)
		} else if len(buf) > 0 {
			addBigrams(buf, set)
			buf = nil
		}
	}
	if len(buf) > 1 {
		addBigrams(buf, set)
	}

	result := make([]string, 0, len(set))
	for k := range set {
		result = append(result, k)
	}
	return result
}

func addBigrams(runes []rune, set map[string]bool) {
	for i := 0; i < len(runes)-1; i++ {
		set[string(runes[i:i+2])] = true
	}
}
