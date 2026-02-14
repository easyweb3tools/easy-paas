package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/datatypes"

	"polymarket/internal/client/polymarket/clob"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type CLOBStreamService struct {
	Repo       repository.CatalogRepository
	Logger     *zap.Logger
	lastPrices map[string]float64
}

type CLOBStreamOptions struct {
	URL             string
	AssetIDs        []string
	RefreshInterval time.Duration
	MaxAssets       int
}

func (s *CLOBStreamService) RunMarketStream(ctx context.Context, opts CLOBStreamOptions) error {
	if s.lastPrices == nil {
		s.lastPrices = map[string]float64{}
	}
	if s.Logger != nil {
		s.Logger.Info("clob stream starting",
			zap.String("url", opts.URL),
			zap.Duration("refresh_interval", opts.RefreshInterval),
			zap.Int("max_assets", opts.MaxAssets),
		)
	}
	var provider clob.AssetIDProvider
	if len(opts.AssetIDs) == 0 {
		provider = func(ctx context.Context) ([]string, error) {
			ids, err := s.fetchStreamAssetIDs(ctx, opts.MaxAssets)
			if err != nil && s.Logger != nil {
				s.Logger.Warn("fetch stream asset ids failed", zap.Error(err))
			}
			if s.Logger != nil {
				s.Logger.Info("stream asset ids refreshed", zap.Int("count", len(ids)))
			}
			return ids, err
		}
	}
	stream := clob.NewMarketStream(clob.MarketStreamOptions{
		URL:             opts.URL,
		AssetIDs:        opts.AssetIDs,
		AssetIDProvider: provider,
		RefreshInterval: opts.RefreshInterval,
		Logger:          s.Logger,
	})
	return stream.Run(ctx, func(env clob.MarketEnvelope, raw []byte) {
		s.handleMarketMessage(ctx, env, raw)
	})
}

func (s *CLOBStreamService) handleMarketMessage(ctx context.Context, env clob.MarketEnvelope, raw []byte) {
	if s == nil || s.Repo == nil {
		return
	}
	now := time.Now().UTC()
	tokenID := strings.TrimSpace(env.AssetID)
	if tokenID == "" {
		tokenID = extractTokenID(raw)
	}

	_ = s.Repo.InsertRawWSEvent(ctx, &models.RawWSEvent{
		TokenID:    strPtr(tokenID),
		EventType:  normalizeEventType(env.EventType, raw),
		Sequence:   extractSequence(raw),
		ReceivedAt: now,
		Payload:    datatypes.JSON(raw),
	})

	eventType := normalizeEventType(env.EventType, raw)
	switch eventType {
	case "book":
		if err := s.handleBook(ctx, tokenID, env, raw); err != nil && s.Logger != nil {
			s.Logger.Warn("handle book failed", zap.Error(err))
		}
	case "price_change", "last_trade_price":
		if eventType == "last_trade_price" {
			if err := s.handleLastTradePrice(ctx, tokenID, env, raw); err != nil && s.Logger != nil {
				s.Logger.Warn("handle last_trade_price failed", zap.Error(err))
			}
		}
		_ = s.updateHealth(ctx, tokenID, now, eventType, nil)
	default:
		_ = s.updateHealth(ctx, tokenID, now, eventType, nil)
	}
}

func (s *CLOBStreamService) fetchStreamAssetIDs(ctx context.Context, maxAssets int) ([]string, error) {
	if s == nil || s.Repo == nil {
		return nil, nil
	}
	if maxAssets <= 0 {
		maxAssets = 200
	}
	marketIDs, err := s.Repo.ListMarketIDsForStream(ctx, maxAssets)
	if err != nil {
		return nil, err
	}
	tokens, err := s.Repo.ListTokensByMarketIDs(ctx, marketIDs)
	if err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token.ID == "" {
			continue
		}
		if _, ok := seen[token.ID]; ok {
			continue
		}
		seen[token.ID] = struct{}{}
		out = append(out, token.ID)
		if len(out) >= maxAssets {
			break
		}
	}
	return out, nil
}

