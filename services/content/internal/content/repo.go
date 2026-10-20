package content

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) ListDocuments(ctx context.Context, limit int) ([]NewsItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.pool.Query(ctx, `
		SELECT news_id, title, COALESCE(published_at, created_at), source_name, source_url, full_text, tags, target_sectors
		FROM documents
		WHERE news_id IS NOT NULL
		ORDER BY COALESCE(published_at, created_at) DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]NewsItem, 0)
	for rows.Next() {
		var item NewsItem
		if err := rows.Scan(
			&item.NewsID,
			&item.Title,
			&item.PublishedAt,
			&item.SourceName,
			&item.SourceURL,
			&item.FullText,
			&item.Tags,
			&item.TargetSectors,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *Repository) UpsertNews(ctx context.Context, items []NewsItem) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, item := range items {
		tags := item.Tags
		if tags == nil {
			tags = []string{}
		}
		sectors := item.TargetSectors
		if sectors == nil {
			sectors = []string{}
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO documents (id, news_id, title, published_at, source_name, source_url, full_text, tags, target_sectors)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (id) DO UPDATE SET
				news_id = EXCLUDED.news_id,
				title = EXCLUDED.title,
				published_at = EXCLUDED.published_at,
				source_name = EXCLUDED.source_name,
				source_url = EXCLUDED.source_url,
				full_text = EXCLUDED.full_text,
				tags = EXCLUDED.tags,
				target_sectors = EXCLUDED.target_sectors
		`, item.NewsID, item.NewsID, item.Title, item.PublishedAt, item.SourceName, item.SourceURL, item.FullText, tags, sectors); err != nil {
			return err
		}
	}

	return tx.Commit(ctx)
}
