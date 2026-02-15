package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Polymarket exposes a stable "method/params" interface for common polymarket upstream routes.
// It is intentionally conservative: a small set of methods with strict param validation,
// stable output (pass-through JSON), and optional caching for GET requests.
type Polymarket struct {
	BaseURL string
	HTTP    *http.Client
	Cache   cacheStore
	TTL     time.Duration
}

func (p Polymarket) Query(ctx context.Context, method string, params map[string]any) (json.RawMessage, error) {
	method = strings.ToLower(strings.TrimSpace(method))
	switch method {
	case "healthz", "health":
		u, err := p.buildURL("/healthz", nil)
		if err != nil {
			return nil, err
		}
		return p.get(ctx, cacheKey("polymarket", "healthz", nil), u)

	// Align with CLI ops first; keep dot aliases for compatibility.
	case "opportunities", "opportunities.list":
		q := map[string]string{
			"limit":    itoa(getInt(params, "limit", 50)),
			"offset":   itoa(getInt(params, "offset", 0)),
			"status":   getString(params, "status"),
			"strategy": getString(params, "strategy"),
			"category": getString(params, "category"),
		}
		trimEmpty(q)
		u, err := p.buildURL("/api/v2/opportunities", q)
		if err != nil {
			return nil, err
		}
		return p.get(ctx, cacheKey("polymarket", "opportunities", q), u)

	case "opportunity-get", "opportunity.get":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			id = getString(params, "opportunity_id")
		}
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		u, err := p.buildURL("/api/v2/opportunities/"+url.PathEscape(id), nil)
		if err != nil {
			return nil, err
		}
		return p.get(ctx, cacheKey("polymarket", "opportunity_get", map[string]string{"id": id}), u)

	case "opportunity-dismiss", "opportunity.dismiss":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			id = getString(params, "opportunity_id")
		}
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		u, err := p.buildURL("/api/v2/opportunities/"+url.PathEscape(id)+"/dismiss", nil)
		if err != nil {
			return nil, err
		}
		return p.post(ctx, "", u, map[string]any{})

	case "opportunity-execute", "opportunity.execute":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			id = getString(params, "opportunity_id")
		}
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		u, err := p.buildURL("/api/v2/opportunities/"+url.PathEscape(id)+"/execute", nil)
		if err != nil {
			return nil, err
		}
		return p.post(ctx, "", u, map[string]any{})

	case "catalog-events", "catalog.events", "catalog.events.list":
		q := map[string]string{
			"limit":  itoa(getInt(params, "limit", 50)),
			"offset": itoa(getInt(params, "offset", 0)),
			"active": getString(params, "active"),
			"closed": getString(params, "closed"),
		}
		trimEmpty(q)
		u, err := p.buildURL("/api/catalog/events", q)
		if err != nil {
			return nil, err
		}
		return p.get(ctx, cacheKey("polymarket", "catalog_events", q), u)

	case "catalog-markets", "catalog.markets", "catalog.markets.list":
		q := map[string]string{
			"limit":    itoa(getInt(params, "limit", 50)),
			"offset":   itoa(getInt(params, "offset", 0)),
			"event_id": getString(params, "event_id"),
			"active":   getString(params, "active"),
			"closed":   getString(params, "closed"),
		}
		// Accept event-id alias for convenience.
		if strings.TrimSpace(q["event_id"]) == "" {
			q["event_id"] = getString(params, "event-id")
		}
		trimEmpty(q)
		u, err := p.buildURL("/api/catalog/markets", q)
		if err != nil {
			return nil, err
		}
		return p.get(ctx, cacheKey("polymarket", "catalog_markets", q), u)

	case "catalog-sync", "catalog.sync":
		q := map[string]string{
			"scope":     getString(params, "scope"),
			"limit":     itoa(getInt(params, "limit", 0)),
			"max_pages": itoa(getInt(params, "max_pages", 0)),
			"resume":    getString(params, "resume"),
			"tag_id":    itoa(getInt(params, "tag_id", 0)),
			"closed":    getString(params, "closed"),
		}
		// Aliases for CLI-style flags.
		if strings.TrimSpace(q["max_pages"]) == "" {
			q["max_pages"] = itoa(getInt(params, "max-pages", 0))
		}
		if strings.TrimSpace(q["tag_id"]) == "" {
			q["tag_id"] = itoa(getInt(params, "tag-id", 0))
		}
		if strings.TrimSpace(q["scope"]) == "" {
			q["scope"] = "all"
		}
		// default resume to true if not specified
		if strings.TrimSpace(q["resume"]) == "" {
			q["resume"] = "true"
		}
		trimEmptyZeroLike(q)
		u, err := p.buildURL("/api/catalog/sync", q)
		if err != nil {
			return nil, err
		}
		// Upstream expects POST, empty body.
		return p.post(ctx, "", u, nil)

	case "executions":
		q := map[string]string{
			"limit":  itoa(getInt(params, "limit", 50)),
			"offset": itoa(getInt(params, "offset", 0)),
			"status": getString(params, "status"),
		}
		trimEmpty(q)
		u, err := p.buildURL("/api/v2/executions", q)
		if err != nil {
			return nil, err
		}
		return p.get(ctx, cacheKey("polymarket", "executions", q), u)

	case "execution-get", "execution.get":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		u, err := p.buildURL("/api/v2/executions/"+url.PathEscape(id), nil)
		if err != nil {
			return nil, err
		}
		return p.get(ctx, cacheKey("polymarket", "execution_get", map[string]string{"id": id}), u)

	case "execution-preflight", "execution.preflight":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		u, err := p.buildURL("/api/v2/executions/"+url.PathEscape(id)+"/preflight", nil)
		if err != nil {
			return nil, err
		}
		return p.post(ctx, "", u, map[string]any{})

	case "execution-mark-executing", "execution.mark-executing":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		u, err := p.buildURL("/api/v2/executions/"+url.PathEscape(id)+"/mark-executing", nil)
		if err != nil {
			return nil, err
		}
		return p.post(ctx, "", u, map[string]any{})

	case "execution-mark-executed", "execution.mark-executed":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		u, err := p.buildURL("/api/v2/executions/"+url.PathEscape(id)+"/mark-executed", nil)
		if err != nil {
			return nil, err
		}
		return p.post(ctx, "", u, map[string]any{})

	case "execution-cancel", "execution.cancel":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		u, err := p.buildURL("/api/v2/executions/"+url.PathEscape(id)+"/cancel", nil)
		if err != nil {
			return nil, err
		}
		return p.post(ctx, "", u, map[string]any{})

	case "execution-fill", "execution.fill":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		body := map[string]any{
			"token_id":    strings.TrimSpace(getString(params, "token_id")),
			"direction":   strings.TrimSpace(getString(params, "direction")),
			"filled_size": strings.TrimSpace(getString(params, "filled_size")),
			"avg_price":   strings.TrimSpace(getString(params, "avg_price")),
			"fee":         strings.TrimSpace(getString(params, "fee")),
			"slippage":    strings.TrimSpace(getString(params, "slippage")),
			"filled_at":   strings.TrimSpace(getString(params, "filled_at")),
		}
		// Accept CLI-style param aliases.
		if body["token_id"] == "" {
			body["token_id"] = strings.TrimSpace(getString(params, "token-id"))
		}
		if body["filled_size"] == "" {
			body["filled_size"] = strings.TrimSpace(getString(params, "filled-size"))
		}
		if body["avg_price"] == "" {
			body["avg_price"] = strings.TrimSpace(getString(params, "avg-price"))
		}
		if body["filled_at"] == "" {
			body["filled_at"] = strings.TrimSpace(getString(params, "filled-at"))
		}
		if strings.TrimSpace(body["token_id"].(string)) == "" || strings.TrimSpace(body["direction"].(string)) == "" ||
			strings.TrimSpace(body["filled_size"].(string)) == "" || strings.TrimSpace(body["avg_price"].(string)) == "" {
			return nil, errors.New("params.token_id, params.direction, params.filled_size, params.avg_price required")
		}
		u, err := p.buildURL("/api/v2/executions/"+url.PathEscape(id)+"/fill", nil)
		if err != nil {
			return nil, err
		}
		return p.post(ctx, "", u, body)

	case "execution-settle", "execution.settle":
		id := getString(params, "id")
		if strings.TrimSpace(id) == "" {
			return nil, errors.New("params.id required")
		}
		raw, ok := params["body"]
		if !ok {
			raw = map[string]any{}
		}
		u, err := p.buildURL("/api/v2/executions/"+url.PathEscape(id)+"/settle", nil)
		if err != nil {
			return nil, err
		}
		return p.post(ctx, "", u, raw)

	default:
		return nil, fmt.Errorf("unsupported method: %s", method)
	}
}

