package paas

import (
	"context"
	"time"
)

func LogBestEffortCtx(ctx context.Context, action, level string, details map[string]any) {
	p := ClientFromContext(ctx)
	if p == nil {
		return
	}
	ctx2, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = p.CreateLog(ctx2, CreateLogRequest{
		Agent:      "polymarket-service",
		Action:     action,
		Level:      level,
		Details:    details,
		SessionKey: "",
		Metadata:   map[string]any{},
	})
}
