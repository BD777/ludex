package httpapi

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"golang.org/x/net/proxy"
	"local/ludex/internal/domain"
	"local/ludex/internal/source/f95zone"
	"local/ludex/internal/storage"
)

//go:embed extensions/ludex-browser-bridge/*
var extensionFiles embed.FS

type Server struct {
	store *storage.Store
}

type progressReporter func(current int, total int, message string)

type importF95zoneResult struct {
	Item       domain.SourceItem `json:"item"`
	Game       *domain.Game      `json:"game"`
	Transcript domain.Transcript `json:"transcript"`
}

const (
	maxCachedImages = 32
	maxHTMLBytes    = 20 << 20
	maxMediaBytes   = 25 << 20

	defaultFetchTimeout = 45 * time.Second
	mediaFetchTimeout   = 90 * time.Second
	mediaFetchAttempts  = 3
)

func New(store *storage.Store) http.Handler {
	server := &Server{store: store}
	r := chi.NewRouter()

	r.Get("/api/health", server.health)
	r.Get("/extensions/ludex-browser-bridge.zip", server.serveBrowserExtensionZip)
	r.Head("/extensions/ludex-browser-bridge.zip", server.serveBrowserExtensionZip)
	r.Get("/api/tasks", server.listTasks)
	r.Get("/api/tasks/{taskID}", server.getTask)

	r.Get("/api/games", server.listGames)
	r.Post("/api/games", server.createGame)
	r.Get("/api/games/{gameID}", server.getGame)
	r.Patch("/api/games/{gameID}", server.updateGame)
	r.Delete("/api/games/{gameID}", server.deleteGame)

	r.Get("/api/sources", server.listSources)
	r.Post("/api/sources", server.createSource)
	r.Post("/api/sources/{sourceID}/fetch", server.fetchSource)
	r.Get("/api/auth-profiles", server.listAuthProfiles)
	r.Post("/api/auth-profiles/import", server.importAuthProfile)

	r.Get("/api/source-items", server.listSourceItems)
	r.Get("/api/source-items/{itemID}/raw", server.serveSourceItemRaw)
	r.Head("/api/source-items/{itemID}/raw", server.serveSourceItemRaw)
	r.Delete("/api/source-items/{itemID}", server.deleteSourceItem)
	r.Post("/api/source-items/{itemID}/retry-media", server.retrySourceItemMedia)
	r.Post("/api/source-items/{itemID}/create-game", server.createGameFromSourceItem)
	r.Post("/api/source-items/{itemID}/match", server.matchSourceItem)

	r.Get("/api/media/{mediaID}/content", server.serveMedia)
	r.Head("/api/media/{mediaID}/content", server.serveMedia)

	r.Post("/api/import/f95zone", server.importF95zone)

	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"dataDir": s.store.DataDir(),
	})
}

func (s *Server) serveBrowserExtensionZip(w http.ResponseWriter, r *http.Request) {
	zipBytes, err := browserExtensionZip()
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Disposition", `attachment; filename="ludex-browser-bridge.zip"`)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(zipBytes)))
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(zipBytes)
}

func browserExtensionZip() ([]byte, error) {
	const root = "extensions/ludex-browser-bridge"
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	err := fs.WalkDir(extensionFiles, root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, ok := strings.CutPrefix(path, root+"/")
		if !ok || rel == "" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = rel
		header.Method = zip.Deflate
		writer, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		data, err := extensionFiles.ReadFile(path)
		if err != nil {
			return err
		}
		_, err = writer.Write(data)
		return err
	})
	if err != nil {
		_ = zw.Close()
		return nil, err
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := s.store.ListTasks(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	taskID, err := urlParamInt(r, "taskID")
	if err != nil {
		writeError(w, err)
		return
	}
	task, err := s.store.GetTask(r.Context(), taskID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

func (s *Server) listGames(w http.ResponseWriter, r *http.Request) {
	games, err := s.store.ListGames(r.Context(), r.URL.Query().Get("q"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, games)
}

func (s *Server) createGame(w http.ResponseWriter, r *http.Request) {
	var input domain.Game
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	game, err := s.store.CreateGame(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, game)
}

func (s *Server) getGame(w http.ResponseWriter, r *http.Request) {
	gameID, err := urlParamInt(r, "gameID")
	if err != nil {
		writeError(w, err)
		return
	}
	game, err := s.store.GetGame(r.Context(), gameID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, game)
}

func (s *Server) updateGame(w http.ResponseWriter, r *http.Request) {
	gameID, err := urlParamInt(r, "gameID")
	if err != nil {
		writeError(w, err)
		return
	}
	var input domain.Game
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	game, err := s.store.UpdateGame(r.Context(), gameID, input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, game)
}

func (s *Server) deleteGame(w http.ResponseWriter, r *http.Request) {
	gameID, err := urlParamInt(r, "gameID")
	if err != nil {
		writeError(w, err)
		return
	}
	paths, err := s.store.DeleteGame(r.Context(), gameID)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.deleteDataFiles(paths); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
	})
}

func (s *Server) listSources(w http.ResponseWriter, r *http.Request) {
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sources)
}

func (s *Server) createSource(w http.ResponseWriter, r *http.Request) {
	var input domain.Source
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	source, err := s.store.CreateSource(r.Context(), input)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, source)
}

func (s *Server) listAuthProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := s.store.ListAuthProfiles(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profiles)
}

