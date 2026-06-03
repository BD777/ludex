package telegramsource

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

type Config struct {
	APIID     int    `json:"api_id"`
	APIHash   string `json:"api_hash,omitempty"`
	Phone     string `json:"phone,omitempty"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type Status struct {
	Configured bool   `json:"configured"`
	Authorized bool   `json:"authorized"`
	UserID     int64  `json:"user_id,omitempty"`
	Username   string `json:"username,omitempty"`
	Phone      string `json:"phone,omitempty"`
	FirstName  string `json:"first_name,omitempty"`
	LastName   string `json:"last_name,omitempty"`
	Error      string `json:"error,omitempty"`
}

type SendCodeResult struct {
	CodeSent bool   `json:"code_sent"`
	Phone    string `json:"phone"`
	CodeType string `json:"code_type"`
	Timeout  int    `json:"timeout,omitempty"`
	Status   Status `json:"status"`
}

type SignInResult struct {
	PasswordRequired bool   `json:"password_required"`
	Status           Status `json:"status"`
}

type Dialog struct {
	PeerID       string `json:"peer_id"`
	Type         string `json:"type"`
	Title        string `json:"title"`
	Username     string `json:"username,omitempty"`
	AccessHash   string `json:"access_hash,omitempty"`
	Participants int    `json:"participants,omitempty"`
	Date         string `json:"date,omitempty"`
	SourceURL    string `json:"source_url"`
	ConfigJSON   string `json:"config_json"`
}

type Source struct {
	Title      string
	SourceURL  string
	ConfigJSON string
}

type Message struct {
	PeerID    string  `json:"peer_id"`
	ID        int     `json:"id"`
	Text      string  `json:"text"`
	Date      string  `json:"date"`
	URL       string  `json:"url"`
	GroupedID int64   `json:"grouped_id,omitempty"`
	Media     []Media `json:"media,omitempty"`
}

type Media struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Kind      string `json:"kind"`
	MIMEType  string `json:"mime_type"`
	MessageID int    `json:"message_id"`
	Position  int    `json:"position"`
}

type MessageQuery struct {
	Limit    int
	OffsetID int
	Search   string
}

type MessagePage struct {
	Messages     []Message `json:"messages"`
	NextOffsetID int       `json:"next_offset_id,omitempty"`
	Total        int       `json:"total,omitempty"`
	Search       string    `json:"search,omitempty"`
}

type SourceConfig struct {
	PeerID     string `json:"peer_id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	Username   string `json:"username"`
	AccessHash string `json:"access_hash"`
	SourceURL  string `json:"source_url"`
}

type mediaRequest struct {
	Config    SourceConfig
	MessageID int
	Position  int
}

type mediaFile struct {
	MessageID int
	Name      string
	MIMEType  string
	Location  tg.InputFileLocationClass
}

const telegramAlbumMessageWindow = 12

type pendingCode struct {
	Phone         string `json:"phone"`
	PhoneCodeHash string `json:"phone_code_hash"`
	SentAt        string `json:"sent_at"`
}

func StatusFor(ctx context.Context, dataDir string, input Config) (Status, error) {
	cfg, err := ResolveConfig(dataDir, input)
	if err != nil {
		return Status{}, err
	}
	status := Status{Configured: cfg.Configured()}
	if !cfg.Configured() {
		return status, nil
	}
	err = withClient(ctx, dataDir, cfg, func(ctx context.Context, client *gotdtelegram.Client) error {
		authStatus, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		status = statusFromAuth(authStatus, cfg)
		return nil
	})
	if err != nil {
		status.Error = err.Error()
		return status, nil
	}
	return status, nil
}

