package httpapi

import (
	"context"
	"sync"
	"testing"

	"local/ludex/internal/domain"
	telegramsource "local/ludex/internal/source/telegram"
	"local/ludex/internal/storage"
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

func TestCreateGameFromTranscriptReusesExistingTitle(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := &Server{store: store}
	ctx := context.Background()

	first, err := server.createGameFromTranscript(ctx, domain.Transcript{
		Title: "Ripples",
		Inferred: domain.TranscriptInferred{
			GameTitle: "Ripples",
			Version:   "0.9.22",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	second, err := server.createGameFromTranscript(ctx, domain.Transcript{
		Title: "Ripples",
		Inferred: domain.TranscriptInferred{
			GameTitle: "Ripples",
			Version:   "0.9.23",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected reused game id %d, got %d", first.ID, second.ID)
	}
	if second.CurrentVersion != "0.9.23" {
		t.Fatalf("current version = %q", second.CurrentVersion)
	}

	games, err := store.ListGames(ctx, "Ripples")
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("game count = %d", len(games))
	}
}

func TestCreateGameFromTranscriptReusesExistingTitleConcurrently(t *testing.T) {
	store, err := storage.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	server := &Server{store: store}
	ctx := context.Background()
	transcript := domain.Transcript{
		Title: "Ripples",
		Inferred: domain.TranscriptInferred{
			GameTitle: "Ripples",
			Version:   "0.9.22",
		},
	}

	const workers = 8
	var wg sync.WaitGroup
	gameIDs := make(chan int64, workers)
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			game, err := server.createGameFromTranscript(ctx, transcript)
			if err != nil {
				errs <- err
				return
			}
			gameIDs <- game.ID
		}()
	}
	wg.Wait()
	close(gameIDs)
	close(errs)

	for err := range errs {
		t.Fatal(err)
	}
	var expectedID int64
	for id := range gameIDs {
		if expectedID == 0 {
			expectedID = id
			continue
		}
		if id != expectedID {
			t.Fatalf("expected game id %d, got %d", expectedID, id)
		}
	}

	games, err := store.ListGames(ctx, "Ripples")
	if err != nil {
		t.Fatal(err)
	}
	if len(games) != 1 {
		t.Fatalf("game count = %d", len(games))
	}
}

func TestTelegramAdapterListItemRequiresTitleAndCover(t *testing.T) {
	source := domain.Source{Name: "Party", URL: "https://t.me/party"}
	media := []telegramsource.Media{{URL: "telegram://media?peer_id=1&message_id=10"}}

	item, ok := telegramAdapterListItem(source, telegramsource.Message{
		PeerID: "1",
		ID:     10,
		Text:   "#Game Name v1.0\n·游戏介绍·\nHello",
		URL:    "https://t.me/party/10",
		Media:  media,
	})
	if !ok {
		t.Fatal("expected title and cover message to be importable")
	}
	if item.Title != "Game Name" {
		t.Fatalf("title = %q", item.Title)
	}
	if item.PreviewURL != media[0].URL {
		t.Fatalf("preview url = %q", item.PreviewURL)
	}

	if _, ok := telegramAdapterListItem(source, telegramsource.Message{
		PeerID: "1",
		ID:     11,
		Text:   "https://example.com/file.zip",
		URL:    "https://t.me/party/11",
		Media:  media,
	}); ok {
		t.Fatal("message without a recognizable title should be filtered")
	}

	if _, ok := telegramAdapterListItem(source, telegramsource.Message{
		PeerID: "1",
		ID:     12,
		Text:   "#Game Name v1.0\n·游戏介绍·\nHello",
		URL:    "https://t.me/party/12",
	}); ok {
		t.Fatal("message without a cover should be filtered")
	}
}

func TestTelegramDownloadGroupsUseMessageLink(t *testing.T) {
	groups := telegramDownloadGroups(telegramsource.Message{URL: "https://t.me/party/10"})
	if len(groups) != 1 || groups[0].Platform != "Telegram" {
		t.Fatalf("download groups = %#v", groups)
	}
	if len(groups[0].Links) != 1 || groups[0].Links[0].URL != "https://t.me/party/10" {
		t.Fatalf("download links = %#v", groups[0].Links)
	}
}

func TestTelegramVersionExtractsInlineAndLabeledFormats(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{
			name: "title next line without spaces",
			text: "#隧道逃生：终极版\nV0.22.0A官方中文版【2026年06月02日 更新】",
			want: "V0.22.0A",
		},
		{
			name: "inline title version",
			text: "#午夜罪孽 #Midnight Sin v1.0.0.1s 官方中文版【2026年06月02日 更新】",
			want: "v1.0.0.1s",
		},
		{
			name: "date-like version",
			text: "#潜入治安官 #Hidden Badge v20260601 官方中文版【2026年06月01日 更新】",
			want: "v20260601",
		},
		{
			name: "labeled numeric version",
			text: "版本：0.87.0 官方中文版",
			want: "0.87.0",
		},
		{
			name: "release date is not version",
			text: "#徘徊病院 日文生肉版【2026年05月21日 发售】",
			want: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := telegramVersion(tc.text); got != tc.want {
				t.Fatalf("telegramVersion() = %q, want %q", got, tc.want)
			}
		})
	}
}
