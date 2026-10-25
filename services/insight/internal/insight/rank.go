package insight

import (
	"math"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	feedSummaryLimit     = 240
	feedScoreScale       = 14
	genericFeedRelevance = 0.1
	genericFeedReason    = "Общая повестка"
)

const (
	scoreTitleCompany = 6.0
	scoreBodyCompany  = 2.5
	scoreTagCompany   = 2.0
	scoreSector       = 4.0
	scoreSectorText   = 1.5
	scoreTagSector    = 2.0
	scoreRegion       = 1.5
	scoreFresh        = 2.0
	scoreRecent       = 1.0
	scoreOlder        = 0.25
)

type sectorProfile struct {
	sectors  []string
	keywords []string
}

type industryProfile struct {
	matches []string
	profile sectorProfile
}

var industryProfiles = []industryProfile{
	{
		matches: []string{"банк", "финанс"},
		profile: sectorProfile{
			sectors:  []string{"финансы", "банки"},
			keywords: []string{"банк", "финанс", "кредит", "ставк", "депозит", "ипотек", "биржа", "брокер"},
		},
	},
	{
		matches: []string{"ритейл", "торгов"},
		profile: sectorProfile{
			sectors:  []string{"торговля", "оптовая торговля", "логистика"},
			keywords: []string{"ритейл", "розничн", "торгов", "магазин", "супермаркет", "покупател", "логист"},
		},
	},
	{
		matches: []string{"металлург", "металл"},
		profile: sectorProfile{
			sectors:  []string{"металлургия", "промышленность", "машиностроение"},
			keywords: []string{"металлург", "сталь", "прокат", "металл", "руда", "труб", "сырье"},
		},
	},
	{
		matches: []string{"нефтегаз", "нефт", "газ"},
		profile: sectorProfile{
			sectors:  []string{"нефтегаз", "энергетика", "топливо"},
			keywords: []string{"нефт", "газ", "топлив", "энерг", "бензин", "добыч", "месторожд"},
		},
	},
	{
		matches: []string{"технолог", "it"},
		profile: sectorProfile{
			sectors:  []string{"it", "технологии", "связь"},
			keywords: []string{"технолог", "софт", "software", "облач", "цифров", "интернет", "платформ", "данн"},
		},
	},
	{
		matches: []string{"строитель", "девелоп"},
		profile: sectorProfile{
			sectors:  []string{"строительство", "недвижимость"},
			keywords: []string{"строитель", "девелоп", "недвижим", "жиль", "объект", "подряд"},
		},
	},
	{
		matches: []string{"транспорт", "логистик"},
		profile: sectorProfile{
			sectors:  []string{"транспорт", "логистика"},
			keywords: []string{"транспорт", "логистик", "перевоз", "железнодор", "груз", "поставк"},
		},
	},
	{
		matches: []string{"энерг"},
		profile: sectorProfile{
			sectors:  []string{"энергетика", "электроэнергетика"},
			keywords: []string{"энерг", "электростанц", "сетев", "тариф", "мощност"},
		},
	},
	{
		matches: []string{"сельхоз", "агро"},
		profile: sectorProfile{
			sectors:  []string{"сельское хозяйство", "агропромышленность"},
			keywords: []string{"сельхоз", "агро", "фермер", "зерн", "урожай", "животновод"},
		},
	},
	{
		matches: []string{"химическ", "химия"},
		profile: sectorProfile{
			sectors:  []string{"химическая промышленность", "промышленность"},
			keywords: []string{"химическ", "удобрен", "реагент", "полимер"},
		},
	},
	{
		matches: []string{"фарма"},
		profile: sectorProfile{
			sectors:  []string{"фармацевтика", "здравоохранение"},
			keywords: []string{"фарма", "лекарств", "аптечн", "медицин", "клиник"},
		},
	},
	{
		matches: []string{"телеком", "связь"},
		profile: sectorProfile{
			sectors:  []string{"телеком", "связь"},
			keywords: []string{"телеком", "связь", "мобильн", "оператор", "трафик"},
		},
	},
	{
		matches: []string{"авто", "машиностро"},
		profile: sectorProfile{
			sectors:  []string{"автомобилестроение", "машиностроение"},
			keywords: []string{"авто", "машин", "двигател", "запчаст", "дилер"},
		},
	},
}

var genericCompanyTokens = map[string]struct{}{
	"банк":     {},
	"group":    {},
	"групп":    {},
	"группа":   {},
	"компания": {},
	"холдинг":  {},
	"пао":      {},
	"оао":      {},
	"зао":      {},
	"ооо":      {},
	"ltd":      {},
	"inc":      {},
	"company":  {},
	"bank":     {},
}

