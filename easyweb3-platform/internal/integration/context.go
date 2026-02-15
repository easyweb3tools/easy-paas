package integration

import (
	"context"
	"strings"
)

type ctxKey string

const (
	ctxKeyBearer  ctxKey = "integration_bearer"
	ctxKeyProject ctxKey = "integration_project"
	ctxKeyRole    ctxKey = "integration_role"
)

func withBearer(ctx context.Context, authorizationHeader string) context.Context {
	h := strings.TrimSpace(authorizationHeader)
	if h == "" {
		return ctx
	}
	// Expect "Bearer <token>". Keep original token only.
	const prefix = "Bearer "
	if strings.HasPrefix(h, prefix) {
		h = strings.TrimSpace(strings.TrimPrefix(h, prefix))
	}
	if h == "" {
		return ctx
	}
	return context.WithValue(ctx, ctxKeyBearer, h)
}

func withClaims(ctx context.Context, projectID, role string) context.Context {
	if strings.TrimSpace(projectID) != "" {
		ctx = context.WithValue(ctx, ctxKeyProject, strings.TrimSpace(projectID))
	}
	if strings.TrimSpace(role) != "" {
		ctx = context.WithValue(ctx, ctxKeyRole, strings.TrimSpace(role))
	}
	return ctx
}

func bearerFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyBearer).(string)
	return strings.TrimSpace(v)
}

func projectFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyProject).(string)
	return strings.TrimSpace(v)
}

func roleFromContext(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyRole).(string)
	return strings.TrimSpace(v)
}
