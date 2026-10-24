package content

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("document not found")

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const newsColumns = `id, news_id, title, COALESCE(published_at, created_at), source_name, source_url, full_text, tags, target_sectors`

func (r *Repository) ListDocuments(ctx context.Context, filter DocumentFilter) ([]NewsItem, int, error) {
	where, args, next := documentWhere(filter, 1)

	total, err := r.countDocuments(ctx, where, args)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT ` + newsColumns + `
		FROM documents` + where + `
		ORDER BY COALESCE(published_at, created_at) DESC, news_id
		LIMIT $` + fmt.Sprint(next) + ` OFFSET $` + fmt.Sprint(next+1)

	rows, err := r.pool.Query(ctx, query, append(args, filter.normalizedLimit(), filter.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]NewsItem, 0)
	for rows.Next() {
		item, err := scanNews(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *Repository) GetDocument(ctx context.Context, id string) (NewsItem, error) {
	item, err := scanNews(r.pool.QueryRow(ctx, `
		SELECT `+newsColumns+`
		FROM documents
		WHERE id = $1 OR news_id = $1
	`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return NewsItem{}, ErrNotFound
	}
	if err != nil {
		return NewsItem{}, err
	}
	return item, nil
}

func (r *Repository) SearchDocuments(ctx context.Context, query string, filter DocumentFilter) ([]SearchHit, int, error) {
	where, args, next := documentWhere(filter, 2)
	match := `search_vector @@ websearch_to_tsquery('russian', $1)`
	if where == "" {
		where = " WHERE " + match
	} else {
		where += " AND " + match
	}

	total, err := r.countDocuments(ctx, where, append([]any{query}, args...))
	if err != nil {
		return nil, 0, err
	}

	sql := `
		SELECT ` + newsColumns + `,
			ts_rank_cd(search_vector, websearch_to_tsquery('russian', $1)) AS rank
		FROM documents` + where + `
		ORDER BY rank DESC, COALESCE(published_at, created_at) DESC, news_id
		LIMIT $` + fmt.Sprint(next) + ` OFFSET $` + fmt.Sprint(next+1)

	rows, err := r.pool.Query(ctx, sql, append(append([]any{query}, args...), filter.normalizedLimit(), filter.Offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	hits := make([]SearchHit, 0)
	for rows.Next() {
		var hit SearchHit
		if err := rows.Scan(append(newsScanTargets(&hit.NewsItem), &hit.Rank)...); err != nil {
			return nil, 0, err
		}
		hits = append(hits, hit)
	}
	return hits, total, rows.Err()
}

func (r *Repository) IngestDocument(ctx context.Context, input IngestInput) (IngestResult, error) {
	input.Normalize()

	chunks := ChunkText(input.FullText, input.ChunkSize, input.ChunkOverlap)
	hash := ContentHash(input.Title, input.FullText)
	canonical := NormalizeURL(input.SourceURL)
	if input.DocumentID == "" {
		input.DocumentID = "doc_" + hash[:16]
	}
	if input.NewsID == "" {
		input.NewsID = input.DocumentID
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return IngestResult{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var existingID, existingNewsID string
	err = tx.QueryRow(ctx, `
		SELECT id, news_id FROM documents
		WHERE id = $1 OR news_id = $2
		   OR ($3 <> '' AND content_hash = $3)
		   OR ($4 <> '' AND canonical_url = $4)
		ORDER BY (id = $1) DESC, (news_id = $2) DESC
		LIMIT 1
	`, input.DocumentID, input.NewsID, hash, canonical).Scan(&existingID, &existingNewsID)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		existingID = ""
	case err != nil:
		return IngestResult{}, err
	}

	if existingID != "" && existingID != input.DocumentID {
		if _, err := tx.Exec(ctx, `
			UPDATE documents SET seen_count = seen_count + 1, last_seen_at = now()
			WHERE id = $1
		`, existingID); err != nil {
			return IngestResult{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return IngestResult{}, err
		}
		return IngestResult{
			Status:     IngestDuplicate,
			DocumentID: existingID,
			NewsID:     existingNewsID,
		}, nil
	}

	status := IngestCreated
	if existingID == input.DocumentID {
		status = IngestUpdated
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO documents (
			id, news_id, kind, title, published_at, source_name, source_url, full_text,
			tags, target_sectors, content_hash, canonical_url, object_key, mime_type,
			byte_size, chunk_count, seen_count, last_seen_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, 1, now())
		ON CONFLICT (id) DO UPDATE SET
			news_id = EXCLUDED.news_id,
			kind = EXCLUDED.kind,
			title = EXCLUDED.title,
			published_at = EXCLUDED.published_at,
			source_name = EXCLUDED.source_name,
			source_url = EXCLUDED.source_url,
			full_text = EXCLUDED.full_text,
			tags = EXCLUDED.tags,
			target_sectors = EXCLUDED.target_sectors,
			content_hash = EXCLUDED.content_hash,
			canonical_url = EXCLUDED.canonical_url,
			object_key = EXCLUDED.object_key,
			mime_type = EXCLUDED.mime_type,
			byte_size = EXCLUDED.byte_size,
			chunk_count = EXCLUDED.chunk_count,
			seen_count = documents.seen_count + 1,
			last_seen_at = now()
	`, input.DocumentID, input.NewsID, input.Kind, input.Title, input.PublishedAt, input.SourceName,
		input.SourceURL, input.FullText, emptyIfNil(input.Tags), emptyIfNil(input.TargetSectors),
		hash, canonical, input.ObjectKey, input.MIMEType, input.ByteSize, len(chunks)); err != nil {
		return IngestResult{}, err
	}

	if _, err := tx.Exec(ctx, `DELETE FROM chunks WHERE document_id = $1`, input.DocumentID); err != nil {
		return IngestResult{}, err
	}
	if len(chunks) > 0 {
		ordinals := make([]int32, len(chunks))
		contents := make([]string, len(chunks))
		for index, chunk := range chunks {
			ordinals[index] = int32(index)
			contents[index] = chunk
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO chunks (document_id, ordinal, content, char_count)
			SELECT $1, item.ordinal, item.content, char_length(item.content)
			FROM unnest($2::int[], $3::text[]) AS item(ordinal, content)
		`, input.DocumentID, ordinals, contents); err != nil {
			return IngestResult{}, err
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return IngestResult{}, err
	}

	return IngestResult{
		Status:     status,
		DocumentID: input.DocumentID,
		NewsID:     input.NewsID,
		Chunks:     len(chunks),
	}, nil
}

func (r *Repository) ListDocumentChunks(ctx context.Context, id string) ([]DocumentChunk, error) {
	var documentID string
	if err := r.pool.QueryRow(ctx, `
		SELECT id FROM documents WHERE id = $1 OR news_id = $1
	`, id).Scan(&documentID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT ordinal, content FROM chunks
		WHERE document_id = $1
		ORDER BY ordinal
	`, documentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	chunks := make([]DocumentChunk, 0)
	for rows.Next() {
		var chunk DocumentChunk
		if err := rows.Scan(&chunk.Ordinal, &chunk.Content); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

func (r *Repository) countDocuments(ctx context.Context, where string, args []any) (int, error) {
	var total int
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM documents`+where, args...).Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func documentWhere(filter DocumentFilter, start int) (string, []any, int) {
	clauses := make([]string, 0, 6)
	args := make([]any, 0, 6)
	next := start

	add := func(clause string, value any) {
		clauses = append(clauses, fmt.Sprintf(clause, next))
		args = append(args, value)
		next++
	}

	if filter.Kind != "" {
		add("kind = $%d", filter.Kind)
	}
	if filter.Sector != "" {
		add("target_sectors @> ARRAY[$%d]::text[]", filter.Sector)
	}
	if filter.Tag != "" {
		add("tags @> ARRAY[$%d]::text[]", filter.Tag)
	}
	if filter.Source != "" {
		add("source_name = $%d", filter.Source)
	}
	if filter.From != nil {
		add("COALESCE(published_at, created_at) >= $%d", *filter.From)
	}
	if filter.To != nil {
		add("COALESCE(published_at, created_at) <= $%d", *filter.To)
	}

	if len(clauses) == 0 {
		return "", args, next
	}
	return " WHERE " + strings.Join(clauses, " AND "), args, next
}

func (filter DocumentFilter) normalizedLimit() int {
	if filter.Limit <= 0 {
		return 50
	}
	if filter.Limit > 200 {
		return 200
	}
	return filter.Limit
}

func scanNews(row interface{ Scan(dest ...any) error }) (NewsItem, error) {
	var item NewsItem
	err := row.Scan(newsScanTargets(&item)...)
	return item, err
}

func newsScanTargets(item *NewsItem) []any {
	return []any{
		&item.DocumentID,
		&item.NewsID,
		&item.Title,
		&item.PublishedAt,
		&item.SourceName,
		&item.SourceURL,
		&item.FullText,
		&item.Tags,
		&item.TargetSectors,
	}
}

func emptyIfNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
