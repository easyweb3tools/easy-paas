package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/nicekwell/easyweb3-cli/internal/output"
)

type Context struct {
	APIBase string
	Token   string
	Project string
	Output  output.Format
}

func Usage(w io.Writer) {
	fmt.Fprint(w, `easyweb3 <command> <subcommand> [flags]

Global Flags:
  --api-base    PaaS API base URL (env: EASYWEB3_API_BASE)
  --token       Bearer Token (env: EASYWEB3_TOKEN)
  --output      json|text|markdown (default json)
  --project     Project id (env: EASYWEB3_PROJECT)

Commands:
  auth     login/refresh/status
  log      create/list/get
  notify   send/broadcast/config
  integrations query
  cache    get/put/delete
  api      raw
  service  list/health/docs
`)
}

func Dispatch(ctx Context, args []string) error {
	if len(args) == 0 {
		Usage(os.Stderr)
		return errors.New("missing command")
	}
	switch args[0] {
	case "auth":
		return authCmd(ctx, args[1:])
	case "log":
		return logCmd(ctx, args[1:])
	case "notify":
		return notifyCmd(ctx, args[1:])
	case "integrations":
		return integrationsCmd(ctx, args[1:])
	case "cache":
		return cacheCmd(ctx, args[1:])
	case "api":
		return apiCmd(ctx, args[1:])
	case "service":
		return serviceCmd(ctx, args[1:])
	case "help", "-h", "--help":
		Usage(os.Stdout)
		return nil
	default:
		Usage(os.Stderr)
		return fmt.Errorf("unknown command: %s", args[0])
	}
}