func (s *Server) importAuthProfile(w http.ResponseWriter, r *http.Request) {
	var input importAuthProfileRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	adapterID := strings.TrimSpace(input.AdapterID)
	domainName := normalizeAuthDomain(input.Domain)
	cookieHeader := normalizeCookieHeader(input.CookieHeader)
	if adapterID == "" {
		writeError(w, errors.New("adapter_id is required"))
		return
	}
	if domainName == "" {
		writeError(w, errors.New("domain is required"))
		return
	}
	if cookieHeader == "" {
		writeError(w, errors.New("cookie_header is required"))
		return
	}
	cookies := authCookiesOrHeader(input.Cookies, cookieHeader, domainName)
	profile, err := s.store.UpsertAuthProfile(r.Context(), domain.AuthProfile{
		AdapterID:       adapterID,
		Domain:          domainName,
		CookieHeader:    cookieHeader,
		CookieCount:     maxInt(len(cookies), countCookies(cookieHeader)),
		CookieExpiresAt: firstCookieExpiry(cookies),
		Cookies:         cookies,
		Username:        sanitizeUsername(input.Username),
		UserAgent:       strings.TrimSpace(input.UserAgent),
		SourceURL:       strings.TrimSpace(input.SourceURL),
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, profile)
}

func (s *Server) fetchSource(w http.ResponseWriter, r *http.Request) {
	sourceID, err := urlParamInt(r, "sourceID")
	if err != nil {
		writeError(w, err)
		return
	}
	source, err := s.store.GetSource(r.Context(), sourceID)
	if err != nil {
		writeError(w, err)
		return
	}
	if source.Type != "f95zone" {
		writeError(w, fmt.Errorf("unsupported source type %q", source.Type))
		return
	}
	req := importF95zoneRequest{
		SourceID: &source.ID,
		URL:      source.URL,
		ProxyURL: source.ProxyURL,
	}
	s.queueF95zoneImport(w, r, req)
}

func (s *Server) listSourceItems(w http.ResponseWriter, r *http.Request) {
	items, err := s.store.ListSourceItems(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) deleteSourceItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := urlParamInt(r, "itemID")
	if err != nil {
		writeError(w, err)
		return
	}
	paths, err := s.store.DeleteSourceItem(r.Context(), itemID)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.deleteDataFiles(paths); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": true,
	})
}

func (s *Server) importF95zone(w http.ResponseWriter, r *http.Request) {
	var req importF95zoneRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	s.queueF95zoneImport(w, r, req)
}

func (s *Server) queueF95zoneImport(w http.ResponseWriter, r *http.Request, req importF95zoneRequest) {
	var err error
	req, err = s.resolveF95zoneImportRequest(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.SourceID == nil && strings.TrimSpace(req.URL) == "" && strings.TrimSpace(req.HTML) == "" {
		writeError(w, errors.New("source_id, url, or html is required"))
		return
	}
	dedupeKey := f95zoneTaskDedupeKey(req)
	if dedupeKey != "" {
		existing, err := s.store.GetActiveTaskByDedupeKey(r.Context(), "import:f95zone", dedupeKey)
		if err == nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"task":      existing,
				"duplicate": true,
			})
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			writeError(w, err)
			return
		}
	}
	task, err := s.store.CreateTask(r.Context(), domain.Task{
		Kind:      "import:f95zone",
		DedupeKey: dedupeKey,
		Status:    "queued",
		Title:     importTaskTitle(req),
		Message:   "Queued",
	})
	if err != nil {
		if dedupeKey != "" {
			existing, lookupErr := s.store.GetActiveTaskByDedupeKey(r.Context(), "import:f95zone", dedupeKey)
			if lookupErr == nil {
				writeJSON(w, http.StatusOK, map[string]any{
					"task":      existing,
					"duplicate": true,
				})
				return
			}
		}
		writeError(w, err)
		return
	}
	go s.runF95zoneImportTask(task.ID, req)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"task":      task,
		"duplicate": false,
	})
}

func (s *Server) resolveF95zoneImportRequest(ctx context.Context, req importF95zoneRequest) (importF95zoneRequest, error) {
	if req.SourceID == nil {
		return req, nil
	}
	source, err := s.store.GetSource(ctx, *req.SourceID)
	if err != nil {
		return req, err
	}
	if source.Type != "f95zone" {
		return req, fmt.Errorf("unsupported source type %q", source.Type)
	}
	if req.URL == "" {
		req.URL = source.URL
	}
	if req.ProxyURL == "" {
		req.ProxyURL = source.ProxyURL
	}
	return req, nil
}

func (s *Server) runF95zoneImportTask(taskID int64, req importF95zoneRequest) {
	ctx := context.Background()
	_, _ = s.store.StartTask(ctx, taskID, "Starting import")
	report := func(current int, total int, message string) {
		_, _ = s.store.UpdateTaskProgress(ctx, taskID, current, total, message)
	}

	result, err := s.executeF95zoneImport(ctx, req, report)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Import failed", err)
		return
	}
	resultMap, err := structToMap(result)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Import finished but result could not be stored", err)
		return
	}
	_, _ = s.store.FinishTask(ctx, taskID, "Import complete", resultMap)
}

