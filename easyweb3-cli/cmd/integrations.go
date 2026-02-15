package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nicekwell/easyweb3-cli/internal/client"
	"github.com/nicekwell/easyweb3-cli/internal/output"
)

func integrationsCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("integrations subcommand required: query|polymarket")
	}
	switch args[0] {
	case "query":
		fs := flag.NewFlagSet("easyweb3 integrations query", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		provider := fs.String("provider", "", "provider name (e.g. dexscreener)")
		method := fs.String("method", "", "provider method (e.g. search|pairs|token)")
		params := fs.String("params", "{}", "json params object")
		_ = fs.Parse(args[1:])

		if strings.TrimSpace(*provider) == "" {
			return errors.New("--provider required")
		}
		if strings.TrimSpace(*method) == "" {
			return errors.New("--method required")
		}
		if strings.TrimSpace(*params) == "" {
			*params = "{}"
		}
		if !json.Valid([]byte(*params)) {
			return errors.New("--params must be valid json")
		}
		var obj any
		if err := json.Unmarshal([]byte(*params), &obj); err != nil {
			return err
		}
		m, ok := obj.(map[string]any)
		if !ok {
			return errors.New("--params must be a json object")
		}

		path := fmt.Sprintf("/api/v1/integrations/%s/query", strings.TrimSpace(*provider))
		tok, err := ensureBearerToken(ctx)
		if err != nil {
			return err
		}
		c := &client.Client{BaseURL: ctx.APIBase, Token: tok}
		req, err := c.NewRequest("POST", path, map[string]any{
			"method": strings.TrimSpace(*method),
			"params": m,
		})
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "polymarket":
		return integrationsPolymarketCmd(ctx, args[1:])
	default:
		return fmt.Errorf("unknown integrations subcommand: %s", args[0])
	}
}

func integrationsPolymarketCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: easyweb3 integrations polymarket <op>")
	}

	op := strings.ToLower(strings.TrimSpace(args[0]))
	switch op {
	case "healthz", "health":
		return integrationsPolymarketQuery(ctx, "healthz", map[string]any{})

	case "opportunities":
		fs := flag.NewFlagSet("easyweb3 integrations polymarket opportunities", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		status := fs.String("status", "", "status")
		strategy := fs.String("strategy", "", "strategy")
		category := fs.String("category", "", "category")
		_ = fs.Parse(args[1:])
		return integrationsPolymarketQuery(ctx, "opportunities", map[string]any{
			"limit":    *limit,
			"offset":   *offset,
			"status":   strings.TrimSpace(*status),
			"strategy": strings.TrimSpace(*strategy),
			"category": strings.TrimSpace(*category),
		})

	case "catalog-events":
		fs := flag.NewFlagSet("easyweb3 integrations polymarket catalog-events", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		active := fs.String("active", "", "true|false")
		closed := fs.String("closed", "", "true|false")
		_ = fs.Parse(args[1:])
		return integrationsPolymarketQuery(ctx, "catalog-events", map[string]any{
			"limit":  *limit,
			"offset": *offset,
			"active": strings.TrimSpace(*active),
			"closed": strings.TrimSpace(*closed),
		})

	case "catalog-markets":
		fs := flag.NewFlagSet("easyweb3 integrations polymarket catalog-markets", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		eventID := fs.String("event-id", "", "event id")
		active := fs.String("active", "", "true|false")
		closed := fs.String("closed", "", "true|false")
		_ = fs.Parse(args[1:])
		return integrationsPolymarketQuery(ctx, "catalog-markets", map[string]any{
			"limit":    *limit,
			"offset":   *offset,
			"event_id": strings.TrimSpace(*eventID),
			"active":   strings.TrimSpace(*active),
			"closed":   strings.TrimSpace(*closed),
		})

	case "catalog-sync":
		fs := flag.NewFlagSet("easyweb3 integrations polymarket catalog-sync", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		scope := fs.String("scope", "all", "events|markets|series|tags|all")
		limit := fs.Int("limit", 0, "page size")
		maxPages := fs.Int("max-pages", 0, "max pages")
		resume := fs.Bool("resume", true, "resume")
		tagID := fs.Int("tag-id", 0, "tag id")
		closed := fs.String("closed", "", "open|closed")
		_ = fs.Parse(args[1:])
		return integrationsPolymarketQuery(ctx, "catalog-sync", map[string]any{
			"scope":     strings.TrimSpace(*scope),
			"limit":     *limit,
			"max_pages": *maxPages,
			"resume":    fmt.Sprintf("%t", *resume),
			"tag_id":    *tagID,
			"closed":    strings.TrimSpace(*closed),
		})

	case "opportunity-get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 integrations polymarket opportunity-get <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return integrationsPolymarketQuery(ctx, "opportunity-get", map[string]any{"id": id})

	case "opportunity-dismiss":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 integrations polymarket opportunity-dismiss <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return integrationsPolymarketQuery(ctx, "opportunity-dismiss", map[string]any{"id": id})

	case "opportunity-execute":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 integrations polymarket opportunity-execute <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return integrationsPolymarketQuery(ctx, "opportunity-execute", map[string]any{"id": id})

	case "executions":
		fs := flag.NewFlagSet("easyweb3 integrations polymarket executions", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		limit := fs.Int("limit", 50, "limit")
		offset := fs.Int("offset", 0, "offset")
		status := fs.String("status", "", "status")
		_ = fs.Parse(args[1:])
		return integrationsPolymarketQuery(ctx, "executions", map[string]any{
			"limit":  *limit,
			"offset": *offset,
			"status": strings.TrimSpace(*status),
		})

	case "execution-get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 integrations polymarket execution-get <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return integrationsPolymarketQuery(ctx, "execution-get", map[string]any{"id": id})

	case "execution-preflight":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 integrations polymarket execution-preflight <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return integrationsPolymarketQuery(ctx, "execution-preflight", map[string]any{"id": id})

	case "execution-mark-executing":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 integrations polymarket execution-mark-executing <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return integrationsPolymarketQuery(ctx, "execution-mark-executing", map[string]any{"id": id})

	case "execution-mark-executed":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 integrations polymarket execution-mark-executed <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return integrationsPolymarketQuery(ctx, "execution-mark-executed", map[string]any{"id": id})

	case "execution-cancel":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 integrations polymarket execution-cancel <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		return integrationsPolymarketQuery(ctx, "execution-cancel", map[string]any{"id": id})

	case "execution-fill":
		fs := flag.NewFlagSet("easyweb3 integrations polymarket execution-fill", flag.ContinueOnError)
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
		return integrationsPolymarketQuery(ctx, "execution-fill", map[string]any{
			"id":          strings.TrimSpace(*planID),
			"token_id":    strings.TrimSpace(*tokenID),
			"direction":   strings.TrimSpace(*direction),
			"filled_size": strings.TrimSpace(*size),
			"avg_price":   strings.TrimSpace(*avgPrice),
			"fee":         strings.TrimSpace(*fee),
			"slippage":    strings.TrimSpace(*slippage),
			"filled_at":   strings.TrimSpace(*filledAt),
		})

	case "execution-settle":
		fs := flag.NewFlagSet("easyweb3 integrations polymarket execution-settle", flag.ContinueOnError)
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
		var anyBody any
		if err := json.Unmarshal([]byte(*body), &anyBody); err != nil {
			return errors.New("--body must be valid json")
		}
		return integrationsPolymarketQuery(ctx, "execution-settle", map[string]any{
			"id":   strings.TrimSpace(*planID),
			"body": anyBody,
		})

	default:
		return fmt.Errorf("unknown polymarket op: %s", op)
	}
}

func integrationsPolymarketQuery(ctx Context, method string, params map[string]any) error {
	path := "/api/v1/integrations/polymarket/query"
	tok, err := ensureBearerToken(ctx)
	if err != nil {
		return err
	}
	c := &client.Client{BaseURL: ctx.APIBase, Token: tok}
	req, err := c.NewRequest("POST", path, map[string]any{
		"method": strings.TrimSpace(method),
		"params": params,
	})
	if err != nil {
		return err
	}
	var resp any
	if err := c.Do(req, &resp); err != nil {
		return err
	}
	return output.Write(os.Stdout, ctx.Output, resp)
}
