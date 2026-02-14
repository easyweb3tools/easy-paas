package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/nicekwell/easyweb3-cli/internal/client"
	"github.com/nicekwell/easyweb3-cli/internal/output"
)

type createLogRequest struct {
	Agent      string          `json:"agent"`
	Action     string          `json:"action"`
	Level      string          `json:"level"`
	Details    json.RawMessage `json:"details"`
	SessionKey string          `json:"session_key"`
	Metadata   json.RawMessage `json:"metadata"`
}

func logCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("log subcommand required: create|list|get")
	}
	switch args[0] {
	case "create":
		fs := flag.NewFlagSet("easyweb3 log create", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		action := fs.String("action", "", "action name")
		details := fs.String("details", "{}", "json details")
		level := fs.String("level", "info", "info|warn|error")
		agent := fs.String("agent", "", "agent name")
		session := fs.String("session-key", "", "session key")
		_ = fs.Parse(args[1:])

		if strings.TrimSpace(*action) == "" {
			return errors.New("--action required")
		}
		if strings.TrimSpace(*agent) == "" {
			*agent = "agent-" + uuid.NewString()
		}
		if strings.TrimSpace(*details) == "" {
			*details = "{}"
		}
		if !json.Valid([]byte(*details)) {
			return errors.New("--details must be valid json")
		}

		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		reqBody := createLogRequest{
			Agent:      strings.TrimSpace(*agent),
			Action:     strings.TrimSpace(*action),
			Level:      strings.TrimSpace(*level),
			Details:    json.RawMessage([]byte(*details)),
			SessionKey: strings.TrimSpace(*session),
			Metadata:   json.RawMessage("{}"),
		}
		req, err := c.NewRequest("POST", "/api/v1/logs", reqBody)
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "list":
		fs := flag.NewFlagSet("easyweb3 log list", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		action := fs.String("action", "", "action filter")
		level := fs.String("level", "", "level filter")
		limit := fs.Int("limit", 20, "limit")
		_ = fs.Parse(args[1:])

		q := "?limit=" + fmt.Sprintf("%d", *limit)
		if strings.TrimSpace(*action) != "" {
			q += "&action=" + urlQueryEscape(strings.TrimSpace(*action))
		}
		if strings.TrimSpace(*level) != "" {
			q += "&level=" + urlQueryEscape(strings.TrimSpace(*level))
		}

		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		req, err := c.NewRequest("GET", "/api/v1/logs"+q, nil)
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "get":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 log get <id>")
		}
		id := strings.TrimSpace(args[1])
		if id == "" {
			return errors.New("id required")
		}
		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		req, err := c.NewRequest("GET", "/api/v1/logs/"+id, nil)
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	default:
		return fmt.Errorf("unknown log subcommand: %s", args[0])
	}
}

func urlQueryEscape(s string) string {
	// minimal escape without pulling in net/url everywhere
	// (still standards-compliant enough for typical action strings)
	r := strings.NewReplacer(" ", "%20", "\n", "%0A", "\t", "%09", "\"", "%22", "#", "%23", "%", "%25", "&", "%26", "+", "%2B", "?", "%3F")
	return r.Replace(s)
}
