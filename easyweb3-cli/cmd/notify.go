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

func notifyCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("notify subcommand required: send|broadcast|config")
	}
	switch args[0] {
	case "send":
		fs := flag.NewFlagSet("easyweb3 notify send", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		channel := fs.String("channel", "", "telegram|webhook")
		to := fs.String("to", "", "chat_id or url")
		message := fs.String("message", "", "message")
		event := fs.String("event", "", "event/action (optional)")
		_ = fs.Parse(args[1:])

		if strings.TrimSpace(*channel) == "" {
			return errors.New("--channel required")
		}
		if strings.TrimSpace(*to) == "" {
			return errors.New("--to required")
		}
		if strings.TrimSpace(*message) == "" {
			return errors.New("--message required")
		}

		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		req, err := c.NewRequest("POST", "/api/v1/notify/send", map[string]any{
			"channel": strings.TrimSpace(*channel),
			"to":      strings.TrimSpace(*to),
			"message": strings.TrimSpace(*message),
			"event":   strings.TrimSpace(*event),
		})
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "broadcast":
		fs := flag.NewFlagSet("easyweb3 notify broadcast", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		message := fs.String("message", "", "message")
		event := fs.String("event", "", "event/action (optional)")
		_ = fs.Parse(args[1:])

		if strings.TrimSpace(*message) == "" {
			return errors.New("--message required")
		}

		c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
		req, err := c.NewRequest("POST", "/api/v1/notify/broadcast", map[string]any{
			"message": strings.TrimSpace(*message),
			"event":   strings.TrimSpace(*event),
		})
		if err != nil {
			return err
		}
		var resp any
		if err := c.Do(req, &resp); err != nil {
			return err
		}
		return output.Write(os.Stdout, ctx.Output, resp)

	case "config":
		if len(args) < 2 {
			return errors.New("notify config subcommand required: get|put")
		}
		switch args[1] {
		case "get":
			c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
			req, err := c.NewRequest("GET", "/api/v1/notify/config", nil)
			if err != nil {
				return err
			}
			var resp any
			if err := c.Do(req, &resp); err != nil {
				return err
			}
			return output.Write(os.Stdout, ctx.Output, resp)
		case "put":
			fs := flag.NewFlagSet("easyweb3 notify config put", flag.ContinueOnError)
			fs.SetOutput(os.Stderr)
			body := fs.String("body", "", "full project config json")
			_ = fs.Parse(args[2:])
			if strings.TrimSpace(*body) == "" {
				return errors.New("--body required")
			}
			if !json.Valid([]byte(*body)) {
				return errors.New("--body must be valid json")
			}
			var v any
			if err := json.Unmarshal([]byte(*body), &v); err != nil {
				return err
			}
			c := &client.Client{BaseURL: ctx.APIBase, Token: ctx.Token}
			req, err := c.NewRequest("PUT", "/api/v1/notify/config", v)
			if err != nil {
				return err
			}
			var resp any
			if err := c.Do(req, &resp); err != nil {
				return err
			}
			return output.Write(os.Stdout, ctx.Output, resp)
		default:
			return fmt.Errorf("unknown notify config subcommand: %s", args[1])
		}
	default:
		return fmt.Errorf("unknown notify subcommand: %s", args[0])
	}
}
