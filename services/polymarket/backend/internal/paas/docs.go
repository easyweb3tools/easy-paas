package paas

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func RegisterDocs(r *gin.Engine) {
	r.GET("/docs", func(c *gin.Context) {
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.String(http.StatusOK, `# Polymarket Service (SaaS)

This service is intended to be accessed via easyweb3 PaaS Gateway.

## Access via PaaS

Base path (through gateway):
- /api/v1/services/polymarket/

Examples:
- GET /api/v1/services/polymarket/healthz
- POST /api/v1/services/polymarket/api/catalog/sync
- GET /api/v1/services/polymarket/api/catalog/events
- GET /api/v1/services/polymarket/api/v2/opportunities

## Auth

All /api/* routes require a Bearer token (validated by the PaaS gateway).
Health endpoints are public.

## Notable Routes (upstream)

- GET /healthz
- GET /readyz
- GET /swagger/index.html
- POST /api/catalog/sync
- GET /api/catalog/events
- GET /api/catalog/markets
- GET /api/catalog/tokens
- GET /api/v2/opportunities
- GET /api/v2/signals
- GET /api/v2/strategies
- GET /api/v2/analytics
- GET /api/v2/settlements
`)
	})
}
