package httpapi

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"local/ludex/internal/domain"
	telegramsource "local/ludex/internal/source/telegram"
)

type importTelegramRequest struct {
	SourceID   int64 `json:"source_id"`
	MessageID  int   `json:"message_id"`
	CreateGame bool  `json:"create_game"`
}

type importTelegramResult struct {
	Item       domain.SourceItem `json:"item"`
	Game       *domain.Game      `json:"game,omitempty"`
	Transcript domain.Transcript `json:"transcript"`
}

var (
	telegramKeyValueRE       = regexp.MustCompile(`^\s*([^:：\n]{1,36})\s*[:：]\s*(.+?)\s*$`)
	telegramVersionRE        = regexp.MustCompile(`(?i)(?:version|ver\.?|版本)?\s*\b(v[0-9][0-9A-Za-z._-]*)\b`)
	telegramURLRE            = regexp.MustCompile(`https?://[^\s<>"'）)]+`)
	telegramHashSegmentRE    = regexp.MustCompile(`#([^#\n]+)`)
	telegramSectionHeadingRE = regexp.MustCompile(`^\s*[·・•]+\s*(.+?)\s*[·・•]+\s*$`)
	telegramDatedStatusRE    = regexp.MustCompile(`【\s*(\d{4})年(\d{1,2})月(\d{1,2})日(?:\d{1,2}时)?\s*([^】]*?(?:发售|更新))[^】]*】`)
)

func (s *Server) browseTelegram(ctx context.Context, req browseAdapterRequest, capabilities domain.AdapterBrowseCapabilities) (domain.AdapterBrowsePage, error) {
	sourceIDs := uniquePositiveIDs(req.SourceIDs)
	if len(sourceIDs) == 0 {
		return domain.AdapterBrowsePage{
			AdapterID:    "telegram",
			Title:        "Telegram messages",
			Page:         1,
			Items:        []domain.AdapterListItem{},
			Warnings:     []string{"Choose at least one saved Telegram group or channel."},
			Capabilities: capabilities,
		}, nil
	}
	search := strings.TrimSpace(req.Search)
	if search != "" && len(sourceIDs) != 1 {
		return domain.AdapterBrowsePage{}, errors.New("Telegram source search supports one selected group or channel at a time")
	}
	limit := clampInt(req.Limit, 1, 100, 30)
	page := domain.AdapterBrowsePage{
		AdapterID:    "telegram",
		Title:        "Telegram messages",
		URL:          telegramBrowseURL(sourceIDs, search, req.OffsetID),
		Page:         1,
		Items:        []domain.AdapterListItem{},
		Capabilities: capabilities,
	}
	nextOffsetID := 0

	for _, sourceID := range sourceIDs {
		source, err := s.store.GetSource(ctx, sourceID)
		if err != nil {
			return domain.AdapterBrowsePage{}, err
		}
		if source.Type != "telegram" {
			return domain.AdapterBrowsePage{}, fmt.Errorf("source #%d is %q, not telegram", sourceID, source.Type)
		}
		messagePage, err := telegramsource.ListMessages(ctx, s.store.DataDir(), telegramsource.Config{}, telegramSourceFromDomain(source), telegramsource.MessageQuery{
			Limit:    limit,
			OffsetID: req.OffsetID,
			Search:   search,
		})
		if err != nil {
			return domain.AdapterBrowsePage{}, err
		}
		for _, message := range messagePage.Messages {
			page.Items = append(page.Items, telegramAdapterListItem(source, message))
		}
		if messagePage.NextOffsetID > 0 && (nextOffsetID == 0 || messagePage.NextOffsetID < nextOffsetID) {
			nextOffsetID = messagePage.NextOffsetID
		}
		if len(sourceIDs) == 1 {
			page.Title = source.Name
			page.TotalPages = messagePage.Total
		}
	}
	sort.SliceStable(page.Items, func(i, j int) bool {
		left := DateMillis(page.Items[i].LatestAt)
		right := DateMillis(page.Items[j].LatestAt)
		if left == right {
			return page.Items[i].ExternalID > page.Items[j].ExternalID
		}
		return left > right
	})
	if nextOffsetID > 0 && len(page.Items) > 0 {
		page.NextURL = telegramBrowseURL(sourceIDs, search, nextOffsetID)
	}
	if len(page.Items) == 0 {
		page.Warnings = append(page.Warnings, "No Telegram messages matched this request.")
	}
	return page, nil
}

