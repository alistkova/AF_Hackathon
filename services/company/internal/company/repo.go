package company

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) List(ctx context.Context) ([]Company, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, inn, industry, region, description, created_at
		FROM companies
		ORDER BY name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]Company, 0)
	for rows.Next() {
		var item Company
		if err := rows.Scan(&item.ID, &item.Name, &item.INN, &item.Industry, &item.Region, &item.Description, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) Get(ctx context.Context, id string) (Company, error) {
	var item Company
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, inn, industry, region, description, created_at
		FROM companies
		WHERE id = $1
	`, id).Scan(&item.ID, &item.Name, &item.INN, &item.Industry, &item.Region, &item.Description, &item.CreatedAt)
	if err != nil {
		return Company{}, err
	}
	return item, nil
}

func (r *Repository) Upsert(ctx context.Context, items []Company) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, item := range items {
		if _, err := tx.Exec(ctx, `
			INSERT INTO companies (id, name, inn, industry, region, description)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (id) DO UPDATE SET
				name = EXCLUDED.name,
				inn = EXCLUDED.inn,
				industry = EXCLUDED.industry,
				region = EXCLUDED.region,
				description = EXCLUDED.description
		`, item.ID, item.Name, item.INN, item.Industry, item.Region, item.Description); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}

func IsNotFound(err error) bool {
	return err == pgx.ErrNoRows
}
