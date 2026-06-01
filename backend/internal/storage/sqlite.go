package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"local/ludex/internal/domain"
)

type Store struct {
	db      *sql.DB
	dataDir string
}

type scanner interface {
	Scan(dest ...any) error
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func Open(dataDir string) (*Store, error) {
	if dataDir == "" {
		dataDir = ".data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, err
	}
	dbPath := defaultDatabasePath(dataDir)
	db, err := sql.Open("sqlite3", dbPath+"?_foreign_keys=on&_busy_timeout=5000")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	store := &Store{db: db, dataDir: dataDir}
	if err := store.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) DataDir() string {
	return s.dataDir
}

func defaultDatabasePath(dataDir string) string {
	current := filepath.Join(dataDir, "ludex.db")
	legacy := filepath.Join(dataDir, "game-meta-browser.db")
	if _, err := os.Stat(current); err == nil {
		return current
	}
	if _, err := os.Stat(legacy); err == nil {
		return legacy
	}
	return current
}

func (s *Store) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	rows, err := s.db.QueryContext(ctx, "PRAGMA table_info("+table+")")
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition))
	return err
}

func (s *Store) migrate(ctx context.Context) error {
	schema := `
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS games (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	title TEXT NOT NULL,
	aliases_json TEXT NOT NULL DEFAULT '[]',
	description TEXT NOT NULL DEFAULT '',
	current_version TEXT NOT NULL DEFAULT '',
	cover_image TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sources (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	name TEXT NOT NULL,
	type TEXT NOT NULL,
	url TEXT NOT NULL DEFAULT '',
	proxy_url TEXT NOT NULL DEFAULT '',
	enabled INTEGER NOT NULL DEFAULT 1,
	trust_level INTEGER NOT NULL DEFAULT 50,
	config_json TEXT NOT NULL DEFAULT '{}',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS source_items (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	source_id INTEGER REFERENCES sources(id) ON DELETE SET NULL,
	source_type TEXT NOT NULL DEFAULT '',
	external_id TEXT NOT NULL DEFAULT '',
	title TEXT NOT NULL,
	raw_url TEXT NOT NULL DEFAULT '',
	raw_content_path TEXT NOT NULL DEFAULT '',
	parsed_json TEXT NOT NULL DEFAULT '{}',
	fetched_at TEXT NOT NULL DEFAULT '',
	published_at TEXT NOT NULL DEFAULT '',
	matched_game_id INTEGER REFERENCES games(id) ON DELETE SET NULL,
	status TEXT NOT NULL DEFAULT 'imported',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS media_assets (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	game_id INTEGER REFERENCES games(id) ON DELETE CASCADE,
	source_item_id INTEGER REFERENCES source_items(id) ON DELETE SET NULL,
	type TEXT NOT NULL,
	local_path TEXT NOT NULL,
	original_url TEXT NOT NULL DEFAULT '',
	hash TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL,
	dedupe_key TEXT NOT NULL DEFAULT '',
	status TEXT NOT NULL,
	title TEXT NOT NULL,
	message TEXT NOT NULL DEFAULT '',
	progress_current INTEGER NOT NULL DEFAULT 0,
	progress_total INTEGER NOT NULL DEFAULT 0,
	result_json TEXT NOT NULL DEFAULT '{}',
	error TEXT NOT NULL DEFAULT '',
	created_at TEXT NOT NULL,
	started_at TEXT NOT NULL DEFAULT '',
	finished_at TEXT NOT NULL DEFAULT '',
	updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_games_title ON games(title);
CREATE INDEX IF NOT EXISTS idx_source_items_source ON source_items(source_id);
CREATE INDEX IF NOT EXISTS idx_source_items_match ON source_items(matched_game_id);
CREATE INDEX IF NOT EXISTS idx_media_assets_game ON media_assets(game_id);
CREATE INDEX IF NOT EXISTS idx_media_assets_source_item ON media_assets(source_item_id);
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
`
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "source_items", "source_type", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "tasks", "dedupe_key", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE source_items
SET source_type = COALESCE((SELECT type FROM sources WHERE sources.id = source_items.source_id), source_type)
WHERE source_type = '' AND source_id IS NOT NULL;

UPDATE source_items
SET source_type = 'f95zone'
WHERE source_type = '' AND external_id != '';

CREATE INDEX IF NOT EXISTS idx_source_items_adapter_key ON source_items(source_type, external_id);
CREATE INDEX IF NOT EXISTS idx_tasks_dedupe_key ON tasks(kind, dedupe_key);
CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_active_dedupe_key ON tasks(kind, dedupe_key)
WHERE dedupe_key != '' AND status IN ('queued', 'running');
`)
	return err
}

func (s *Store) ListGames(ctx context.Context, query string) ([]domain.Game, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, title, aliases_json, description, current_version, cover_image, created_at, updated_at
FROM games
WHERE ? = '' OR title LIKE '%' || ? || '%' OR aliases_json LIKE '%' || ? || '%'
ORDER BY updated_at DESC, id DESC
LIMIT 500`, query, query, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	games := []domain.Game{}
	for rows.Next() {
		game, err := scanGame(rows)
		if err != nil {
			return nil, err
		}
		games = append(games, game)
	}
	return games, rows.Err()
}