func SendCode(ctx context.Context, dataDir string, input Config) (SendCodeResult, error) {
	cfg, err := ResolveConfig(dataDir, input)
	if err != nil {
		return SendCodeResult{}, err
	}
	if !cfg.Configured() {
		return SendCodeResult{}, errors.New("telegram api_id and api_hash are required")
	}
	if strings.TrimSpace(cfg.Phone) == "" {
		return SendCodeResult{}, errors.New("telegram phone number is required")
	}
	if err := SaveConfig(dataDir, cfg); err != nil {
		return SendCodeResult{}, err
	}

	var sent tg.AuthSentCodeClass
	if err := withClient(ctx, dataDir, cfg, func(ctx context.Context, client *gotdtelegram.Client) error {
		next, err := client.Auth().SendCode(ctx, cfg.Phone, auth.SendCodeOptions{})
		if err != nil {
			return err
		}
		sent = next
		return nil
	}); err != nil {
		return SendCodeResult{}, err
	}

	hash, ok := phoneCodeHash(sent)
	if !ok || hash == "" {
		return SendCodeResult{}, errors.New("telegram did not return a phone code hash")
	}
	if err := savePendingCode(dataDir, pendingCode{
		Phone:         cfg.Phone,
		PhoneCodeHash: hash,
		SentAt:        time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		return SendCodeResult{}, err
	}

	status, _ := StatusFor(ctx, dataDir, Config{})
	return SendCodeResult{
		CodeSent: true,
		Phone:    cfg.Phone,
		CodeType: sent.TypeName(),
		Timeout:  sentCodeTimeout(sent),
		Status:   status,
	}, nil
}

func SignIn(ctx context.Context, dataDir string, input Config, code string, password string) (SignInResult, error) {
	cfg, err := ResolveConfig(dataDir, input)
	if err != nil {
		return SignInResult{}, err
	}
	if !cfg.Configured() {
		return SignInResult{}, errors.New("telegram api_id and api_hash are required")
	}
	pending, err := loadPendingCode(dataDir)
	if err != nil {
		return SignInResult{}, err
	}
	if cfg.Phone == "" {
		cfg.Phone = pending.Phone
	}
	if strings.TrimSpace(code) == "" && strings.TrimSpace(password) == "" {
		return SignInResult{}, errors.New("telegram login code is required")
	}
	if err := SaveConfig(dataDir, cfg); err != nil {
		return SignInResult{}, err
	}

	passwordRequired := false
	err = withClient(ctx, dataDir, cfg, func(ctx context.Context, client *gotdtelegram.Client) error {
		authClient := client.Auth()
		if strings.TrimSpace(code) != "" {
			if _, err := authClient.SignIn(ctx, cfg.Phone, strings.TrimSpace(code), pending.PhoneCodeHash); err != nil {
				if errors.Is(err, auth.ErrPasswordAuthNeeded) {
					passwordRequired = true
				} else {
					return err
				}
			}
		}
		if passwordRequired || strings.TrimSpace(password) != "" {
			if strings.TrimSpace(password) == "" {
				return nil
			}
			if _, err := authClient.Password(ctx, password); err != nil {
				return err
			}
			passwordRequired = false
		}
		return nil
	})
	if err != nil {
		return SignInResult{}, err
	}
	if !passwordRequired {
		_ = os.Remove(pendingCodePath(dataDir))
	}
	status, _ := StatusFor(ctx, dataDir, Config{})
	return SignInResult{PasswordRequired: passwordRequired, Status: status}, nil
}

func ListDialogs(ctx context.Context, dataDir string, input Config, limit int) ([]Dialog, error) {
	cfg, err := ResolveConfig(dataDir, input)
	if err != nil {
		return nil, err
	}
	if !cfg.Configured() {
		return nil, errors.New("telegram api_id and api_hash are required")
	}
	if limit <= 0 || limit > 200 {
		limit = 100
	}

	var dialogs []Dialog
	if err := withClient(ctx, dataDir, cfg, func(ctx context.Context, client *gotdtelegram.Client) error {
		authStatus, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !authStatus.Authorized {
			return errors.New("telegram account is not authorized")
		}
		response, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetPeer: &tg.InputPeerEmpty{},
			Limit:      limit,
		})
		if err != nil {
			return err
		}
		if response == nil {
			dialogs = []Dialog{}
			return nil
		}
		modified, ok := response.AsModified()
		if !ok || modified == nil {
			dialogs = []Dialog{}
			return nil
		}
		dialogs = dialogsFromChats(modified.GetChats())
		return nil
	}); err != nil {
		return nil, err
	}
	return dialogs, nil
}

