package clob

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"
	"nhooyr.io/websocket"
)

const DefaultMarketWSSURL = "wss://ws-subscriptions-clob.polymarket.com/ws/market"

type MarketSubscribeRequest struct {
	Type                 string   `json:"type"`
	AssetsIDs            []string `json:"assets_ids,omitempty"`
	CustomFeatureEnabled *bool    `json:"custom_feature_enabled,omitempty"`
}

type MarketSubscriptionUpdate struct {
	AssetsIDs            []string `json:"assets_ids"`
	Operation            string   `json:"operation"`
	CustomFeatureEnabled *bool    `json:"custom_feature_enabled,omitempty"`
}

type MarketEnvelope struct {
	EventType string `json:"event_type"`
	AssetID   string `json:"asset_id"`
	Market    string `json:"market"`
	Timestamp string `json:"timestamp"`
}

type AssetIDProvider func(context.Context) ([]string, error)

type WSClient struct {
	url  string
	conn *websocket.Conn
}

func NewWSClient(url string) *WSClient {
	if strings.TrimSpace(url) == "" {
		url = DefaultMarketWSSURL
	}
	return &WSClient{url: url}
}

func (c *WSClient) Connect(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("ws client is nil")
	}
	conn, _, err := websocket.Dial(ctx, c.url, nil)
	if err != nil {
		return err
	}
	// Polymarket book updates can be large; raise read limit above default.
	conn.SetReadLimit(2 << 20) // 2MB
	c.conn = conn
	return nil
}

func (c *WSClient) Close(status websocket.StatusCode, reason string) error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close(status, reason)
}

func (c *WSClient) SubscribeMarket(ctx context.Context, assetIDs []string) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("ws not connected")
	}
	req := MarketSubscribeRequest{
		Type:      "market",
		AssetsIDs: assetIDs,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return c.conn.Write(ctx, websocket.MessageText, payload)
}

func (c *WSClient) UpdateMarketSubscription(ctx context.Context, assetIDs []string, operation string) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("ws not connected")
	}
	op := strings.ToLower(strings.TrimSpace(operation))
	if op != "subscribe" && op != "unsubscribe" {
		return fmt.Errorf("invalid operation: %s", operation)
	}
	req := MarketSubscriptionUpdate{
		AssetsIDs: assetIDs,
		Operation: op,
	}
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	return c.conn.Write(ctx, websocket.MessageText, payload)
}

func (c *WSClient) Read(ctx context.Context) (MarketEnvelope, []byte, error) {
	if c == nil || c.conn == nil {
		return MarketEnvelope{}, nil, fmt.Errorf("ws not connected")
	}
	_, data, err := c.conn.Read(ctx)
	if err != nil {
		return MarketEnvelope{}, nil, err
	}
	var env MarketEnvelope
	_ = json.Unmarshal(data, &env)
	return env, data, nil
}

type MarketStreamOptions struct {
	URL               string
	AssetIDs          []string
	AssetIDProvider   AssetIDProvider
	RefreshInterval   time.Duration
	HeartbeatInterval time.Duration
	PingTimeout       time.Duration
	BackoffMin        time.Duration
	BackoffMax        time.Duration
	Logger            *zap.Logger
}

type MarketStream struct {
	opts      MarketStreamOptions
	seenFirst bool
}

func NewMarketStream(opts MarketStreamOptions) *MarketStream {
	if opts.URL == "" {
		opts.URL = DefaultMarketWSSURL
	}
	if opts.HeartbeatInterval == 0 {
		opts.HeartbeatInterval = 20 * time.Second
	}
	if opts.PingTimeout == 0 {
		opts.PingTimeout = 5 * time.Second
	}
	if opts.BackoffMin == 0 {
		opts.BackoffMin = 1 * time.Second
	}
	if opts.BackoffMax == 0 {
		opts.BackoffMax = 30 * time.Second
	}
	if opts.RefreshInterval == 0 {
		opts.RefreshInterval = 30 * time.Second
	}
	return &MarketStream{opts: opts}
}

