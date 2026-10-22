package main

import (
	"context"
	"flag"
	"log/slog"
	"os"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
	"github.com/alfa-hack/alfa-hack/internal/platform/db"
	"github.com/alfa-hack/alfa-hack/services/company/internal/company"
	companymigrations "github.com/alfa-hack/alfa-hack/services/company/migrations"
)

func main() {
	file := flag.String("file", "seed/companies.csv", "path to companies csv")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	f, err := os.Open(*file)
	if err != nil {
		logger.Error("open companies", "error", err, "file", *file)
		os.Exit(1)
	}
	defer func() { _ = f.Close() }()

	items, err := company.ParseCSV(f)
	if err != nil {
		logger.Error("read companies", "error", err, "file", *file)
		os.Exit(1)
	}

	pool, err := db.Connect(ctx, config.String("COMPANY_DATABASE_URL", "postgres://company_svc:company@localhost:5432/company_db?sslmode=disable"))
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, companymigrations.FS, "."); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}

	if err := company.NewRepository(pool).Upsert(ctx, items); err != nil {
		logger.Error("upsert companies", "error", err)
		os.Exit(1)
	}

	logger.Info("companies seeded", "count", len(items))
}
