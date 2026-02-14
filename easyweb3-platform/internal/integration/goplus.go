package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type GoPlus struct {
	BaseURL string
	APIKey  string
	HTTP    *http.Client
	Cache   cacheStore
	TTL     time.Duration
}

func (g GoPlus) Query(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case "token_security", "token-security", "tokenSecurity":
		chainID := getString(params, "chain_id")
		addrs := getString(params, "contract_addresses")
		if strings.TrimSpace(chainID) == "" {
			return nil, errors.New("params.chain_id required")
		}
		if strings.TrimSpace(addrs) == "" {
			return nil, errors.New("params.contract_addresses required")
		}
		path := fmt.Sprintf("/api/v1/token_security/%s", url.PathEscape(chainID))
		u, err := g.buildURL(path, map[string]string{"contract_addresses": addrs})
		if err != nil {
			return nil, err
		}
		k := cacheKey("goplus", "token_security", map[string]string{"chain_id": chainID, "contract_addresses": addrs})
		return g.get(ctx, k, u)
	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

func (g GoPlus) buildURL(path string, query map[string]string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(g.BaseURL), "/")
	if base == "" {
		base = "https://api.gopluslabs.io"
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + path
	q := u.Query()
	for k, v := range query {
		q.Set(k, v)
	}
	if strings.TrimSpace(g.APIKey) != "" {
		q.Set("api_key", strings.TrimSpace(g.APIKey))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (g GoPlus) get(ctx context.Context, key string, u string) (json.RawMessage, error) {
	if g.Cache != nil && key != "" {
		if b, found, err := g.Cache.Get(ctx, key); err == nil && found && json.Valid(b) {
			return json.RawMessage(b), nil
		}
	}

	client := g.HTTP
	if client == nil {
		client = &http.Client{Timeout: 8 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("goplus http %d", resp.StatusCode)
	}

	if g.Cache != nil && key != "" && json.Valid(b) {
		ttl := g.TTL
		if ttl <= 0 {
			ttl = 30 * time.Second
		}
		_ = g.Cache.Set(ctx, key, b, ttl)
	}
	return json.RawMessage(b), nil
}