func (s *Store) GetGame(ctx context.Context, id int64) (domain.Game, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, title, aliases_json, description, current_version, cover_image, created_at, updated_at
FROM games
WHERE id = ?`, id)
	return scanGame(row)
}

func (s *Store) CreateGame(ctx context.Context, input domain.Game) (domain.Game, error) {
	if input.Title == "" {
		return domain.Game{}, errors.New("title is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	aliases, err := json.Marshal(input.Aliases)
	if err != nil {
		return domain.Game{}, err
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO games (title, aliases_json, description, current_version, cover_image, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		input.Title, string(aliases), input.Description, input.CurrentVersion, input.CoverImage, now, now)
	if err != nil {
		return domain.Game{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Game{}, err
	}
	return s.GetGame(ctx, id)
}

func (s *Store) UpdateGame(ctx context.Context, id int64, input domain.Game) (domain.Game, error) {
	if input.Title == "" {
		return domain.Game{}, errors.New("title is required")
	}
	aliases, err := json.Marshal(input.Aliases)
	if err != nil {
		return domain.Game{}, err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE games
SET title = ?, aliases_json = ?, description = ?, current_version = ?, cover_image = ?, updated_at = ?
WHERE id = ?`,
		input.Title,
		string(aliases),
		input.Description,
		input.CurrentVersion,
		input.CoverImage,
		time.Now().UTC().Format(time.RFC3339),
		id,
	)
	if err != nil {
		return domain.Game{}, err
	}
	return s.GetGame(ctx, id)
}

func (s *Store) DeleteGame(ctx context.Context, id int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	paths, err := queryStrings(ctx, tx, `
SELECT local_path
FROM media_assets
WHERE game_id = ? AND source_item_id IS NULL`, id)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM media_assets
WHERE game_id = ? AND source_item_id IS NULL`, id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE media_assets
SET game_id = NULL
WHERE game_id = ? AND source_item_id IS NOT NULL`, id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE source_items
SET matched_game_id = NULL, status = 'imported', updated_at = ?
WHERE matched_game_id = ?`, time.Now().UTC().Format(time.RFC3339), id); err != nil {
		return nil, err
	}
	res, err := tx.ExecContext(ctx, `
DELETE FROM games
WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, err
	}
	if affected == 0 {
		return nil, sql.ErrNoRows
	}
	paths, err = unreferencedLocalPaths(ctx, tx, paths)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return paths, nil
}

func (s *Store) ListSources(ctx context.Context) ([]domain.Source, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, name, type, url, proxy_url, enabled, trust_level, config_json, created_at, updated_at
FROM sources
ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sources := []domain.Source{}
	for rows.Next() {
		source, err := scanSource(rows)
		if err != nil {
			return nil, err
		}
		sources = append(sources, source)
	}
	return sources, rows.Err()
}