func ListMessages(ctx context.Context, dataDir string, input Config, source Source, query MessageQuery) (MessagePage, error) {
	cfg, err := ResolveConfig(dataDir, input)
	if err != nil {
		return MessagePage{}, err
	}
	if !cfg.Configured() {
		return MessagePage{}, errors.New("telegram api_id and api_hash are required")
	}
	sourceConfig, err := ParseSourceConfig(source.ConfigJSON)
	if err != nil {
		return MessagePage{}, err
	}
	if sourceConfig.SourceURL == "" {
		sourceConfig.SourceURL = source.SourceURL
	}
	if query.Limit <= 0 || query.Limit > 200 {
		query.Limit = 50
	}
	query.Search = strings.TrimSpace(query.Search)

	var page MessagePage
	page.Search = query.Search
	if err := withClient(ctx, dataDir, cfg, func(ctx context.Context, client *gotdtelegram.Client) error {
		authStatus, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !authStatus.Authorized {
			return errors.New("telegram account is not authorized")
		}
		peer, err := inputPeerFromSource(sourceConfig)
		if err != nil {
			return err
		}
		var response tg.MessagesMessagesClass
		if query.Search != "" {
			response, err = client.API().MessagesSearch(ctx, &tg.MessagesSearchRequest{
				Peer:     peer,
				Q:        query.Search,
				Filter:   &tg.InputMessagesFilterEmpty{},
				OffsetID: query.OffsetID,
				Limit:    query.Limit,
			})
		} else {
			response, err = client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
				Peer:     peer,
				OffsetID: query.OffsetID,
				Limit:    query.Limit,
			})
		}
		if err != nil {
			return err
		}
		if response == nil {
			page.Messages = []Message{}
			return nil
		}
		modified, ok := response.AsModified()
		if !ok || modified == nil {
			page.Messages = []Message{}
			return nil
		}
		page.Messages = messagesFromClasses(sourceConfig, modified.GetMessages())
		page.NextOffsetID = nextOffsetID(page.Messages)
		page.Total = len(page.Messages)
		if counted, ok := modified.(interface{ GetCount() int }); ok {
			page.Total = counted.GetCount()
		}
		return nil
	}); err != nil {
		return MessagePage{}, err
	}
	return page, nil
}

func GetMessage(ctx context.Context, dataDir string, input Config, source Source, messageID int) (Message, error) {
	if messageID <= 0 {
		return Message{}, errors.New("telegram message_id is required")
	}
	cfg, err := ResolveConfig(dataDir, input)
	if err != nil {
		return Message{}, err
	}
	if !cfg.Configured() {
		return Message{}, errors.New("telegram api_id and api_hash are required")
	}
	sourceConfig, err := ParseSourceConfig(source.ConfigJSON)
	if err != nil {
		return Message{}, err
	}
	if sourceConfig.SourceURL == "" {
		sourceConfig.SourceURL = source.SourceURL
	}

	var found Message
	err = withClient(ctx, dataDir, cfg, func(ctx context.Context, client *gotdtelegram.Client) error {
		authStatus, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !authStatus.Authorized {
			return errors.New("telegram account is not authorized")
		}

		messages, err := getTGMessagesByIDs(ctx, client, sourceConfig, []int{messageID})
		if err != nil {
			return err
		}
		var selected *tg.Message
		for _, message := range messages {
			if message.ID == messageID {
				selected = message
				break
			}
		}
		if selected == nil {
			return errors.New("telegram message was not found")
		}
		relatedMessages, err := telegramRelatedMediaMessages(ctx, client, sourceConfig, selected)
		if err != nil {
			return err
		}
		found, _ = messageFromTG(sourceConfig, selected, true)
		found.Media = mediaDescriptorsFromMessages(sourceConfig, selected.ID, relatedMessages)
		return nil
	})
	if err != nil {
		return Message{}, err
	}
	return found, nil
}

