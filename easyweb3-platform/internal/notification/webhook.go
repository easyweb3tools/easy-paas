package notification

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type WebhookSender struct {
	HTTP *http.Client
}

type WebhookPayload struct {
	Project string `json:"project"`
	Event   string `json:"event"`
	Message string `json:"message"`
}

func (s WebhookSender) Send(ctx context.Context, url string, payload WebhookPayload) error {
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	client := s.HTTP
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &httpError{StatusCode: resp.StatusCode}
	}
	return nil
}

type httpError struct {
	StatusCode int
}

func (e *httpError) Error() string {
	return "webhook http status " + http.StatusText(e.StatusCode)
}