func (s *Store) GetSource(ctx context.Context, id int64) (domain.Source, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, type, url, proxy_url, enabled, trust_level, config_json, created_at, updated_at
FROM sources
WHERE id = ?`, id)
	return scanSource(row)
}

func (s *Store) CreateSource(ctx context.Context, input domain.Source) (domain.Source, error) {
	if input.Name == "" {
		return domain.Source{}, errors.New("name is required")
	}
	if input.Type == "" {
		input.Type = "f95zone"
	}
	if input.ConfigJSON == "" {
		input.ConfigJSON = "{}"
	}
	if input.TrustLevel == 0 {
		input.TrustLevel = 50
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO sources (name, type, url, proxy_url, enabled, trust_level, config_json, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Name, input.Type, input.URL, input.ProxyURL, boolInt(input.Enabled), input.TrustLevel, input.ConfigJSON, now, now)
	if err != nil {
		return domain.Source{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Source{}, err
	}
	return s.GetSource(ctx, id)
}

func (s *Store) ListSourceItems(ctx context.Context) ([]domain.SourceItem, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, source_id, source_type, external_id, title, raw_url, raw_content_path, parsed_json, fetched_at, published_at, matched_game_id, status, created_at, updated_at
FROM source_items
ORDER BY fetched_at DESC, id DESC
LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []domain.SourceItem{}
	for rows.Next() {
		item, err := scanSourceItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) GetSourceItem(ctx context.Context, id int64) (domain.SourceItem, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, source_id, source_type, external_id, title, raw_url, raw_content_path, parsed_json, fetched_at, published_at, matched_game_id, status, created_at, updated_at
FROM source_items
WHERE id = ?`, id)
	return scanSourceItem(row)
}

func (s *Store) DeleteSourceItem(ctx context.Context, id int64) ([]string, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var rawPath string
	if err := tx.QueryRowContext(ctx, `
SELECT raw_content_path
FROM source_items
WHERE id = ?`, id).Scan(&rawPath); err != nil {
		return nil, err
	}
	paths, err := queryStrings(ctx, tx, `
SELECT local_path
FROM media_assets
WHERE source_item_id = ? AND game_id IS NULL`, id)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM media_assets
WHERE source_item_id = ? AND game_id IS NULL`, id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
UPDATE media_assets
SET source_item_id = NULL
WHERE source_item_id = ? AND game_id IS NOT NULL`, id); err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `
DELETE FROM source_items
WHERE id = ?`, id); err != nil {
		return nil, err
	}
	paths, err = unreferencedLocalPaths(ctx, tx, paths)
	if err != nil {
		return nil, err
	}
	if rawPath != "" {
		var count int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM source_items
WHERE raw_content_path = ?`, rawPath).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 && !containsString(paths, rawPath) {
			paths = append(paths, rawPath)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return paths, nil
}

func (s *Store) CreateSourceItem(ctx context.Context, input domain.SourceItem) (domain.SourceItem, error) {
	if input.Title == "" {
		return domain.SourceItem{}, errors.New("title is required")
	}
	if input.Status == "" {
		input.Status = "imported"
	}
	if input.FetchedAt == "" {
		input.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	}
	parsed, err := json.Marshal(input.ParsedJSON)
	if err != nil {
		return domain.SourceItem{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO source_items (
	source_id, source_type, external_id, title, raw_url, raw_content_path, parsed_json,
	fetched_at, published_at, matched_game_id, status, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		nullableInt(input.SourceID),
		input.SourceType,
		input.ExternalID,
		input.Title,
		input.RawURL,
		input.RawContentPath,
		string(parsed),
		input.FetchedAt,
		input.PublishedAt,
		nullableInt(input.MatchedGameID),
		input.Status,
		now,
		now,
	)
	if err != nil {
		return domain.SourceItem{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.SourceItem{}, err
	}
	return s.GetSourceItem(ctx, id)
}

func (s *Store) UpsertSourceItemByAdapterKey(ctx context.Context, input domain.SourceItem) (domain.SourceItem, bool, error) {
	if input.SourceType == "" || input.ExternalID == "" {
		item, err := s.CreateSourceItem(ctx, input)
		return item, false, err
	}
	existing, err := s.GetSourceItemByAdapterKey(ctx, input.SourceType, input.ExternalID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			item, createErr := s.CreateSourceItem(ctx, input)
			return item, false, createErr
		}
		return domain.SourceItem{}, false, err
	}
	if input.Title == "" {
		return domain.SourceItem{}, true, errors.New("title is required")
	}
	if input.FetchedAt == "" {
		input.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	}
	matchedGameID := input.MatchedGameID
	if matchedGameID == nil {
		matchedGameID = existing.MatchedGameID
	}
	status := input.Status
	if status == "" {
		status = "imported"
	}
	if matchedGameID != nil {
		status = "matched"
	}
	parsed, err := json.Marshal(input.ParsedJSON)
	if err != nil {
		return domain.SourceItem{}, true, err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE source_items
SET source_id = ?, source_type = ?, external_id = ?, title = ?, raw_url = ?, raw_content_path = ?,
	parsed_json = ?, fetched_at = ?, published_at = ?, matched_game_id = ?, status = ?, updated_at = ?
WHERE id = ?`,
		nullableInt(input.SourceID),
		input.SourceType,
		input.ExternalID,
		input.Title,
		input.RawURL,
		input.RawContentPath,
		string(parsed),
		input.FetchedAt,
		input.PublishedAt,
		nullableInt(matchedGameID),
		status,
		time.Now().UTC().Format(time.RFC3339),
		existing.ID,
	)
	if err != nil {
		return domain.SourceItem{}, true, err
	}
	item, err := s.GetSourceItem(ctx, existing.ID)
	return item, true, err
}