func (p Polymarket) buildURL(path string, query map[string]string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
	if base == "" {
		return "", errors.New("polymarket base_url is empty (service not configured)")
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

func (p Polymarket) httpClient() *http.Client {
	if p.HTTP != nil {
		return p.HTTP
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (p Polymarket) get(ctx context.Context, key, u string) (json.RawMessage, error) {
	if p.Cache != nil && strings.TrimSpace(key) != "" {
		if b, found, err := p.Cache.Get(ctx, key); err == nil && found && json.Valid(b) {
			return json.RawMessage(b), nil
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	p.addUpstreamHeaders(ctx, req)
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("polymarket http %d", resp.StatusCode)
	}
	if p.Cache != nil && strings.TrimSpace(key) != "" && json.Valid(b) {
		ttl := p.TTL
		if ttl <= 0 {
			ttl = 15 * time.Second
		}
		_ = p.Cache.Set(ctx, key, b, ttl)
	}
	return json.RawMessage(b), nil
}

func (p Polymarket) post(ctx context.Context, key, u string, body any) (json.RawMessage, error) {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, r)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	p.addUpstreamHeaders(ctx, req)
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("polymarket http %d", resp.StatusCode)
	}
	if len(bytes.TrimSpace(b)) == 0 {
		b = []byte("{}")
	}
	if !json.Valid(b) {
		return nil, errors.New("polymarket returned invalid json")
	}
	// No caching for POST by default (even if idempotent).
	_ = key
	return json.RawMessage(b), nil
}

func (p Polymarket) addUpstreamHeaders(ctx context.Context, req *http.Request) {
	// Polymarket backend expects a Bearer token for /api/* routes.
	tok := bearerFromContext(ctx)
	if tok == "" {
		tok = "public"
	}
	req.Header.Set("Authorization", "Bearer "+tok)

	// Some deployments require gateway-style headers for upstream enforcement.
	if v := projectFromContext(ctx); v != "" {
		req.Header.Set("X-Easyweb3-Project", v)
	}
	if v := roleFromContext(ctx); v != "" {
		req.Header.Set("X-Easyweb3-Role", v)
	}
}

func trimEmpty(m map[string]string) {
	for k, v := range m {
		if strings.TrimSpace(v) == "" {
			delete(m, k)
		}
	}
}

func trimEmptyZeroLike(m map[string]string) {
	for k, v := range m {
		v = strings.TrimSpace(v)
		if v == "" || v == "0" {
			delete(m, k)
		}
	}
}

func itoa(n int) string { return strconv.Itoa(n) }

func getInt(m map[string]any, k string, def int) int {
	if m == nil {
		return def
	}
	v, ok := m[k]
	if !ok || v == nil {
		return def
	}
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case int64:
		return int(t)
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n)
		}
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return def
}
