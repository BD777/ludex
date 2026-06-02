package httpapi

import (
	"testing"

	"local/ludex/internal/domain"
)

func TestImportF95zoneRequestFromLegacyTaskTitle(t *testing.T) {
	task := domain.Task{
		Kind:   "import:f95zone",
		Status: "failed",
		Title:  "Import F95zone: https://f95zone.to/threads/example-game.12345/unread",
	}

	req, err := importF95zoneRequestFromTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL != "https://f95zone.to/threads/example-game.12345/unread" {
		t.Fatalf("url = %q", req.URL)
	}
	if !req.CreateGame {
		t.Fatal("create game should default to true")
	}
}

func TestImportF95zoneRequestFromTaskMetadata(t *testing.T) {
	task := domain.Task{
		Kind:   "import:f95zone",
		Status: "failed",
		Title:  "Import F95zone: https://f95zone.to/threads/example-game.12345/",
		ResultJSON: map[string]any{
			"import_request": map[string]any{
				"url":         "https://f95zone.to/threads/newer-game.67890/",
				"proxy_url":   "socks5://127.0.0.1:7890",
				"create_game": false,
			},
		},
	}

	req, err := importF95zoneRequestFromTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if req.URL != "https://f95zone.to/threads/newer-game.67890/" {
		t.Fatalf("url = %q", req.URL)
	}
	if req.ProxyURL != "socks5://127.0.0.1:7890" {
		t.Fatalf("proxy url = %q", req.ProxyURL)
	}
	if req.CreateGame {
		t.Fatal("create game should come from metadata")
	}
}
