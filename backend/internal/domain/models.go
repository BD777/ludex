package domain

type Game struct {
	ID             int64    `json:"id"`
	Title          string   `json:"title"`
	Aliases        []string `json:"aliases"`
	Description    string   `json:"description"`
	CurrentVersion string   `json:"current_version"`
	CoverImage     string   `json:"cover_image"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

type Source struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	URL        string `json:"url"`
	ProxyURL   string `json:"proxy_url"`
	Enabled    bool   `json:"enabled"`
	TrustLevel int    `json:"trust_level"`
	ConfigJSON string `json:"config_json"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

type SourceItem struct {
	ID             int64          `json:"id"`
	SourceID       *int64         `json:"source_id,omitempty"`
	SourceType     string         `json:"source_type"`
	ExternalID     string         `json:"external_id"`
	Title          string         `json:"title"`
	RawURL         string         `json:"raw_url"`
	RawContentPath string         `json:"raw_content_path"`
	ParsedJSON     map[string]any `json:"parsed_json"`
	FetchedAt      string         `json:"fetched_at"`
	PublishedAt    string         `json:"published_at"`
	MatchedGameID  *int64         `json:"matched_game_id,omitempty"`
	Status         string         `json:"status"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

type MediaAsset struct {
	ID           int64  `json:"id"`
	GameID       *int64 `json:"game_id,omitempty"`
	SourceItemID *int64 `json:"source_item_id,omitempty"`
	Type         string `json:"type"`
	LocalPath    string `json:"local_path"`
	OriginalURL  string `json:"original_url"`
	Hash         string `json:"hash"`
	PublicURL    string `json:"public_url"`
	CreatedAt    string `json:"created_at"`
}

type Task struct {
	ID              int64          `json:"id"`
	Kind            string         `json:"kind"`
	DedupeKey       string         `json:"dedupe_key"`
	Status          string         `json:"status"`
	Title           string         `json:"title"`
	Message         string         `json:"message"`
	ProgressCurrent int            `json:"progress_current"`
	ProgressTotal   int            `json:"progress_total"`
	ResultJSON      map[string]any `json:"result_json,omitempty"`
	Error           string         `json:"error"`
	CreatedAt       string         `json:"created_at"`
	StartedAt       string         `json:"started_at"`
	FinishedAt      string         `json:"finished_at"`
	UpdatedAt       string         `json:"updated_at"`
}

type Transcript struct {
	Source      string              `json:"source"`
	SourceURL   string              `json:"source_url"`
	ExternalID  string              `json:"external_id"`
	Title       string              `json:"title"`
	KeyValues   map[string][]string `json:"key_values"`
	Sections    []TranscriptSection `json:"sections"`
	Images      []string            `json:"images"`
	MediaAssets []MediaAsset        `json:"media_assets"`
	Tags        []string            `json:"tags"`
	Inferred    TranscriptInferred  `json:"inferred"`
	Warnings    []string            `json:"warnings"`
}

type TranscriptSection struct {
	Heading string `json:"heading"`
	Body    string `json:"body"`
}

type TranscriptInferred struct {
	GameTitle   string `json:"game_title"`
	Version     string `json:"version"`
	Developer   string `json:"developer"`
	Description string `json:"description"`
	CoverImage  string `json:"cover_image"`
}
