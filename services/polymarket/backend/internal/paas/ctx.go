package paas

import "context"

type ctxKey int

const clientCtxKey ctxKey = 1

func WithClient(ctx context.Context, c *Client) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, clientCtxKey, c)
}

func ClientFromContext(ctx context.Context) *Client {
	if ctx == nil {
		return nil
	}
	v := ctx.Value(clientCtxKey)
	c, _ := v.(*Client)
	return c
}
