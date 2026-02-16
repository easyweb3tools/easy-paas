package clob

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type TradingAuth struct {
	APIKeyHeader string
	APIKey       string
	BearerToken  string
	APISecret    string
	SignRequests bool
	// Optional headers for HMAC mode.
	TimestampHeader  string
	SignatureHeader  string
	Passphrase       string
	PassphraseHeader string
	Address          string
	AddressHeader    string
}

type PlaceOrderRequest struct {
	TokenID       string  `json:"token_id"`
	Side          string  `json:"side"`
	OrderType     string  `json:"order_type,omitempty"`
	Price         float64 `json:"price"`
	SizeUSD       float64 `json:"size_usd"`
	ClientOrderID string  `json:"client_order_id,omitempty"`
	PlanID        uint64  `json:"plan_id,omitempty"`
}

type PlaceSignedOrderRequest struct {
	Order     any    `json:"order"`
	Owner     string `json:"owner,omitempty"`
	OrderType string `json:"orderType,omitempty"`
	PostOnly  *bool  `json:"postOnly,omitempty"`
}

type TradingOrder struct {
	OrderID     string
	Status      string
	FilledUSD   float64
	AvgPrice    float64
	Fee         float64
	Failure     string
	SubmittedAt *time.Time
	FilledAt    *time.Time
	CancelledAt *time.Time
}

func (c *Client) PlaceOrder(ctx context.Context, path string, req PlaceOrderRequest, auth TradingAuth) (*TradingOrder, error) {
	path = normalizePath(path, "/orders")
	body, err := c.doJSON(ctx, http.MethodPost, path, nil, req, auth)
	if err != nil {
		return nil, err
	}
	return parseTradingOrder(body)
}

func (c *Client) PlaceSignedOrder(ctx context.Context, path string, req PlaceSignedOrderRequest, auth TradingAuth) (*TradingOrder, error) {
	path = normalizePath(path, "/order")
	body, err := c.doJSON(ctx, http.MethodPost, path, nil, req, auth)
	if err != nil {
		return nil, err
	}
	return parseTradingOrder(body)
}

func (c *Client) GetOrder(ctx context.Context, pathTemplate, orderID string, auth TradingAuth) (*TradingOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("order id is required")
	}
	path := renderOrderPath(pathTemplate, "/orders/{order_id}", orderID)
	body, err := c.doJSON(ctx, http.MethodGet, path, nil, nil, auth)
	if err != nil {
		return nil, err
	}
	return parseTradingOrder(body)
}

func (c *Client) CancelOrder(ctx context.Context, pathTemplate, orderID string, auth TradingAuth) (*TradingOrder, error) {
	orderID = strings.TrimSpace(orderID)
	if orderID == "" {
		return nil, fmt.Errorf("order id is required")
	}
	path := renderOrderPath(pathTemplate, "/orders/{order_id}/cancel", orderID)
	body, err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]any{}, auth)
	if err != nil {
		return nil, err
	}
	return parseTradingOrder(body)
}

func (c *Client) doJSON(ctx context.Context, method, path string, query url.Values, payload any, auth TradingAuth) ([]byte, error) {
	if c == nil || c.httpClient == nil {
		return nil, fmt.Errorf("client is nil")
	}
	fullURL := c.host + normalizePath(path, "")
	if query != nil && len(query) > 0 {
		fullURL += "?" + query.Encode()
	}
	var body io.Reader
	bodyRaw := []byte{}
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyRaw = raw
		body = bytes.NewReader(raw)
	}
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if v := strings.TrimSpace(auth.APIKey); v != "" {
		h := strings.TrimSpace(auth.APIKeyHeader)
		if h == "" {
			h = "X-API-Key"
		}
		req.Header.Set(h, v)
	}
	if v := strings.TrimSpace(auth.BearerToken); v != "" {
		req.Header.Set("Authorization", "Bearer "+v)
	}
	if v := strings.TrimSpace(auth.Passphrase); v != "" {
		h := strings.TrimSpace(auth.PassphraseHeader)
		if h == "" {
			h = "X-Passphrase"
		}
		req.Header.Set(h, v)
	}
	if v := strings.TrimSpace(auth.Address); v != "" {
		h := strings.TrimSpace(auth.AddressHeader)
		if h == "" {
			h = "X-Address"
		}
		req.Header.Set(h, v)
	}
	if auth.SignRequests && strings.TrimSpace(auth.APISecret) != "" {
		ts := strconv.FormatInt(time.Now().UTC().Unix(), 10)
		th := strings.TrimSpace(auth.TimestampHeader)
		if th == "" {
			th = "X-Timestamp"
		}
		sh := strings.TrimSpace(auth.SignatureHeader)
		if sh == "" {
			sh = "X-Signature"
		}
		canonicalPath := normalizePath(path, "")
		if query != nil && len(query) > 0 {
			canonicalPath += "?" + query.Encode()
		}
		payloadToSign := ts + "\n" + strings.ToUpper(strings.TrimSpace(method)) + "\n" + canonicalPath + "\n" + string(bodyRaw)
		mac := hmac.New(sha256.New, []byte(auth.APISecret))
		_, _ = mac.Write([]byte(payloadToSign))
		sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
		req.Header.Set(th, ts)
		req.Header.Set(sh, sig)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Body: string(respBody)}
	}
	return respBody, nil
}

func normalizePath(path, fallback string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		path = fallback
	}
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func renderOrderPath(pathTemplate, fallback, orderID string) string {
	path := normalizePath(pathTemplate, fallback)
	path = strings.ReplaceAll(path, "{order_id}", url.PathEscape(orderID))
	path = strings.ReplaceAll(path, ":order_id", url.PathEscape(orderID))
	if strings.Contains(path, "{order_id}") || strings.Contains(path, ":order_id") {
		return normalizePath(fallback, fallback)
	}
	return path
}

func parseTradingOrder(raw []byte) (*TradingOrder, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, err
	}
	// common envelopes: {data:{...}} or {...}
	if data, ok := root["data"].(map[string]any); ok {
		root = data
	}
	if order, ok := root["order"].(map[string]any); ok {
		root = order
	}
	out := &TradingOrder{}
	out.OrderID = firstString(root, "order_id", "id", "clob_order_id")
	out.Status = strings.ToLower(strings.TrimSpace(firstString(root, "status", "state")))
	out.Failure = firstString(root, "failure_reason", "error", "message")
	out.FilledUSD = firstFloat(root, "filled_usd", "filled_value", "filled")
	out.AvgPrice = firstFloat(root, "avg_price", "average_price", "price")
	out.Fee = firstFloat(root, "fee", "fees")
	out.SubmittedAt = firstTime(root, "submitted_at", "created_at")
	out.FilledAt = firstTime(root, "filled_at", "done_at")
	out.CancelledAt = firstTime(root, "cancelled_at", "canceled_at")
	if out.OrderID == "" {
		return nil, fmt.Errorf("order id missing in response")
	}
	return out, nil
}

func firstString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			s := strings.TrimSpace(fmt.Sprintf("%v", v))
			if s != "" && s != "<nil>" {
				return s
			}
		}
	}
	return ""
}

func firstFloat(m map[string]any, keys ...string) float64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "" || s == "<nil>" {
			continue
		}
		f, err := strconv.ParseFloat(s, 64)
		if err == nil {
			return f
		}
	}
	return 0
}

func firstTime(m map[string]any, keys ...string) *time.Time {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		s := strings.TrimSpace(fmt.Sprintf("%v", v))
		if s == "" || s == "<nil>" {
			continue
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			u := t.UTC()
			return &u
		}
	}
	return nil
}
