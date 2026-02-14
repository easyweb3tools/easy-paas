package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/nicekwell/easyweb3-cli/internal/client"
	"github.com/nicekwell/easyweb3-cli/internal/output"
)

func serviceCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("service subcommand required: list|health|docs")
	}
	switch args[0] {
	case "list":
		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		req, err := c.NewRequest("GET", "/api/v1/service/list", nil)
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "health":
		fs := flag.NewFlagSet("easyweb3 service health", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "service name")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*name) == "" {
			return errors.New("--name required")
		}
		path := "/api/v1/service/health?name=" + urlQueryEscape(strings.TrimSpace(*name))
		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		req, err := c.NewRequest("GET", path, nil)
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "docs":
		fs := flag.NewFlagSet("easyweb3 service docs", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		name := fs.String("name", "", "service name")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*name) == "" {
			return errors.New("--name required")
		}

		// Docs are markdown, so bypass JSON unmarshal.
		path := "/api/v1/service/docs?name=" + urlQueryEscape(strings.TrimSpace(*name))
		hc := &http.Client{}
		url := strings.TrimRight(ctx.APIBase, "/") + path
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(ctx.Token))
		resp, err := hc.Do(req)
		if err != nil {
			return err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			return fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
		}
		b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			return err
		}
		// For docs we always print raw markdown.
		fmt.Fprintln(os.Stdout, string(b))
		return nil

	default:
		return fmt.Errorf("unknown service subcommand: %s", args[0])
	}
}
