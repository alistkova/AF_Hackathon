package content

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

func ragFilterClauses(filter RAGFilter, start int) ([]string, []any) {
	clauses := make([]string, 0, 4)
	args := make([]any, 0, 4)
	next := start

	if filter.CompanyID != "" {
		clauses = append(clauses, fmt.Sprintf("company_id = $%d", next))
		args = append(args, filter.CompanyID)
		next++
	}
	if filter.Sector != "" {
		clauses = append(clauses, fmt.Sprintf("sector = $%d", next))
		args = append(args, filter.Sector)
		next++
	}
	if filter.PublishedAfter != "" {
		clauses = append(clauses, fmt.Sprintf("published_at >= $%d::text::date", next))
		args = append(args, filter.PublishedAfter)
		next++
	}
	if filter.PublishedBefore != "" {
		clauses = append(clauses, fmt.Sprintf("published_at <= $%d::text::date", next))
		args = append(args, filter.PublishedBefore)
	}
	return clauses, args
}

func ragWhere(clauses []string) string {
	if len(clauses) == 0 {
		return ""
	}
	return " WHERE " + strings.Join(clauses, " AND ")
}

func formatVector(values []float32) string {
	var builder strings.Builder
	builder.WriteByte('[')
	for i, value := range values {
		if i > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.FormatFloat(float64(value), 'f', -1, 32))
	}
	builder.WriteByte(']')
	return builder.String()
}

func (r *Repository) UpsertRAGChunks(ctx context.Context, chunks []RAGChunkInput) (int, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, chunk := range chunks {
		publishedAt := ""
		if chunk.PublishedAt != nil {
			publishedAt = *chunk.PublishedAt
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO rag_chunks (content, source, page, chunk, company_id, sector, published_at, embedding)
			VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7::text, '')::date, $8::text::vector)
			ON CONFLICT (source, page, chunk) DO UPDATE SET
				content = EXCLUDED.content,
				company_id = EXCLUDED.company_id,
				sector = EXCLUDED.sector,
				published_at = EXCLUDED.published_at,
				embedding = EXCLUDED.embedding
		`, chunk.Content, chunk.Source, chunk.Page, chunk.Chunk, chunk.CompanyID, chunk.Sector, publishedAt, formatVector(chunk.Embedding)); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(chunks), nil
}

func (r *Repository) SearchRAGChunks(ctx context.Context, query RAGSearchQuery) ([]RAGChunk, error) {
	topK := normalizeTopK(query.TopK, ragDefaultSearchTopK)
	params := []any{formatVector(query.Embedding)}
	clauses, filterArgs := ragFilterClauses(query.RAGFilter, 2)
	params = append(params, filterArgs...)
	if query.MaxDistance != nil {
		params = append(params, *query.MaxDistance)
		clauses = append(clauses, fmt.Sprintf("(embedding <=> $1::text::vector) <= $%d", len(params)))
	}

	sql := `
		SELECT content, source, page, chunk, company_id, sector,
			COALESCE(to_char(published_at, 'YYYY-MM-DD'), ''),
			embedding <=> $1::text::vector AS distance
		FROM rag_chunks` + ragWhere(clauses)
	params = append(params, topK)
	sql += fmt.Sprintf(" ORDER BY embedding <=> $1::text::vector LIMIT $%d", len(params))

	rows, err := r.pool.Query(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]RAGChunk, 0)
	for rows.Next() {
		var chunk RAGChunk
		var publishedAt string
		var distance float64
		if err := rows.Scan(
			&chunk.Content,
			&chunk.Source,
			&chunk.Page,
			&chunk.Chunk,
			&chunk.CompanyID,
			&chunk.Sector,
			&publishedAt,
			&distance,
		); err != nil {
			return nil, err
		}
		if publishedAt != "" {
			chunk.PublishedAt = &publishedAt
		}
		chunk.Distance = &distance
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (r *Repository) ListRAGChunks(ctx context.Context, query RAGListQuery) ([]RAGChunk, error) {
	topK := normalizeTopK(query.TopK, ragDefaultListTopK)
	clauses, params := ragFilterClauses(query.RAGFilter, 1)
	sql := `
		SELECT content, source, page, chunk, company_id, sector,
			COALESCE(to_char(published_at, 'YYYY-MM-DD'), '')
		FROM rag_chunks` + ragWhere(clauses)
	params = append(params, topK)
	sql += fmt.Sprintf(" ORDER BY published_at DESC NULLS LAST, id DESC LIMIT $%d", len(params))

	rows, err := r.pool.Query(ctx, sql, params...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]RAGChunk, 0)
	for rows.Next() {
		var chunk RAGChunk
		var publishedAt string
		if err := rows.Scan(
			&chunk.Content,
			&chunk.Source,
			&chunk.Page,
			&chunk.Chunk,
			&chunk.CompanyID,
			&chunk.Sector,
			&publishedAt,
		); err != nil {
			return nil, err
		}
		if publishedAt != "" {
			chunk.PublishedAt = &publishedAt
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (r *Repository) CountRAGChunks(ctx context.Context) (int64, error) {
	var count int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM rag_chunks`).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

func (r *Repository) DeleteRAGChunks(ctx context.Context) (int64, error) {
	tag, err := r.pool.Exec(ctx, `DELETE FROM rag_chunks`)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func normalizeTopK(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value > ragMaxTopK {
		return ragMaxTopK
	}
	return value
}