func (s *Server) importTelegram(w http.ResponseWriter, r *http.Request) {
	var req importTelegramRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	task, duplicate, err := s.enqueueTelegramImport(r.Context(), req)
	if err != nil {
		writeError(w, err)
		return
	}
	status := http.StatusAccepted
	if duplicate {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{
		"task":      task,
		"duplicate": duplicate,
	})
}

func (s *Server) enqueueTelegramImport(ctx context.Context, req importTelegramRequest) (domain.Task, bool, error) {
	if req.SourceID <= 0 {
		return domain.Task{}, false, errors.New("source_id is required")
	}
	if req.MessageID <= 0 {
		return domain.Task{}, false, errors.New("message_id is required")
	}
	source, err := s.store.GetSource(ctx, req.SourceID)
	if err != nil {
		return domain.Task{}, false, err
	}
	if source.Type != "telegram" {
		return domain.Task{}, false, fmt.Errorf("source #%d is %q, not telegram", source.ID, source.Type)
	}
	dedupeKey := telegramTaskDedupeKey(source, req.MessageID)
	if dedupeKey != "" {
		existing, err := s.store.GetActiveTaskByDedupeKey(ctx, "import:telegram", dedupeKey)
		if err == nil {
			return existing, true, nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, false, err
		}
	}
	task, err := s.store.CreateTask(ctx, domain.Task{
		Kind:       "import:telegram",
		DedupeKey:  dedupeKey,
		Status:     "queued",
		Title:      telegramImportTaskTitle(source, req.MessageID),
		Message:    "Queued",
		ResultJSON: map[string]any{"import_request": telegramImportRequestMetadata(req)},
	})
	if err != nil {
		if dedupeKey != "" {
			existing, lookupErr := s.store.GetActiveTaskByDedupeKey(ctx, "import:telegram", dedupeKey)
			if lookupErr == nil {
				return existing, true, nil
			}
		}
		return domain.Task{}, false, err
	}
	go s.runTelegramImportTask(task.ID, req)
	return task, false, nil
}

