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
		return errors.New("auth subcommand required: login|register|grant|refresh|status")
	}
	switch args[0] {
	case "login":
		fs := flag.NewFlagSet("easyweb3 auth login", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		apiKey := fs.String("api-key", "", "API key")
		username := fs.String("username", "", "Username (for user login)")
		password := fs.String("password", "", "Password (for user login)")
		projectID := fs.String("project-id", "", "Project id (for user login)")
		_ = fs.Parse(args[1:])

		c := &client.Client{BaseURL: ctx.APIBase}
		var payload map[string]any
		if strings.TrimSpace(*apiKey) != "" {
			payload = map[string]any{"api_key": strings.TrimSpace(*apiKey)}
		} else {
			if strings.TrimSpace(*username) == "" || strings.TrimSpace(*password) == "" || strings.TrimSpace(*projectID) == "" {
				return errors.New("usage: easyweb3 auth login --api-key <key> OR --username <u> --password <p> --project-id <project>")
			}
			payload = map[string]any{
				"username":   strings.TrimSpace(*username),
				"password":   strings.TrimSpace(*password),
				"project_id": strings.TrimSpace(*projectID),
			}
		}
		req, err := c.NewRequest("POST", "/api/v1/auth/login", payload)
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
		// Persist api-key only if this was an api-key login.
		if strings.TrimSpace(*apiKey) != "" {
			cred.APIKey = strings.TrimSpace(*apiKey)
		}
		_ = config.SaveCredentials(cred)
		return output.Write(os.Stdout, ctx.Output, resp)

	case "register":
		fs := flag.NewFlagSet("easyweb3 auth register", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		username := fs.String("username", "", "Username")
		password := fs.String("password", "", "Password")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*username) == "" || strings.TrimSpace(*password) == "" {
			return errors.New("usage: easyweb3 auth register --username <u> --password <p>")
		}
		c := &client.Client{BaseURL: ctx.APIBase}
		req, err := c.NewRequest("POST", "/api/v1/auth/register", map[string]any{
			"username": strings.TrimSpace(*username),
			"password": strings.TrimSpace(*password),
		})
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "grant":
		fs := flag.NewFlagSet("easyweb3 auth grant", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		user := fs.String("user", "", "User id or username")
		projectID := fs.String("project-id", "", "Project id")
		role := fs.String("role", "", "Role: viewer|agent|admin")
		_ = fs.Parse(args[1:])
		if strings.TrimSpace(*user) == "" || strings.TrimSpace(*projectID) == "" || strings.TrimSpace(*role) == "" {
			return errors.New("usage: easyweb3 auth grant --user <id|username> --project-id <project> --role <viewer|agent|admin>")
		}
		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		req, err := c.NewRequest("POST", "/api/v1/auth/grants", map[string]any{
			"user":       strings.TrimSpace(*user),
			"project_id": strings.TrimSpace(*projectID),
			"role":       strings.TrimSpace(*role),
		})
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
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