func DownloadMedia(ctx context.Context, dataDir string, input Config, rawURL string) ([]byte, string, error) {
	request, err := parseMediaURL(rawURL)
	if err != nil {
		return nil, "", err
	}
	cfg, err := ResolveConfig(dataDir, input)
	if err != nil {
		return nil, "", err
	}
	if !cfg.Configured() {
		return nil, "", errors.New("telegram api_id and api_hash are required")
	}

	var body bytes.Buffer
	var contentType string
	err = withClient(ctx, dataDir, cfg, func(ctx context.Context, client *gotdtelegram.Client) error {
		authStatus, err := client.Auth().Status(ctx)
		if err != nil {
			return err
		}
		if !authStatus.Authorized {
			return errors.New("telegram account is not authorized")
		}
		files, err := mediaFilesForRootMessage(ctx, client, request.Config, request.MessageID)
		if err != nil {
			return err
		}
		if request.Position < 0 || request.Position >= len(files) {
			return fmt.Errorf("telegram media position %d is unavailable", request.Position)
		}
		file := files[request.Position]
		if file.MIMEType == "" {
			file.MIMEType = "image/jpeg"
		}
		contentType = file.MIMEType
		_, err = client.Download(file.Location).Stream(ctx, &body)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	return body.Bytes(), contentType, nil
}

func IsMediaURL(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	return err == nil && parsed.Scheme == "telegram" && parsed.Host == "media"
}

func ParseSourceConfig(raw string) (SourceConfig, error) {
	var cfg SourceConfig
	if strings.TrimSpace(raw) == "" {
		return cfg, errors.New("telegram source config is empty")
	}
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return cfg, err
	}
	if strings.TrimSpace(cfg.PeerID) == "" {
		return cfg, errors.New("telegram source config is missing peer_id")
	}
	cfg.PeerID = strings.TrimSpace(cfg.PeerID)
	cfg.Type = strings.TrimSpace(cfg.Type)
	cfg.Title = strings.TrimSpace(cfg.Title)
	cfg.Username = strings.TrimSpace(cfg.Username)
	cfg.AccessHash = strings.TrimSpace(cfg.AccessHash)
	cfg.SourceURL = strings.TrimSpace(cfg.SourceURL)
	return cfg, nil
}

func ResolveConfig(dataDir string, input Config) (Config, error) {
	stored, _ := loadConfig(dataDir)
	cfg := stored
	if input.APIID != 0 {
		cfg.APIID = input.APIID
	}
	if strings.TrimSpace(input.APIHash) != "" {
		cfg.APIHash = strings.TrimSpace(input.APIHash)
	}
	if strings.TrimSpace(input.Phone) != "" {
		cfg.Phone = strings.TrimSpace(input.Phone)
	}
	if cfg.APIID == 0 {
		if raw := strings.TrimSpace(os.Getenv("LUDEX_TELEGRAM_API_ID")); raw != "" {
			id, err := strconv.Atoi(raw)
			if err != nil {
				return Config{}, fmt.Errorf("invalid LUDEX_TELEGRAM_API_ID: %w", err)
			}
			cfg.APIID = id
		}
	}
	if cfg.APIHash == "" {
		cfg.APIHash = strings.TrimSpace(os.Getenv("LUDEX_TELEGRAM_API_HASH"))
	}
	return cfg, nil
}

func inputPeerFromSource(cfg SourceConfig) (tg.InputPeerClass, error) {
	peerID, err := strconv.ParseInt(cfg.PeerID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid telegram peer_id %q: %w", cfg.PeerID, err)
	}
	switch cfg.Type {
	case "group":
		return &tg.InputPeerChat{ChatID: peerID}, nil
	case "channel", "supergroup":
		accessHash, err := strconv.ParseInt(cfg.AccessHash, 10, 64)
		if err != nil || accessHash == 0 {
			return nil, fmt.Errorf("telegram source %q requires access_hash", cfg.Title)
		}
		return &tg.InputPeerChannel{ChannelID: peerID, AccessHash: accessHash}, nil
	default:
		if cfg.AccessHash != "" {
			accessHash, err := strconv.ParseInt(cfg.AccessHash, 10, 64)
			if err == nil && accessHash != 0 {
				return &tg.InputPeerChannel{ChannelID: peerID, AccessHash: accessHash}, nil
			}
		}
		return &tg.InputPeerChat{ChatID: peerID}, nil
	}
}

func inputChannelFromSource(cfg SourceConfig) (tg.InputChannelClass, error) {
	peerID, err := strconv.ParseInt(cfg.PeerID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid telegram peer_id %q: %w", cfg.PeerID, err)
	}
	accessHash, err := strconv.ParseInt(cfg.AccessHash, 10, 64)
	if err != nil || accessHash == 0 {
		return nil, fmt.Errorf("telegram source %q requires access_hash", cfg.Title)
	}
	return &tg.InputChannel{ChannelID: peerID, AccessHash: accessHash}, nil
}

func messagesFromClasses(cfg SourceConfig, classes []tg.MessageClass) []Message {
	tgMessages := tgMessagesFromClasses(classes)
	messages := make([]Message, 0, len(tgMessages))
	for _, message := range tgMessages {
		next, ok := messageFromTG(cfg, message, false)
		if !ok {
			continue
		}
		if len(next.Media) == 0 {
			next.Media = mediaDescriptorsFromMessages(cfg, message.ID, relatedMediaMessagesFromPool(message, tgMessages))
		}
		messages = append(messages, next)
	}
	return messages
}

func tgMessagesFromClasses(classes []tg.MessageClass) []*tg.Message {
	messages := make([]*tg.Message, 0, len(classes))
	for _, messageClass := range classes {
		message, ok := messageClass.(*tg.Message)
		if !ok || message == nil {
			continue
		}
		messages = append(messages, message)
	}
	return sortedTGMessages(messages)
}

func messageFromTG(cfg SourceConfig, message *tg.Message, includeEmptyText bool) (Message, bool) {
	if message == nil {
		return Message{}, false
	}
	text := strings.TrimSpace(message.GetMessage())
	if text == "" && !includeEmptyText {
		return Message{}, false
	}
	next := Message{
		PeerID: cfg.PeerID,
		ID:     message.GetID(),
		Text:   text,
		URL:    messageURL(cfg, message.GetID()),
		Media:  mediaDescriptorsFromMessages(cfg, message.GetID(), []*tg.Message{message}),
	}
	if groupedID, ok := message.GetGroupedID(); ok {
		next.GroupedID = groupedID
	}
	if date := message.GetDate(); date > 0 {
		next.Date = time.Unix(int64(date), 0).UTC().Format(time.RFC3339)
	}
	return next, true
}

func telegramRelatedMediaMessages(ctx context.Context, client *gotdtelegram.Client, cfg SourceConfig, selected *tg.Message) ([]*tg.Message, error) {
	if selected == nil {
		return nil, errors.New("telegram message was not found")
	}
	groupedID, ok := selected.GetGroupedID()
	if !ok || groupedID == 0 {
		return []*tg.Message{selected}, nil
	}
	ids := make([]int, 0, telegramAlbumMessageWindow*2+1)
	for id := selected.ID - telegramAlbumMessageWindow; id <= selected.ID+telegramAlbumMessageWindow; id++ {
		if id > 0 {
			ids = append(ids, id)
		}
	}
	messages, err := getTGMessagesByIDs(ctx, client, cfg, ids)
	if err != nil {
		return nil, err
	}
	album := make([]*tg.Message, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		if candidateGroupedID, ok := message.GetGroupedID(); ok && candidateGroupedID == groupedID {
			album = append(album, message)
		}
	}
	if len(album) > 0 {
		return sortedTGMessages(album), nil
	}
	return []*tg.Message{selected}, nil
}

func relatedMediaMessagesFromPool(selected *tg.Message, messages []*tg.Message) []*tg.Message {
	if selected == nil {
		return nil
	}
	if groupedID, ok := selected.GetGroupedID(); ok && groupedID != 0 {
		grouped := make([]*tg.Message, 0, len(messages))
		for _, message := range messages {
			if message == nil {
				continue
			}
			if candidateGroupedID, ok := message.GetGroupedID(); ok && candidateGroupedID == groupedID {
				grouped = append(grouped, message)
			}
		}
		if len(grouped) > 0 {
			return sortedTGMessages(grouped)
		}
	}
	return []*tg.Message{selected}
}

func getTGMessagesByIDs(ctx context.Context, client *gotdtelegram.Client, cfg SourceConfig, ids []int) ([]*tg.Message, error) {
	inputMessages := make([]tg.InputMessageClass, 0, len(ids))
	seen := map[int]struct{}{}
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		inputMessages = append(inputMessages, &tg.InputMessageID{ID: id})
	}
	if len(inputMessages) == 0 {
		return nil, errors.New("telegram message_id is required")
	}

	var response tg.MessagesMessagesClass
	var err error
	switch cfg.Type {
	case "channel", "supergroup":
		channel, err := inputChannelFromSource(cfg)
		if err != nil {
			return nil, err
		}
		response, err = client.API().ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: channel,
			ID:      inputMessages,
		})
	default:
		response, err = client.API().MessagesGetMessages(ctx, inputMessages)
	}
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, errors.New("telegram did not return the requested message")
	}
	modified, ok := response.AsModified()
	if !ok || modified == nil {
		return nil, errors.New("telegram did not return the requested message")
	}
	messages := make([]*tg.Message, 0, len(modified.GetMessages()))
	for _, messageClass := range modified.GetMessages() {
		message, ok := messageClass.(*tg.Message)
		if ok && message != nil {
			messages = append(messages, message)
		}
	}
	return sortedTGMessages(messages), nil
}