func (s *MarketStream) Run(ctx context.Context, onMessage func(MarketEnvelope, []byte)) error {
	if s == nil {
		return fmt.Errorf("stream is nil")
	}
	backoff := s.opts.BackoffMin
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		client := NewWSClient(s.opts.URL)
		if err := client.Connect(ctx); err != nil {
			if s.opts.Logger != nil {
				s.opts.Logger.Warn("clob ws connect failed", zap.Error(err))
			}
			if err := sleepWithJitter(ctx, backoff); err != nil {
				return err
			}
			backoff = nextBackoff(backoff, s.opts.BackoffMax)
			continue
		}
		if s.opts.Logger != nil {
			s.opts.Logger.Info("clob ws connected")
		}
		assetIDs := s.opts.AssetIDs
		if s.opts.AssetIDProvider != nil {
			if ids, err := s.opts.AssetIDProvider(ctx); err == nil {
				assetIDs = ids
			}
		}
		if len(assetIDs) == 0 {
			if s.opts.Logger != nil {
				s.opts.Logger.Warn("clob ws subscribe skipped: no assets")
			}
			_ = client.Close(websocket.StatusInternalError, "no assets to subscribe")
			if err := sleepWithJitter(ctx, backoff); err != nil {
				return err
			}
			backoff = nextBackoff(backoff, s.opts.BackoffMax)
			continue
		}
		if len(assetIDs) > 0 {
			if err := client.SubscribeMarket(ctx, assetIDs); err != nil {
				if s.opts.Logger != nil {
					s.opts.Logger.Warn("clob ws subscribe failed", zap.Error(err))
				}
				_ = client.Close(websocket.StatusInternalError, "subscribe failed")
				if err := sleepWithJitter(ctx, backoff); err != nil {
					return err
				}
				backoff = nextBackoff(backoff, s.opts.BackoffMax)
				continue
			}
			if s.opts.Logger != nil {
				s.opts.Logger.Info("clob ws subscribed", zap.Int("assets", len(assetIDs)))
			}
		}
		backoff = s.opts.BackoffMin

		current := setFromSlice(assetIDs)
		err := s.consume(ctx, client, onMessage, current)
		_ = client.Close(websocket.StatusNormalClosure, "reconnect")
		if err == nil || errors.Is(err, context.Canceled) {
			return err
		}
		if err := sleepWithJitter(ctx, backoff); err != nil {
			return err
		}
		backoff = nextBackoff(backoff, s.opts.BackoffMax)
	}
}

func (s *MarketStream) consume(ctx context.Context, client *WSClient, onMessage func(MarketEnvelope, []byte), current map[string]struct{}) error {
	if client == nil {
		return fmt.Errorf("ws client is nil")
	}
	heartbeatErr := make(chan error, 1)
	heartbeatCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	var refreshErr chan error
	refreshCtx, cancelRefresh := context.WithCancel(ctx)
	defer cancelRefresh()

	go func() {
		ticker := time.NewTicker(s.opts.HeartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-heartbeatCtx.Done():
				heartbeatErr <- heartbeatCtx.Err()
				return
			case <-ticker.C:
				pingCtx, cancelPing := context.WithTimeout(heartbeatCtx, s.opts.PingTimeout)
				err := client.conn.Ping(pingCtx)
				cancelPing()
				if err != nil {
					heartbeatErr <- err
					return
				}
			}
		}
	}()

	if s.opts.AssetIDProvider != nil && s.opts.RefreshInterval > 0 {
		refreshErr = make(chan error, 1)
		go func() {
			ticker := time.NewTicker(s.opts.RefreshInterval)
			defer ticker.Stop()
			for {
				select {
				case <-refreshCtx.Done():
					refreshErr <- refreshCtx.Err()
					return
				case <-ticker.C:
					ids, err := s.opts.AssetIDProvider(refreshCtx)
					if err != nil {
						continue
					}
					next := setFromSlice(ids)
					added, removed := diffSets(current, next)
					if len(added) > 0 {
						_ = client.UpdateMarketSubscription(refreshCtx, added, "subscribe")
					}
					if len(removed) > 0 {
						_ = client.UpdateMarketSubscription(refreshCtx, removed, "unsubscribe")
					}
					current = next
				}
			}
		}()
	}

	for {
		select {
		case err := <-heartbeatErr:
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		case err := <-refreshErr:
			if err != nil && !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		default:
		}
		env, raw, err := client.Read(ctx)
		if err != nil {
			if s.opts.Logger != nil && !errors.Is(err, context.Canceled) {
				s.opts.Logger.Warn("clob ws read failed", zap.Error(err))
			}
			return err
		}
		if isPingPayload(env, raw) {
			_ = client.respondPong(ctx)
			continue
		}
		if s.opts.Logger != nil && !s.seenFirst {
			s.seenFirst = true
			s.opts.Logger.Info("clob ws first message", zap.String("event_type", env.EventType))
		}
		if onMessage != nil {
			onMessage(env, raw)
		}
	}
}

func (c *WSClient) respondPong(ctx context.Context) error {
	if c == nil || c.conn == nil {
		return fmt.Errorf("ws not connected")
	}
	payload := []byte(`{"event_type":"pong"}`)
	return c.conn.Write(ctx, websocket.MessageText, payload)
}

func isPingPayload(env MarketEnvelope, raw []byte) bool {
	if strings.EqualFold(env.EventType, "ping") {
		return true
	}
	if len(raw) == 0 {
		return false
	}
	if strings.TrimSpace(string(raw)) == "ping" {
		return true
	}
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err == nil {
		if strings.EqualFold(probe.Type, "ping") {
			return true
		}
	}
	return false
}

func nextBackoff(current, max time.Duration) time.Duration {
	next := current * 2
	if next > max {
		return max
	}
	return next
}

func sleepWithJitter(ctx context.Context, base time.Duration) error {
	if base <= 0 {
		return nil
	}
	jitter := time.Duration(rand.Int63n(int64(base / 2)))
	timer := time.NewTimer(base + jitter)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func setFromSlice(items []string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out[item] = struct{}{}
	}
	return out
}

func diffSets(current, next map[string]struct{}) ([]string, []string) {
	added := make([]string, 0)
	removed := make([]string, 0)
	for key := range next {
		if _, ok := current[key]; !ok {
			added = append(added, key)
		}
	}
	for key := range current {
		if _, ok := next[key]; !ok {
			removed = append(removed, key)
		}
	}
	return added, removed
}