func (s *Server) runTelegramImportTask(taskID int64, req importTelegramRequest) {
	ctx := context.Background()
	_, _ = s.store.StartTask(ctx, taskID, "Reading Telegram message")
	report := func(current int, total int, message string) {
		_, _ = s.store.UpdateTaskProgress(ctx, taskID, current, total, message)
	}
	result, err := s.executeTelegramImport(ctx, req, report)
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

func (s *Server) executeTelegramImport(ctx context.Context, req importTelegramRequest, report progressReporter) (importTelegramResult, error) {
	source, err := s.store.GetSource(ctx, req.SourceID)
	if err != nil {
		return importTelegramResult{}, err
	}
	if source.Type != "telegram" {
		return importTelegramResult{}, fmt.Errorf("source #%d is %q, not telegram", source.ID, source.Type)
	}
	report(1, 5, "Fetching Telegram message")
	message, err := telegramsource.GetMessage(ctx, s.store.DataDir(), telegramsource.Config{}, telegramSourceFromDomain(source), req.MessageID)
	if err != nil {
		return importTelegramResult{}, err
	}
	report(2, 5, "Parsing Telegram message")
	transcript := telegramTranscript(source, message)
	parsed, err := structToMap(transcript)
	if err != nil {
		return importTelegramResult{}, err
	}
	report(3, 5, "Saving raw message")
	rawPath, err := s.saveRawContent("telegram", firstNonEmpty(message.URL, source.URL), []byte(message.Text))
	if err != nil {
		return importTelegramResult{}, err
	}
	report(4, 5, "Saving source item")
	item, _, err := s.store.UpsertSourceItemByAdapterKey(ctx, domain.SourceItem{
		SourceID:       &source.ID,
		SourceType:     "telegram",
		ExternalID:     transcript.ExternalID,
		Title:          firstNonEmpty(transcript.Inferred.GameTitle, transcript.Title, "Telegram message"),
		RawURL:         firstNonEmpty(message.URL, source.URL),
		RawContentPath: rawPath,
		ParsedJSON:     parsed,
		FetchedAt:      time.Now().UTC().Format(time.RFC3339),
		PublishedAt:    message.Date,
		Status:         "imported",
	})
	if err != nil {
		return importTelegramResult{}, err
	}

	var game *domain.Game
	if req.CreateGame {
		created, err := s.saveGameFromTranscript(ctx, transcript, item.MatchedGameID)
		if err != nil {
			return importTelegramResult{}, err
		}
		item, err = s.store.SetSourceItemMatch(ctx, item.ID, &created.ID)
		if err != nil {
			return importTelegramResult{}, err
		}
		game = &created
	}
	report(5, 5, "Import complete")
	return importTelegramResult{Item: item, Game: game, Transcript: transcript}, nil
}

func telegramSourceFromDomain(source domain.Source) telegramsource.Source {
	return telegramsource.Source{
		Title:      source.Name,
		SourceURL:  source.URL,
		ConfigJSON: source.ConfigJSON,
	}
}

func telegramAdapterListItem(source domain.Source, message telegramsource.Message) domain.AdapterListItem {
	title := telegramMessageTitle(message.Text)
	if title == "" {
		title = fmt.Sprintf("Message #%d", message.ID)
	}
	tags := telegramMessageTags(message.Text)
	return domain.AdapterListItem{
		AdapterID:  "telegram",
		ExternalID: telegramExternalID(message),
		Title:      title,
		URL:        firstNonEmpty(message.URL, source.URL),
		Author:     source.Name,
		Summary:    telegramMessagePreview(message.Text),
		LatestAt:   message.Date,
		Prefixes:   tags,
		Tags:       tags,
		Importable: true,
	}
}

func telegramTranscript(source domain.Source, message telegramsource.Message) domain.Transcript {
	keyValues := telegramKeyValues(message.Text)
	title := firstNonEmpty(telegramLabeledValue(keyValues, "game", "title", "name", "游戏", "游戏名", "名称", "标题"), telegramMessageTitle(message.Text))
	version := firstNonEmpty(telegramLabeledValue(keyValues, "version", "ver", "版本"), telegramVersion(message.Text))
	developer := telegramLabeledValue(keyValues, "developer", "author", "circle", "studio", "开发", "开发者", "作者", "社团", "制作")
	sections := telegramSections(message.Text)
	descriptionSection := telegramSectionBody(sections, "游戏介绍", "简介", "故事梗概", "剧情梗概", "story", "overview", "description")
	changelog := telegramSectionBody(sections, "更新介绍", "更新内容", "更新日志", "changelog", "change log")
	description := firstNonEmpty(
		telegramLabeledValue(keyValues, "overview", "description", "intro", "简介", "介绍"),
		descriptionSection,
	)
	if description == "" && changelog == "" {
		description = telegramFallbackDescription(message.Text, title)
	}
	tags := telegramMessageTags(message.Text)
	releaseDate, threadUpdated := telegramReleaseDates(message.Text)
	fields := domain.TranscriptFields{
		GameName:         title,
		Description:      description,
		ThreadUpdated:    threadUpdated,
		ReleaseDate:      releaseDate,
		Developer:        developer,
		Version:          version,
		OperatingSystems: telegramOperatingSystems(message.Text, tags),
		Languages:        telegramLanguages(message.Text),
		Genres:           telegramGenres(tags),
		Changelog:        changelog,
		DownloadGroups:   telegramDownloadGroups(message),
	}
	return domain.Transcript{
		Source:     "telegram",
		SourceURL:  firstNonEmpty(message.URL, source.URL),
		ExternalID: telegramExternalID(message),
		Title:      title,
		Fields:     fields,
		KeyValues:  keyValues,
		Sections:   append(sections, domain.TranscriptSection{Heading: "Message", Body: strings.TrimSpace(message.Text)}),
		Tags:       tags,
		Inferred: domain.TranscriptInferred{
			GameTitle:   title,
			Version:     version,
			Developer:   developer,
			Description: description,
		},
		Warnings: telegramTranscriptWarnings(fields),
	}
}

func telegramTranscriptWarnings(fields domain.TranscriptFields) []string {
	warnings := []string{}
	if strings.TrimSpace(fields.GameName) == "" {
		warnings = append(warnings, "Telegram message did not include a recognizable game title.")
	}
	if strings.TrimSpace(fields.Description) == "" && strings.TrimSpace(fields.Changelog) == "" {
		warnings = append(warnings, "Telegram message did not include a recognizable overview or changelog.")
	}
	if len(fields.DownloadGroups) == 0 {
		warnings = append(warnings, "Telegram message link was not available for downloads.")
	}
	return warnings
}

func telegramKeyValues(text string) map[string][]string {
	values := map[string][]string{}
	for _, line := range strings.Split(text, "\n") {
		match := telegramKeyValueRE.FindStringSubmatch(line)
		if len(match) != 3 {
			continue
		}
		key := strings.TrimSpace(match[1])
		value := strings.TrimSpace(match[2])
		if key == "" || value == "" {
			continue
		}
		values[key] = append(values[key], value)
	}
	return values
}

func telegramLabeledValue(values map[string][]string, labels ...string) string {
	for _, label := range labels {
		for key, entries := range values {
			if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(label)) && len(entries) > 0 {
				return entries[0]
			}
		}
	}
	return ""
}