func sortedTGMessages(messages []*tg.Message) []*tg.Message {
	messages = compactTGMessages(messages)
	sort.SliceStable(messages, func(i, j int) bool {
		return messages[i].ID < messages[j].ID
	})
	return messages
}

func compactTGMessages(messages []*tg.Message) []*tg.Message {
	compact := messages[:0]
	for _, message := range messages {
		if message != nil {
			compact = append(compact, message)
		}
	}
	return compact
}

func mediaDescriptorsFromMessages(cfg SourceConfig, rootMessageID int, messages []*tg.Message) []Media {
	files := mediaFilesFromMessages(messages)
	media := make([]Media, 0, len(files))
	for index, file := range files {
		media = append(media, Media{
			ID:        fmt.Sprintf("%d:%d", rootMessageID, index),
			URL:       telegramMediaURL(cfg, rootMessageID, index),
			Kind:      "image",
			MIMEType:  firstNonEmpty(file.MIMEType, "image/jpeg"),
			MessageID: file.MessageID,
			Position:  index,
		})
	}
	return media
}

func mediaFilesForRootMessage(ctx context.Context, client *gotdtelegram.Client, cfg SourceConfig, messageID int) ([]mediaFile, error) {
	messages, err := getTGMessagesByIDs(ctx, client, cfg, []int{messageID})
	if err != nil {
		return nil, err
	}
	var selected *tg.Message
	for _, message := range messages {
		if message.ID == messageID {
			selected = message
			break
		}
	}
	if selected == nil {
		return nil, errors.New("telegram message was not found")
	}
	related, err := telegramRelatedMediaMessages(ctx, client, cfg, selected)
	if err != nil {
		return nil, err
	}
	files := mediaFilesFromMessages(related)
	if len(files) == 0 {
		return nil, errors.New("telegram message has no downloadable image media")
	}
	return files, nil
}

