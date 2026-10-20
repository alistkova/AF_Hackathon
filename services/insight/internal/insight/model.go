package insight

type Company struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	INN         string `json:"inn,omitempty"`
	Industry    string `json:"industry,omitempty"`
	Region      string `json:"region,omitempty"`
	Description string `json:"description,omitempty"`
}

type EventContext struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Summary     string `json:"summary,omitempty"`
	Source      string `json:"source,omitempty"`
	URL         string `json:"url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type Evidence struct {
	SourceID    string `json:"source_id"`
	Title       string `json:"title"`
	URL         string `json:"url"`
	Snippet     string `json:"snippet,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type Source struct {
	SourceID string `json:"source_id"`
	Title    string `json:"title"`
	URL      string `json:"url"`
}

type Insight struct {
	Summary       string         `json:"summary"`
	WhyItMatters  string         `json:"why_it_matters"`
	Risks         []string       `json:"risks"`
	Opportunities []string       `json:"opportunities"`
	BankPosition  string         `json:"bank_position"`
	AIConclusion  string         `json:"ai_conclusion"`
	Sources       []Source       `json:"sources"`
	Actions       []string       `json:"actions"`
	Confidence    float64        `json:"confidence"`
	Meta          map[string]any `json:"meta,omitempty"`
}

type FeedItem struct {
	EventID     string `json:"event_id"`
	CompanyID   string `json:"company_id"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Source      string `json:"source"`
	URL         string `json:"url"`
	PublishedAt string `json:"published_at"`
}