func telegramMessageTitle(text string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
			continue
		}
		if telegramDecorativeLine(line) {
			continue
		}
		if title := telegramTitleFromHashLine(line); title != "" {
			return title
		}
		line = strings.Trim(line, " \t-_*#[]()【】「」『』《》:：.")
		if line == "" || telegramURLRE.MatchString(line) {
			continue
		}
		if match := telegramKeyValueRE.FindStringSubmatch(line); len(match) == 3 {
			key := strings.TrimSpace(match[1])
			if strings.EqualFold(key, "version") || strings.EqualFold(key, "ver") || key == "版本" {
				continue
			}
			return truncateRunes(strings.TrimSpace(match[2]), 120)
		}
		return truncateRunes(line, 120)
	}
	return ""
}

func telegramVersion(text string) string {
	match := telegramVersionRE.FindStringSubmatch(text)
	if len(match) == 2 {
		return strings.TrimSpace(match[1])
	}
	return ""
}

func telegramMessageTags(text string) []string {
	tags := []string{}
	for _, match := range telegramHashSegmentRE.FindAllStringSubmatch(text, -1) {
		if len(match) != 2 {
			continue
		}
		tag := telegramCleanHashSegment(match[1])
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return uniqueStrings(tags)
}

func telegramFallbackDescription(text string, title string) string {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || line == title || telegramURLRE.MatchString(line) || strings.Contains(line, "#") || telegramDecorativeLine(line) {
			continue
		}
		if telegramKeyValueRE.MatchString(line) {
			continue
		}
		return truncateRunes(line, 260)
	}
	return ""
}

func telegramMessagePreview(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	return truncateRunes(text, 180)
}

func telegramDecorativeLine(line string) bool {
	line = strings.TrimSpace(line)
	if line == "" {
		return true
	}
	if strings.Contains(line, "作品推荐") {
		return true
	}
	trimmed := strings.Trim(line, " \t—-_=~*·・•🚢⭐️✨[]【】()（）")
	return trimmed == ""
}