func mediaFilesFromMessages(messages []*tg.Message) []mediaFile {
	messages = sortedTGMessages(append([]*tg.Message{}, messages...))
	files := make([]mediaFile, 0, len(messages))
	for _, message := range messages {
		if message == nil {
			continue
		}
		file, ok := imageFileFromMessage(message)
		if ok {
			files = append(files, file)
		}
	}
	return files
}

type sizedPhoto interface {
	GetW() int
	GetH() int
	GetType() string
}

func imageFileFromMessage(message *tg.Message) (mediaFile, bool) {
	if message == nil || message.Media == nil {
		return mediaFile{}, false
	}
	switch media := message.Media.(type) {
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.AsNotEmpty()
		if !ok {
			return mediaFile{}, false
		}
		thumbSize := largestPhotoThumbSize(photo.Sizes)
		if thumbSize == "" {
			return mediaFile{}, false
		}
		return mediaFile{
			MessageID: message.ID,
			Name:      fmt.Sprintf("telegram-photo-%d.jpg", photo.ID),
			MIMEType:  "image/jpeg",
			Location: &tg.InputPhotoFileLocation{
				ID:            photo.ID,
				AccessHash:    photo.AccessHash,
				FileReference: photo.FileReference,
				ThumbSize:     thumbSize,
			},
		}, true
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.AsNotEmpty()
		if !ok || !documentIsImage(doc) {
			return mediaFile{}, false
		}
		return mediaFile{
			MessageID: message.ID,
			Name:      documentFileName(doc),
			MIMEType:  firstNonEmpty(doc.MimeType, "image/jpeg"),
			Location:  doc.AsInputDocumentFileLocation(),
		}, true
	default:
		return mediaFile{}, false
	}
}

func largestPhotoThumbSize(sizes []tg.PhotoSizeClass) string {
	bestType := ""
	bestArea := 0
	for _, size := range sizes {
		sized, ok := size.(sizedPhoto)
		if !ok {
			continue
		}
		area := sized.GetW() * sized.GetH()
		if area > bestArea {
			bestArea = area
			bestType = sized.GetType()
		}
	}
	return bestType
}

func documentIsImage(doc *tg.Document) bool {
	if strings.HasPrefix(strings.ToLower(doc.MimeType), "image/") {
		return true
	}
	for _, attr := range doc.Attributes {
		if _, ok := attr.(*tg.DocumentAttributeImageSize); ok {
			return true
		}
	}
	return false
}

func documentFileName(doc *tg.Document) string {
	for _, attr := range doc.Attributes {
		if filename, ok := attr.(*tg.DocumentAttributeFilename); ok && strings.TrimSpace(filename.FileName) != "" {
			return filename.FileName
		}
	}
	ext := ".jpg"
	switch strings.ToLower(doc.MimeType) {
	case "image/png":
		ext = ".png"
	case "image/webp":
		ext = ".webp"
	case "image/gif":
		ext = ".gif"
	}
	return fmt.Sprintf("telegram-document-%d%s", doc.ID, ext)
}

func telegramMediaURL(cfg SourceConfig, messageID int, position int) string {
	values := url.Values{}
	values.Set("type", cfg.Type)
	values.Set("peer_id", cfg.PeerID)
	values.Set("message_id", strconv.Itoa(messageID))
	values.Set("position", strconv.Itoa(position))
	values.Set("resolver", "direct-v2")
	if cfg.AccessHash != "" {
		values.Set("access_hash", cfg.AccessHash)
	}
	if cfg.Username != "" {
		values.Set("username", cfg.Username)
	}
	if cfg.SourceURL != "" {
		values.Set("source_url", cfg.SourceURL)
	}
	return "telegram://media?" + values.Encode()
}