func (s *Server) executeF95zoneImport(ctx context.Context, req importF95zoneRequest, report progressReporter) (importF95zoneResult, error) {
	report(0, 0, "Resolving source")
	if req.SourceID != nil {
		source, err := s.store.GetSource(ctx, *req.SourceID)
		if err != nil {
			return importF95zoneResult{}, err
		}
		if req.URL == "" {
			req.URL = source.URL
		}
		if req.ProxyURL == "" {
			req.ProxyURL = source.ProxyURL
		}
	}
	authProfile, err := s.authProfileForURL(ctx, "f95zone", req.URL)
	if err != nil {
		return importF95zoneResult{}, err
	}

	rawHTML := []byte(req.HTML)
	if len(rawHTML) == 0 {
		if req.URL == "" {
			return importF95zoneResult{}, errors.New("url or html is required")
		}
		report(1, 0, "Fetching thread HTML")
		body, err := fetchURL(ctx, req.URL, req.ProxyURL, authProfile)
		if err != nil {
			return importF95zoneResult{}, err
		}
		rawHTML = body
	} else {
		report(1, 0, "Using pasted HTML")
	}

	report(2, 0, "Parsing F95zone thread")
	transcript, err := f95zone.ParseHTML(req.URL, bytes.NewReader(rawHTML))
	if err != nil {
		return importF95zoneResult{}, err
	}
	parsed, err := structToMap(transcript)
	if err != nil {
		return importF95zoneResult{}, err
	}
	total := 6 + minInt(len(transcript.Images), maxCachedImages)

	report(3, total, "Saving raw HTML")
	rawPath, err := s.saveRawContent("f95zone", req.URL, rawHTML)
	if err != nil {
		return importF95zoneResult{}, err
	}

	report(4, total, "Saving source item")
	itemTitle := firstNonEmpty(transcript.Inferred.GameTitle, transcript.Title, "Untitled F95zone thread")
	item, replaced, err := s.store.UpsertSourceItemByAdapterKey(ctx, domain.SourceItem{
		SourceID:       req.SourceID,
		SourceType:     "f95zone",
		ExternalID:     transcript.ExternalID,
		Title:          itemTitle,
		RawURL:         req.URL,
		RawContentPath: rawPath,
		ParsedJSON:     parsed,
		FetchedAt:      time.Now().UTC().Format(time.RFC3339),
		Status:         "imported",
	})
	if err != nil {
		return importF95zoneResult{}, err
	}
	if replaced {
		report(4, total, "Updating existing source item")
	}
	transcript = s.cacheTranscriptMedia(ctx, transcript, item.ID, req.ProxyURL, authProfile, report, 5, total)
	parsed, err = structToMap(transcript)
	if err != nil {
		return importF95zoneResult{}, err
	}
	item, err = s.store.UpdateSourceItemParsedJSON(ctx, item.ID, parsed)
	if err != nil {
		return importF95zoneResult{}, err
	}

	var game *domain.Game
	if req.CreateGame {
		report(total-1, total, "Saving game record")
		created, err := s.saveGameFromTranscript(ctx, transcript, item.MatchedGameID)
		if err != nil {
			return importF95zoneResult{}, err
		}
		matched, err := s.store.SetSourceItemMatch(ctx, item.ID, &created.ID)
		if err != nil {
			return importF95zoneResult{}, err
		}
		if err := s.store.SetMediaAssetsGameForSourceItem(ctx, item.ID, created.ID); err != nil {
			return importF95zoneResult{}, err
		}
		item = matched
		game = &created
	} else {
		if item.MatchedGameID != nil {
			if err := s.store.SetMediaAssetsGameForSourceItem(ctx, item.ID, *item.MatchedGameID); err != nil {
				return importF95zoneResult{}, err
			}
		}
		report(total-1, total, "Finalizing import")
	}

	report(total, total, "Import complete")
	return importF95zoneResult{Item: item, Game: game, Transcript: transcript}, nil
}

func (s *Server) createGameFromSourceItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := urlParamInt(r, "itemID")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.store.GetSourceItem(r.Context(), itemID)
	if err != nil {
		writeError(w, err)
		return
	}
	transcript, err := transcriptFromMap(item.ParsedJSON)
	if err != nil {
		writeError(w, err)
		return
	}
	game, err := s.createGameFromTranscript(r.Context(), transcript)
	if err != nil {
		writeError(w, err)
		return
	}
	item, err = s.store.SetSourceItemMatch(r.Context(), item.ID, &game.ID)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.store.SetMediaAssetsGameForSourceItem(r.Context(), item.ID, game.ID); err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"game": game,
		"item": item,
	})
}

