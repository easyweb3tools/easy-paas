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

	default:
		return fmt.Errorf("unknown polymarket operation: %s", args[0])
	}
}

func polymarketDo(ctx Context, method, path string, body any) error {
	route := "/api/v1/services/polymarket" + path
	c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
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
