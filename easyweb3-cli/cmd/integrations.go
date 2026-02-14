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
		return errors.New("integrations subcommand required: query")
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
		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
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
	default:
		return fmt.Errorf("unknown integrations subcommand: %s", args[0])
	}
}