func (s *Store) GetSourceItemByAdapterKey(ctx context.Context, sourceType string, externalID string) (domain.SourceItem, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, source_id, source_type, external_id, title, raw_url, raw_content_path, parsed_json, fetched_at, published_at, matched_game_id, status, created_at, updated_at
FROM source_items
WHERE source_type = ? AND external_id = ?
ORDER BY updated_at DESC, id DESC
LIMIT 1`, sourceType, externalID)
	return scanSourceItem(row)
}

func (s *Store) SetSourceItemMatch(ctx context.Context, itemID int64, gameID *int64) (domain.SourceItem, error) {
	status := "matched"
	if gameID == nil {
		status = "imported"
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE source_items
SET matched_game_id = ?, status = ?, updated_at = ?
WHERE id = ?`,
		nullableInt(gameID),
		status,
		time.Now().UTC().Format(time.RFC3339),
		itemID,
	)
	if err != nil {
		return domain.SourceItem{}, err
	}
	return s.GetSourceItem(ctx, itemID)
}

func (s *Store) UpdateSourceItemParsedJSON(ctx context.Context, itemID int64, parsedJSON map[string]any) (domain.SourceItem, error) {
	parsed, err := json.Marshal(parsedJSON)
	if err != nil {
		return domain.SourceItem{}, err
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE source_items
SET parsed_json = ?, updated_at = ?
WHERE id = ?`,
		string(parsed),
		time.Now().UTC().Format(time.RFC3339),
		itemID,
	)
	if err != nil {
		return domain.SourceItem{}, err
	}
	return s.GetSourceItem(ctx, itemID)
}

func (s *Store) CreateMediaAsset(ctx context.Context, input domain.MediaAsset) (domain.MediaAsset, error) {
	if input.Type == "" {
		return domain.MediaAsset{}, errors.New("media type is required")
	}
	if input.LocalPath == "" {
		return domain.MediaAsset{}, errors.New("local path is required")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO media_assets (game_id, source_item_id, type, local_path, original_url, hash, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?)`,
		nullableInt(input.GameID),
		nullableInt(input.SourceItemID),
		input.Type,
		input.LocalPath,
		input.OriginalURL,
		input.Hash,
		now,
	)
	if err != nil {
		return domain.MediaAsset{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.MediaAsset{}, err
	}
	return s.GetMediaAsset(ctx, id)
}

func (s *Store) GetMediaAsset(ctx context.Context, id int64) (domain.MediaAsset, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, game_id, source_item_id, type, local_path, original_url, hash, created_at
FROM media_assets
WHERE id = ?`, id)
	return scanMediaAsset(row)
}

func (s *Store) ListMediaAssetsForSourceItem(ctx context.Context, sourceItemID int64) ([]domain.MediaAsset, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, game_id, source_item_id, type, local_path, original_url, hash, created_at
FROM media_assets
WHERE source_item_id = ?
ORDER BY id`, sourceItemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	assets := []domain.MediaAsset{}
	for rows.Next() {
		asset, err := scanMediaAsset(rows)
		if err != nil {
			return nil, err
		}
		assets = append(assets, asset)
	}
	return assets, rows.Err()
}

func (s *Store) SetMediaAssetsGameForSourceItem(ctx context.Context, sourceItemID int64, gameID int64) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE media_assets
SET game_id = ?
WHERE source_item_id = ?`,
		gameID,
		sourceItemID,
	)
	return err
}

func (s *Store) DeleteMediaAssetsForSourceItem(ctx context.Context, sourceItemID int64) error {
	_, err := s.db.ExecContext(ctx, `
DELETE FROM media_assets
WHERE source_item_id = ?`,
		sourceItemID,
	)
	return err
}

func (s *Store) CreateTask(ctx context.Context, input domain.Task) (domain.Task, error) {
	if input.Kind == "" {
		return domain.Task{}, errors.New("task kind is required")
	}
	if input.Title == "" {
		return domain.Task{}, errors.New("task title is required")
	}
	if input.Status == "" {
		input.Status = "queued"
	}
	now := time.Now().UTC().Format(time.RFC3339)
	resultJSON := "{}"
	if input.ResultJSON != nil {
		raw, err := json.Marshal(input.ResultJSON)
		if err != nil {
			return domain.Task{}, err
		}
		resultJSON = string(raw)
	}
	res, err := s.db.ExecContext(ctx, `
INSERT INTO tasks (
	kind, dedupe_key, status, title, message, progress_current, progress_total,
	result_json, error, created_at, started_at, finished_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		input.Kind,
		input.DedupeKey,
		input.Status,
		input.Title,
		input.Message,
		input.ProgressCurrent,
		input.ProgressTotal,
		resultJSON,
		input.Error,
		now,
		input.StartedAt,
		input.FinishedAt,
		now,
	)
	if err != nil {
		return domain.Task{}, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return domain.Task{}, err
	}
	return s.GetTask(ctx, id)
}