func telegramTitleFromHashLine(line string) string {
	for _, match := range telegramHashSegmentRE.FindAllStringSubmatch(line, -1) {
		if len(match) != 2 {
			continue
		}
		candidate := telegramCleanHashSegment(match[1])
		if candidate == "" || telegramIsMetaTag(candidate) {
			continue
		}
		return truncateRunes(candidate, 120)
	}
	return ""
}

func telegramCleanHashSegment(value string) string {
	value = strings.TrimSpace(value)
	cutMarkers := []string{" 官方", " AI", " 日文", " 英文", " 中文", " 汉化", "【", " v"}
	lower := strings.ToLower(value)
	cut := len(value)
	for _, marker := range cutMarkers {
		index := strings.Index(lower, strings.ToLower(marker))
		if index >= 0 && index < cut {
			cut = index
		}
	}
	value = value[:cut]
	value = strings.Trim(value, " \t-_*#[]()【】「」『』《》:：.。")
	return value
}

func telegramSections(text string) []domain.TranscriptSection {
	sections := []domain.TranscriptSection{}
	currentHeading := ""
	currentBody := []string{}
	flush := func() {
		body := strings.TrimSpace(strings.Join(currentBody, "\n"))
		if currentHeading != "" && body != "" {
			sections = append(sections, domain.TranscriptSection{Heading: currentHeading, Body: body})
		}
		currentBody = []string{}
	}
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		if match := telegramSectionHeadingRE.FindStringSubmatch(trimmed); len(match) == 2 {
			flush()
			currentHeading = strings.TrimSpace(match[1])
			continue
		}
		if currentHeading != "" {
			currentBody = append(currentBody, line)
		}
	}
	flush()
	return sections
}

func telegramSectionBody(sections []domain.TranscriptSection, headings ...string) string {
	for _, section := range sections {
		sectionHeading := strings.ToLower(strings.TrimSpace(section.Heading))
		for _, heading := range headings {
			heading = strings.ToLower(strings.TrimSpace(heading))
			if sectionHeading == heading || strings.Contains(sectionHeading, heading) {
				return strings.TrimSpace(section.Body)
			}
		}
	}
	return ""
}

func telegramReleaseDates(text string) (string, string) {
	releaseDate := ""
	threadUpdated := ""
	for _, match := range telegramDatedStatusRE.FindAllStringSubmatch(text, -1) {
		if len(match) != 5 {
			continue
		}
		year, _ := strconv.Atoi(match[1])
		month, _ := strconv.Atoi(match[2])
		day, _ := strconv.Atoi(match[3])
		date := fmt.Sprintf("%04d-%02d-%02d", year, month, day)
		status := match[4]
		if strings.Contains(status, "更新") {
			threadUpdated = date
		}
		if strings.Contains(status, "发售") && releaseDate == "" {
			releaseDate = date
		}
	}
	return releaseDate, threadUpdated
}

func telegramLanguages(text string) []string {
	languages := []string{}
	if strings.Contains(text, "官方中文") || strings.Contains(text, "汉化") || strings.Contains(text, "中文版") || strings.Contains(text, "中文步兵") {
		languages = append(languages, "Chinese")
	}
	if strings.Contains(text, "日文") || strings.Contains(text, "生肉") {
		languages = append(languages, "Japanese")
	}
	if strings.Contains(strings.ToLower(text), "english") || strings.Contains(text, "英文") {
		languages = append(languages, "English")
	}
	return uniqueStrings(languages)
}

func telegramOperatingSystems(text string, tags []string) []string {
	values := []string{}
	for _, tag := range tags {
		switch strings.ToLower(strings.TrimSpace(tag)) {
		case "pc", "windows", "win":
			values = append(values, "PC")
		case "android", "安卓":
			values = append(values, "Android")
		case "mac", "macos":
			values = append(values, "Mac")
		case "linux":
			values = append(values, "Linux")
		}
	}
	lower := strings.ToLower(text)
	if strings.Contains(text, "安卓") || strings.Contains(lower, "android") {
		values = append(values, "Android")
	}
	return uniqueStrings(values)
}

