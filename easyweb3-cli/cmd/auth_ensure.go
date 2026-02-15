package cmd

import (
	"errors"
	"strings"
	"time"

	"github.com/nicekwell/easyweb3-cli/internal/client"
	"github.com/nicekwell/easyweb3-cli/internal/config"
)

const refreshSkew = 2 * time.Minute

func ensureBearerToken(ctx Context) (string, error) {
	// If caller provided an explicit token (flag/env), trust it.
	// We only auto-refresh / auto-login when we are using the persisted credentials token.
	if strings.TrimSpace(ctx.Token) != "" {
		return strings.TrimSpace(ctx.Token), nil
	}

	cred, err := config.LoadCredentials()
	if err != nil {
		return "", errors.New("missing credentials; run: easyweb3 auth login --api-key <key>")
	}

	tok := strings.TrimSpace(cred.Token)
	exp, hasExp := cred.ExpiresAtTime()

	needsRenew := tok == ""
	if !needsRenew && hasExp && !exp.IsZero() && time.Until(exp) < refreshSkew {
		needsRenew = true
	}
	if !needsRenew {
		return tok, nil
	}

	c := &client.Client{BaseURL: ctx.APIBase, Token: tok}

	// Prefer refresh if we have a token; otherwise login via persisted API key.
	var resp tokenResponse
	if tok != "" {
		req, err := c.NewRequest("POST", "/api/v1/auth/refresh", map[string]any{})
		if err == nil && c.Do(req, &resp) == nil && strings.TrimSpace(resp.Token) != "" {
			cred.Token = strings.TrimSpace(resp.Token)
			cred.ExpiresAt = strings.TrimSpace(resp.ExpiresAt)
			_ = config.SaveCredentials(cred)
			return cred.Token, nil
		}
	}

	if strings.TrimSpace(cred.APIKey) == "" {
		return "", errors.New("token expired/missing and no api_key stored; run: easyweb3 auth login --api-key <key>")
	}

	// Login via stored API key.
	c.Token = ""
	req, err := c.NewRequest("POST", "/api/v1/auth/login", map[string]any{"api_key": strings.TrimSpace(cred.APIKey)})
	if err != nil {
		return "", err
	}
	if err := c.Do(req, &resp); err != nil {
		return "", err
	}
	cred.Token = strings.TrimSpace(resp.Token)
	cred.ExpiresAt = strings.TrimSpace(resp.ExpiresAt)
	_ = config.SaveCredentials(cred)
	if cred.Token == "" {
		return "", errors.New("login succeeded but token was empty")
	}
	return cred.Token, nil
}
