package paas

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
)

func InjectClientMiddleware(p *Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		if p != nil && c.Request != nil {
			c.Request = c.Request.WithContext(WithClient(c.Request.Context(), p))
		}
		c.Next()
	}
}

func ClientFromGin(c *gin.Context) *Client {
	if c == nil {
		return nil
	}
	if c.Request == nil {
		return nil
	}
	return ClientFromContext(c.Request.Context())
}

func LogBestEffort(c *gin.Context, action, level string, details map[string]any) {
	p := ClientFromGin(c)
	if p == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.CreateLog(ctx, CreateLogRequest{
		Agent:      "polymarket-service",
		Action:     action,
		Level:      level,
		Details:    details,
		SessionKey: "",
		Metadata:   map[string]any{},
	})
}