type rankedFeedItem struct {
	item      FeedItem
	score     float64
	published time.Time
	eventID   string
}

func RankFeed(company Company, news []NewsItem, now time.Time) []FeedItem {
	ranked := make([]rankedFeedItem, 0, len(news))
	for _, item := range news {
		eventID := feedEventID(item)
		if eventID == "" {
			continue
		}

		score, reasons := scoreNews(company, item, now)
		relevance := normalizeRelevance(score)
		if score < 1 || !hasRelevanceReason(reasons) {
			score = 0
			relevance = genericFeedRelevance
			reasons = []string{genericFeedReason}
		}

		ranked = append(ranked, rankedFeedItem{
			item:      toFeedItem(company, item, relevance, reasons),
			score:     score,
			published: item.PublishedAt,
			eventID:   eventID,
		})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		if !ranked[i].published.Equal(ranked[j].published) {
			return ranked[i].published.After(ranked[j].published)
		}
		return ranked[i].eventID < ranked[j].eventID
	})

	items := make([]FeedItem, 0, len(ranked))
	for _, entry := range ranked {
		items = append(items, entry.item)
	}
	return items
}

func scoreNews(company Company, item NewsItem, now time.Time) (float64, []string) {
	title := normalizeText(item.Title)
	body := normalizeText(item.FullText)
	tags := normalizeText(strings.Join(item.Tags, " "))
	text := strings.TrimSpace(title + " " + body)
	profile := profileFor(company.Industry)
	terms := companyTerms(company)

	score := 0.0
	reasons := make([]string, 0, 4)
	add := func(points float64, reason string) {
		score += points
		if reason == "" {
			return
		}
		for _, existing := range reasons {
			if existing == reason {
				return
			}
		}
		reasons = append(reasons, reason)
	}

	switch {
	case matchesAny(title, terms, false):
		add(scoreTitleCompany, "Упоминание компании")
	case matchesAny(body, terms, true):
		add(scoreBodyCompany, "Упоминание компании")
	}
	if matchesAny(tags, terms, false) {
		add(scoreTagCompany, "Тег компании")
	}

	if reason, ok := matchedSector(item.TargetSectors, profile.sectors); ok {
		add(scoreSector, reason)
	}
	if matchesAny(tags, profile.keywords, true) {
		add(scoreTagSector, "Отраслевой тег")
	}
	if matchesAny(text, profile.keywords, true) {
		add(scoreSectorText, "Отраслевой контекст")
	}

	if region := strings.TrimSpace(company.Region); region != "" {
		reason := "Регион: " + region
		region = stem(normalizeText(region))
		switch {
		case containsTerm(title, region, true):
			add(scoreRegion, reason)
		case containsTerm(body, region, true):
			add(scoreRegion/2, reason)
		}
	}

	if !item.PublishedAt.IsZero() {
		age := now.Sub(item.PublishedAt)
		switch {
		case age >= 0 && age <= 72*time.Hour:
			add(scoreFresh, "Свежая новость")
		case age >= 0 && age <= 14*24*time.Hour:
			add(scoreRecent, "Свежая новость")
		case age >= 0 && age <= 45*24*time.Hour:
			add(scoreOlder, "")
		}
	}

	return score, reasons
}

func profileFor(industry string) sectorProfile {
	normalized := normalizeText(industry)
	for _, entry := range industryProfiles {
		for _, match := range entry.matches {
			if normalized == match || strings.Contains(normalized, match) {
				return entry.profile
			}
		}
	}
	if normalized == "" {
		return sectorProfile{}
	}
	return sectorProfile{sectors: []string{normalized}, keywords: []string{normalized}}
}

func companyTerms(company Company) []string {
	terms := make([]string, 0, 8)
	seen := make(map[string]struct{}, 8)

	add := func(value string) {
		value = normalizeText(value)
		if value == "" {
			return
		}
		if _, exists := seen[value]; exists {
			return
		}
		seen[value] = struct{}{}
		terms = append(terms, value)
		if trimmed := stem(value); trimmed != value {
			if _, exists := seen[trimmed]; !exists {
				seen[trimmed] = struct{}{}
				terms = append(terms, trimmed)
			}
		}
	}

	for _, value := range []string{company.Name, company.ID} {
		add(value)
		normalized := normalizeText(value)
		if strings.Contains(normalized, "-") {
			add(strings.ReplaceAll(normalized, "-", " "))
		}
		for _, token := range strings.FieldsFunc(normalized, func(r rune) bool {
			return !unicode.IsLetter(r) && !unicode.IsDigit(r)
		}) {
			if utf8.RuneCountInString(token) < 4 {
				continue
			}
			if _, generic := genericCompanyTokens[token]; generic {
				continue
			}
			add(token)
		}
	}

	return terms
}

