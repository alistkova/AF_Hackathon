package content

import (
	"strings"
	"time"
)

type NewsItem struct {
	DocumentID    string    `json:"document_id"`
	NewsID        string    `json:"news_id"`
	Title         string    `json:"title"`
	PublishedAt   time.Time `json:"published_at"`
	SourceName    string    `json:"source_name"`
	SourceURL     string    `json:"source_url"`
	FullText      string    `json:"full_text"`
	Tags          []string  `json:"tags"`
	TargetSectors []string  `json:"target_sectors"`
}

type SearchHit struct {
	NewsItem
	Rank float64 `json:"rank"`
}

type DocumentChunk struct {
	Ordinal int    `json:"ordinal"`
	Content string `json:"content"`
}

type DocumentFilter struct {
	Kind   string
	Sector string
	Tag    string
	Source string
	From   *time.Time
	To     *time.Time
	Limit  int
	Offset int
}

const (
	IngestCreated   = "created"
	IngestUpdated   = "updated"
	IngestDuplicate = "duplicate"
)

type IngestInput struct {
	DocumentID    string
	NewsID        string
	Kind          string
	Title         string
	PublishedAt   time.Time
	SourceName    string
	SourceURL     string
	FullText      string
	Tags          []string
	TargetSectors []string
	ObjectKey     string
	MIMEType      string
	ByteSize      int64
	ChunkSize     int
	ChunkOverlap  int
}

type IngestResult struct {
	Status     string `json:"status"`
	DocumentID string `json:"document_id"`
	NewsID     string `json:"news_id"`
	Chunks     int    `json:"chunks"`
}

func (input *IngestInput) Normalize() {
	input.Kind = strings.ToLower(strings.TrimSpace(input.Kind))
	if input.Kind == "" {
		input.Kind = "news"
	}
	input.Title = strings.TrimSpace(input.Title)
	input.SourceName = strings.TrimSpace(input.SourceName)
	input.SourceURL = strings.TrimSpace(input.SourceURL)
	input.NewsID = strings.TrimSpace(input.NewsID)
	input.DocumentID = strings.TrimSpace(input.DocumentID)
	if input.PublishedAt.IsZero() {
		input.PublishedAt = time.Now().UTC()
	}
}