func (s *Store) ListTasks(ctx context.Context) ([]domain.Task, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, kind, dedupe_key, status, title, message, progress_current, progress_total, result_json, error, created_at, started_at, finished_at, updated_at
FROM tasks
ORDER BY updated_at DESC, id DESC
LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tasks := []domain.Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) GetTask(ctx context.Context, id int64) (domain.Task, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, kind, dedupe_key, status, title, message, progress_current, progress_total, result_json, error, created_at, started_at, finished_at, updated_at
FROM tasks
WHERE id = ?`, id)
	return scanTask(row)
}

func (s *Store) GetActiveTaskByDedupeKey(ctx context.Context, kind string, dedupeKey string) (domain.Task, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, kind, dedupe_key, status, title, message, progress_current, progress_total, result_json, error, created_at, started_at, finished_at, updated_at
FROM tasks
WHERE kind = ? AND dedupe_key = ? AND status IN ('queued', 'running')
ORDER BY updated_at DESC, id DESC
LIMIT 1`, kind, dedupeKey)
	return scanTask(row)
}

func (s *Store) StartTask(ctx context.Context, id int64, message string) (domain.Task, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
UPDATE tasks
SET status = 'running', message = ?, started_at = ?, updated_at = ?
WHERE id = ?`,
		message,
		now,
		now,
		id,
	)
	if err != nil {
		return domain.Task{}, err
	}
	return s.GetTask(ctx, id)
}

func (s *Store) UpdateTaskProgress(ctx context.Context, id int64, current int, total int, message string) (domain.Task, error) {
	if current < 0 {
		current = 0
	}
	if total < 0 {
		total = 0
	}
	if total > 0 && current > total {
		current = total
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE tasks
SET progress_current = ?, progress_total = ?, message = ?, updated_at = ?
WHERE id = ?`,
		current,
		total,
		message,
		time.Now().UTC().Format(time.RFC3339),
		id,
	)
	if err != nil {
		return domain.Task{}, err
	}
	return s.GetTask(ctx, id)
}

