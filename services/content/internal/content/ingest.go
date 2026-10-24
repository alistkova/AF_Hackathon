package content

import (
	"crypto/sha256"
	"encoding/hex"
	"net/url"
	"sort"
	"strings"
	"unicode"
)

const (
	DefaultChunkSize    = 1200
	DefaultChunkOverlap = 200
	MaxChunkSize        = 4000
)

var trackingParams = map[string]struct{}{
	"fbclid":    {},
	"gclid":     {},
	"yclid":     {},
	"igshid":    {},
	"mc_cid":    {},
	"mc_eid":    {},
	"ref_src":   {},
	"_openstat": {},
}

func NormalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return ""
	}

	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return ""
	}

	host := strings.ToLower(parsed.Host)
	host = strings.TrimPrefix(host, "www.")
	host = strings.TrimSuffix(host, ":80")
	host = strings.TrimSuffix(host, ":443")

	path := parsed.Path
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	if path == "" {
		path = "/"
	}

	query := parsed.Query()
	keys := make([]string, 0, len(query))
	for key := range query {
		lower := strings.ToLower(key)
		if strings.HasPrefix(lower, "utm_") {
			continue
		}
		if _, tracked := trackingParams[lower]; tracked {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	values := url.Values{}
	for _, key := range keys {
		for _, value := range query[key] {
			values.Add(key, value)
		}
	}

	normalized := scheme + "://" + host + path
	if encoded := values.Encode(); encoded != "" {
		normalized += "?" + encoded
	}
	return normalized
}

func ContentHash(title, text string) string {
	canonical := collapseSpaces(CleanText(title)) + "\n" + collapseSpaces(CleanText(text))
	sum := sha256.Sum256([]byte(strings.ToLower(canonical)))
	return hex.EncodeToString(sum[:])
}

func collapseSpaces(text string) string {
	return strings.Join(strings.Fields(text), " ")
}

func CleanText(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	lines := strings.Split(text, "\n")
	cleaned := make([]string, 0, len(lines))
	blank := 0
	for _, line := range lines {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		cleaned = append(cleaned, line)
	}
	return strings.TrimSpace(strings.Join(cleaned, "\n"))
}

func ChunkText(text string, size, overlap int) []string {
	text = CleanText(text)
	if text == "" {
		return nil
	}

	size = normalizeChunkSize(size)
	if overlap < 0 || overlap >= size {
		overlap = DefaultChunkOverlap
		if overlap >= size {
			overlap = size / 5
		}
	}

	runes := []rune(text)
	if len(runes) <= size {
		return []string{text}
	}

	chunks := make([]string, 0, len(runes)/size+1)
	start := 0
	for start < len(runes) {
		end := start + size
		if end >= len(runes) {
			end = len(runes)
		} else {
			end = chunkBreak(runes, start, end)
		}

		if chunk := strings.TrimSpace(string(runes[start:end])); chunk != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(runes) {
			break
		}

		next := end - overlap
		if next <= start {
			next = end
		}
		start = next
	}
	return chunks
}

func normalizeChunkSize(size int) int {
	if size <= 0 {
		return DefaultChunkSize
	}
	if size > MaxChunkSize {
		return MaxChunkSize
	}
	return size
}

func chunkBreak(runes []rune, start, end int) int {
	minimum := start + (end-start)/2

	for index := end; index > minimum; index-- {
		if index >= 2 && runes[index-1] == '\n' && runes[index-2] == '\n' {
			return index
		}
	}
	for index := end; index > minimum; index-- {
		if runes[index-1] == '\n' {
			return index
		}
	}
	for index := end; index > minimum; index-- {
		if strings.ContainsRune(".!?", runes[index-1]) {
			if index >= len(runes) || unicode.IsSpace(runes[index]) {
				return index
			}
		}
	}
	return end
}