func telegramGenres(tags []string) []string {
	genres := []string{}
	for _, tag := range tags {
		if telegramIsGenreTag(tag) {
			genres = append(genres, strings.ToUpper(strings.TrimSpace(tag)))
		}
	}
	return uniqueStrings(genres)
}

func telegramIsMetaTag(tag string) bool {
	return telegramIsGenreTag(tag) || telegramIsPlatformTag(tag)
}

func telegramIsGenreTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "adv", "act", "slg", "rpg", "rpgm", "vn", "html", "sim", "sandbox", "puzzle", "renpy", "ren'py":
		return true
	default:
		return false
	}
}

func telegramIsPlatformTag(tag string) bool {
	switch strings.ToLower(strings.TrimSpace(tag)) {
	case "pc", "windows", "win", "android", "安卓", "mac", "macos", "linux":
		return true
	default:
		return false
	}
}

func telegramDownloadGroups(message telegramsource.Message) []domain.DownloadGroup {
	if strings.TrimSpace(message.URL) == "" {
		return nil
	}
	return []domain.DownloadGroup{{
		Platform: "Telegram",
		Links: []domain.NamedURL{{
			Name: "Telegram message",
			URL:  message.URL,
		}},
	}}
}

func telegramExternalID(message telegramsource.Message) string {
	return strings.TrimSpace(message.PeerID) + ":" + strconv.Itoa(message.ID)
}

func telegramTaskDedupeKey(source domain.Source, messageID int) string {
	cfg, err := telegramsource.ParseSourceConfig(source.ConfigJSON)
	if err != nil || cfg.PeerID == "" || messageID <= 0 {
		return ""
	}
	return "telegram:" + cfg.PeerID + ":" + strconv.Itoa(messageID)
}

func telegramImportTaskTitle(source domain.Source, messageID int) string {
	return fmt.Sprintf("Import Telegram: %s #%d", firstNonEmpty(source.Name, source.URL, "source"), messageID)
}

func telegramImportRequestMetadata(req importTelegramRequest) map[string]any {
	return map[string]any{
		"source_id":   req.SourceID,
		"message_id":  req.MessageID,
		"create_game": req.CreateGame,
	}
}

func importTelegramRequestFromTask(task domain.Task) (importTelegramRequest, error) {
	var req importTelegramRequest
	if rawRequest, ok := task.ResultJSON["import_request"]; ok {
		if raw, err := json.Marshal(rawRequest); err == nil {
			_ = json.Unmarshal(raw, &req)
		}
	}
	if req.SourceID <= 0 || req.MessageID <= 0 {
		return req, errors.New("could not recover the original Telegram import request for this task")
	}
	return req, nil
}

func telegramBrowseURL(sourceIDs []int64, search string, offsetID int) string {
	values := url.Values{}
	for _, id := range sourceIDs {
		values.Add("source_id", strconv.FormatInt(id, 10))
	}
	if search != "" {
		values.Set("search", search)
	}
	if offsetID > 0 {
		values.Set("offset_id", strconv.Itoa(offsetID))
	}
	return "telegram://messages?" + values.Encode()
}

func uniquePositiveIDs(values []int64) []int64 {
	seen := map[int64]struct{}{}
	next := make([]int64, 0, len(values))
	for _, value := range values {
		if value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		next = append(next, value)
	}
	return next
}

func uniqueStrings(values []string) []string {
	seen := map[string]struct{}{}
	next := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		key := strings.ToLower(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		next = append(next, value)
	}
	return next
}

func truncateRunes(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= limit {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:limit])) + "..."
}

func clampInt(value int, minValue int, maxValue int, fallback int) int {
	if value <= 0 {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}

func DateMillis(value string) int64 {
	if value == "" {
		return 0
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return 0
	}
	return parsed.UnixMilli()
}
