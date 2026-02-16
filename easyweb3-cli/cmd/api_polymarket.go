package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/nicekwell/easyweb3-cli/internal/client"
	"github.com/nicekwell/easyweb3-cli/internal/output"
)

func apiPolymarketCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: easyweb3 api polymarket <operation>")
	}

	op := strings.ToLower(strings.TrimSpace(args[0]))
	switch op {
	case "catalog-sync":
		fs := flag.NewFlagSet("easyweb3 api polymarket catalog-sync", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		scope := fs.String("scope", "all", "events|markets|series|tags|all")
		limit := fs.Int("limit", 0, "page size")
		maxPages := fs.Int("max-pages", 0, "max pages")
		resume := fs.Bool("resume", true, "resume")
		tagID := fs.Int("tag-id", 0, "tag id")
		closed := fs.String("closed", "", "open|closed")
		_ = fs.Parse(args[1:])

		q := "?scope=" + urlQueryEscape(*scope)
		if *limit > 0 {
			q += fmt.Sprintf("&limit=%d", *limit)
		}
		if *maxPages > 0 {
			q += fmt.Sprintf("&max_pages=%d", *maxPages)
		}
		q += fmt.Sprintf("&resume=%t", *resume)
		if *tagID > 0 {
			q += fmt.Sprintf("&tag_id=%d", *tagID)
		}
		if strings.TrimSpace(*closed) != "" {
			q += "&closed=" + urlQueryEscape(strings.TrimSpace(*closed))
		}

		return polymarketDo(ctx, http.MethodPost, "/api/catalog/sync"+q, nil)

	case "catalog-events":
		fs := flag.NewFlagSet("easyweb3 api polymarket catalog-events", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		active := fs.String("active", "", "true|false")
		closed := fs.String("closed", "", "true|false")
		_ = fs.Parse(args[1:])

		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*active) != "" {
			q += "&active=" + urlQueryEscape(strings.TrimSpace(*active))
		}
		if strings.TrimSpace(*closed) != "" {
			q += "&closed=" + urlQueryEscape(strings.TrimSpace(*closed))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/catalog/events"+q, nil)

	case "catalog-markets":
		fs := flag.NewFlagSet("easyweb3 api polymarket catalog-markets", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		eventID := fs.String("event-id", "", "event id")
		active := fs.String("active", "", "true|false")
		closed := fs.String("closed", "", "true|false")
		_ = fs.Parse(args[1:])

		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*eventID) != "" {
			q += "&event_id=" + urlQueryEscape(strings.TrimSpace(*eventID))
		}
		if strings.TrimSpace(*active) != "" {
			q += "&active=" + urlQueryEscape(strings.TrimSpace(*active))
		}
		if strings.TrimSpace(*closed) != "" {
			q += "&closed=" + urlQueryEscape(strings.TrimSpace(*closed))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/catalog/markets"+q, nil)

	case "opportunities":
		fs := flag.NewFlagSet("easyweb3 api polymarket opportunities", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		status := fs.String("status", "", "status")
		strategy := fs.String("strategy", "", "strategy")
		category := fs.String("category", "", "category")
		_ = fs.Parse(args[1:])

		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*status) != "" {
			q += "&status=" + urlQueryEscape(strings.TrimSpace(*status))
		}
		if strings.TrimSpace(*strategy) != "" {
			q += "&strategy=" + urlQueryEscape(strings.TrimSpace(*strategy))
		}
		if strings.TrimSpace(*category) != "" {
			q += "&category=" + urlQueryEscape(strings.TrimSpace(*category))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/opportunities"+q, nil)

	case "opportunity-get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket opportunity-get <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/opportunities/"+id, nil)

	case "opportunity-dismiss":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket opportunity-dismiss <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/opportunities/"+id+"/dismiss", map[string]any{})

	case "opportunity-execute":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket opportunity-execute <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/opportunities/"+id+"/execute", map[string]any{})

	case "executions":
		fs := flag.NewFlagSet("easyweb3 api polymarket executions", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		status := fs.String("status", "", "status")
		_ = fs.Parse(args[1:])

		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*status) != "" {
			q += "&status=" + urlQueryEscape(strings.TrimSpace(*status))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/executions"+q, nil)

	case "execution-get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket execution-get <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/executions/"+id, nil)

	case "execution-preflight":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket execution-preflight <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/executions/"+id+"/preflight", map[string]any{})

	case "execution-mark-executing":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket execution-mark-executing <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/executions/"+id+"/mark-executing", map[string]any{})

	case "execution-mark-executed":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket execution-mark-executed <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/executions/"+id+"/mark-executed", map[string]any{})

	case "execution-cancel":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket execution-cancel <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/executions/"+id+"/cancel", map[string]any{})

	case "execution-fill":
		fs := flag.NewFlagSet("easyweb3 api polymarket execution-fill", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		planID := fs.String("id", "", "plan id")
		tokenID := fs.String("token-id", "", "token id")
		direction := fs.String("direction", "", "e.g. BUY_YES")
		size := fs.String("filled-size", "", "filled size")
		avgPrice := fs.String("avg-price", "", "avg price")
		fee := fs.String("fee", "", "fee")
		slippage := fs.String("slippage", "", "slippage")
		filledAt := fs.String("filled-at", "", "RFC3339")
		_ = fs.Parse(args[1:])

		if strings.TrimSpace(*planID) == "" {
			return errors.New("--id required")
		}
		if strings.TrimSpace(*tokenID) == "" || strings.TrimSpace(*direction) == "" || strings.TrimSpace(*size) == "" || strings.TrimSpace(*avgPrice) == "" {
			return errors.New("--token-id, --direction, --filled-size, --avg-price required")
		}
		body := map[string]any{
			"token_id":    strings.TrimSpace(*tokenID),
			"direction":   strings.TrimSpace(*direction),
			"filled_size": strings.TrimSpace(*size),
			"avg_price":   strings.TrimSpace(*avgPrice),
			"fee":         strings.TrimSpace(*fee),
			"slippage":    strings.TrimSpace(*slippage),
			"filled_at":   strings.TrimSpace(*filledAt),
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/executions/"+strings.TrimSpace(*planID)+"/fill", body)

	case "execution-settle":
		fs := flag.NewFlagSet("easyweb3 api polymarket execution-settle", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		planID := fs.String("id", "", "plan id")
		body := fs.String("body", "{}", "json body")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*planID) == "" {
			return errors.New("--id required")
		}
		if strings.TrimSpace(*body) == "" {
			*body = "{}"
		}
		// Reuse raw handler in cmd/api.go? Keep simple: send as opaque json map.
		var anyBody any
		if err := json.Unmarshal([]byte(*body), &anyBody); err != nil {
			return errors.New("--body must be valid json")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/executions/"+strings.TrimSpace(*planID)+"/settle", anyBody)

	case "execution-submit":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket execution-submit <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/executions/"+id+"/submit", map[string]any{})

	case "orders":
		fs := flag.NewFlagSet("easyweb3 api polymarket orders", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		status := fs.String("status", "", "status")
		planID := fs.String("plan-id", "", "plan id")
		tokenID := fs.String("token-id", "", "token id")
		_ = fs.Parse(args[1:])
		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*status) != "" {
			q += "&status=" + urlQueryEscape(strings.TrimSpace(*status))
		}
		if strings.TrimSpace(*planID) != "" {
			q += "&plan_id=" + urlQueryEscape(strings.TrimSpace(*planID))
		}
		if strings.TrimSpace(*tokenID) != "" {
			q += "&token_id=" + urlQueryEscape(strings.TrimSpace(*tokenID))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/orders"+q, nil)

	case "order-get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket order-get <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/orders/"+id, nil)

	case "order-cancel":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket order-cancel <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/orders/"+id+"/cancel", map[string]any{})

	case "positions":
		fs := flag.NewFlagSet("easyweb3 api polymarket positions", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		status := fs.String("status", "", "open|closed")
		strategy := fs.String("strategy", "", "strategy_name")
		marketID := fs.String("market-id", "", "market id")
		_ = fs.Parse(args[1:])
		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*status) != "" {
			q += "&status=" + urlQueryEscape(strings.TrimSpace(*status))
		}
		if strings.TrimSpace(*strategy) != "" {
			q += "&strategy_name=" + urlQueryEscape(strings.TrimSpace(*strategy))
		}
		if strings.TrimSpace(*marketID) != "" {
			q += "&market_id=" + urlQueryEscape(strings.TrimSpace(*marketID))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/positions"+q, nil)

	case "position-get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket position-get <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/positions/"+id, nil)

	case "portfolio-summary":
		return polymarketDo(ctx, http.MethodGet, "/api/v2/positions/summary", nil)

	case "portfolio-history":
		fs := flag.NewFlagSet("easyweb3 api polymarket portfolio-history", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 168, "limit")
		offset := fs.Int("offset", 0, "offset")
		since := fs.String("since", "", "RFC3339")
		until := fs.String("until", "", "RFC3339")
		_ = fs.Parse(args[1:])
		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*since) != "" {
			q += "&since=" + urlQueryEscape(strings.TrimSpace(*since))
		}
		if strings.TrimSpace(*until) != "" {
			q += "&until=" + urlQueryEscape(strings.TrimSpace(*until))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/portfolio/history"+q, nil)

	case "analytics-daily":
		fs := flag.NewFlagSet("easyweb3 api polymarket analytics-daily", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 365, "limit")
		offset := fs.Int("offset", 0, "offset")
		strategy := fs.String("strategy", "", "strategy_name")
		since := fs.String("since", "", "RFC3339")
		until := fs.String("until", "", "RFC3339")
		_ = fs.Parse(args[1:])
		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*strategy) != "" {
			q += "&strategy_name=" + urlQueryEscape(strings.TrimSpace(*strategy))
		}
		if strings.TrimSpace(*since) != "" {
			q += "&since=" + urlQueryEscape(strings.TrimSpace(*since))
		}
		if strings.TrimSpace(*until) != "" {
			q += "&until=" + urlQueryEscape(strings.TrimSpace(*until))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/analytics/daily"+q, nil)

	case "analytics-attribution":
		fs := flag.NewFlagSet("easyweb3 api polymarket analytics-attribution", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("strategy", "", "strategy name")
		since := fs.String("since", "", "RFC3339")
		until := fs.String("until", "", "RFC3339")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*name) == "" {
			return errors.New("--strategy required")
		}
		q := ""
		if strings.TrimSpace(*since) != "" || strings.TrimSpace(*until) != "" {
			q = "?"
			first := true
			if strings.TrimSpace(*since) != "" {
				q += "since=" + urlQueryEscape(strings.TrimSpace(*since))
				first = false
			}
			if strings.TrimSpace(*until) != "" {
				if !first {
					q += "&"
				}
				q += "until=" + urlQueryEscape(strings.TrimSpace(*until))
			}
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/analytics/strategy/"+urlQueryEscape(strings.TrimSpace(*name))+"/attribution"+q, nil)

	case "analytics-drawdown":
		return polymarketDo(ctx, http.MethodGet, "/api/v2/analytics/drawdown", nil)

	case "analytics-correlation":
		return polymarketDo(ctx, http.MethodGet, "/api/v2/analytics/correlation", nil)

	case "analytics-ratios":
		return polymarketDo(ctx, http.MethodGet, "/api/v2/analytics/ratios", nil)

	case "review":
		fs := flag.NewFlagSet("easyweb3 api polymarket review", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 100, "limit")
		offset := fs.Int("offset", 0, "offset")
		ourAction := fs.String("our-action", "", "traded|dismissed|expired|missed")
		strategy := fs.String("strategy", "", "strategy_name")
		since := fs.String("since", "", "RFC3339")
		until := fs.String("until", "", "RFC3339")
		_ = fs.Parse(args[1:])
		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		if strings.TrimSpace(*ourAction) != "" {
			q += "&our_action=" + urlQueryEscape(strings.TrimSpace(*ourAction))
		}
		if strings.TrimSpace(*strategy) != "" {
			q += "&strategy_name=" + urlQueryEscape(strings.TrimSpace(*strategy))
		}
		if strings.TrimSpace(*since) != "" {
			q += "&since=" + urlQueryEscape(strings.TrimSpace(*since))
		}
		if strings.TrimSpace(*until) != "" {
			q += "&until=" + urlQueryEscape(strings.TrimSpace(*until))
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/review"+q, nil)

	case "review-missed":
		fs := flag.NewFlagSet("easyweb3 api polymarket review-missed", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 100, "limit")
		offset := fs.Int("offset", 0, "offset")
		_ = fs.Parse(args[1:])
		q := fmt.Sprintf("?limit=%d&offset=%d", *limit, *offset)
		return polymarketDo(ctx, http.MethodGet, "/api/v2/review/missed"+q, nil)

	case "review-regret-index":
		return polymarketDo(ctx, http.MethodGet, "/api/v2/review/regret-index", nil)

	case "review-label-performance":
		return polymarketDo(ctx, http.MethodGet, "/api/v2/review/label-performance", nil)

	case "review-notes":
		fs := flag.NewFlagSet("easyweb3 api polymarket review-notes", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		id := fs.String("id", "", "review id")
		notes := fs.String("notes", "", "notes")
		lessonTags := fs.String("lesson-tags", "", "comma-separated lesson tags")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*id) == "" {
			return errors.New("--id required")
		}
		var tags []string
		for _, v := range strings.Split(strings.TrimSpace(*lessonTags), ",") {
			tag := strings.TrimSpace(v)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
		return polymarketDo(ctx, http.MethodPut, "/api/v2/review/"+urlQueryEscape(strings.TrimSpace(*id))+"/notes", map[string]any{
			"notes":       strings.TrimSpace(*notes),
			"lesson_tags": tags,
		})

	case "switches":
		return polymarketDo(ctx, http.MethodGet, "/api/v2/system-settings/switches", nil)

	case "switch-get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket switch-get <name>")
		}
		name := strings.TrimSpace(args[1])
		if name == "" {
			return errors.New("name required")
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/system-settings/switches/"+urlQueryEscape(name), nil)

	case "switch-enable":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket switch-enable <name>")
		}
		name := strings.TrimSpace(args[1])
		if name == "" {
			return errors.New("name required")
		}
		return polymarketDo(ctx, http.MethodPut, "/api/v2/system-settings/switches/"+urlQueryEscape(name), map[string]any{"enabled": true})

	case "switch-disable":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket switch-disable <name>")
		}
		name := strings.TrimSpace(args[1])
		if name == "" {
			return errors.New("name required")
		}
		return polymarketDo(ctx, http.MethodPut, "/api/v2/system-settings/switches/"+urlQueryEscape(name), map[string]any{"enabled": false})

	case "switch-set":
		fs := flag.NewFlagSet("easyweb3 api polymarket switch-set", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "switch name, e.g. auto_executor")
		enabled := fs.String("enabled", "", "true|false")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*name) == "" {
			return errors.New("--name required")
		}
		val := strings.ToLower(strings.TrimSpace(*enabled))
		if val != "true" && val != "false" {
			return errors.New("--enabled must be true or false")
		}
		return polymarketDo(ctx, http.MethodPut, "/api/v2/system-settings/switches/"+urlQueryEscape(strings.TrimSpace(*name)), map[string]any{
			"enabled": val == "true",
		})

	case "setting-get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 api polymarket setting-get <key>")
		}
		key := strings.TrimSpace(args[1])
		if key == "" {
			return errors.New("key required")
		}
		return polymarketDo(ctx, http.MethodGet, "/api/v2/system-settings/"+urlQueryEscape(key), nil)

	case "setting-set":
		fs := flag.NewFlagSet("easyweb3 api polymarket setting-set", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		key := fs.String("key", "", "setting key")
		value := fs.String("value", "", "json value, e.g. true or {\"k\":1}")
		desc := fs.String("desc", "", "description")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*key) == "" {
			return errors.New("--key required")
		}
		if strings.TrimSpace(*value) == "" {
			return errors.New("--value required (json)")
		}
		var parsed any
		if err := json.Unmarshal([]byte(strings.TrimSpace(*value)), &parsed); err != nil {
			return errors.New("--value must be valid json")
		}
		return polymarketDo(ctx, http.MethodPut, "/api/v2/system-settings/"+urlQueryEscape(strings.TrimSpace(*key)), map[string]any{
			"value":       parsed,
			"description": strings.TrimSpace(*desc),
		})

	case "settings-reencrypt-sensitive":
		fs := flag.NewFlagSet("easyweb3 api polymarket settings-reencrypt-sensitive", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		prefix := fs.String("prefix", "", "optional key prefix")
		limit := fs.Int("limit", 5000, "scan limit")
		_ = fs.Parse(args[1:])
		q := fmt.Sprintf("?limit=%d", *limit)
		if strings.TrimSpace(*prefix) != "" {
			q += "&prefix=" + urlQueryEscape(strings.TrimSpace(*prefix))
		}
		return polymarketDo(ctx, http.MethodPost, "/api/v2/system-settings/re-encrypt-sensitive"+q, map[string]any{})

	default:
		return fmt.Errorf("unknown polymarket operation: %s", args[0])
	}
}

func polymarketDo(ctx Context, method, path string, body any) error {
	route := "/api/v1/services/polymarket" + path
	tok := strings.TrimSpace(ctx.Token)
	m := strings.ToUpper(strings.TrimSpace(method))
	if tok == "" && (m == http.MethodPost || m == http.MethodPut || m == http.MethodPatch || m == http.MethodDelete) {
		ensured, err := ensureBearerToken(ctx)
		if err != nil {
			return err
		}
		tok = ensured
	}
	c := &client.Client{BaseURL: ctx.APIBase, Token: tok}
	req, err := c.NewRequest(method, route, body)
	if err != nil {
		return err
	}
	var resp any
	if err := c.Do(req, &resp); err != nil {
		return err
	}
	return output.Write(os.Stdout, ctx.Output, resp)
}
