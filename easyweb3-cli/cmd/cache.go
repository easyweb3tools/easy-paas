package cmd

import (
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nicekwell/easyweb3-cli/internal/client"
	"github.com/nicekwell/easyweb3-cli/internal/output"
)

func cacheCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("cache subcommand required: get|put|delete")
	}
	switch args[0] {
	case "get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 cache get <key>")
		}
		key := strings.TrimSpace(args[1])
		if key == "" {
			return errors.New("key required")
		}
		tok, err := ensureBearerToken(ctx)
		if err != nil {
			return err
		}
		c := &client.Client{BaseURL: ctx.APIBase, Token: tok}
		req, err := c.NewRequest("GET", "/api/v1/cache/"+key, nil)
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "put":
		fs := flag.NewFlagSet("easyweb3 cache put", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		key := fs.String("key", "", "cache key")
		value := fs.String("value", "", "json value")
		ttl := fs.Int64("ttl-seconds", 0, "ttl seconds (0 uses server default; negative disables expiration)")
		_ = fs.Parse(args[1:])

		if strings.TrimSpace(*key) == "" {
			return errors.New("--key required")
		}
		// keep as opaque json string on server side; send as base64 if not json.
		payload := map[string]any{"ttl_seconds": *ttl}
		if strings.TrimSpace(*value) == "" {
			payload["value"] = nil
		} else {
			payload["value_base64"] = base64.StdEncoding.EncodeToString([]byte(*value))
		}

		tok, err := ensureBearerToken(ctx)
		if err != nil {
			return err
		}
		c := &client.Client{BaseURL: ctx.APIBase, Token: tok}
		req, err := c.NewRequest("PUT", "/api/v1/cache/"+strings.TrimSpace(*key), payload)
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "delete", "del", "rm":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 cache delete <key>")
		}
		key := strings.TrimSpace(args[1])
		if key == "" {
			return errors.New("key required")
		}
		tok, err := ensureBearerToken(ctx)
		if err != nil {
			return err
		}
		c := &client.Client{BaseURL: ctx.APIBase, Token: tok}
		req, err := c.NewRequest("DELETE", "/api/v1/cache/"+key, nil)
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)
	default:
		return fmt.Errorf("unknown cache subcommand: %s", args[0])
	}
}