func (s *Store) FinishTask(ctx context.Context, id int64, message string, result map[string]any) (domain.Task, error) {
	resultJSON := "{}"
	if result != nil {
		raw, err := json.Marshal(result)
		if err != nil {
			return domain.Task{}, err
		}
		resultJSON = string(raw)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.ExecContext(ctx, `
UPDATE tasks
SET status = 'succeeded', message = ?, result_json = ?, error = '', finished_at = ?, updated_at = ?,
	progress_current = CASE WHEN progress_total > 0 THEN progress_total ELSE progress_current END
WHERE id = ?`,
		message,
		resultJSON,
		now,
		now,
		id,
	)
	if err != nil {
		return domain.Task{}, err
	}
	return s.GetTask(ctx, id)
}

func (s *Store) FailTask(ctx context.Context, id int64, message string, taskErr error) (domain.Task, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	errText := ""
	if taskErr != nil {
		errText = taskErr.Error()
	}
	_, err := s.db.ExecContext(ctx, `
UPDATE tasks
SET status = 'failed', message = ?, error = ?, finished_at = ?, updated_at = ?
WHERE id = ?`,
		message,
		errText,
		now,
		now,
		id,
	)
	if err != nil {
		return domain.Task{}, err
	}
	return s.GetTask(ctx, id)
}

func scanGame(row scanner) (domain.Game, error) {
	var game domain.Game
	var aliasesJSON string
	if err := row.Scan(
		&game.ID,
		&game.Title,
		&aliasesJSON,
		&game.Description,
		&game.CurrentVersion,
		&game.CoverImage,
		&game.CreatedAt,
		&game.UpdatedAt,
	); err != nil {
		return domain.Game{}, err
	}
	if aliasesJSON != "" {
		if err := json.Unmarshal([]byte(aliasesJSON), &game.Aliases); err != nil {
			return domain.Game{}, fmt.Errorf("decode aliases: %w", err)
		}
	}
	if game.Aliases == nil {
		game.Aliases = []string{}
	}
	return game, nil
}

func scanSource(row scanner) (domain.Source, error) {
	var source domain.Source
	var enabled int
	if err := row.Scan(
		&source.ID,
		&source.Name,
		&source.Type,
		&source.URL,
		&source.ProxyURL,
		&enabled,
		&source.TrustLevel,
		&source.ConfigJSON,
		&source.CreatedAt,
		&source.UpdatedAt,
	); err != nil {
		return domain.Source{}, err
	}
	source.Enabled = enabled != 0
	return source, nil
}

func scanSourceItem(row scanner) (domain.SourceItem, error) {
	var item domain.SourceItem
	var sourceID sql.NullInt64
	var matchedGameID sql.NullInt64
	var parsedJSON string
	if err := row.Scan(
		&item.ID,
		&sourceID,
		&item.SourceType,
		&item.ExternalID,
		&item.Title,
		&item.RawURL,
		&item.RawContentPath,
		&parsedJSON,
		&item.FetchedAt,
		&item.PublishedAt,
		&matchedGameID,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return domain.SourceItem{}, err
	}
	if sourceID.Valid {
		item.SourceID = &sourceID.Int64
	}
	if matchedGameID.Valid {
		item.MatchedGameID = &matchedGameID.Int64
	}
	if parsedJSON != "" {
		if err := json.Unmarshal([]byte(parsedJSON), &item.ParsedJSON); err != nil {
			return domain.SourceItem{}, fmt.Errorf("decode parsed json: %w", err)
		}
	}
	if item.ParsedJSON == nil {
		item.ParsedJSON = map[string]any{}
	}
	return item, nil
}

func scanMediaAsset(row scanner) (domain.MediaAsset, error) {
	var asset domain.MediaAsset
	var gameID sql.NullInt64
	var sourceItemID sql.NullInt64
	if err := row.Scan(
		&asset.ID,
		&gameID,
		&sourceItemID,
		&asset.Type,
		&asset.LocalPath,
		&asset.OriginalURL,
		&asset.Hash,
		&asset.CreatedAt,
	); err != nil {
		return domain.MediaAsset{}, err
	}
	if gameID.Valid {
		asset.GameID = &gameID.Int64
	}
	if sourceItemID.Valid {
		asset.SourceItemID = &sourceItemID.Int64
	}
	return asset, nil
}

func scanTask(row scanner) (domain.Task, error) {
	var task domain.Task
	var resultJSON string
	if err := row.Scan(
		&task.ID,
		&task.Kind,
		&task.DedupeKey,
		&task.Status,
		&task.Title,
		&task.Message,
		&task.ProgressCurrent,
		&task.ProgressTotal,
		&resultJSON,
		&task.Error,
		&task.CreatedAt,
		&task.StartedAt,
		&task.FinishedAt,
		&task.UpdatedAt,
	); err != nil {
		return domain.Task{}, err
	}
	if resultJSON != "" {
		if err := json.Unmarshal([]byte(resultJSON), &task.ResultJSON); err != nil {
			return domain.Task{}, fmt.Errorf("decode task result json: %w", err)
		}
	}
	if task.ResultJSON == nil {
		task.ResultJSON = map[string]any{}
	}
	return task, nil
}

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func nullableInt(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func queryStrings(ctx context.Context, q queryer, query string, args ...any) ([]string, error) {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value != "" {
			values = append(values, value)
		}
	}
	return values, rows.Err()
}

func unreferencedLocalPaths(ctx context.Context, tx *sql.Tx, paths []string) ([]string, error) {
	out := []string{}
	seen := map[string]bool{}
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		var count int
		if err := tx.QueryRowContext(ctx, `
SELECT COUNT(*)
FROM media_assets
WHERE local_path = ?`, path).Scan(&count); err != nil {
			return nil, err
		}
		if count == 0 {
			out = append(out, path)
		}
	}
	return out, nil
}

func containsString(values []string, needle string) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}
	return false
}
