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

func apiCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("api subcommand required: raw|polymarket")
	}
	switch args[0] {
	case "raw":
		fs := flag.NewFlagSet("easyweb3 api raw", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		service := fs.String("service", "", "service name")
		method := fs.String("method", "GET", "http method")
		path := fs.String("path", "/", "path on upstream")
		body := fs.String("body", "", "json body")
		_ = fs.Parse(args[1:])

		if strings.TrimSpace(*service) == "" {
			return errors.New("--service required")
		}
		m := strings.ToUpper(strings.TrimSpace(*method))
		if m == "" {
			m = http.MethodGet
		}
		p := strings.TrimSpace(*path)
		if p == "" {
			p = "/"
		}
		if !strings.HasPrefix(p, "/") {
			p = "/" + p
		}

		// Route through platform gateway.
		route := fmt.Sprintf("/api/v1/services/%s%s", strings.TrimSpace(*service), p)

		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		var req *http.Request
		var err error
		if strings.TrimSpace(*body) == "" {
			req, err = c.NewRequest(m, route, nil)
		} else {
			if !json.Valid([]byte(*body)) {
				return errors.New("--body must be valid json")
			}
			var anyBody any
			if err := json.Unmarshal([]byte(*body), &anyBody); err != nil {
				return err
			}
			req, err = c.NewRequest(m, route, anyBody)
		}
		if err != nil {
			return err
		}

		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "polymarket":
		return apiPolymarketCmd(ctx, args[1:])

	default:
		return fmt.Errorf("unknown api subcommand: %s", args[0])
	}
}