func (s *CLOBStreamService) handleBook(ctx context.Context, tokenID string, env clob.MarketEnvelope, raw []byte) error {
	if tokenID == "" {
		return fmt.Errorf("token_id missing")
	}
	book, err := parseBookPayload(raw)
	if err != nil {
		return err
	}
	bestBid := topPrice(book.Bids)
	bestAsk := topPrice(book.Asks)
	mid := computeMid(bestBid, bestAsk)
	snapshotTS := parseTimestamp(env.Timestamp)
	if snapshotTS.IsZero() {
		snapshotTS = time.Now().UTC()
	}
	item := &models.OrderbookLatest{
		TokenID:        tokenID,
		SnapshotTS:     snapshotTS,
		BidsJSON:       datatypes.JSON(book.BidsRaw),
		AsksJSON:       datatypes.JSON(book.AsksRaw),
		BestBid:        bestBid,
		BestAsk:        bestAsk,
		Mid:            mid,
		Source:         strPtr("ws"),
		DataAgeSeconds: 0,
		UpdatedAt:      time.Now().UTC(),
	}
	if err := s.Repo.UpsertOrderbookLatest(ctx, item); err != nil {
		return err
	}
	return s.updateHealthWithBook(ctx, tokenID, time.Now().UTC(), "book", &snapshotTS, bestBid, bestAsk, mid)
}

func (s *CLOBStreamService) updateHealth(ctx context.Context, tokenID string, now time.Time, reason string, lastWSTS *time.Time) error {
	if tokenID == "" {
		return nil
	}
	item := &models.MarketDataHealth{
		TokenID:        tokenID,
		WSConnected:    true,
		LastWSTS:       lastWSTS,
		DataAgeSeconds: 0,
		Stale:          false,
		NeedsResync:    false,
		Reason:         strPtr(reason),
		UpdatedAt:      now,
	}
	return s.Repo.UpsertMarketDataHealth(ctx, item)
}

func (s *CLOBStreamService) updateHealthWithBook(ctx context.Context, tokenID string, now time.Time, reason string, lastWSTS *time.Time, bestBid, bestAsk, mid *float64) error {
	if tokenID == "" {
		return nil
	}
	spread, spreadBps := computeSpread(bestBid, bestAsk, mid)
	item := &models.MarketDataHealth{
		TokenID:          tokenID,
		WSConnected:      true,
		LastWSTS:         lastWSTS,
		LastBookChangeTS: lastWSTS,
		DataAgeSeconds:   0,
		Stale:            false,
		NeedsResync:      false,
		Spread:           spread,
		SpreadBps:        spreadBps,
		Reason:           strPtr(reason),
		UpdatedAt:        now,
	}
	return s.Repo.UpsertMarketDataHealth(ctx, item)
}

func (s *CLOBStreamService) handleLastTradePrice(ctx context.Context, tokenID string, env clob.MarketEnvelope, raw []byte) error {
	if tokenID == "" {
		return fmt.Errorf("token_id missing")
	}
	price := parseLastTradePrice(raw)
	if price <= 0 {
		return fmt.Errorf("invalid trade price")
	}
	tradeTS := parseTimestamp(env.Timestamp)
	if tradeTS.IsZero() {
		tradeTS = parseEventTimestamp(raw)
	}
	item := &models.LastTradePrice{
		TokenID:   tokenID,
		Price:     price,
		TradeTS:   timePtr(tradeTS),
		Source:    strPtr("ws"),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.Repo.UpsertLastTradePrice(ctx, item); err != nil {
		return err
	}
	prev, _ := s.lastTradePrice(tokenID)
	jumpBps := computePriceJumpBps(prev, price)
	s.setLastTradePrice(tokenID, price)
	now := time.Now().UTC()
	_ = s.Repo.UpsertMarketDataHealth(ctx, &models.MarketDataHealth{
		TokenID:        tokenID,
		WSConnected:    true,
		LastWSTS:       timePtr(tradeTS),
		DataAgeSeconds: 0,
		Stale:          false,
		NeedsResync:    false,
		PriceJumpBps:   jumpBps,
		Reason:         strPtr("last_trade_price"),
		UpdatedAt:      now,
	})
	return nil
}

func (s *CLOBStreamService) lastTradePrice(tokenID string) (float64, bool) {
	if s.lastPrices == nil {
		return 0, false
	}
	val, ok := s.lastPrices[tokenID]
	return val, ok
}

func (s *CLOBStreamService) setLastTradePrice(tokenID string, price float64) {
	if s.lastPrices == nil {
		s.lastPrices = map[string]float64{}
	}
	s.lastPrices[tokenID] = price
}

func computePriceJumpBps(prev, current float64) *float64 {
	if prev <= 0 || current <= 0 {
		return nil
	}
	jump := ((current - prev) / prev) * 10000
	return &jump
}

type bookPayload struct {
	BidsRaw json.RawMessage
	AsksRaw json.RawMessage
	Bids    []priceLevel
	Asks    []priceLevel
}

type priceLevel struct {
	Price float64
	Size  float64
}

func parseBookPayload(raw []byte) (bookPayload, error) {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err != nil {
		return bookPayload{}, err
	}
	payload := root["book"]
	if len(payload) == 0 {
		payload = root["data"]
	}
	if len(payload) == 0 {
		payload = raw
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(payload, &obj); err != nil {
		return bookPayload{}, err
	}
	bidsRaw := obj["bids"]
	asksRaw := obj["asks"]
	bids := parseLevels(bidsRaw)
	asks := parseLevels(asksRaw)
	return bookPayload{
		BidsRaw: bidsRaw,
		AsksRaw: asksRaw,
		Bids:    bids,
		Asks:    asks,
	}, nil
}

func parseLevels(raw json.RawMessage) []priceLevel {
	if len(raw) == 0 {
		return nil
	}
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil {
		out := make([]priceLevel, 0, len(arr))
		for _, item := range arr {
			if level, ok := parseLevel(item); ok {
				out = append(out, level)
			}
		}
		return out
	}
	var objArr []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &objArr); err == nil {
		out := make([]priceLevel, 0, len(objArr))
		for _, item := range objArr {
			level := priceLevel{
				Price: parseFloat(item["price"]),
				Size:  parseFloat(firstRaw(item, "size", "qty", "amount")),
			}
			if level.Price > 0 {
				out = append(out, level)
			}
		}
		return out
	}
	return nil
}

func parseLevel(raw json.RawMessage) (priceLevel, bool) {
	var arr []json.RawMessage
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) >= 2 {
		return priceLevel{
			Price: parseFloat(arr[0]),
			Size:  parseFloat(arr[1]),
		}, true
	}
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err == nil {
		level := priceLevel{
			Price: parseFloat(obj["price"]),
			Size:  parseFloat(firstRaw(obj, "size", "qty", "amount")),
		}
		if level.Price > 0 {
			return level, true
		}
	}
	return priceLevel{}, false
}