func hasRelevanceReason(reasons []string) bool {
	for _, reason := range reasons {
		if reason != "Свежая новость" {
			return true
		}
	}
	return false
}

func matchedSector(values, wanted []string) (string, bool) {
	for _, value := range values {
		for _, target := range wanted {
			if sectorMatches(value, target) {
				return "Отрасль: " + strings.TrimSpace(value), true
			}
		}
	}
	return "", false
}

func sectorMatches(value, target string) bool {
	value = normalizeText(value)
	target = normalizeText(target)
	if value == "" || target == "" {
		return false
	}
	if value == target {
		return true
	}
	if utf8.RuneCountInString(value) >= 4 && strings.Contains(value, target) {
		return true
	}
	return utf8.RuneCountInString(target) >= 4 && strings.Contains(target, value)
}

func matchesAny(text string, terms []string, prefix bool) bool {
	for _, term := range terms {
		if containsTerm(text, term, prefix || utf8.RuneCountInString(term) >= 5) {
			return true
		}
	}
	return false
}

func containsTerm(text, term string, prefix bool) bool {
	if text == "" || term == "" {
		return false
	}

	for start := 0; start <= len(text)-len(term); {
		index := strings.Index(text[start:], term)
		if index < 0 {
			return false
		}
		index += start

		beforeOK := index == 0
		if !beforeOK {
			r, _ := utf8.DecodeLastRuneInString(text[:index])
			beforeOK = !isWordRune(r)
		}

		end := index + len(term)
		afterOK := end == len(text)
		if !afterOK {
			r, _ := utf8.DecodeRuneInString(text[end:])
			afterOK = !isWordRune(r)
		}

		if beforeOK && (afterOK || prefix) {
			return true
		}
		start = index + 1
	}
	return false
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r)
}

func stem(value string) string {
	runes := []rune(value)
	for _, suffix := range []string{"ий", "ый", "ая", "ое", "ые", "ии", "а", "я", "ь", "й"} {
		if utf8.RuneCountInString(value) <= utf8.RuneCountInString(suffix)+3 {
			continue
		}
		if strings.HasSuffix(value, suffix) {
			return string(runes[:len(runes)-utf8.RuneCountInString(suffix)])
		}
	}
	return value
}

func normalizeText(value string) string {
	value = strings.ToLower(strings.ReplaceAll(value, "ё", "е"))
	return strings.Join(strings.Fields(value), " ")
}

func normalizeRelevance(score float64) float64 {
	return math.Round(math.Min(1, score/feedScoreScale)*100) / 100
}

func toFeedItem(company Company, item NewsItem, relevance float64, reasons []string) FeedItem {
	summary := summarize(item.FullText, feedSummaryLimit)
	if summary == "" {
		summary = strings.TrimSpace(item.Title)
	}
	if reasons == nil {
		reasons = []string{}
	}

	publishedAt := ""
	if !item.PublishedAt.IsZero() {
		publishedAt = item.PublishedAt.UTC().Format(time.RFC3339)
	}

	tags := item.Tags
	if tags == nil {
		tags = []string{}
	}
	sectors := item.TargetSectors
	if sectors == nil {
		sectors = []string{}
	}

	return FeedItem{
		EventID:       feedEventID(item),
		CompanyID:     company.ID,
		Title:         item.Title,
		Summary:       summary,
		Source:        item.SourceName,
		URL:           item.SourceURL,
		PublishedAt:   publishedAt,
		Relevance:     relevance,
		Reasons:       reasons,
		Tags:          tags,
		TargetSectors: sectors,
	}
}

func feedEventID(item NewsItem) string {
	if trimmed := strings.TrimSpace(item.NewsID); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(item.DocumentID)
}

func summarize(text string, limit int) string {
	compact := strings.Join(strings.Fields(text), " ")
	runes := []rune(compact)
	if len(runes) <= limit {
		return compact
	}

	cut := string(runes[:limit])
	if index := strings.LastIndex(cut, " "); index > 0 {
		cut = cut[:index]
	}
	return strings.TrimSpace(cut) + "…"
}
