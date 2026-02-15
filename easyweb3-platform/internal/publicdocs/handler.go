package publicdocs

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
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
	_, _ = w.Write([]byte("# Public Docs\n\n- /docs/architecture\n- /docs/openclaw\n"))
}

func (h Handler) Architecture(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "ARCHITECTURE.md")
}

func (h Handler) OpenClaw(w http.ResponseWriter, r *http.Request) {
	h.serve(w, r, "OPENCLAW.md")
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