func topPrice(levels []priceLevel) *float64 {
	if len(levels) == 0 {
		return nil
	}
	val := levels[0].Price
	if val <= 0 {
		return nil
	}
	return &val
}

func parseTimestamp(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Time{}
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return ts
	}
	if v, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(v, 0).UTC()
	}
	return time.Time{}
}

func normalizeEventType(eventType string, raw []byte) string {
	val := strings.ToLower(strings.TrimSpace(eventType))
	if val != "" {
		return val
	}
	var probe struct {
		EventType string `json:"event_type"`
		Type      string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err == nil {
		if probe.EventType != "" {
			return strings.ToLower(strings.TrimSpace(probe.EventType))
		}
		if probe.Type != "" {
			return strings.ToLower(strings.TrimSpace(probe.Type))
		}
	}
	return "unknown"
}

func extractSequence(raw []byte) *int64 {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	seqRaw := firstRaw(obj, "sequence", "seq", "sequence_number")
	if len(seqRaw) == 0 {
		return nil
	}
	val, err := strconv.ParseInt(strings.Trim(string(seqRaw), "\""), 10, 64)
	if err != nil {
		return nil
	}
	return &val
}

func parseLastTradePrice(raw []byte) float64 {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(raw, &root); err == nil {
		if val := parseFloat(firstRaw(root, "last_trade_price", "lastTradePrice", "price", "last_trade")); val > 0 {
			return val
		}
		if data := root["data"]; len(data) > 0 {
			var obj map[string]json.RawMessage
			if err := json.Unmarshal(data, &obj); err == nil {
				if val := parseFloat(firstRaw(obj, "last_trade_price", "lastTradePrice", "price", "last_trade")); val > 0 {
					return val
				}
			}
		}
	}
	return 0
}

func extractTokenID(raw []byte) string {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return ""
	}
	rawID := firstRaw(obj, "asset_id", "token_id", "tokenId")
	if len(rawID) == 0 {
		return ""
	}
	return strings.Trim(string(rawID), "\"")
}

func parseFloat(raw json.RawMessage) float64 {
	if len(raw) == 0 {
		return 0
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if val, err := strconv.ParseFloat(s, 64); err == nil {
			return val
		}
	}
	return 0
}

func firstRaw(m map[string]json.RawMessage, keys ...string) json.RawMessage {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return nil
}

func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

func parseEventTimestamp(raw []byte) time.Time {
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return time.Time{}
	}
	val := firstRaw(obj, "timestamp", "ts", "time")
	if len(val) == 0 {
		if data := obj["data"]; len(data) > 0 {
			var inner map[string]json.RawMessage
			if err := json.Unmarshal(data, &inner); err == nil {
				val = firstRaw(inner, "timestamp", "ts", "time")
			}
		}
	}
	if len(val) == 0 {
		return time.Time{}
	}
	return parseTimestamp(strings.Trim(string(val), "\""))
}
