package paas

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Client struct {
	BaseURL string
	APIKey  string

	mu        sync.RWMutex
	token     string
	expiresAt time.Time

	HTTP *http.Client
}

type loginResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func (c *Client) Login(ctx context.Context) error {
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	if base == "" {
		return errors.New("paas base url is empty")
	}
	apiKey := strings.TrimSpace(c.APIKey)
	if apiKey == "" {
		return errors.New("paas api key is empty")
	}

	body, _ := json.Marshal(map[string]any{"api_key": apiKey})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/auth/login", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("paas login http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	var lr loginResponse
	if err := json.Unmarshal(b, &lr); err != nil {
		return err
	}
	exp, _ := time.Parse(time.RFC3339, strings.TrimSpace(lr.ExpiresAt))

	c.mu.Lock()
	c.token = strings.TrimSpace(lr.Token)
	c.expiresAt = exp
	c.mu.Unlock()
	return nil
}

func (c *Client) Token() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.token
}

func (c *Client) EnsureToken(ctx context.Context) error {
	c.mu.RLock()
	tok := c.token
	exp := c.expiresAt
	c.mu.RUnlock()
	if strings.TrimSpace(tok) == "" {
		return c.Login(ctx)
	}
	if !exp.IsZero() && time.Until(exp) < 2*time.Minute {
		return c.Login(ctx)
	}
	return nil
}

type CreateLogRequest struct {
	Agent      string         `json:"agent"`
	Action     string         `json:"action"`
	Level      string         `json:"level"`
	Details    map[string]any `json:"details"`
	SessionKey string         `json:"session_key"`
	Metadata   map[string]any `json:"metadata"`
}

func (c *Client) CreateLog(ctx context.Context, req CreateLogRequest) error {
	if err := c.EnsureToken(ctx); err != nil {
		return err
	}
	base := strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	b, err := json.Marshal(req)
	if err != nil {
		return err
	}
	hreq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/api/v1/logs", bytes.NewReader(b))
	if err != nil {
		return err
	}
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("Authorization", "Bearer "+c.Token())

	resp, err := c.httpClient().Do(hreq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		bb, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("paas create log http %d: %s", resp.StatusCode, strings.TrimSpace(string(bb)))
	}
	return nil
}

func (c *Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 10 * time.Second}
}
