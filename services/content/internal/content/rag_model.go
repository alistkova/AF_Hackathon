package content

const (
	RAGEmbeddingDimensions = 384
	ragDefaultSearchTopK   = 10
	ragDefaultListTopK     = 100
	ragMaxTopK             = 200
)

type RAGChunk struct {
	Content     string   `json:"content"`
	Source      string   `json:"source"`
	Page        int      `json:"page"`
	Chunk       int      `json:"chunk"`
	CompanyID   string   `json:"company_id"`
	Sector      string   `json:"sector"`
	PublishedAt *string  `json:"published_at,omitempty"`
	Distance    *float64 `json:"distance,omitempty"`
}

type RAGChunkInput struct {
	Content     string    `json:"content"`
	Source      string    `json:"source"`
	Page        int       `json:"page"`
	Chunk       int       `json:"chunk"`
	CompanyID   string    `json:"company_id"`
	Sector      string    `json:"sector"`
	PublishedAt *string   `json:"published_at"`
	Embedding   []float32 `json:"embedding"`
}

type RAGFilter struct {
	CompanyID       string `json:"company_id"`
	Sector          string `json:"sector"`
	PublishedAfter  string `json:"published_after"`
	PublishedBefore string `json:"published_before"`
}

type RAGUpsertRequest struct {
	Chunks []RAGChunkInput `json:"chunks"`
}

type RAGSearchRequest struct {
	RAGFilter
	Embedding   []float32 `json:"embedding"`
	TopK        int       `json:"top_k"`
	MaxDistance *float64  `json:"max_distance"`
}

type RAGListRequest struct {
	RAGFilter
	TopK int `json:"top_k"`
}

type RAGSearchQuery struct {
	RAGFilter
	Embedding   []float32
	TopK        int
	MaxDistance *float64
}

type RAGListQuery struct {
	RAGFilter
	TopK int
}

type RAGChunksResponse struct {
	Chunks []RAGChunk `json:"chunks"`
}

type RAGUpsertResponse struct {
	Upserted int `json:"upserted"`
}

type RAGCountResponse struct {
	Count int64 `json:"count"`
}

type RAGDeleteResponse struct {
	Deleted int64 `json:"deleted"`
}
