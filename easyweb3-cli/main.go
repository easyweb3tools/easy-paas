package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nicekwell/easyweb3-cli/cmd"
	"github.com/nicekwell/easyweb3-cli/internal/config"
	"github.com/nicekwell/easyweb3-cli/internal/output"
)

func main() {
	var (
		apiBase = flag.String("api-base", "", "PaaS API base URL (env: EASYWEB3_API_BASE)")
		token   = flag.String("token", "", "Bearer token (env: EASYWEB3_TOKEN)")
		outFmt  = flag.String("output", "json", "Output format: json|text|markdown")
		project = flag.String("project", "", "Project id (env: EASYWEB3_PROJECT)")
	)
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		cmd.Usage(os.Stderr)
		os.Exit(2)
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "config error:", err)
		os.Exit(1)
	}

	if strings.TrimSpace(*apiBase) != "" {
		cfg.APIBase = strings.TrimRight(strings.TrimSpace(*apiBase), "/")
	}
	if strings.TrimSpace(*project) != "" {
		cfg.Project = strings.TrimSpace(*project)
	}

	ctx := cmd.Context{
		APIBase: cfg.APIBase,
		Project: cfg.Project,
		Output:  output.Format(strings.TrimSpace(*outFmt)),
	}

	// Token resolution order:
	// 1) flag --token
	// 2) env EASYWEB3_TOKEN
	// 3) credentials file
	if strings.TrimSpace(*token) != "" {
		ctx.Token = strings.TrimSpace(*token)
	} else if v := strings.TrimSpace(os.Getenv("EASYWEB3_TOKEN")); v != "" {
		ctx.Token = v
	} else if cred, err := config.LoadCredentials(); err == nil {
		ctx.Token = strings.TrimSpace(cred.Token)
	}

	if err := cmd.Dispatch(ctx, args); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