func parseMediaURL(rawURL string) (mediaRequest, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return mediaRequest{}, err
	}
	if parsed.Scheme != "telegram" || parsed.Host != "media" {
		return mediaRequest{}, fmt.Errorf("unsupported telegram media URL %q", rawURL)
	}
	values := parsed.Query()
	messageID, err := strconv.Atoi(values.Get("message_id"))
	if err != nil || messageID <= 0 {
		return mediaRequest{}, errors.New("telegram media URL is missing message_id")
	}
	position := 0
	if rawPosition := strings.TrimSpace(values.Get("position")); rawPosition != "" {
		position, err = strconv.Atoi(rawPosition)
		if err != nil || position < 0 {
			return mediaRequest{}, errors.New("telegram media URL has invalid position")
		}
	}
	cfg := SourceConfig{
		Type:       values.Get("type"),
		PeerID:     values.Get("peer_id"),
		AccessHash: values.Get("access_hash"),
		Username:   values.Get("username"),
		SourceURL:  values.Get("source_url"),
	}
	if strings.TrimSpace(cfg.PeerID) == "" {
		return mediaRequest{}, errors.New("telegram media URL is missing peer_id")
	}
	return mediaRequest{
		Config:    cfg,
		MessageID: messageID,
		Position:  position,
	}, nil
}

func nextOffsetID(messages []Message) int {
	next := 0
	for _, message := range messages {
		if message.ID <= 0 {
			continue
		}
		if next == 0 || message.ID < next {
			next = message.ID
		}
	}
	return next
}

