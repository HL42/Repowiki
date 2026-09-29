package wiki

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"repowiki/internal/storage"
)

// MeilisearchSearch 基于 Meilisearch 的搜索引擎实现（纯 HTTP，零外部依赖）
// 相比 InMemorySearch，支持中文分词、BM25 排序、拼写容错、持久化存储
type MeilisearchSearch struct {
	baseURL   string       // Meilisearch 服务地址，如 http://localhost:7700
	indexUID  string       // 索引名，用于隔离不同引擎（根/项目/子项目）
	masterKey string       // 可选密钥，生产环境使用
	client    *http.Client // HTTP 客户端
	built     bool         // 索引是否已构建（避免重复 Rebuild）
	builtOnce sync.Once    // 确保 Rebuild 只执行一次（并发安全）
}

// NewMeilisearchSearch 创建 Meilisearch 搜索适配器
func NewMeilisearchSearch(indexUID string) *MeilisearchSearch {
	return &MeilisearchSearch{
		baseURL:   "http://localhost:7700",
		indexUID:  indexUID,
		masterKey: os.Getenv("MEILISEARCH_MASTER_KEY"),
		client:    &http.Client{Timeout: 30 * time.Second},
	}
}

// meiliDocument Meilisearch 文档格式
type meiliDocument struct {
	ID      string `json:"id"`
	Source  string `json:"source"`
	Title   string `json:"title"`
	Content string `json:"content"`
}

// meiliSearchRequest Meilisearch 搜索请求
type meiliSearchRequest struct {
	Q     string `json:"q"`
	Limit int    `json:"limit"`
}

// meiliSearchResponse Meilisearch 搜索响应
type meiliSearchResponse struct {
	Hits []struct {
		Source        string  `json:"source"`
		Title         string  `json:"title"`
		Content       string  `json:"content"`
		RankingScore  float64 `json:"_rankingScore"`
	} `json:"hits"`
}