func (s *Server) matchSourceItem(w http.ResponseWriter, r *http.Request) {
	itemID, err := urlParamInt(r, "itemID")
	if err != nil {
		writeError(w, err)
		return
	}
	var input struct {
		GameID *int64 `json:"game_id"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, err)
		return
	}
	item, err := s.store.SetSourceItemMatch(r.Context(), itemID, input.GameID)
	if err != nil {
		writeError(w, err)
		return
	}
	if input.GameID != nil {
		if err := s.store.SetMediaAssetsGameForSourceItem(r.Context(), item.ID, *input.GameID); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, item)
}

func (s *Server) retrySourceItemMedia(w http.ResponseWriter, r *http.Request) {
	itemID, err := urlParamInt(r, "itemID")
	if err != nil {
		writeError(w, err)
		return
	}
	var req retryMediaRequest
	if r.Body != nil {
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, err)
			return
		}
	}
	item, err := s.store.GetSourceItem(r.Context(), itemID)
	if err != nil {
		writeError(w, err)
		return
	}
	if item.SourceType != "f95zone" {
		writeError(w, fmt.Errorf("unsupported source item type %q", item.SourceType))
		return
	}
	dedupeKey := fmt.Sprintf("source-item:%d:media", item.ID)
	existing, err := s.store.GetActiveTaskByDedupeKey(r.Context(), "retry-media:f95zone", dedupeKey)
	if err == nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"task":      existing,
			"duplicate": true,
		})
		return
	}
	if !errors.Is(err, sql.ErrNoRows) {
		writeError(w, err)
		return
	}
	task, err := s.store.CreateTask(r.Context(), domain.Task{
		Kind:      "retry-media:f95zone",
		DedupeKey: dedupeKey,
		Status:    "queued",
		Title:     retryMediaTaskTitle(item, req.OriginalURL),
		Message:   "Queued",
	})
	if err != nil {
		writeError(w, err)
		return
	}
	go s.runF95zoneRetryMediaTask(task.ID, item.ID, req.ProxyURL, req.OriginalURL)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"task":      task,
		"duplicate": false,
	})
}

func (s *Server) runF95zoneRetryMediaTask(taskID int64, itemID int64, proxyURL string, originalURL string) {
	ctx := context.Background()
	_, _ = s.store.StartTask(ctx, taskID, "Loading raw HTML")
	report := func(current int, total int, message string) {
		_, _ = s.store.UpdateTaskProgress(ctx, taskID, current, total, message)
	}
	item, err := s.store.GetSourceItem(ctx, itemID)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	if item.SourceID != nil && strings.TrimSpace(proxyURL) == "" {
		source, err := s.store.GetSource(ctx, *item.SourceID)
		if err != nil {
			_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
			return
		}
		proxyURL = source.ProxyURL
	}
	authProfile, err := s.authProfileForURL(ctx, "f95zone", item.RawURL)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	if strings.TrimSpace(originalURL) != "" {
		s.runF95zoneSingleMediaRetryTask(ctx, taskID, item, proxyURL, authProfile, originalURL, report)
		return
	}
	rawPath, err := s.safeDataPath(item.RawContentPath)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	rawHTML, err := os.ReadFile(rawPath)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	report(1, 0, "Parsing raw HTML")
	transcript, err := f95zone.ParseHTML(item.RawURL, bytes.NewReader(rawHTML))
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	total := 3 + minInt(len(transcript.Images), maxCachedImages)
	transcript = s.cacheTranscriptMedia(ctx, transcript, item.ID, proxyURL, authProfile, report, 2, total)
	parsed, err := structToMap(transcript)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	item, err = s.store.UpdateSourceItemParsedJSON(ctx, item.ID, parsed)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	if item.MatchedGameID != nil {
		if err := s.store.SetMediaAssetsGameForSourceItem(ctx, item.ID, *item.MatchedGameID); err != nil {
			_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
			return
		}
	}
	report(total, total, "Retry complete")
	result, err := structToMap(importF95zoneResult{Item: item, Transcript: transcript})
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry finished but result could not be stored", err)
		return
	}
	_, _ = s.store.FinishTask(ctx, taskID, "Retry complete", result)
}

func (s *Server) runF95zoneSingleMediaRetryTask(ctx context.Context, taskID int64, item domain.SourceItem, proxyURL string, authProfile *domain.AuthProfile, originalURL string, report progressReporter) {
	report(1, 4, "Loading media state")
	transcript, err := transcriptFromMap(item.ParsedJSON)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	targetIndex := findMediaItemIndex(transcript.MediaItems, originalURL)
	if targetIndex < 0 {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", fmt.Errorf("image is not tracked on this item"))
		return
	}

	target := transcript.MediaItems[targetIndex]
	resolvedURL, err := resolveMediaURL(firstNonEmpty(transcript.SourceURL, item.RawURL), target.OriginalURL)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}

	sourceItemRef := item.ID
	var asset domain.MediaAsset
	if cached, ok := s.findCachedMediaAsset(ctx, item.ID, resolvedURL); ok {
		report(2, 4, "Using cached image")
		asset = cached
	} else {
		report(2, 4, "Downloading image")
		asset, err = s.cacheMediaAsset(ctx, resolvedURL, &sourceItemRef, proxyURL, authProfile)
		if err != nil {
			transcript.MediaItems[targetIndex] = failedMediaItem(target.Position, firstNonEmpty(target.Role, mediaRole(target.Position)), resolvedURL, err)
			transcript = rebuildTranscriptMediaState(transcript)
			if parsed, parseErr := structToMap(transcript); parseErr == nil {
				_, _ = s.store.UpdateSourceItemParsedJSON(ctx, item.ID, parsed)
			}
			_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
			return
		}
		asset.PublicURL = mediaPublicURL(asset.ID)
	}

	report(3, 4, "Updating item")
	transcript.MediaItems[targetIndex] = domain.MediaItem{
		Position:    target.Position,
		Role:        firstNonEmpty(target.Role, mediaRole(target.Position)),
		Status:      "cached",
		OriginalURL: resolvedURL,
		PublicURL:   asset.PublicURL,
	}
	transcript.MediaAssets = upsertTranscriptMediaAsset(transcript.MediaAssets, asset)
	transcript = rebuildTranscriptMediaState(transcript)
	parsed, err := structToMap(transcript)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	item, err = s.store.UpdateSourceItemParsedJSON(ctx, item.ID, parsed)
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
		return
	}
	if item.MatchedGameID != nil {
		if err := s.store.SetMediaAssetsGameForSourceItem(ctx, item.ID, *item.MatchedGameID); err != nil {
			_, _ = s.store.FailTask(ctx, taskID, "Retry failed", err)
			return
		}
		if transcript.MediaItems[targetIndex].Role == "cover" {
			if game, err := s.store.GetGame(ctx, *item.MatchedGameID); err == nil {
				game.CoverImage = asset.PublicURL
				_, _ = s.store.UpdateGame(ctx, game.ID, game)
			}
		}
	}

	report(4, 4, "Retry complete")
	result, err := structToMap(importF95zoneResult{Item: item, Transcript: transcript})
	if err != nil {
		_, _ = s.store.FailTask(ctx, taskID, "Retry finished but result could not be stored", err)
		return
	}
	_, _ = s.store.FinishTask(ctx, taskID, "Retry complete", result)
}

func (s *Server) serveSourceItemRaw(w http.ResponseWriter, r *http.Request) {
	itemID, err := urlParamInt(r, "itemID")
	if err != nil {
		writeError(w, err)
		return
	}
	item, err := s.store.GetSourceItem(r.Context(), itemID)
	if err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(item.RawContentPath) == "" {
		writeError(w, errors.New("raw HTML is not saved for this item"))
		return
	}
	localPath, err := s.safeDataPath(item.RawContentPath)
	if err != nil {
		writeError(w, err)
		return
	}
	filename := fmt.Sprintf("source-item-%d.html", item.ID)
	w.Header().Set("Cache-Control", "private, max-age=0")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": filename}))
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, localPath)
}

func (s *Server) serveMedia(w http.ResponseWriter, r *http.Request) {
	mediaID, err := urlParamInt(r, "mediaID")
	if err != nil {
		writeError(w, err)
		return
	}
	asset, err := s.store.GetMediaAsset(r.Context(), mediaID)
	if err != nil {
		writeError(w, err)
		return
	}
	localPath, err := s.safeDataPath(asset.LocalPath)
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(w, r, localPath)
}

func (s *Server) createGameFromTranscript(ctx context.Context, transcript domain.Transcript) (domain.Game, error) {
	title := firstNonEmpty(transcript.Inferred.GameTitle, transcript.Title)
	if title == "" {
		return domain.Game{}, errors.New("cannot create a game without a title")
	}
	aliases := []string{}
	if transcript.Title != "" && !strings.EqualFold(transcript.Title, title) {
		aliases = append(aliases, transcript.Title)
	}
	return s.store.CreateGame(ctx, domain.Game{
		Title:          title,
		Aliases:        aliases,
		Description:    transcript.Inferred.Description,
		CurrentVersion: transcript.Inferred.Version,
		CoverImage:     transcript.Inferred.CoverImage,
	})
}

func (s *Server) saveGameFromTranscript(ctx context.Context, transcript domain.Transcript, existingGameID *int64) (domain.Game, error) {
	if existingGameID == nil {
		return s.createGameFromTranscript(ctx, transcript)
	}
	existing, err := s.store.GetGame(ctx, *existingGameID)
	if err != nil {
		return domain.Game{}, err
	}
	title := firstNonEmpty(transcript.Inferred.GameTitle, transcript.Title, existing.Title)
	if title == "" {
		return domain.Game{}, errors.New("cannot update a game without a title")
	}
	aliases := append([]string{}, existing.Aliases...)
	if transcript.Title != "" && !strings.EqualFold(transcript.Title, title) && !containsFold(aliases, transcript.Title) {
		aliases = append(aliases, transcript.Title)
	}
	return s.store.UpdateGame(ctx, existing.ID, domain.Game{
		Title:          title,
		Aliases:        aliases,
		Description:    firstNonEmpty(transcript.Inferred.Description, existing.Description),
		CurrentVersion: firstNonEmpty(transcript.Inferred.Version, existing.CurrentVersion),
		CoverImage:     firstNonEmpty(transcript.Inferred.CoverImage, existing.CoverImage),
	})
}

func (s *Server) saveRawContent(sourceType string, rawURL string, content []byte) (string, error) {
	dir := filepath.Join(s.store.DataDir(), "raw", sourceType)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	hash := sha256.Sum256([]byte(rawURL + "\n" + time.Now().UTC().Format(time.RFC3339Nano)))
	name := hex.EncodeToString(hash[:])[:16] + ".html"
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path, nil
	}
	return abs, nil
}

func (s *Server) cacheTranscriptMedia(ctx context.Context, transcript domain.Transcript, sourceItemID int64, proxyURL string, authProfile *domain.AuthProfile, report progressReporter, startProgress int, totalProgress int) domain.Transcript {
	if len(transcript.Images) == 0 {
		return transcript
	}

	sourceItemRef := sourceItemID
	transcript.MediaItems = []domain.MediaItem{}
	transcript.MediaFailures = []domain.MediaFailure{}
	transcript.MediaAssets = []domain.MediaAsset{}
	cachedAssetsByOriginal := map[string]domain.MediaAsset{}
	if existingAssets, err := s.store.ListMediaAssetsForSourceItem(ctx, sourceItemID); err == nil {
		for _, asset := range existingAssets {
			if strings.TrimSpace(asset.OriginalURL) == "" {
				continue
			}
			if safePath, err := s.safeDataPath(asset.LocalPath); err == nil {
				if _, statErr := os.Stat(safePath); statErr == nil {
					asset.PublicURL = mediaPublicURL(asset.ID)
					cachedAssetsByOriginal[asset.OriginalURL] = asset
				}
			}
		}
	} else {
		transcript.Warnings = append(transcript.Warnings, fmt.Sprintf("Could not inspect existing cached images: %v", err))
	}
	limit := len(transcript.Images)
	if limit > maxCachedImages {
		limit = maxCachedImages
		transcript.Warnings = append(transcript.Warnings, fmt.Sprintf("Only cached the first %d images.", maxCachedImages))
	}

	for index, imageURL := range transcript.Images[:limit] {
		report(startProgress+index, totalProgress, fmt.Sprintf("Downloading image %d of %d", index+1, limit))
		role := mediaRole(index)
		resolvedURL, err := resolveMediaURL(transcript.SourceURL, imageURL)
		if err != nil {
			transcript.MediaItems = append(transcript.MediaItems, failedMediaItem(index, role, imageURL, err))
			continue
		}
		if cached, ok := cachedAssetsByOriginal[resolvedURL]; ok {
			transcript.MediaAssets = append(transcript.MediaAssets, cached)
			transcript.MediaItems = append(transcript.MediaItems, domain.MediaItem{
				Position:    index,
				Role:        role,
				Status:      "cached",
				OriginalURL: resolvedURL,
				PublicURL:   cached.PublicURL,
			})
			continue
		}
		asset, err := s.cacheMediaAsset(ctx, resolvedURL, &sourceItemRef, proxyURL, authProfile)
		if err != nil {
			transcript.MediaItems = append(transcript.MediaItems, failedMediaItem(index, role, resolvedURL, err))
			continue
		}
		asset.PublicURL = mediaPublicURL(asset.ID)
		transcript.MediaAssets = append(transcript.MediaAssets, asset)
		transcript.MediaItems = append(transcript.MediaItems, domain.MediaItem{
			Position:    index,
			Role:        role,
			Status:      "cached",
			OriginalURL: resolvedURL,
			PublicURL:   asset.PublicURL,
		})
	}
	report(startProgress+limit, totalProgress, "Media cache updated")

	return rebuildTranscriptMediaState(transcript)
}

func mediaRole(position int) string {
	if position == 0 {
		return "cover"
	}
	return "screenshot"
}

func failedMediaItem(position int, role string, originalURL string, err error) domain.MediaItem {
	return domain.MediaItem{
		Position:    position,
		Role:        role,
		Status:      "failed",
		OriginalURL: originalURL,
		Error:       err.Error(),
	}
}

func rebuildTranscriptMediaState(transcript domain.Transcript) domain.Transcript {
	images := []string{}
	screenshots := []string{}
	failures := []domain.MediaFailure{}
	coverImage := ""

	for index := range transcript.MediaItems {
		item := &transcript.MediaItems[index]
		if item.Role == "" {
			item.Role = mediaRole(item.Position)
		}
		if item.Status == "cached" && item.PublicURL != "" {
			images = append(images, item.PublicURL)
			if item.Role == "cover" {
				coverImage = item.PublicURL
			} else {
				screenshots = append(screenshots, item.PublicURL)
			}
			continue
		}
		item.Status = "failed"
		failures = append(failures, domain.MediaFailure{
			Position:    item.Position,
			Role:        item.Role,
			OriginalURL: item.OriginalURL,
			Error:       item.Error,
		})
	}

	transcript.Images = images
	if coverImage == "" && len(images) > 0 {
		coverImage = images[0]
	}
	transcript.Fields.CoverImage = coverImage
	transcript.Fields.Screenshots = screenshots
	transcript.Inferred.CoverImage = coverImage
	transcript.MediaFailures = failures
	transcript.Warnings = mediaWarnings(transcript.Warnings, failures)
	return transcript
}

func mediaWarnings(existing []string, failures []domain.MediaFailure) []string {
	warnings := []string{}
	for _, warning := range existing {
		if isMediaCacheWarning(warning) {
			continue
		}
		warnings = append(warnings, warning)
	}
	if len(failures) > 0 {
		warnings = append(warnings, fmt.Sprintf("%d images failed to cache.", len(failures)))
	}
	return warnings
}

func isMediaCacheWarning(warning string) bool {
	return strings.HasPrefix(warning, "Could not cache image ") ||
		strings.HasPrefix(warning, "Skipped image ") ||
		strings.HasPrefix(warning, "Only cached the first ") ||
		strings.HasPrefix(warning, "Could not inspect existing cached images:") ||
		strings.HasSuffix(warning, " images failed to cache.")
}

func (s *Server) findCachedMediaAsset(ctx context.Context, sourceItemID int64, originalURL string) (domain.MediaAsset, bool) {
	assets, err := s.store.ListMediaAssetsForSourceItem(ctx, sourceItemID)
	if err != nil {
		return domain.MediaAsset{}, false
	}
	for _, asset := range assets {
		if strings.TrimSpace(asset.OriginalURL) != strings.TrimSpace(originalURL) {
			continue
		}
		safePath, err := s.safeDataPath(asset.LocalPath)
		if err != nil {
			continue
		}
		if _, err := os.Stat(safePath); err != nil {
			continue
		}
		asset.PublicURL = mediaPublicURL(asset.ID)
		return asset, true
	}
	return domain.MediaAsset{}, false
}

func (s *Server) cacheMediaAsset(ctx context.Context, rawURL string, sourceItemID *int64, proxyURL string, authProfile *domain.AuthProfile) (domain.MediaAsset, error) {
	body, contentType, err := fetchBytesWithPolicy(ctx, rawURL, proxyURL, "image/avif,image/webp,image/apng,image/svg+xml,image/*,*/*;q=0.8", maxMediaBytes, authProfile, fetchPolicy{
		Timeout:  mediaFetchTimeout,
		Attempts: mediaFetchAttempts,
		Backoff:  []time.Duration{1 * time.Second, 3 * time.Second},
	})
	if err != nil {
		return domain.MediaAsset{}, err
	}
	detectedType := http.DetectContentType(body)
	if contentType == "" || contentType == "application/octet-stream" {
		contentType = detectedType
	}
	if !strings.HasPrefix(contentType, "image/") && !strings.HasPrefix(detectedType, "image/") {
		return domain.MediaAsset{}, fmt.Errorf("downloaded content is %s, not an image", firstNonEmpty(contentType, detectedType))
	}

	sum := sha256.Sum256(body)
	hash := hex.EncodeToString(sum[:])
	ext := mediaExtension(rawURL, contentType, detectedType)
	dir := filepath.Join(s.store.DataDir(), "media", "f95zone", "images", hash[:2])
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return domain.MediaAsset{}, err
	}
	localPath := filepath.Join(dir, hash+ext)
	if _, err := os.Stat(localPath); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(localPath, body, 0o644); err != nil {
			return domain.MediaAsset{}, err
		}
	} else if err != nil {
		return domain.MediaAsset{}, err
	}
	abs, err := filepath.Abs(localPath)
	if err != nil {
		abs = localPath
	}

	return s.store.CreateMediaAsset(ctx, domain.MediaAsset{
		SourceItemID: sourceItemID,
		Type:         "image",
		LocalPath:    abs,
		OriginalURL:  rawURL,
		Hash:         hash,
	})
}

func (s *Server) safeDataPath(localPath string) (string, error) {
	dataDir, err := filepath.Abs(s.store.DataDir())
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(dataDir, absPath)
	if err != nil {
		return "", err
	}
	if rel == "." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("media path is outside data directory")
	}
	return absPath, nil
}

func (s *Server) deleteDataFiles(paths []string) error {
	seen := map[string]bool{}
	for _, path := range paths {
		if strings.TrimSpace(path) == "" || seen[path] {
			continue
		}
		seen[path] = true
		safePath, err := s.safeDataPath(path)
		if err != nil {
			return err
		}
		if err := os.Remove(safePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

type importF95zoneRequest struct {
	SourceID   *int64 `json:"source_id"`
	URL        string `json:"url"`
	HTML       string `json:"html"`
	ProxyURL   string `json:"proxy_url"`
	CreateGame bool   `json:"create_game"`
}

type retryMediaRequest struct {
	ProxyURL    string `json:"proxy_url"`
	OriginalURL string `json:"original_url"`
}

type importAuthProfileRequest struct {
	AdapterID    string                     `json:"adapter_id"`
	Domain       string                     `json:"domain"`
	CookieHeader string                     `json:"cookie_header"`
	Cookies      []authProfileCookieRequest `json:"cookies"`
	Username     string                     `json:"username"`
	UserAgent    string                     `json:"user_agent"`
	SourceURL    string                     `json:"source_url"`
}

type authProfileCookieRequest struct {
	Name      string `json:"name"`
	Domain    string `json:"domain"`
	Path      string `json:"path"`
	ExpiresAt string `json:"expires_at"`
	Session   bool   `json:"session"`
	Secure    bool   `json:"secure"`
	HTTPOnly  bool   `json:"http_only"`
}

func fetchURL(ctx context.Context, rawURL string, proxyURL string, authProfile *domain.AuthProfile) ([]byte, error) {
	body, _, err := fetchBytes(ctx, rawURL, proxyURL, "text/html,application/xhtml+xml", maxHTMLBytes, authProfile)
	return body, err
}

func fetchBytes(ctx context.Context, rawURL string, proxyURL string, accept string, maxBytes int64, authProfile *domain.AuthProfile) ([]byte, string, error) {
	return fetchBytesWithPolicy(ctx, rawURL, proxyURL, accept, maxBytes, authProfile, fetchPolicy{
		Timeout:  defaultFetchTimeout,
		Attempts: 1,
	})
}

type fetchPolicy struct {
	Timeout  time.Duration
	Attempts int
	Backoff  []time.Duration
}

type httpStatusError struct {
	URL        string
	StatusCode int
	Status     string
}

func (e httpStatusError) Error() string {
	return fmt.Sprintf("fetch %s: %s", e.URL, e.Status)
}

func fetchBytesWithPolicy(ctx context.Context, rawURL string, proxyURL string, accept string, maxBytes int64, authProfile *domain.AuthProfile, policy fetchPolicy) ([]byte, string, error) {
	if policy.Timeout <= 0 {
		policy.Timeout = defaultFetchTimeout
	}
	if policy.Attempts <= 0 {
		policy.Attempts = 1
	}
	var lastErr error
	attemptsMade := 0
	for attempt := 1; attempt <= policy.Attempts; attempt++ {
		attemptsMade = attempt
		body, contentType, err := fetchBytesOnce(ctx, rawURL, proxyURL, accept, maxBytes, authProfile, policy.Timeout)
		if err == nil {
			return body, contentType, nil
		}
		lastErr = err
		if attempt == policy.Attempts || !isRetryableFetchError(err) || ctx.Err() != nil {
			break
		}
		backoff := fetchRetryBackoff(policy, attempt)
		if backoff <= 0 {
			continue
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, "", ctx.Err()
		case <-timer.C:
		}
	}
	if attemptsMade > 1 {
		return nil, "", fmt.Errorf("download failed after %d attempts: %w", attemptsMade, lastErr)
	}
	return nil, "", lastErr
}

func fetchRetryBackoff(policy fetchPolicy, attempt int) time.Duration {
	if attempt <= 0 {
		return 0
	}
	if attempt <= len(policy.Backoff) {
		return policy.Backoff[attempt-1]
	}
	return time.Duration(attempt) * time.Second
}

func isRetryableFetchError(err error) bool {
	if err == nil {
		return false
	}
	var statusErr httpStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= 500
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}
	message := strings.ToLower(err.Error())
	retryableFragments := []string{
		"client.timeout",
		"context deadline exceeded",
		"connection reset",
		"broken pipe",
		"unexpected eof",
		"tls handshake timeout",
		"http2: stream closed",
	}
	for _, fragment := range retryableFragments {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

func fetchBytesOnce(ctx context.Context, rawURL string, proxyURL string, accept string, maxBytes int64, authProfile *domain.AuthProfile, timeout time.Duration) ([]byte, string, error) {
	transport := &http.Transport{}
	if proxyURL != "" {
		parsedProxy, err := url.Parse(proxyURL)
		if err != nil {
			return nil, "", fmt.Errorf("parse proxy url: %w", err)
		}
		if strings.HasPrefix(parsedProxy.Scheme, "socks5") {
			dialer, err := socks5Dialer(parsedProxy)
			if err != nil {
				return nil, "", err
			}
			transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
				type contextDialer interface {
					DialContext(context.Context, string, string) (net.Conn, error)
				}
				if d, ok := dialer.(contextDialer); ok {
					return d.DialContext(ctx, network, address)
				}
				return dialer.Dial(network, address)
			}
		} else {
			transport.Proxy = http.ProxyURL(parsedProxy)
		}
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   timeout,
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", err
	}
	userAgent := "Ludex/0.1 (+local)"
	if authProfile != nil && strings.TrimSpace(authProfile.UserAgent) != "" {
		userAgent = strings.TrimSpace(authProfile.UserAgent)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", accept)
	if authProfile != nil && strings.TrimSpace(authProfile.CookieHeader) != "" {
		req.Header.Set("Cookie", normalizeCookieHeader(authProfile.CookieHeader))
	}
	if referer := refererForURL(rawURL); referer != "" {
		req.Header.Set("Referer", referer)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, "", httpStatusError{URL: rawURL, StatusCode: resp.StatusCode, Status: resp.Status}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}
	if int64(len(body)) > maxBytes {
		return nil, "", fmt.Errorf("download exceeds %d bytes", maxBytes)
	}
	contentType := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	return body, contentType, nil
}

func socks5Dialer(proxyURL *url.URL) (proxy.Dialer, error) {
	var auth *proxy.Auth
	if proxyURL.User != nil {
		auth = &proxy.Auth{User: proxyURL.User.Username()}
		if password, ok := proxyURL.User.Password(); ok {
			auth.Password = password
		}
	}
	return proxy.SOCKS5("tcp", proxyURL.Host, auth, proxy.Direct)
}

func mediaPublicURL(id int64) string {
	return fmt.Sprintf("/api/media/%d/content", id)
}

func resolveMediaURL(baseURL string, mediaURL string) (string, error) {
	mediaURL = strings.TrimSpace(mediaURL)
	if mediaURL == "" {
		return "", errors.New("empty media URL")
	}
	if strings.HasPrefix(mediaURL, "/api/media/") {
		return "", errors.New("media is already cached")
	}
	if strings.HasPrefix(mediaURL, "//") {
		if base, err := url.Parse(baseURL); err == nil && base.Scheme != "" {
			return base.Scheme + ":" + mediaURL, nil
		}
		return "https:" + mediaURL, nil
	}
	parsed, err := url.Parse(mediaURL)
	if err != nil {
		return "", err
	}
	if parsed.IsAbs() {
		return parsed.String(), nil
	}
	base, err := url.Parse(baseURL)
	if err != nil || base.Scheme == "" || base.Host == "" {
		return "", fmt.Errorf("relative image URL %q has no usable base URL", mediaURL)
	}
	return base.ResolveReference(parsed).String(), nil
}

func mediaExtension(rawURL string, contentTypes ...string) string {
	for _, contentType := range contentTypes {
		if contentType == "" {
			continue
		}
		extensions, err := mime.ExtensionsByType(contentType)
		if err == nil && len(extensions) > 0 {
			if extensions[0] == ".jpe" {
				return ".jpg"
			}
			return extensions[0]
		}
	}
	if parsed, err := url.Parse(rawURL); err == nil {
		ext := strings.ToLower(filepath.Ext(parsed.Path))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".avif", ".svg":
			return ext
		}
	}
	return ".img"
}

func refererForURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	return parsed.Scheme + "://" + parsed.Host + "/"
}

func importTaskTitle(req importF95zoneRequest) string {
	if req.URL != "" {
		return "Import F95zone: " + req.URL
	}
	if req.SourceID != nil {
		return fmt.Sprintf("Import F95zone source #%d", *req.SourceID)
	}
	return "Import pasted F95zone HTML"
}

func f95zoneTaskDedupeKey(req importF95zoneRequest) string {
	externalID := f95zone.InferExternalID(req.URL)
	if externalID == "" {
		return ""
	}
	return "f95zone:" + externalID
}

func retryMediaTaskTitle(item domain.SourceItem, originalURL string) string {
	title := firstNonEmpty(item.Title, item.ExternalID)
	if strings.TrimSpace(originalURL) != "" {
		return "Retry image: " + title
	}
	return "Retry images: " + title
}

func (s *Server) authProfileForURL(ctx context.Context, adapterID string, rawURL string) (*domain.AuthProfile, error) {
	domainName := authDomainFromURL(rawURL)
	if adapterID == "" || domainName == "" {
		return nil, nil
	}
	profile, err := s.store.GetAuthProfile(ctx, adapterID, domainName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(profile.CookieHeader) == "" {
		return nil, nil
	}
	_ = s.store.TouchAuthProfileUsed(ctx, profile.ID)
	return &profile, nil
}

func authDomainFromURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return normalizeAuthDomain(parsed.Hostname())
}

func normalizeAuthDomain(value string) string {
	domainName := strings.ToLower(strings.TrimSpace(value))
	domainName = strings.TrimPrefix(domainName, ".")
	domainName = strings.TrimPrefix(domainName, "www.")
	return domainName
}

func normalizeCookieHeader(value string) string {
	cookie := strings.TrimSpace(value)
	if strings.HasPrefix(strings.ToLower(cookie), "cookie:") {
		cookie = strings.TrimSpace(cookie[len("cookie:"):])
	}
	return cookie
}

func authCookiesOrHeader(inputs []authProfileCookieRequest, cookieHeader string, domainName string) []domain.AuthCookie {
	cookies := sanitizeAuthCookies(inputs, domainName)
	if len(cookies) > 0 {
		return cookies
	}
	return authCookiesFromHeader(cookieHeader, domainName)
}

func sanitizeAuthCookies(inputs []authProfileCookieRequest, domainName string) []domain.AuthCookie {
	seen := map[string]bool{}
	cookies := []domain.AuthCookie{}
	for _, input := range inputs {
		name := sanitizeCookieName(input.Name)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		expiresAt := normalizeCookieExpiry(input.ExpiresAt)
		cookies = append(cookies, domain.AuthCookie{
			Name:      name,
			Domain:    firstNonEmpty(normalizeAuthDomain(input.Domain), domainName),
			Path:      firstNonEmpty(strings.TrimSpace(input.Path), "/"),
			ExpiresAt: expiresAt,
			Session:   input.Session || expiresAt == "",
			Secure:    input.Secure,
			HTTPOnly:  input.HTTPOnly,
		})
	}
	return cookies
}

func authCookiesFromHeader(cookieHeader string, domainName string) []domain.AuthCookie {
	seen := map[string]bool{}
	cookies := []domain.AuthCookie{}
	for _, part := range strings.Split(cookieHeader, ";") {
		pair := strings.TrimSpace(part)
		if pair == "" {
			continue
		}
		name, _, ok := strings.Cut(pair, "=")
		name = sanitizeCookieName(name)
		if !ok || name == "" || seen[name] {
			continue
		}
		seen[name] = true
		cookies = append(cookies, domain.AuthCookie{
			Name:    name,
			Domain:  domainName,
			Path:    "/",
			Session: true,
		})
	}
	return cookies
}

func sanitizeCookieName(value string) string {
	name := strings.TrimSpace(value)
	if len(name) > 120 {
		name = name[:120]
	}
	return name
}

func sanitizeUsername(value string) string {
	username := strings.TrimSpace(value)
	if len(username) > 120 {
		username = username[:120]
	}
	return username
}

func normalizeCookieExpiry(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return ""
	}
	return parsed.UTC().Format(time.RFC3339)
}

func firstCookieExpiry(cookies []domain.AuthCookie) string {
	var first time.Time
	for _, cookie := range cookies {
		if cookie.ExpiresAt == "" {
			continue
		}
		parsed, err := time.Parse(time.RFC3339, cookie.ExpiresAt)
		if err != nil {
			continue
		}
		if first.IsZero() || parsed.Before(first) {
			first = parsed
		}
	}
	if first.IsZero() {
		return ""
	}
	return first.UTC().Format(time.RFC3339)
}

func countCookies(cookieHeader string) int {
	count := 0
	for _, part := range strings.Split(cookieHeader, ";") {
		if strings.Contains(strings.TrimSpace(part), "=") {
			count++
		}
	}
	return count
}

func maxInt(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func findMediaItemIndex(items []domain.MediaItem, originalURL string) int {
	needle := strings.TrimSpace(originalURL)
	for index, item := range items {
		if strings.TrimSpace(item.OriginalURL) == needle {
			return index
		}
	}
	return -1
}

func upsertTranscriptMediaAsset(assets []domain.MediaAsset, asset domain.MediaAsset) []domain.MediaAsset {
	for index, existing := range assets {
		if existing.ID == asset.ID || (existing.OriginalURL != "" && existing.OriginalURL == asset.OriginalURL) {
			assets[index] = asset
			return assets
		}
	}
	return append(assets, asset)
}

func transcriptFromMap(value map[string]any) (domain.Transcript, error) {
	var transcript domain.Transcript
	raw, err := json.Marshal(value)
	if err != nil {
		return transcript, err
	}
	if err := json.Unmarshal(raw, &transcript); err != nil {
		return transcript, err
	}
	return transcript, nil
}

func structToMap(value any) (map[string]any, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func decodeJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(r.Body, 16<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	return nil
}

func urlParamInt(r *http.Request, key string) (int64, error) {
	value := chi.URLParam(r, key)
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, fmt.Errorf("invalid %s", key)
	}
	return id, nil
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	if errors.Is(err, sql.ErrNoRows) {
		status = http.StatusNotFound
	}
	writeJSON(w, status, map[string]any{
		"error": err.Error(),
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func containsFold(values []string, needle string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(needle)) {
			return true
		}
	}
	return false
}
