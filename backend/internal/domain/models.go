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

type Adapter struct {
	ID              string                `json:"id"`
	Name            string                `json:"name"`
	Kind            string                `json:"kind"`
	Status          string                `json:"status"`
	Input           string                `json:"input"`
	Dedupe          string                `json:"dedupe"`
	Endpoint        string                `json:"endpoint"`
	Attachments     string                `json:"attachments"`
	AuthDomain      string                `json:"auth_domain"`
	AuthCookieNames []string              `json:"auth_cookie_names"`
	WithoutBridge   string                `json:"without_bridge"`
	Browse          AdapterBrowseManifest `json:"browse"`
}

type AdapterBrowseManifest struct {
	Enabled      bool                      `json:"enabled"`
	Description  string                    `json:"description"`
	Presets      []AdapterBrowsePreset     `json:"presets"`
	Capabilities AdapterBrowseCapabilities `json:"capabilities"`
}

type AdapterBrowseCapabilities struct {
	CustomURL  bool   `json:"custom_url"`
	Pagination bool   `json:"pagination"`
	Search     bool   `json:"search"`
	Filter     bool   `json:"filter"`
	Sort       bool   `json:"sort"`
	Import     bool   `json:"import"`
	SearchNote string `json:"search_note"`
	FilterNote string `json:"filter_note"`
	SortNote   string `json:"sort_note"`
}

type AdapterBrowsePreset struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

type AdapterBrowseFilter struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Count string `json:"count"`
	URL   string `json:"url"`
}

type AdapterListItem struct {
	AdapterID  string   `json:"adapter_id"`
	ExternalID string   `json:"external_id"`
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	PreviewURL string   `json:"preview_url"`
	CoverImage string   `json:"cover_image"`
	Author     string   `json:"author"`
	StartedAt  string   `json:"started_at"`
	LatestAt   string   `json:"latest_at"`
	LatestBy   string   `json:"latest_by"`
	Prefixes   []string `json:"prefixes"`
	Tags       []string `json:"tags"`
	Replies    string   `json:"replies"`
	Views      string   `json:"views"`
	Rating     string   `json:"rating"`
	Votes      string   `json:"votes"`
	Importable bool     `json:"importable"`
}

type AdapterBrowsePage struct {
	AdapterID    string                    `json:"adapter_id"`
	Title        string                    `json:"title"`
	URL          string                    `json:"url"`
	Page         int                       `json:"page"`
	TotalPages   int                       `json:"total_pages"`
	PrevURL      string                    `json:"prev_url"`
	NextURL      string                    `json:"next_url"`
	Items        []AdapterListItem         `json:"items"`
	Filters      []AdapterBrowseFilter     `json:"filters"`
	Warnings     []string                  `json:"warnings"`
	Capabilities AdapterBrowseCapabilities `json:"capabilities"`
}

type AuthProfile struct {
	ID              int64        `json:"id"`
	AdapterID       string       `json:"adapter_id"`
	Domain          string       `json:"domain"`
	CookieHeader    string       `json:"-"`
	CookieCount     int          `json:"cookie_count"`
	CookieExpiresAt string       `json:"cookie_expires_at"`
	Cookies         []AuthCookie `json:"cookies"`
	Username        string       `json:"username"`
	UserAgent       string       `json:"user_agent"`
	SourceURL       string       `json:"source_url"`
	ImportedAt      string       `json:"imported_at"`
	LastUsedAt      string       `json:"last_used_at"`
	CreatedAt       string       `json:"created_at"`
	UpdatedAt       string       `json:"updated_at"`
}

type AuthCookie struct {
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	Path      string `json:"path"`
	ExpiresAt string `json:"expires_at"`
	Session   bool   `json:"session"`
	Secure    bool   `json:"secure"`
	HTTPOnly  bool   `json:"http_only"`
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
	Source        string              `json:"source"`
	SourceURL     string              `json:"source_url"`
	ExternalID    string              `json:"external_id"`
	Title         string              `json:"title"`
	Fields        TranscriptFields    `json:"fields"`
	KeyValues     map[string][]string `json:"key_values"`
	Sections      []TranscriptSection `json:"sections"`
	Images        []string            `json:"images"`
	MediaItems    []MediaItem         `json:"media_items"`
	MediaAssets   []MediaAsset        `json:"media_assets"`
	MediaFailures []MediaFailure      `json:"media_failures"`
	Tags          []string            `json:"tags"`
	Inferred      TranscriptInferred  `json:"inferred"`
	Warnings      []string            `json:"warnings"`
}

type MediaItem struct {
	Position    int    `json:"position"`
	Role        string `json:"role"`
	Status      string `json:"status"`
	OriginalURL string `json:"original_url"`
	PublicURL   string `json:"public_url"`
	Error       string `json:"error,omitempty"`
}

type MediaFailure struct {
	Position    int    `json:"position"`
	Role        string `json:"role"`
	OriginalURL string `json:"original_url"`
	Error       string `json:"error"`
}

type NamedURL struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type DownloadGroup struct {
	Platform string     `json:"platform"`
	Note     string     `json:"note"`
	Links    []NamedURL `json:"links"`
}

type TranscriptFields struct {
	GameName         string          `json:"game_name"`
	Prefixes         []string        `json:"prefixes"`
	Engine           string          `json:"engine"`
	CoverImage       string          `json:"cover_image"`
	Description      string          `json:"description"`
	ThreadUpdated    string          `json:"thread_updated"`
	ReleaseDate      string          `json:"release_date"`
	Developer        string          `json:"developer"`
	DeveloperLinks   []NamedURL      `json:"developer_links"`
	Censored         *bool           `json:"censored,omitempty"`
	Version          string          `json:"version"`
	OperatingSystems []string        `json:"operating_systems"`
	Languages        []string        `json:"languages"`
	Genres           []string        `json:"genres"`
	Changelog        string          `json:"changelog"`
	ChangelogHTML    string          `json:"changelog_html"`
	DownloadGroups   []DownloadGroup `json:"download_groups"`
	Screenshots      []string        `json:"screenshots"`
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
