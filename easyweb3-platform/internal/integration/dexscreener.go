package integration

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

type Dexscreener struct {
	BaseURL string
	HTTP    *http.Client
	Cache   cacheStore
	TTL     time.Duration
}

func (d Dexscreener) Query(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case "search":
		q := getString(params, "q")
		if strings.TrimSpace(q) == "" {
			return nil, errors.New("params.q required")
		}
		path := "/latest/dex/search"
		u, err := d.buildURL(path, map[string]string{"q": q})
		if err != nil {
			return nil, err
		}
		return d.get(ctx, cacheKey("dexscreener", "search", map[string]string{"q": q}), u)

	case "pairs", "getpairs", "get-pairs":
		chain := getString(params, "chain")
		pair := getString(params, "pair_address")
		if strings.TrimSpace(chain) == "" {
			return nil, errors.New("params.chain required")
		}
		if strings.TrimSpace(pair) == "" {
			return nil, errors.New("params.pair_address required")
		}
		path := fmt.Sprintf("/latest/dex/pairs/%s/%s", url.PathEscape(chain), url.PathEscape(pair))
		u, err := d.buildURL(path, nil)
		if err != nil {
			return nil, err
		}
		return d.get(ctx, cacheKey("dexscreener", "pairs", map[string]string{"chain": chain, "pair": pair}), u)

	case "token", "gettoken", "get-token":
		addr := getString(params, "token_address")
		if strings.TrimSpace(addr) == "" {
			return nil, errors.New("params.token_address required")
		}
		path := fmt.Sprintf("/latest/dex/tokens/%s", url.PathEscape(addr))
		u, err := d.buildURL(path, nil)
		if err != nil {
			return nil, err
		}
		return d.get(ctx, cacheKey("dexscreener", "token", map[string]string{"token": addr}), u)

	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

func (d Dexscreener) buildURL(path string, query map[string]string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(d.BaseURL), "/")
	if base == "" {
		base = "https://api.dexscreener.com"
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
	u.RawQuery = q.Encode()
	return u.String(), nil
}

type cacheStore interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

func (d Dexscreener) get(ctx context.Context, key string, u string) (json.RawMessage, error) {
	if d.Cache != nil && key != "" {
		if b, found, err := d.Cache.Get(ctx, key); err == nil && found && json.Valid(b) {
			return json.RawMessage(b), nil
		}
	}

	client := d.HTTP
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
		return nil, fmt.Errorf("dexscreener http %d", resp.StatusCode)
	}

	if d.Cache != nil && key != "" && json.Valid(b) {
		ttl := d.TTL
		if ttl <= 0 {
			ttl = 30 * time.Second
		}
		_ = d.Cache.Set(ctx, key, b, ttl)
	}
	return json.RawMessage(b), nil
}

func getString(m map[string]any, k string) string {
	if m == nil {
		return ""
	}
	v, ok := m[k]
	if !ok || v == nil {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return ""
}

func cacheKey(provider, method string, parts map[string]string) string {
	// Deterministic, readable key (good enough for MVP).
	// Example: int:dexscreener:search:q=pepe
	sb := strings.Builder{}
	sb.WriteString("int:")
	sb.WriteString(provider)
	sb.WriteString(":")
	sb.WriteString(method)
	if len(parts) > 0 {
		keys := make([]string, 0, len(parts))
		for k := range parts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			sb.WriteString(":")
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(parts[k])
		}
	}
	return sb.String()
}
