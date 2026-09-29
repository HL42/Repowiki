package wiki

import "time"

type PageType string

const (
	PageTypeConcept       PageType = "concept"
	PageTypeEntity        PageType = "entity"
	PageTypeSource        PageType = "source"
	PageTypeSynthesis     PageType = "synthesis"
	PageTypeContradiction PageType = "contradiction"
)

type Frontmatter struct {
	Title      string    `json:"title"`
	Type       PageType  `json:"type"`
	Status     string    `json:"status"`
	Created    time.Time `json:"created"`
	Updated    time.Time `json:"updated"`
	Sources    []string  `json:"sources"`
	Confidence string    `json:"confidence"`
	Tags       []string  `json:"tags"`
}

type WikiPage struct {
	Frontmatter Frontmatter
	Body        string
	FilePath    string
}

// Req/Resp

type IngestRequest struct {
	RawPath    string `json:"raw_path"`
	Project    string `json:"project,omitempty"`
	SubProject string `json:"sub_project,omitempty"`
}

type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type QueryRequest struct {
	Question   string        `json:"question"`
	History    []ChatMessage `json:"history,omitempty"`
	Project    string        `json:"project,omitempty"`
	SubProject string        `json:"sub_project,omitempty"`
}

type QueryResponse struct {
	Answer         string           `json:"answer"`
	SourcePages    []string         `json:"source_pages"`
	Citations      []SourceCitation `json:"citations,omitempty"`
	Confidence     string           `json:"confidence"`
	MatchedChunks  int              `json:"matched_chunks"`
	SearchScope    string           `json:"search_scope,omitempty"`
	RefinedQueries []string         `json:"refined_queries,omitempty"`
}

type SourceCitation struct {
	Page    string `json:"page"`
	Section string `json:"section"`
	Score   int    `json:"score"`
	Excerpt string `json:"excerpt"`
}

type SynthesizeRequest struct {
	Topic      string `json:"topic"`
	Project    string `json:"project,omitempty"`
	SubProject string `json:"sub_project,omitempty"`
}

type SynthesizeResponse struct {
	PagePath string `json:"page_path"`
	Result   string `json:"result"`
}

type StatsResponse struct {
	TotalPages     int    `json:"total_pages"`
	SourcePages    int    `json:"source_pages"`
	EntityPages    int    `json:"entity_pages"`
	ConceptPages   int    `json:"concept_pages"`
	SynthesisPages int    `json:"synthesis_pages"`
	LintErrors     int    `json:"lint_errors"`
	LintWarnings   int    `json:"lint_warnings"`
	LintInfo       int    `json:"lint_info"`
	LintTime       string `json:"lint_time"`
	StaleDocs      int    `json:"stale_docs"`
}

// ---------- 项目相关 ----------

type SubProjectInfo struct {
	Name           string `json:"name"`
	SourcePages    int    `json:"source_pages"`
	EntityPages    int    `json:"entity_pages"`
	ConceptPages   int    `json:"concept_pages"`
	SynthesisPages int    `json:"synthesis_pages"`
}

type ProjectInfo struct {
	Name           string           `json:"name"`
	TotalPages     int              `json:"total_pages"`
	SourcePages    int              `json:"source_pages"`
	EntityPages    int              `json:"entity_pages"`
	ConceptPages   int              `json:"concept_pages"`
	SynthesisPages int              `json:"synthesis_pages"`
	Description    string           `json:"description"`
	SubProjects    string           `json:"sub_projects"`
	SubProjectList []SubProjectInfo `json:"sub_project_list,omitempty"`
}

type ProjectCreateRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	SubProjects string `json:"sub_projects"`
}