// meiliTaskResponse Meilisearch 异步任务响应
type meiliTaskResponse struct {
	TaskUID int    `json:"taskUid"`
	Status  string `json:"status"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// meiliStatsResponse Meilisearch 索引统计
type meiliStatsResponse struct {
	NumberOfDocuments int `json:"numberOfDocuments"`
}

// ---------- SearchEngine 接口实现 ----------

// Index 索引一个文档的所有分块（同步等待任务完成，确保导入后立即可搜索）
func (m *MeilisearchSearch) Index(docPath string, chunks []storage.Chunk) {
	docs := make([]meiliDocument, 0, len(chunks))
	for _, ch := range chunks {
		docs = append(docs, meiliDocument{
			ID:      docID(ch.Source, ch.Title),
			Source:  ch.Source,
			Title:   ch.Title,
			Content: ch.Content,
		})
	}
	m.sendDocumentsSync(docs)
}

// Search 搜索，返回评分最高的 topN 个块
func (m *MeilisearchSearch) Search(keywords []string, topN int) []scoredChunk {
	query := strings.Join(keywords, " ")
	if query == "" {
		return nil
	}

	reqBody := meiliSearchRequest{Q: query, Limit: topN}
	body, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Printf("[meilisearch] marshal error: %v\n", err)
		return nil
	}

	resp, err := m.doRequest("POST", "/indexes/"+m.indexUID+"/search", body)
	if err != nil {
		fmt.Printf("[meilisearch] search error: %v\n", err)
		return nil
	}
	defer resp.Body.Close()

	var result meiliSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Printf("[meilisearch] decode error: %v\n", err)
		return nil
	}

	chunks := make([]scoredChunk, 0, len(result.Hits))
	for _, hit := range result.Hits {
		// 将 _rankingScore (0-1) 放大 1000 倍保留精度，避免同类分数合并丢失排序区分度
		score := int(hit.RankingScore*1000 + 0.5)
		if score < 1 && hit.RankingScore > 0 {
			score = 1
		}
		chunks = append(chunks, scoredChunk{
			source:  hit.Source,
			title:   hit.Title,
			content: hit.Content,
			score:   score,
		})
	}

	// 按分数降序排列
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].score > chunks[j].score
	})

	return chunks
}

// Remove 移除某个文档的所有分块索引
func (m *MeilisearchSearch) Remove(docPath string) {
	// 拒绝包含换行符的路径，防止过滤器注入
	if strings.ContainsAny(docPath, "\n\r") {
		fmt.Printf("[meilisearch] invalid docPath contains newline: %s\n", docPath)
		return
	}
	filter := fmt.Sprintf(`source = "%s"`, escapeFilterValue(docPath))
	payload, err := json.Marshal(map[string]string{"filter": filter})
	if err != nil {
		fmt.Printf("[meilisearch] marshal error: %v\n", err)
		return
	}
	m.doRequest("POST", "/indexes/"+m.indexUID+"/documents/delete-batch", payload)
}

// Rebuild 从存储全量重建索引（sync.Once 保护，并发安全）
func (m *MeilisearchSearch) Rebuild(
	listPages func() ([]string, error),
	getChunks func(string) ([]storage.Chunk, error),
) error {
	var rebuildErr error
	m.builtOnce.Do(func() {
		rebuildErr = m.doRebuild(listPages, getChunks)
	})
	return rebuildErr
}

func (m *MeilisearchSearch) doRebuild(
	listPages func() ([]string, error),
	getChunks func(string) ([]storage.Chunk, error),
) error {
	fmt.Printf("[meilisearch] 开始重建索引 %s ...\n", m.indexUID)

	// 清空旧索引（DELETE 是异步操作，需等待完成）
	if resp, err := m.doRequest("DELETE", "/indexes/"+m.indexUID, nil); err == nil {
		var task meiliTaskResponse
		json.NewDecoder(resp.Body).Decode(&task)
		resp.Body.Close()
		if task.TaskUID > 0 {
			if err := m.waitForTask(context.Background(), task.TaskUID); err != nil {
				fmt.Printf("[meilisearch] %v\n", err)
			}
		}
	}

	pages, err := listPages()
	if err != nil {
		return fmt.Errorf("列出页面失败: %w", err)
	}

	// 配置索引：只搜索 content 和 title，不搜索 source 路径
	m.configureIndex()

	// 批量添加文档
	var batch []meiliDocument
	batchSize := 500

	for _, page := range pages {
		chunks, err := getChunks(page)
		if err != nil {
			continue
		}
		for _, ch := range chunks {
			batch = append(batch, meiliDocument{
				ID:      docID(ch.Source, ch.Title),
				Source:  ch.Source,
				Title:   ch.Title,
				Content: ch.Content,
			})
			if len(batch) >= batchSize {
				m.sendDocumentsSync(batch)
				batch = nil
			}
		}
	}
	if len(batch) > 0 {
		m.sendDocumentsSync(batch)
	}

	m.built = true
	fmt.Printf("[meilisearch] 索引 %s 构建完成：%d 个页面 → %d 个文档\n",
		m.indexUID, len(pages), m.documentCount())
	return nil
}

// ---------- 内部 HTTP 方法 ----------

func (m *MeilisearchSearch) doRequest(method, path string, body []byte) (*http.Response, error) {
	url := m.baseURL + path
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.masterKey != "" {
		req.Header.Set("Authorization", "Bearer "+m.masterKey)
	}

	return m.client.Do(req)
}

// sendDocuments 异步添加文档（用于导入时增量更新）
func (m *MeilisearchSearch) sendDocuments(docs []meiliDocument) {
	if len(docs) == 0 {
		return
	}
	body, err := json.Marshal(docs)
	if err != nil {
		fmt.Printf("[meilisearch] marshal error: %v\n", err)
		return
	}
	resp, err := m.doRequest("POST", "/indexes/"+m.indexUID+"/documents", body)
	if err != nil {
		fmt.Printf("[meilisearch] index error: %v\n", err)
		return
	}
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("[meilisearch] index failed: status=%d, body=%s\n", resp.StatusCode, string(respBody))
	}
	resp.Body.Close()
}

// sendDocumentsSync 同步添加文档并等待任务完成（用于 Rebuild 全量构建）
func (m *MeilisearchSearch) sendDocumentsSync(docs []meiliDocument) {
	if len(docs) == 0 {
		return
	}
	body, err := json.Marshal(docs)
	if err != nil {
		fmt.Printf("[meilisearch] marshal error: %v\n", err)
		return
	}
	resp, err := m.doRequest("POST", "/indexes/"+m.indexUID+"/documents", body)
	if err != nil {
		fmt.Printf("[meilisearch] index error: %v\n", err)
		return
	}
	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		fmt.Printf("[meilisearch] index failed: status=%d, body=%s\n", resp.StatusCode, string(respBody))
		resp.Body.Close()
		return
	}

	var task meiliTaskResponse
	json.NewDecoder(resp.Body).Decode(&task)
	resp.Body.Close()

	if task.TaskUID > 0 {
		if err := m.waitForTask(context.Background(), task.TaskUID); err != nil {
			fmt.Printf("[meilisearch] %v\n", err)
		}
	}
}

// waitForTask 轮询等待 Meilisearch 异步任务完成（支持 context 超时/取消）
func (m *MeilisearchSearch) waitForTask(ctx context.Context, taskUID int) error {
	for range 120 { // 最多等 60 秒
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}

		resp, err := m.doRequest("GET", fmt.Sprintf("/tasks/%d", taskUID), nil)
		if err != nil {
			continue
		}

		var task meiliTaskResponse
		json.NewDecoder(resp.Body).Decode(&task)
		resp.Body.Close()

		switch task.Status {
		case "succeeded":
			return nil
		case "failed":
			if task.Error != nil {
				return fmt.Errorf("meilisearch task %d failed: %s", taskUID, task.Error.Message)
			}
			return fmt.Errorf("meilisearch task %d failed", taskUID)
		}
	}
	return fmt.Errorf("meilisearch task %d timed out after 60s", taskUID)
}

// documentCount 查询索引中文档数量
func (m *MeilisearchSearch) documentCount() int {
	resp, err := m.doRequest("GET", "/indexes/"+m.indexUID+"/stats", nil)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()

	var stats meiliStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		fmt.Printf("[meilisearch] stats decode error: %v\n", err)
		return 0
	}
	return stats.NumberOfDocuments
}

// configureIndex 配置索引的可搜索字段
func (m *MeilisearchSearch) configureIndex() {
	settings := `{"searchableAttributes": ["title", "content"]}`
	m.doRequest("PATCH", "/indexes/"+m.indexUID+"/settings",
		[]byte(settings))
}

// ---------- 工具函数 ----------

// docID 生成稳定的文档唯一标识（基于 source + title 的完整 MD5）
func docID(source, title string) string {
	h := md5.Sum([]byte(source + "\x00" + title))
	return fmt.Sprintf("%x", h)
}

// escapeFilterValue 转义 Meilisearch 过滤器中的特殊字符
func escapeFilterValue(s string) string {
	// 转义反斜杠和双引号
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	return s
}

// sanitizeIndexUID 将字符串转为合法的 Meilisearch 索引名
// 只保留字母、数字、连字符、下划线
func sanitizeIndexUID(name string) string {
	var buf strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '-' || r == '_' {
			buf.WriteRune(r)
		} else {
			buf.WriteRune('_')
		}
	}
	result := buf.String()
	if result == "" {
		return "wiki"
	}
	return result
}
