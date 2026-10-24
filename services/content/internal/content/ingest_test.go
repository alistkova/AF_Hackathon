package content

import (
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestNormalizeURL(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"uppercase host", "HTTPS://WWW.RBC.RU/Finances/2026/", "https://rbc.ru/Finances/2026"},
		{"trailing slash", "https://rbc.ru/news/", "https://rbc.ru/news"},
		{"default port", "https://rbc.ru:443/news", "https://rbc.ru/news"},
		{"tracking params", "https://rbc.ru/news?a=1&utm_source=tg&fbclid=xyz", "https://rbc.ru/news?a=1"},
		{"sorted params", "https://rbc.ru/n?b=2&a=1", "https://rbc.ru/n?a=1&b=2"},
		{"fragment", "https://rbc.ru/news#top", "https://rbc.ru/news"},
		{"root", "https://rbc.ru/", "https://rbc.ru/"},
		{"empty", "  ", ""},
		{"not a url", "rbc.ru/news", ""},
		{"ftp", "ftp://rbc.ru/news", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeURL(tc.in); got != tc.want {
				t.Fatalf("NormalizeURL(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestNormalizeURLKeepsDistinctArticles(t *testing.T) {
	first := NormalizeURL("https://rbc.ru/news/1?utm_source=tg")
	second := NormalizeURL("https://www.rbc.ru/news/1/")
	if first != second {
		t.Fatalf("same article normalized differently: %q vs %q", first, second)
	}
	if third := NormalizeURL("https://rbc.ru/news/2"); third == first {
		t.Fatal("different articles normalized to the same url")
	}
}

func TestContentHashIgnoresFormatting(t *testing.T) {
	first := ContentHash("Банк России сохранил ставку", "Совет директоров  принял\n\n\nрешение")
	second := ContentHash("банк россии СОХРАНИЛ ставку", "Совет директоров принял решение")
	if first != second {
		t.Fatal("hash should ignore case and whitespace")
	}
	if first == ContentHash("Банк России сохранил ставку", "Другой текст") {
		t.Fatal("hash should change with the text")
	}
}

func TestChunkTextEmpty(t *testing.T) {
	if chunks := ChunkText("   \n\n  ", 100, 10); chunks != nil {
		t.Fatalf("chunks = %v, want nil", chunks)
	}
}

func TestChunkTextShort(t *testing.T) {
	chunks := ChunkText("Короткая новость про ставку.", 100, 10)
	if len(chunks) != 1 || chunks[0] != "Короткая новость про ставку." {
		t.Fatalf("chunks = %v", chunks)
	}
}

func TestChunkTextLongDocument(t *testing.T) {
	var builder strings.Builder
	for i := 0; i < 200; i++ {
		builder.WriteString("Банк России сохранил ключевую ставку на уровне 14% годовых. ")
	}
	text := builder.String()

	chunks := ChunkText(text, 300, 50)
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want multiple chunks", len(chunks))
	}
	for index, chunk := range chunks {
		if len([]rune(chunk)) > 300 {
			t.Fatalf("chunk %d has %d runes, want <= 300", index, len([]rune(chunk)))
		}
		if strings.TrimSpace(chunk) == "" {
			t.Fatalf("chunk %d is empty", index)
		}
	}

	joined := strings.Join(chunks, " ")
	if !strings.Contains(joined, "годовых") {
		t.Fatal("chunks lost content")
	}
	if !strings.HasSuffix(chunks[len(chunks)-1], "годовых.") {
		t.Fatalf("last chunk = %q, want it to end with a sentence", chunks[len(chunks)-1])
	}
}

func TestChunkTextPrefersParagraphBreaks(t *testing.T) {
	paragraph := strings.Repeat("а", 90)
	text := paragraph + "\n\n" + paragraph + "\n\n" + paragraph

	chunks := ChunkText(text, 120, 10)
	if len(chunks) < 2 {
		t.Fatalf("len(chunks) = %d, want multiple chunks", len(chunks))
	}
	for index, chunk := range chunks {
		if strings.HasPrefix(chunk, "\n") || strings.HasSuffix(chunk, "\n") {
			t.Fatalf("chunk %d has stray newlines: %q", index, chunk)
		}
	}
}

func TestChunkTextNormalizesChunkSize(t *testing.T) {
	text := strings.Repeat("слово ", 100)

	chunks := ChunkText(text, 0, 0)
	if len(chunks) == 0 {
		t.Fatal("no chunks for default size")
	}
	for _, chunk := range chunks {
		if len([]rune(chunk)) > DefaultChunkSize {
			t.Fatalf("chunk longer than default size: %d", len([]rune(chunk)))
		}
	}

	huge := ChunkText(text, MaxChunkSize*2, 0)
	for _, chunk := range huge {
		if len([]rune(chunk)) > MaxChunkSize {
			t.Fatalf("chunk longer than max size: %d", len([]rune(chunk)))
		}
	}
}

func TestIsUniqueViolation(t *testing.T) {
	if isUniqueViolation(nil) {
		t.Fatal("nil should not be a unique violation")
	}
	if isUniqueViolation(fmt.Errorf("boom")) {
		t.Fatal("generic error should not be a unique violation")
	}
	if !isUniqueViolation(&pgconn.PgError{Code: "23505"}) {
		t.Fatal("23505 should be a unique violation")
	}
	if isUniqueViolation(&pgconn.PgError{Code: "23503"}) {
		t.Fatal("23503 should not be a unique violation")
	}
}
