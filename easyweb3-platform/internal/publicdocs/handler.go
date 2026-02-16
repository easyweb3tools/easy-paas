package publicdocs

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/nicekwell/easyweb3-platform/internal/httpx"
)

type Handler struct {
	// Dir holds markdown files to be served publicly.
	// Expected file names: ARCHITECTURE.md, OPENCLAW.md.
	Dir string
}

func (h Handler) Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	dir := strings.TrimSpace(h.Dir)
	if dir == "" {
		_, _ = w.Write([]byte("# Public Docs\n\n- docs not configured\n"))
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		_, _ = w.Write([]byte("# Public Docs\n\n- failed to read docs directory\n"))
		return
	}
	names := make([]string, 0, len(entries))
	for _, ent := range entries {
		if ent.IsDir() {
			continue
		}
		name := strings.TrimSpace(ent.Name())
		if !strings.HasSuffix(strings.ToLower(name), ".md") {
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		_, _ = w.Write([]byte("# Public Docs\n\n- no markdown files found\n"))
		return
	}
	var b strings.Builder
	b.WriteString("# Public Docs\n\n")
	for _, name := range names {
		b.WriteString("- /docs/")
		b.WriteString(name)
		b.WriteByte('\n')
	}
	_, _ = w.Write([]byte(b.String()))
}

func (h Handler) Architecture(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "ARCHITECTURE.md")
}

func (h Handler) OpenClaw(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "OPENCLAW.md")
}

func (h Handler) File(w http.ResponseWriter, r *http.Request, name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		httpx.WriteError(w, http.StatusNotFound, "doc not found")
		return
	}
	switch strings.ToLower(name) {
	case "architecture":
		h.serve(w, r, "ARCHITECTURE.md")
		return
	case "openclaw":
		h.serve(w, r, "OPENCLAW.md")
		return
	}
	if !strings.HasSuffix(strings.ToLower(name), ".md") {
		name = name + ".md"
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		httpx.WriteError(w, http.StatusBadRequest, "invalid doc")
		return
	}
	h.serve(w, r, name)
}

func (h Handler) serve(w http.ResponseWriter, r *http.Request, filename string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		httpx.WriteError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	dir := strings.TrimSpace(h.Dir)
	if dir == "" {
		httpx.WriteError(w, http.StatusNotFound, "docs not configured")
		return
	}

	filename = strings.TrimSpace(filename)
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		httpx.WriteError(w, http.StatusBadRequest, "invalid doc")
		return
	}

	p := filepath.Join(dir, filename)
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			httpx.WriteError(w, http.StatusNotFound, "doc not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "failed to read doc")
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
}
