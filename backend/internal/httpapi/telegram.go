package httpapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/BD777/ludex/backend/internal/domain"
	telegramsource "github.com/BD777/ludex/backend/internal/source/telegram"
)

type telegramConfigRequest struct {
	APIID   int    `json:"api_id"`
	APIHash string `json:"api_hash"`
	Phone   string `json:"phone"`
}

type telegramSignInRequest struct {
	APIID    int    `json:"api_id"`
	APIHash  string `json:"api_hash"`
	Phone    string `json:"phone"`
	Code     string `json:"code"`
	Password string `json:"password"`
}

type createTelegramSourceRequest struct {
	Dialog     telegramsource.Dialog `json:"dialog"`
	TrustLevel int                   `json:"trust_level"`
	Enabled    *bool                 `json:"enabled"`
}

func (s *Server) telegramStatus(w http.ResponseWriter, r *http.Request) {
	status, err := telegramsource.StatusFor(r.Context(), s.store.DataDir(), telegramConfigFromQuery(r))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) telegramSendCode(w http.ResponseWriter, r *http.Request) {
	var req telegramConfigRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	result, err := telegramsource.SendCode(r.Context(), s.store.DataDir(), telegramsource.Config{
		APIID:   req.APIID,
		APIHash: req.APIHash,
		Phone:   req.Phone,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) telegramSignIn(w http.ResponseWriter, r *http.Request) {
	var req telegramSignInRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	result, err := telegramsource.SignIn(r.Context(), s.store.DataDir(), telegramsource.Config{
		APIID:   req.APIID,
		APIHash: req.APIHash,
		Phone:   req.Phone,
	}, req.Code, req.Password)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) telegramDialogs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	dialogs, err := telegramsource.ListDialogs(r.Context(), s.store.DataDir(), telegramConfigFromQuery(r), limit)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, dialogs)
}

func (s *Server) createTelegramSource(w http.ResponseWriter, r *http.Request) {
	var req createTelegramSourceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, err)
		return
	}
	if strings.TrimSpace(req.Dialog.Title) == "" {
		writeError(w, errors.New("dialog title is required"))
		return
	}
	if strings.TrimSpace(req.Dialog.SourceURL) == "" {
		writeError(w, errors.New("dialog source_url is required"))
		return
	}
	sources, err := s.store.ListSources(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	for _, source := range sources {
		if source.Type == "telegram" && source.URL == req.Dialog.SourceURL {
			writeJSON(w, http.StatusOK, source)
			return
		}
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	trustLevel := req.TrustLevel
	if trustLevel == 0 {
		trustLevel = 50
	}
	configJSON := req.Dialog.ConfigJSON
	if strings.TrimSpace(configJSON) == "" {
		configJSON = "{}"
	}
	source, err := s.store.CreateSource(r.Context(), domain.Source{
		Name:       req.Dialog.Title,
		Type:       "telegram",
		URL:        req.Dialog.SourceURL,
		Enabled:    enabled,
		TrustLevel: trustLevel,
		ConfigJSON: configJSON,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, source)
}

func telegramConfigFromQuery(r *http.Request) telegramsource.Config {
	apiID, _ := strconv.Atoi(r.URL.Query().Get("api_id"))
	return telegramsource.Config{
		APIID:   apiID,
		APIHash: r.URL.Query().Get("api_hash"),
		Phone:   r.URL.Query().Get("phone"),
	}
}