func messageURL(cfg SourceConfig, messageID int) string {
	if cfg.Username != "" {
		return fmt.Sprintf("https://t.me/%s/%d", cfg.Username, messageID)
	}
	if strings.HasPrefix(cfg.SourceURL, "https://t.me/") {
		return strings.TrimRight(cfg.SourceURL, "/") + "/" + strconv.Itoa(messageID)
	}
	if cfg.SourceURL != "" {
		return strings.TrimRight(cfg.SourceURL, "/") + "/message/" + strconv.Itoa(messageID)
	}
	return fmt.Sprintf("telegram://%s/%s/%d", firstNonEmpty(cfg.Type, "peer"), cfg.PeerID, messageID)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func SaveConfig(dataDir string, cfg Config) error {
	if cfg.APIID == 0 || strings.TrimSpace(cfg.APIHash) == "" {
		return errors.New("telegram api_id and api_hash are required")
	}
	cfg.APIHash = strings.TrimSpace(cfg.APIHash)
	cfg.Phone = strings.TrimSpace(cfg.Phone)
	cfg.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := os.MkdirAll(telegramDir(dataDir), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(dataDir), data, 0o600)
}

func (c Config) Configured() bool {
	return c.APIID != 0 && strings.TrimSpace(c.APIHash) != ""
}

func withClient(ctx context.Context, dataDir string, cfg Config, fn func(context.Context, *gotdtelegram.Client) error) error {
	if err := os.MkdirAll(telegramDir(dataDir), 0o700); err != nil {
		return err
	}
	client := gotdtelegram.NewClient(cfg.APIID, cfg.APIHash, gotdtelegram.Options{
		NoUpdates:      true,
		SessionStorage: &session.FileStorage{Path: sessionPath(dataDir)},
		Device: gotdtelegram.DeviceConfig{
			DeviceModel:    "Ludex Local",
			SystemVersion:  "local",
			AppVersion:     "0.1.0",
			SystemLangCode: "en",
			LangCode:       "en",
		},
	})
	return client.Run(ctx, func(ctx context.Context) error {
		return fn(ctx, client)
	})
}

func statusFromAuth(authStatus *auth.Status, cfg Config) Status {
	status := Status{Configured: cfg.Configured()}
	if authStatus == nil || !authStatus.Authorized || authStatus.User == nil {
		return status
	}
	status.Authorized = true
	status.UserID = authStatus.User.ID
	if username, ok := authStatus.User.GetUsername(); ok {
		status.Username = username
	}
	if phone, ok := authStatus.User.GetPhone(); ok {
		status.Phone = phone
	}
	if firstName, ok := authStatus.User.GetFirstName(); ok {
		status.FirstName = firstName
	}
	if lastName, ok := authStatus.User.GetLastName(); ok {
		status.LastName = lastName
	}
	return status
}

func phoneCodeHash(sent tg.AuthSentCodeClass) (string, bool) {
	withHash, ok := sent.(interface {
		GetPhoneCodeHash() string
	})
	if !ok {
		return "", false
	}
	return withHash.GetPhoneCodeHash(), true
}

func sentCodeTimeout(sent tg.AuthSentCodeClass) int {
	withTimeout, ok := sent.(interface {
		GetTimeout() (int, bool)
	})
	if !ok {
		return 0
	}
	timeout, ok := withTimeout.GetTimeout()
	if !ok {
		return 0
	}
	return timeout
}

func dialogsFromChats(chats []tg.ChatClass) []Dialog {
	dialogs := make([]Dialog, 0, len(chats))
	for _, chat := range chats {
		switch value := chat.(type) {
		case *tg.Chat:
			if value.Left || value.Deactivated {
				continue
			}
			dialogs = append(dialogs, newDialog(value.ID, "group", value.Title, "", 0, value.ParticipantsCount, value.Date))
		case *tg.Channel:
			if value.Left {
				continue
			}
			kind := "channel"
			if value.Megagroup {
				kind = "supergroup"
			}
			accessHash, _ := value.GetAccessHash()
			username, _ := value.GetUsername()
			participants, _ := value.GetParticipantsCount()
			dialogs = append(dialogs, newDialog(value.ID, kind, value.Title, username, accessHash, participants, value.Date))
		case *tg.ChatForbidden:
			dialogs = append(dialogs, newDialog(value.ID, "group", value.Title, "", 0, 0, 0))
		case *tg.ChannelForbidden:
			kind := "channel"
			if value.Megagroup {
				kind = "supergroup"
			}
			dialogs = append(dialogs, newDialog(value.ID, kind, value.Title, "", value.AccessHash, 0, 0))
		}
	}
	return dialogs
}

func newDialog(id int64, kind string, title string, username string, accessHash int64, participants int, date int) Dialog {
	dialog := Dialog{
		PeerID:       strconv.FormatInt(id, 10),
		Type:         kind,
		Title:        strings.TrimSpace(title),
		Username:     strings.TrimSpace(username),
		Participants: participants,
		SourceURL:    telegramSourceURL(kind, id, username),
	}
	if accessHash != 0 {
		dialog.AccessHash = strconv.FormatInt(accessHash, 10)
	}
	if date > 0 {
		dialog.Date = time.Unix(int64(date), 0).UTC().Format(time.RFC3339)
	}
	configJSON, _ := json.Marshal(map[string]any{
		"peer_id":     dialog.PeerID,
		"type":        dialog.Type,
		"title":       dialog.Title,
		"username":    dialog.Username,
		"access_hash": dialog.AccessHash,
		"source_url":  dialog.SourceURL,
	})
	dialog.ConfigJSON = string(configJSON)
	return dialog
}

func telegramSourceURL(kind string, id int64, username string) string {
	if username != "" {
		return "https://t.me/" + username
	}
	return fmt.Sprintf("telegram://%s/%d", kind, id)
}

func loadConfig(dataDir string) (Config, error) {
	data, err := os.ReadFile(configPath(dataDir))
	if os.IsNotExist(err) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func savePendingCode(dataDir string, pending pendingCode) error {
	if err := os.MkdirAll(telegramDir(dataDir), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(pending, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(pendingCodePath(dataDir), data, 0o600)
}

func loadPendingCode(dataDir string) (pendingCode, error) {
	data, err := os.ReadFile(pendingCodePath(dataDir))
	if os.IsNotExist(err) {
		return pendingCode{}, errors.New("send a Telegram login code first")
	}
	if err != nil {
		return pendingCode{}, err
	}
	var pending pendingCode
	if err := json.Unmarshal(data, &pending); err != nil {
		return pendingCode{}, err
	}
	if pending.Phone == "" || pending.PhoneCodeHash == "" {
		return pendingCode{}, errors.New("saved Telegram login code state is incomplete")
	}
	return pending, nil
}

func telegramDir(dataDir string) string {
	return filepath.Join(dataDir, "telegram")
}

func configPath(dataDir string) string {
	return filepath.Join(telegramDir(dataDir), "config.json")
}

func sessionPath(dataDir string) string {
	return filepath.Join(telegramDir(dataDir), "session.bin")
}

func pendingCodePath(dataDir string) string {
	return filepath.Join(telegramDir(dataDir), "pending-code.json")
}
