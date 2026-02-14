package cmd

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nicekwell/easyweb3-cli/internal/client"
	"github.com/nicekwell/easyweb3-cli/internal/config"
	"github.com/nicekwell/easyweb3-cli/internal/output"
)

type tokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
}

func authCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("auth subcommand required: login|refresh|status")
	}
	switch args[0] {
	case "login":
		fs := flag.NewFlagSet("easyweb3 auth login", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		apiKey := fs.String("api-key", "", "API key")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*apiKey) == "" {
			return errors.New("--api-key required")
		}

		c := &client.Client{BaseURL: ctx.APIBase}
		req, err := c.NewRequest("POST", "/api/v1/auth/login", map[string]any{"api_key": strings.TrimSpace(*apiKey)})
		if err != nil {
			return err
		}
		var resp tokenResponse
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		cred := config.Credentials{Token: resp.Token, ExpiresAt: resp.ExpiresAt, APIKey: strings.TrimSpace(*apiKey)}
		_ = config.SaveCredentials(cred)
		return output.Write(os.Stdout, ctx.Output, resp)

	case "refresh":
		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		req, err := c.NewRequest("POST", "/api/v1/auth/refresh", map[string]any{})
		if err != nil {
			return err
		}
		var resp tokenResponse
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		cred, _ := config.LoadCredentials()
		cred.Token = resp.Token
		cred.ExpiresAt = resp.ExpiresAt
		_ = config.SaveCredentials(cred)
		return output.Write(os.Stdout, ctx.Output, resp)

	case "status":
		c := &client.Client{BaseURL: ctx.APIBase}
		path := "/api/v1/auth/status"
		req, err := c.NewRequest("GET", path, nil)
		if err != nil {
			return err
		}
		// If token exists, pass it. Status is public but can return more if authenticated.
		if strings.TrimSpace(ctx.Token) != "" {
			req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(ctx.Token))
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	default:
		return fmt.Errorf("unknown auth subcommand: %s", args[0])
	}
}
