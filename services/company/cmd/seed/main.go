package main

import (
	"context"
	"flag"
	"io"
	"log/slog"
	"os"

	"github.com/alfa-hack/alfa-hack/internal/platform/config"
	"github.com/alfa-hack/alfa-hack/internal/platform/db"
	"github.com/alfa-hack/alfa-hack/services/company/internal/company"
	companymigrations "github.com/alfa-hack/alfa-hack/services/company/migrations"
)

func main() {
	file := flag.String("file", "seed/companies.csv", "path to companies csv")
	peopleFile := flag.String("people", "", "optional path to people/links csv")
	paymentsFile := flag.String("payments", "", "optional path to payments csv")
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	ctx := context.Background()

	items, err := parseFile(*file, company.ParseCSV)
	if err != nil {
		logger.Error("read companies", "error", err, "file", *file)
		os.Exit(1)
	}

	var links []company.PersonLink
	if *peopleFile != "" {
		links, err = parseFile(*peopleFile, company.ParsePeopleCSV)
		if err != nil {
			logger.Error("read people", "error", err, "file", *peopleFile)
			os.Exit(1)
		}
	}

	var payments []company.Payment
	if *paymentsFile != "" {
		payments, err = parseFile(*paymentsFile, company.ParsePaymentsCSV)
		if err != nil {
			logger.Error("read payments", "error", err, "file", *paymentsFile)
			os.Exit(1)
		}
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

	repo := company.NewRepository(pool)
	if err := repo.Upsert(ctx, items); err != nil {
		logger.Error("upsert companies", "error", err)
		os.Exit(1)
	}
	if len(links) > 0 {
		if err := repo.UpsertPeople(ctx, links); err != nil {
			logger.Error("upsert people", "error", err)
			os.Exit(1)
		}
	}
	if len(payments) > 0 {
		if err := repo.UpsertPayments(ctx, payments); err != nil {
			logger.Error("upsert payments", "error", err)
			os.Exit(1)
		}
	}

	logger.Info("company data imported", "companies", len(items), "people_links", len(links), "payments", len(payments))
}

func parseFile[T any](path string, parse func(reader io.Reader) (T, error)) (T, error) {
	var zero T
	f, err := os.Open(path)
	if err != nil {
		return zero, err
	}
	defer func() { _ = f.Close() }()
	return parse(f)
}
