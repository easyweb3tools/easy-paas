package notification

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/nicekwell/easyweb3-platform/internal/auth"
	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Handler struct {
	Store   *FileStore
	Webhook WebhookSender
	TG      TelegramSender
}

type sendRequest struct {
	Channel string `json:"channel"`
	To      string `json:"to"`
	Message string `json:"message"`
	Event   string `json:"event"`
}

type sendResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (h Handler) Send(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}

	var req sendRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Channel = strings.TrimSpace(req.Channel)
	req.To = strings.TrimSpace(req.To)
	req.Message = strings.TrimSpace(req.Message)
	req.Event = strings.TrimSpace(req.Event)

	if req.Channel == "" {
		httpx.WriteError(w, http.StatusBadRequest, "channel required")
		return
	}
	if req.To == "" {
		httpx.WriteError(w, http.StatusBadRequest, "to required")
		return
	}
	if req.Message == "" {
		httpx.WriteError(w, http.StatusBadRequest, "message required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.sendOne(ctx, c.ProjectID, req.Channel, req.To, req.Message, req.Event, nil); err != nil {
		httpx.WriteJSON(w, http.StatusOK, sendResult{OK: false, Error: err.Error()})
		return
	}
	httpx.WriteJSON(w, http.StatusOK, sendResult{OK: true})
}

type broadcastRequest struct {
	Message string `json:"message"`
	Event   string `json:"event"`
}

type broadcastItem struct {
	Channel string `json:"channel"`
	Target  string `json:"target"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
}

type broadcastResponse struct {
	Project string          `json:"project"`
	Event   string          `json:"event"`
	Items   []broadcastItem `json:"items"`
}

func (h Handler) Broadcast(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}

	var req broadcastRequest
	if err := httpx.ReadJSON(r, &req, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	req.Message = strings.TrimSpace(req.Message)
	req.Event = strings.TrimSpace(req.Event)
	if req.Message == "" {
		httpx.WriteError(w, http.StatusBadRequest, "message required")
		return
	}

	cfg, ok := h.Store.Get(c.ProjectID)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "notify config not found")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()

	resp := broadcastResponse{Project: c.ProjectID, Event: req.Event}
	for _, ch := range cfg.Channels {
		if !eventMatch(ch.Events, req.Event) {
			continue
		}
		var target string
		switch strings.ToLower(strings.TrimSpace(ch.Type)) {
		case "webhook":
			target = strings.TrimSpace(ch.URL)
		case "telegram":
			target = strings.TrimSpace(ch.ChatID)
		default:
			resp.Items = append(resp.Items, broadcastItem{Channel: ch.Type, Target: "", OK: false, Error: "unsupported channel"})
			continue
		}

		err := h.sendOne(ctx, c.ProjectID, ch.Type, target, req.Message, req.Event, &ch)
		if err != nil {
			resp.Items = append(resp.Items, broadcastItem{Channel: ch.Type, Target: target, OK: false, Error: err.Error()})
			continue
		}
		resp.Items = append(resp.Items, broadcastItem{Channel: ch.Type, Target: target, OK: true})
	}

	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h Handler) GetConfig(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}
	cfg, ok := h.Store.Get(c.ProjectID)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "notify config not found")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cfg)
}

func (h Handler) PutConfig(w http.ResponseWriter, r *http.Request) {
	c, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "missing token")
		return
	}

	var cfg ProjectConfig
	if err := httpx.ReadJSON(r, &cfg, 1<<20); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, err.Error())
		return
	}
	cfg.Project = c.ProjectID
	if cfg.Channels == nil {
		cfg.Channels = []ChannelConfig{}
	}

	for i := range cfg.Channels {
		cfg.Channels[i].Type = strings.ToLower(strings.TrimSpace(cfg.Channels[i].Type))
		for j := range cfg.Channels[i].Events {
			cfg.Channels[i].Events[j] = strings.TrimSpace(cfg.Channels[i].Events[j])
		}
	}

	if err := h.Store.Put(c.ProjectID, cfg); err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "failed to store notify config")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, cfg)
}

func (h Handler) sendOne(ctx context.Context, project, channel, to, message, event string, cfg *ChannelConfig) error {
	switch strings.ToLower(strings.TrimSpace(channel)) {
	case "webhook":
		url := to
		if cfg != nil && strings.TrimSpace(cfg.URL) != "" {
			url = strings.TrimSpace(cfg.URL)
		}
		if strings.TrimSpace(url) == "" {
			return errors.New("webhook url missing")
		}
		return h.Webhook.Send(ctx, url, WebhookPayload{Project: project, Event: event, Message: message})
	case "telegram":
		chatID := to
		botToken := ""
		if cfg != nil {
			if strings.TrimSpace(cfg.ChatID) != "" {
				chatID = strings.TrimSpace(cfg.ChatID)
			}
			botToken = strings.TrimSpace(cfg.BotToken)
		}
		if chatID == "" {
			return errors.New("telegram chat_id missing")
		}
		if botToken == "" {
			return errors.New("telegram bot_token missing")
		}
		return h.TG.Send(ctx, botToken, chatID, message)
	default:
		return errors.New("unsupported channel")
	}
}

func eventMatch(events []string, event string) bool {
	// Empty events means allow all.
	if len(events) == 0 {
		return true
	}
	event = strings.TrimSpace(event)
	for _, e := range events {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if e == "*" {
			return true
		}
		if event != "" && e == event {
			return true
		}
	}
	return false
}
