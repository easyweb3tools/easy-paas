package paas

import (
	"context"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func RequireBearerMiddleware() gin.HandlerFunc {
	disabled := strings.EqualFold(os.Getenv("PM_AUTH_DISABLED"), "true") || os.Getenv("PM_AUTH_DISABLED") == "1"
	requireGatewayHeader := strings.EqualFold(os.Getenv("PM_REQUIRE_GATEWAY"), "true") || os.Getenv("PM_REQUIRE_GATEWAY") == "1"

	return func(c *gin.Context) {
		if disabled {
			c.Next()
			return
		}
		p := c.Request.URL.Path
		// Keep infra endpoints open.
		if p == "/healthz" || p == "/readyz" || p == "/metrics" {
			c.Next()
			return
		}
		// Protect API + swagger + docs.
		if strings.HasPrefix(p, "/api/") || strings.HasPrefix(p, "/swagger") || p == "/docs" {
			auth := strings.TrimSpace(c.GetHeader("Authorization"))
			if !strings.HasPrefix(auth, "Bearer ") {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
				return
			}
			if requireGatewayHeader {
				if strings.TrimSpace(c.GetHeader("X-Easyweb3-Project")) == "" {
					c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing X-Easyweb3-Project"})
					return
				}
			}
		}
		c.Next()
	}
}

func PaaSWriteAuditMiddleware(p *Client, logger *zap.Logger) gin.HandlerFunc {
	if p == nil {
		return func(c *gin.Context) { c.Next() }
	}
	agent := strings.TrimSpace(os.Getenv("PM_PAAS_AGENT"))
	if agent == "" {
		agent = "polymarket-service"
	}

	return func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.Request.URL.Path
		method := strings.ToUpper(c.Request.Method)
		if !(strings.HasPrefix(path, "/api/") || strings.HasPrefix(path, "/swagger")) {
			return
		}
		// Only log write-ish methods by default.
		if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
			return
		}

		status := c.Writer.Status()
		dur := time.Since(start)
		proj := strings.TrimSpace(c.GetHeader("X-Easyweb3-Project"))
		role := strings.TrimSpace(c.GetHeader("X-Easyweb3-Role"))

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := p.CreateLog(ctx, CreateLogRequest{
			Agent:  agent,
			Action: "polymarket_http_write",
			Level:  levelFromStatus(status),
			Details: map[string]any{
				"method":   method,
				"path":     path,
				"status":   status,
				"duration": dur.String(),
				"project":  proj,
				"role":     role,
			},
			SessionKey: "",
			Metadata:   map[string]any{},
		})
		if err != nil && logger != nil {
			logger.Debug("paas audit log failed", zap.Error(err))
		}
	}
}

func levelFromStatus(status int) string {
	if status >= 500 {
		return "error"
	}
	if status >= 400 {
		return "warn"
	}
	return "info"
}
