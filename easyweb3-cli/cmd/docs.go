package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func docsCmd(ctx Context, args []string) error {
	if len(args) == 0 {
		return errors.New("docs subcommand required: url|get")
	}
	switch args[0] {
	case "url":
		if len(args) < 2 {
			return errors.New("usage: easyweb3 docs url architecture|openclaw")
		}
		u, err := docURL(ctx.APIBase, args[1])
		if err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout, u)
		return nil

	case "get":
		fs := flag.NewFlagSet("easyweb3 docs get", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		outPath := fs.String("out", "", "write to file (optional)")
		_ = fs.Parse(args[1:])
		if fs.NArg() < 1 {
			return errors.New("usage: easyweb3 docs get [--out file] architecture|openclaw")
		}

		u, err := docURL(ctx.APIBase, fs.Arg(0))
		if err != nil {
			return err
		}

		body, ct, err := httpGetRaw(u)
		if err != nil {
			return err
		}
		_ = ct // informational; we write raw bytes

		if strings.TrimSpace(*outPath) != "" {
			return os.WriteFile(*outPath, body, 0o644)
		}
		_, _ = os.Stdout.Write(body)
		if len(body) > 0 && body[len(body)-1] != '\n' {
			_, _ = io.WriteString(os.Stdout, "\n")
		}
		return nil

	default:
		return fmt.Errorf("unknown docs subcommand: %s", args[0])
	}
}

func docURL(apiBase, name string) (string, error) {
	base := strings.TrimRight(strings.TrimSpace(apiBase), "/")
	if base == "" {
		return "", errors.New("--api-base is empty")
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "architecture", "arch":
		return base + "/docs/architecture", nil
	case "openclaw", "picoclaw":
		return base + "/docs/openclaw", nil
	default:
		return "", errors.New("unknown doc: " + name + " (expected architecture|openclaw)")
	}
}

func httpGetRaw(url string) ([]byte, string, error) {
	c := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
		return nil, "", fmt.Errorf("http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
	b, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, "", err
	}
	return b, ct, nil
}
