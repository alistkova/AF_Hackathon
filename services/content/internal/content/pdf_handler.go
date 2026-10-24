package content

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/alfa-hack/alfa-hack/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

const (
	maxPDFBytes   = 25 << 20
	maxTitleRunes = 200
)

func (h *Handler) UploadPDF(c *gin.Context) {
	if h.files == nil {
		httpx.Error(c, http.StatusServiceUnavailable, "storage_unavailable", "Хранилище документов не настроено")
		return
	}

	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxPDFBytes)

	header, err := c.FormFile("file")
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Передайте PDF в поле file")
		return
	}
	if header.Size > maxPDFBytes {
		httpx.Error(c, http.StatusRequestEntityTooLarge, "file_too_large", "Файл больше 25 МБ")
		return
	}

	file, err := header.Open()
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Не удалось прочитать файл")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_request", "Не удалось прочитать файл")
		return
	}
	if !bytes.HasPrefix(data, []byte("%PDF-")) {
		httpx.Error(c, http.StatusUnsupportedMediaType, "invalid_file", "Файл не является PDF")
		return
	}

	text, err := ExtractPDFText(c.Request.Context(), data)
	if errors.Is(err, ErrPDFUnavailable) {
		httpx.Error(c, http.StatusUnprocessableEntity, "pdf_unavailable", "Извлечение текста из PDF не настроено на этом хосте")
		return
	}
	if err != nil {
		h.logger.Error("extract pdf text", "error", err, "file", header.Filename)
		httpx.Error(c, http.StatusUnprocessableEntity, "pdf_unreadable", "Не удалось извлечь текст из PDF")
		return
	}
	if strings.TrimSpace(text) == "" {
		httpx.Error(c, http.StatusUnprocessableEntity, "pdf_empty", "В PDF не найден текстовый слой")
		return
	}

	sum := sha256.Sum256(data)
	digest := hex.EncodeToString(sum[:])
	key := path.Join("pdf", digest[:2], digest+".pdf")

	if err := h.files.Put(c.Request.Context(), key, data); err != nil {
		h.logger.Error("store pdf", "error", err, "key", key)
		httpx.Error(c, http.StatusInternalServerError, "storage_unavailable", "Не удалось сохранить файл")
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		title = deriveTitle(text, header.Filename)
	}
	sourceName := c.PostForm("source_name")
	sourceURL := c.PostForm("source_url")

	if !utf8.ValidString(title) || !utf8.ValidString(sourceName) || !utf8.ValidString(sourceURL) {
		httpx.Error(c, http.StatusBadRequest, "invalid_encoding", "Текстовые поля должны быть в кодировке UTF-8")
		return
	}

	result, err := h.repo.IngestDocument(c.Request.Context(), IngestInput{
		DocumentID:    "pdf_" + digest[:16],
		Kind:          "pdf",
		Title:         title,
		PublishedAt:   parseFormTime(c.PostForm("published_at")),
		SourceName:    sourceName,
		SourceURL:     sourceURL,
		FullText:      text,
		Tags:          splitList(c.PostForm("tags")),
		TargetSectors: splitList(c.PostForm("sectors")),
		ObjectKey:     key,
		MIMEType:      "application/pdf",
		ByteSize:      int64(len(data)),
	})
	if err != nil {
		h.logger.Error("ingest pdf", "error", err, "key", key)
		httpx.Error(c, http.StatusInternalServerError, "content_unavailable", "Не удалось сохранить документ")
		return
	}

	c.JSON(http.StatusOK, result)
}

func deriveTitle(text, filename string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > maxTitleRunes {
			return string(runes[:maxTitleRunes])
		}
		return line
	}

	name := strings.TrimSpace(filename)
	name = strings.TrimSuffix(name, ".pdf")
	name = strings.TrimSuffix(name, ".PDF")
	if name == "" {
		return "PDF документ"
	}
	return name
}

func splitList(value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			items = append(items, trimmed)
		}
	}
	return items
}

func parseFormTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
