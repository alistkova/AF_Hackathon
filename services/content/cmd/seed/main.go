package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"

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

	if err := content.NewRepository(pool).UpsertNews(ctx, items); err != nil {
		logger.Error("upsert news", "error", err)
		os.Exit(1)
	}

	logger.Info("news seeded", "count", len(items))
}

func readNews(path string) ([]content.NewsItem, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var items []content.NewsItem
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}
	return items, nil
}
