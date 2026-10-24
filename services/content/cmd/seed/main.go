package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"time"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
	"github.com/alfa-hack/alfa-hack/internal/platform/db"
	"github.com/alfa-hack/alfa-hack/services/content/internal/content"
	contentmigrations "github.com/alfa-hack/alfa-hack/services/content/migrations"
)

func main() {
	file := flag.String("file", "seed/news/sample.json", "path to news json")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	items, err := readNews(*file)
	if err != nil {
		logger.Error("read news", "error", err, "file", *file)
		os.Exit(1)
	}

	pool, err := db.Connect(ctx, config.String("CONTENT_DATABASE_URL", "postgres://content_svc:content@localhost:5432/content_db?sslmode=disable"))
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, contentmigrations.FS, "."); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}

	repo := content.NewRepository(pool)
	created, updated, duplicated, chunks := 0, 0, 0, 0
	for _, item := range items {
		result, err := repo.IngestDocument(ctx, item)
		if err != nil {
			logger.Error("ingest news", "error", err, "news_id", item.NewsID)
			os.Exit(1)
		}
		chunks += result.Chunks
		switch result.Status {
		case content.IngestCreated:
			created++
		case content.IngestUpdated:
			updated++
		case content.IngestDuplicate:
			duplicated++
		}
	}

	logger.Info("news seeded",
		"created", created,
		"updated", updated,
		"duplicates", duplicated,
		"chunks", chunks,
	)
}

func readNews(path string) ([]content.IngestInput, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var payload []struct {
		NewsID        string   `json:"news_id"`
		Title         string   `json:"title"`
		PublishedAt   string   `json:"published_at"`
		SourceName    string   `json:"source_name"`
		SourceURL     string   `json:"source_url"`
		FullText      string   `json:"full_text"`
		Tags          []string `json:"tags"`
		TargetSectors []string `json:"target_sectors"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, err
	}

	items := make([]content.IngestInput, 0, len(payload))
	for _, entry := range payload {
		items = append(items, content.IngestInput{
			NewsID:        entry.NewsID,
			Kind:          "news",
			Title:         entry.Title,
			SourceName:    entry.SourceName,
			SourceURL:     entry.SourceURL,
			FullText:      entry.FullText,
			Tags:          entry.Tags,
			TargetSectors: entry.TargetSectors,
			PublishedAt:   parseTime(entry.PublishedAt),
		})
	}
	return items, nil
}

func parseTime(value string) time.Time {
	if value == "" {
		return time.Time{}
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return parsed
}
