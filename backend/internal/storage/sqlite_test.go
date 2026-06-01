package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"local/ludex/internal/domain"
)

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
