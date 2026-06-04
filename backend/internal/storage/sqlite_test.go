package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BD777/ludex/backend/internal/domain"
)

func TestDefaultDatabasePathPrefersLegacyWhenCurrentIsEmpty(t *testing.T) {
	dataDir := t.TempDir()
	current := filepath.Join(dataDir, "ludex.db")
	legacy := filepath.Join(dataDir, "game-meta-browser.db")
	if err := os.WriteFile(current, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte("legacy"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := defaultDatabasePath(dataDir); got != legacy {
		t.Fatalf("database path = %q, want %q", got, legacy)
	}
}

func TestBrowseCoverCacheRoundTripsMediaAsset(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	asset, err := store.CreateMediaAsset(ctx, domain.MediaAsset{
		Type:        "image",
		LocalPath:   "/tmp/ludex-cover.jpg",
		OriginalURL: "https://attachments.example.test/cover.jpg",
		Hash:        "cover-hash",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertBrowseCoverMediaAsset(ctx, "f95zone", "https://f95zone.to/threads/example.1/preview", asset.ID); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetBrowseCoverMediaAsset(ctx, "f95zone", "https://f95zone.to/threads/example.1/preview")
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != asset.ID || got.OriginalURL != asset.OriginalURL {
		t.Fatalf("asset = %#v, want id %d url %q", got, asset.ID, asset.OriginalURL)
	}
}

func TestUpsertSourceItemByAdapterKeyReplacesExistingItem(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	first, replaced, err := store.UpsertSourceItemByAdapterKey(ctx, domain.SourceItem{
		SourceType: "f95zone",
		ExternalID: "12345",
		Title:      "First title",
		RawURL:     "https://f95zone.to/threads/first.12345/",
		ParsedJSON: map[string]any{"title": "First title"},
		Status:     "imported",
	})
	if err != nil {
		t.Fatal(err)
	}
	if replaced {
		t.Fatal("first upsert should create a new item")
	}

	game, err := store.CreateGame(ctx, domain.Game{Title: "Matched game"})
	if err != nil {
		t.Fatal(err)
	}
	first, err = store.SetSourceItemMatch(ctx, first.ID, &game.ID)
	if err != nil {
		t.Fatal(err)
	}

	second, replaced, err := store.UpsertSourceItemByAdapterKey(ctx, domain.SourceItem{
		SourceType: "f95zone",
		ExternalID: "12345",
		Title:      "Second title",
		RawURL:     "https://f95zone.to/threads/second.12345/",
		ParsedJSON: map[string]any{"title": "Second title"},
		Status:     "imported",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !replaced {
		t.Fatal("second upsert should replace the existing item")
	}
	if second.ID != first.ID {
		t.Fatalf("item id changed: got %d want %d", second.ID, first.ID)
	}
	if second.Title != "Second title" {
		t.Fatalf("title = %q", second.Title)
	}
	if second.MatchedGameID == nil || *second.MatchedGameID != game.ID {
		t.Fatalf("matched game was not preserved: %#v", second.MatchedGameID)
	}

	items, err := store.ListSourceItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d", len(items))
	}
}

func TestDeduplicateSourceItemsByAdapterKeyMergesExistingDuplicates(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	if _, err := store.db.ExecContext(ctx, `DROP INDEX IF EXISTS idx_source_items_adapter_key`); err != nil {
		t.Fatal(err)
	}

	first, err := store.CreateSourceItem(ctx, domain.SourceItem{
		SourceType: "f95zone",
		ExternalID: "12345",
		Title:      "Duplicate without match",
		RawURL:     "https://f95zone.to/threads/old.12345/",
		ParsedJSON: map[string]any{"title": "old"},
		Status:     "imported",
	})
	if err != nil {
		t.Fatal(err)
	}
	asset, err := store.CreateMediaAsset(ctx, domain.MediaAsset{
		SourceItemID: &first.ID,
		Type:         "image",
		LocalPath:    "media/old.jpg",
		OriginalURL:  "https://example.test/old.jpg",
	})
	if err != nil {
		t.Fatal(err)
	}
	game, err := store.CreateGame(ctx, domain.Game{Title: "Matched game"})
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.CreateSourceItem(ctx, domain.SourceItem{
		SourceType:    "f95zone",
		ExternalID:    "12345",
		Title:         "Duplicate with match",
		RawURL:        "https://f95zone.to/threads/new.12345/",
		ParsedJSON:    map[string]any{"title": "new"},
		MatchedGameID: &game.ID,
		Status:        "matched",
	})
	if err != nil {
		t.Fatal(err)
	}

	removed, err := store.deduplicateSourceItemsByAdapterKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
	items, err := store.ListSourceItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("item count = %d", len(items))
	}
	if items[0].ID != second.ID {
		t.Fatalf("kept item id = %d, want %d", items[0].ID, second.ID)
	}
	if items[0].MatchedGameID == nil || *items[0].MatchedGameID != game.ID {
		t.Fatalf("matched game = %#v", items[0].MatchedGameID)
	}
	asset, err = store.GetMediaAsset(ctx, asset.ID)
	if err != nil {
		t.Fatal(err)
	}
	if asset.SourceItemID == nil || *asset.SourceItemID != second.ID {
		t.Fatalf("asset source item = %#v, want %d", asset.SourceItemID, second.ID)
	}
	if asset.GameID == nil || *asset.GameID != game.ID {
		t.Fatalf("asset game = %#v, want %d", asset.GameID, game.ID)
	}

	if _, err := store.db.ExecContext(ctx, `
CREATE UNIQUE INDEX IF NOT EXISTS idx_source_items_adapter_key ON source_items(source_type, external_id)
WHERE source_type != '' AND external_id != ''`); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateSourceItem(ctx, domain.SourceItem{
		SourceType: "f95zone",
		ExternalID: "12345",
		Title:      "Should fail",
		ParsedJSON: map[string]any{"title": "fail"},
	}); err == nil {
		t.Fatal("expected unique adapter key to reject duplicate source item")
	}
}

func TestDeduplicateGamesByTitleKeepsLinkedGame(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	emptyDuplicate, err := store.CreateGame(ctx, domain.Game{Title: "Ripples", CurrentVersion: "0.9.21"})
	if err != nil {
		t.Fatal(err)
	}
	linkedDuplicate, err := store.CreateGame(ctx, domain.Game{Title: "Ripples", CurrentVersion: "0.9.22"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateSourceItem(ctx, domain.SourceItem{
		SourceType:    "telegram",
		ExternalID:    "group:1",
		Title:         "Ripples",
		ParsedJSON:    map[string]any{"title": "Ripples"},
		MatchedGameID: &linkedDuplicate.ID,
		Status:        "matched",
	})
	if err != nil {
		t.Fatal(err)
	}

	removed, err := store.deduplicateGamesByTitle(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if removed != 1 {
		t.Fatalf("removed = %d", removed)
	}
	if _, err := store.GetGame(ctx, emptyDuplicate.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("empty duplicate should be removed, err = %v", err)
	}
	kept, err := store.GetGame(ctx, linkedDuplicate.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kept.Title != "Ripples" {
		t.Fatalf("kept title = %q", kept.Title)
	}
	item, err = store.GetSourceItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.MatchedGameID == nil || *item.MatchedGameID != linkedDuplicate.ID {
		t.Fatalf("matched game id = %#v", item.MatchedGameID)
	}
}

func TestAuthProfileStoresMetadataWithoutJSONCookieValues(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	profile, err := store.UpsertAuthProfile(ctx, domain.AuthProfile{
		AdapterID:       "f95zone",
		Domain:          "f95zone.to",
		CookieHeader:    "xf_user=secret-user-cookie; xf_csrf=secret-csrf",
		CookieCount:     2,
		CookieExpiresAt: "2026-06-03T00:00:00Z",
		Username:        "demo-user",
		UserAgent:       "Ludex Test UA",
		SourceURL:       "https://f95zone.to/account/",
		Cookies: []domain.AuthCookie{
			{
				Name:      "xf_user",
				Domain:    "f95zone.to",
				Path:      "/",
				ExpiresAt: "2026-06-03T00:00:00Z",
				Secure:    true,
				HTTPOnly:  true,
			},
			{
				Name:    "xf_csrf",
				Domain:  "f95zone.to",
				Path:    "/",
				Session: true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if profile.Username != "demo-user" || profile.CookieExpiresAt == "" || len(profile.Cookies) != 2 {
		t.Fatalf("profile metadata = %#v", profile)
	}
	payload, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "secret-user-cookie") || strings.Contains(string(payload), "secret-csrf") {
		t.Fatalf("json leaked cookie value: %s", payload)
	}
	if !strings.Contains(string(payload), "xf_user") {
		t.Fatalf("json missing cookie metadata: %s", payload)
	}
}

func TestActiveTaskDedupeKeyOnlyBlocksQueuedAndRunning(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	first, err := store.CreateTask(ctx, domain.Task{
		Kind:      "import:f95zone",
		DedupeKey: "f95zone:12345",
		Status:    "queued",
		Title:     "First import",
		Message:   "Queued",
	})
	if err != nil {
		t.Fatal(err)
	}
	active, err := store.GetActiveTaskByDedupeKey(ctx, "import:f95zone", "f95zone:12345")
	if err != nil {
		t.Fatal(err)
	}
	if active.ID != first.ID {
		t.Fatalf("active task id = %d, want %d", active.ID, first.ID)
	}
	if _, err := store.CreateTask(ctx, domain.Task{
		Kind:      "import:f95zone",
		DedupeKey: "f95zone:12345",
		Status:    "queued",
		Title:     "Duplicate import",
		Message:   "Queued",
	}); err == nil {
		t.Fatal("expected active duplicate task create to fail")
	}
	if _, err := store.FinishTask(ctx, first.ID, "Done", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetActiveTaskByDedupeKey(ctx, "import:f95zone", "f95zone:12345"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("active lookup after finish err = %v", err)
	}
	if _, err := store.CreateTask(ctx, domain.Task{
		Kind:      "import:f95zone",
		DedupeKey: "f95zone:12345",
		Status:    "queued",
		Title:     "Second import",
		Message:   "Queued",
	}); err != nil {
		t.Fatal(err)
	}
}

func TestMarkTaskRetriedStoresReplacementTaskID(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	failed, err := store.CreateTask(ctx, domain.Task{
		Kind:       "import:f95zone",
		DedupeKey:  "f95zone:12345",
		Status:     "failed",
		Title:      "Failed import",
		Message:    "Import failed",
		ResultJSON: map[string]any{"import_request": map[string]any{"url": "https://f95zone.to/threads/example.12345/"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	retry, err := store.CreateTask(ctx, domain.Task{
		Kind:      "import:f95zone",
		DedupeKey: "f95zone:12345",
		Status:    "queued",
		Title:     "Retry import",
		Message:   "Queued",
	})
	if err != nil {
		t.Fatal(err)
	}

	marked, err := store.MarkTaskRetried(ctx, failed.ID, retry.ID)
	if err != nil {
		t.Fatal(err)
	}
	if marked.ResultJSON["retried_by_task_id"] != float64(retry.ID) {
		t.Fatalf("retried_by_task_id = %#v", marked.ResultJSON["retried_by_task_id"])
	}
	if marked.ResultJSON["import_request"] == nil {
		t.Fatalf("import request was not preserved: %#v", marked.ResultJSON)
	}
}

func TestDeleteGameDetachesSourceItemsAndDeletesOnlyGameOwnedMedia(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	game, err := store.CreateGame(ctx, domain.Game{Title: "Delete me"})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.CreateSourceItem(ctx, domain.SourceItem{
		SourceType:    "f95zone",
		ExternalID:    "delete-game",
		Title:         "Source record",
		ParsedJSON:    map[string]any{"title": "Source record"},
		MatchedGameID: &game.ID,
		Status:        "matched",
	})
	if err != nil {
		t.Fatal(err)
	}
	shared, err := store.CreateMediaAsset(ctx, domain.MediaAsset{
		GameID:       &game.ID,
		SourceItemID: &item.ID,
		Type:         "image",
		LocalPath:    "/tmp/shared-media.png",
	})
	if err != nil {
		t.Fatal(err)
	}
	gameOnlyPath := "/tmp/game-only-media.png"
	gameOnly, err := store.CreateMediaAsset(ctx, domain.MediaAsset{
		GameID:    &game.ID,
		Type:      "image",
		LocalPath: gameOnlyPath,
	})
	if err != nil {
		t.Fatal(err)
	}

	paths, err := store.DeleteGame(ctx, game.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !containsString(paths, gameOnlyPath) {
		t.Fatalf("deleted paths = %#v, want %q", paths, gameOnlyPath)
	}
	if containsString(paths, shared.LocalPath) {
		t.Fatalf("shared media should remain referenced by item: %#v", paths)
	}
	if _, err := store.GetGame(ctx, game.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted game err = %v", err)
	}
	item, err = store.GetSourceItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.MatchedGameID != nil || item.Status != "imported" {
		t.Fatalf("item match = %#v status = %q", item.MatchedGameID, item.Status)
	}
	shared, err = store.GetMediaAsset(ctx, shared.ID)
	if err != nil {
		t.Fatal(err)
	}
	if shared.GameID != nil || shared.SourceItemID == nil || *shared.SourceItemID != item.ID {
		t.Fatalf("shared media refs = game %#v item %#v", shared.GameID, shared.SourceItemID)
	}
	if _, err := store.GetMediaAsset(ctx, gameOnly.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted game-only media err = %v", err)
	}
}

func TestDeleteSourceItemDeletesRawAndOnlyItemOwnedMedia(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	ctx := context.Background()
	game, err := store.CreateGame(ctx, domain.Game{Title: "Keep me"})
	if err != nil {
		t.Fatal(err)
	}
	rawPath := "/tmp/source-item.html"
	item, err := store.CreateSourceItem(ctx, domain.SourceItem{
		SourceType:     "f95zone",
		ExternalID:     "delete-item",
		Title:          "Delete source record",
		RawContentPath: rawPath,
		ParsedJSON:     map[string]any{"title": "Delete source record"},
		MatchedGameID:  &game.ID,
		Status:         "matched",
	})
	if err != nil {
		t.Fatal(err)
	}
	itemOnlyPath := "/tmp/item-only-media.png"
	itemOnly, err := store.CreateMediaAsset(ctx, domain.MediaAsset{
		SourceItemID: &item.ID,
		Type:         "image",
		LocalPath:    itemOnlyPath,
	})
	if err != nil {
		t.Fatal(err)
	}
	shared, err := store.CreateMediaAsset(ctx, domain.MediaAsset{
		GameID:       &game.ID,
		SourceItemID: &item.ID,
		Type:         "image",
		LocalPath:    "/tmp/game-owned-media.png",
	})
	if err != nil {
		t.Fatal(err)
	}

	paths, err := store.DeleteSourceItem(ctx, item.ID)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{rawPath, itemOnlyPath} {
		if !containsString(paths, want) {
			t.Fatalf("deleted paths = %#v, want %q", paths, want)
		}
	}
	if containsString(paths, shared.LocalPath) {
		t.Fatalf("game-owned media should remain referenced by game: %#v", paths)
	}
	if _, err := store.GetSourceItem(ctx, item.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted item err = %v", err)
	}
	if _, err := store.GetMediaAsset(ctx, itemOnly.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("get deleted item-only media err = %v", err)
	}
	shared, err = store.GetMediaAsset(ctx, shared.ID)
	if err != nil {
		t.Fatal(err)
	}
	if shared.SourceItemID != nil || shared.GameID == nil || *shared.GameID != game.ID {
		t.Fatalf("shared media refs = game %#v item %#v", shared.GameID, shared.SourceItemID)
	}
}

func TestDeleteSourceItemMissingIsIdempotent(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	paths, err := store.DeleteSourceItem(context.Background(), 404)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 0 {
		t.Fatalf("deleted paths = %#v", paths)
	}
}
