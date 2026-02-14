package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"go.uber.org/zap"
	"gorm.io/datatypes"
	"gorm.io/gorm"

	"polymarket/internal/client/polymarket/clob"
	polymarketgamma "polymarket/internal/client/polymarket/gamma"
	"polymarket/internal/models"
	"polymarket/internal/repository"
)

type CatalogSyncService struct {
	Store  repository.CatalogRepository
	Gamma  *polymarketgamma.Client
	Clob   *clob.Client
	Logger *zap.Logger
}

type SyncOptions struct {
	Scope             string
	Limit             int
	MaxPages          int
	Resume            bool
	TagID             *int
	Closed            *bool
	BookMaxAssets     int
	BookBatchSize     int
	BookSleepPerBatch time.Duration
}

type SyncResult struct {
	Scope      string `json:"scope"`
	Pages      int    `json:"pages"`
	Events     int    `json:"events"`
	Markets    int    `json:"markets"`
	Tokens     int    `json:"tokens"`
	Series     int    `json:"series"`
	Tags       int    `json:"tags"`
	EventTags  int    `json:"event_tags"`
	BookAssets int    `json:"book_assets"`
	BookErrors int    `json:"book_errors"`
	NextOffset int    `json:"next_offset"`
	Done       bool   `json:"done"`
}

func (s *CatalogSyncService) Sync(ctx context.Context, opts SyncOptions) (SyncResult, error) {
	scope := strings.ToLower(strings.TrimSpace(opts.Scope))
	if scope == "" {
		scope = "events"
	}
	switch scope {
	case "events":
		return s.syncEvents(ctx, opts)
	case "series":
		return s.syncSeries(ctx, opts)
	case "tags":
		return s.syncTags(ctx, opts)
	case "markets":
		return s.syncMarkets(ctx, opts)
	case "all":
		result := SyncResult{Scope: "all"}
		res, err := s.syncEvents(ctx, opts)
		if err != nil {
			return result, err
		}
		result.Events += res.Events
		result.Markets += res.Markets
		result.Tokens += res.Tokens
		result.Series += res.Series
		result.Tags += res.Tags
		result.EventTags += res.EventTags
		result.Pages += res.Pages
		result.NextOffset = res.NextOffset
		result.Done = res.Done
		book, err := s.resyncBooks(ctx, opts)
		if err != nil {
			return result, err
		}
		result.BookAssets = book.Assets
		result.BookErrors = book.Errors
		return result, nil
	default:
		return SyncResult{}, fmt.Errorf("unsupported scope: %s", scope)
	}
}

func (s *CatalogSyncService) syncEvents(ctx context.Context, opts SyncOptions) (SyncResult, error) {
	if s.Gamma == nil {
		return SyncResult{}, fmt.Errorf("gamma client is nil")
	}
	limit := normalizeLimit(opts.Limit)
	maxPages := normalizeMaxPages(opts.MaxPages)
	offset := 0
	if opts.Resume {
		state, err := s.Store.GetSyncState(ctx, "events")
		if err != nil {
			return SyncResult{}, err
		}
		if state != nil && state.Cursor != nil {
			if parsed, err := strconv.Atoi(*state.Cursor); err == nil {
				offset = parsed
			}
		}
	}

	now := time.Now().UTC()
	result := SyncResult{Scope: "events"}
	for page := 0; page < maxPages; page++ {
		params := &polymarketgamma.GetEventsParams{
			Limit:  limit,
			Offset: offset,
			TagID:  opts.TagID,
			Closed: opts.Closed,
		}
		events, err := s.Gamma.GetEvents(ctx, params)
		if err != nil {
			s.writeSyncError(ctx, "events", err)
			return result, err
		}
		if len(events) == 0 {
			result.Done = true
			break
		}

		series, tags, eventTags, markets, tokens, eventsOut := mapEventsPayload(events, now)
		markets, tokens, err = s.filterMarketsAndTokens(ctx, markets, tokens)
		if err != nil {
			s.writeSyncError(ctx, "events", err)
			return result, err
		}
		nextOffset := offset + len(events)

		err = s.Store.InTx(ctx, func(tx *gorm.DB) error {
			if err := s.Store.UpsertSeriesTx(ctx, tx, series); err != nil {
				return err
			}
			if err := s.Store.UpsertTagsTx(ctx, tx, tags); err != nil {
				return err
			}
			if err := s.Store.UpsertEventsTx(ctx, tx, eventsOut); err != nil {
				return err
			}
			if err := s.Store.UpsertMarketsTx(ctx, tx, markets); err != nil {
				return err
			}
			if err := s.Store.UpsertTokensTx(ctx, tx, tokens); err != nil {
				return err
			}
			if err := s.Store.UpsertEventTagsTx(ctx, tx, eventTags); err != nil {
				return err
			}
			state := &models.SyncState{
				Scope:         "events",
				Cursor:        strPtr(strconv.Itoa(nextOffset)),
				LastAttemptAt: &now,
				LastSuccessAt: &now,
				LastError:     nil,
				StatsJSON:     statsJSON(map[string]int{"events": len(events), "markets": len(markets), "tokens": len(tokens), "tags": len(tags), "series": len(series)}),
			}
			return s.Store.SaveSyncStateTx(ctx, tx, state)
		})
		if err != nil {
			s.writeSyncError(ctx, "events", err)
			return result, err
		}

		result.Pages++
		result.Events += len(events)
		result.Markets += len(markets)
		result.Tokens += len(tokens)
		result.Series += len(series)
		result.Tags += len(tags)
		result.EventTags += len(eventTags)
		result.NextOffset = nextOffset

		offset = nextOffset
		if len(events) < limit {
			result.Done = true
			break
		}
	}
	return result, nil
}

func (s *CatalogSyncService) syncSeries(ctx context.Context, opts SyncOptions) (SyncResult, error) {
	if s.Gamma == nil {
		return SyncResult{}, fmt.Errorf("gamma client is nil")
	}
	limit := normalizeLimit(opts.Limit)
	maxPages := normalizeMaxPages(opts.MaxPages)
	offset := 0
	if opts.Resume {
		state, err := s.Store.GetSyncState(ctx, "series")
		if err != nil {
			return SyncResult{}, err
		}
		if state != nil && state.Cursor != nil {
			if parsed, err := strconv.Atoi(*state.Cursor); err == nil {
				offset = parsed
			}
		}
	}

	now := time.Now().UTC()
	result := SyncResult{Scope: "series"}
	for page := 0; page < maxPages; page++ {
		params := &polymarketgamma.GetSeriesParams{
			Limit:  limit,
			Offset: offset,
			Closed: opts.Closed,
		}
		items, err := s.Gamma.GetSeries(ctx, params)
		if err != nil {
			s.writeSyncError(ctx, "series", err)
			return result, err
		}
		if len(items) == 0 {
			result.Done = true
			break
		}
		series := make([]models.Series, 0, len(items))
		for _, item := range items {
			series = append(series, models.Series{
				ID:                item.ID,
				Title:             item.Title,
				Slug:              strPtr(item.Slug),
				Image:             strPtr(item.Image),
				ExternalUpdatedAt: normalizedTimePtr(item.UpdatedAt),
				LastSeenAt:        now,
				RawJSON:           mustJSON(item),
			})
		}
		nextOffset := offset + len(items)

		err = s.Store.InTx(ctx, func(tx *gorm.DB) error {
			if err := s.Store.UpsertSeriesTx(ctx, tx, series); err != nil {
				return err
			}
			state := &models.SyncState{
				Scope:         "series",
				Cursor:        strPtr(strconv.Itoa(nextOffset)),
				LastAttemptAt: &now,
				LastSuccessAt: &now,
				LastError:     nil,
				StatsJSON:     statsJSON(map[string]int{"series": len(series)}),
			}
			return s.Store.SaveSyncStateTx(ctx, tx, state)
		})
		if err != nil {
			s.writeSyncError(ctx, "series", err)
			return result, err
		}

		result.Pages++
		result.Series += len(series)
		result.NextOffset = nextOffset
		offset = nextOffset
		if len(items) < limit {
			result.Done = true
			break
		}
	}
	return result, nil
}

func (s *CatalogSyncService) syncTags(ctx context.Context, opts SyncOptions) (SyncResult, error) {
	if s.Gamma == nil {
		return SyncResult{}, fmt.Errorf("gamma client is nil")
	}
	limit := normalizeLimit(opts.Limit)
	maxPages := normalizeMaxPages(opts.MaxPages)
	offset := 0
	if opts.Resume {
		state, err := s.Store.GetSyncState(ctx, "tags")
		if err != nil {
			return SyncResult{}, err
		}
		if state != nil && state.Cursor != nil {
			if parsed, err := strconv.Atoi(*state.Cursor); err == nil {
				offset = parsed
			}
		}
	}

	now := time.Now().UTC()
	result := SyncResult{Scope: "tags"}
	for page := 0; page < maxPages; page++ {
		params := &polymarketgamma.GetTagsParams{
			Limit:  limit,
			Offset: offset,
		}
		items, err := s.Gamma.GetTags(ctx, params)
		if err != nil {
			s.writeSyncError(ctx, "tags", err)
			return result, err
		}
		if len(items) == 0 {
			result.Done = true
			break
		}
		tags := make([]models.Tag, 0, len(items))
		for _, item := range items {
			tags = append(tags, models.Tag{
				ID:                item.ID,
				Label:             item.Label,
				Slug:              item.Slug,
				ExternalUpdatedAt: normalizedTimePtr(item.UpdatedAt),
				LastSeenAt:        now,
				RawJSON:           mustJSON(item),
			})
		}
		nextOffset := offset + len(items)

		err = s.Store.InTx(ctx, func(tx *gorm.DB) error {
			if err := s.Store.UpsertTagsTx(ctx, tx, tags); err != nil {
				return err
			}
			state := &models.SyncState{
				Scope:         "tags",
				Cursor:        strPtr(strconv.Itoa(nextOffset)),
				LastAttemptAt: &now,
				LastSuccessAt: &now,
				LastError:     nil,
				StatsJSON:     statsJSON(map[string]int{"tags": len(tags)}),
			}
			return s.Store.SaveSyncStateTx(ctx, tx, state)
		})
		if err != nil {
			s.writeSyncError(ctx, "tags", err)
			return result, err
		}

		result.Pages++
		result.Tags += len(tags)
		result.NextOffset = nextOffset
		offset = nextOffset
		if len(items) < limit {
			result.Done = true
			break
		}
	}
	return result, nil
}

func (s *CatalogSyncService) syncMarkets(ctx context.Context, opts SyncOptions) (SyncResult, error) {
	if s.Gamma == nil {
		return SyncResult{}, fmt.Errorf("gamma client is nil")
	}
	limit := normalizeLimit(opts.Limit)
	maxPages := normalizeMaxPages(opts.MaxPages)
	offset := 0
	if opts.Resume {
		state, err := s.Store.GetSyncState(ctx, "markets")
		if err != nil {
			return SyncResult{}, err
		}
		if state != nil && state.Cursor != nil {
			if parsed, err := strconv.Atoi(*state.Cursor); err == nil {
				offset = parsed
			}
		}
	}

	now := time.Now().UTC()
	result := SyncResult{Scope: "markets"}
	for page := 0; page < maxPages; page++ {
		params := &polymarketgamma.GetMarketsParams{
			Limit:  limit,
			Offset: offset,
			Closed: opts.Closed,
		}
		items, err := s.Gamma.GetMarkets(ctx, params)
		if err != nil {
			s.writeSyncError(ctx, "markets", err)
			return result, err
		}
		if len(items) == 0 {
			result.Done = true
			break
		}
		markets := make([]models.Market, 0, len(items))
		tokens := make([]models.Token, 0)
		for _, item := range items {
			if item == nil {
				continue
			}
			eventID := ""
			if len(item.Events) > 0 {
				eventID = item.Events[0].ID
			}
			if eventID == "" {
				continue
			}
			market := models.Market{
				ID:                item.ID,
				EventID:           eventID,
				Slug:              strPtr(item.Slug),
				Question:          item.Question,
				ConditionID:       item.ConditionID,
				MarketAddress:     strPtr(item.MarketMakerAddress),
				TickSize:          decimal.NewFromFloat(item.OrderPriceMinTickSize),
				Volume:            decimalPtr(item.VolumeNum),
				Liquidity:         decimalPtr(item.LiquidityNum),
				Active:            item.Active,
				Closed:            item.Closed,
				NegRisk:           boolPtr(item.NegRiskOther),
				Status:            marketStatus(item),
				ExternalCreatedAt: normalizedTimePtr(item.CreatedAt),
				ExternalUpdatedAt: normalizedTimePtr(item.UpdatedAt),
				LastSeenAt:        now,
				RawJSON:           mustJSON(item),
			}
			markets = append(markets, market)
			tokens = append(tokens, buildTokensFromMarket(item, market.ExternalUpdatedAt, now)...)
		}
		markets, tokens, err = s.filterMarketsAndTokens(ctx, markets, tokens)
		if err != nil {
			s.writeSyncError(ctx, "markets", err)
			return result, err
		}
		nextOffset := offset + len(items)

		err = s.Store.InTx(ctx, func(tx *gorm.DB) error {
			if err := s.Store.UpsertMarketsTx(ctx, tx, markets); err != nil {
				return err
			}
			if err := s.Store.UpsertTokensTx(ctx, tx, tokens); err != nil {
				return err
			}
			state := &models.SyncState{
				Scope:         "markets",
				Cursor:        strPtr(strconv.Itoa(nextOffset)),
				LastAttemptAt: &now,
				LastSuccessAt: &now,
				LastError:     nil,
				StatsJSON:     statsJSON(map[string]int{"markets": len(markets), "tokens": len(tokens)}),
			}
			return s.Store.SaveSyncStateTx(ctx, tx, state)
		})
		if err != nil {
			s.writeSyncError(ctx, "markets", err)
			return result, err
		}

		result.Pages++
		result.Markets += len(markets)
		result.Tokens += len(tokens)
		result.NextOffset = nextOffset
		offset = nextOffset
		if len(items) < limit {
			result.Done = true
			break
		}
	}
	return result, nil
}

func (s *CatalogSyncService) writeSyncError(ctx context.Context, scope string, err error) {
	if s.Logger != nil {
		s.Logger.Warn("catalog sync failed", zap.String("scope", scope), zap.Error(err))
	}
	now := time.Now().UTC()
	_ = s.Store.InTx(ctx, func(tx *gorm.DB) error {
		state := &models.SyncState{
			Scope:         scope,
			LastAttemptAt: &now,
			LastError:     strPtr(err.Error()),
		}
		return s.Store.SaveSyncStateTx(ctx, tx, state)
	})
}

type bookResyncResult struct {
	Assets int
	Errors int
}

func (s *CatalogSyncService) resyncBooks(ctx context.Context, opts SyncOptions) (bookResyncResult, error) {
	if s == nil || s.Store == nil || s.Clob == nil {
		return bookResyncResult{}, fmt.Errorf("book resync unavailable")
	}
	maxAssets := opts.BookMaxAssets
	if maxAssets <= 0 {
		maxAssets = 200
	}
	batchSize := opts.BookBatchSize
	if batchSize <= 0 {
		batchSize = 50
	}
	marketIDs, err := s.Store.ListMarketIDsForStream(ctx, maxAssets)
	if err != nil {
		return bookResyncResult{}, err
	}
	tokens, err := s.Store.ListTokensByMarketIDs(ctx, marketIDs)
	if err != nil {
		return bookResyncResult{}, err
	}
	assetIDs := uniqueTokenIDs(tokens, maxAssets)
	result := bookResyncResult{}
	for i := 0; i < len(assetIDs); i += batchSize {
		end := i + batchSize
		if end > len(assetIDs) {
			end = len(assetIDs)
		}
		for _, tokenID := range assetIDs[i:end] {
			if tokenID == "" {
				continue
			}
			if err := s.resyncToken(ctx, tokenID); err != nil {
				result.Errors++
				if s.Logger != nil && !isBookNotFound(err) {
					s.Logger.Warn("book resync failed", zap.String("token_id", tokenID), zap.Error(err))
				}
				continue
			}
			result.Assets++
		}
		if opts.BookSleepPerBatch > 0 && end < len(assetIDs) {
			select {
			case <-ctx.Done():
				return result, ctx.Err()
			case <-time.After(opts.BookSleepPerBatch):
			}
		}
	}
	return result, nil
}

func (s *CatalogSyncService) resyncToken(ctx context.Context, tokenID string) error {
	raw, book, err := s.getBookWithRetry(ctx, tokenID, 2)
	if err != nil {
		return err
	}
	if book == nil {
		return fmt.Errorf("empty orderbook")
	}
	now := time.Now().UTC()
	bestBid := topOrderPrice(book.Bids)
	bestAsk := topOrderPrice(book.Asks)
	mid := computeMid(bestBid, bestAsk)
	spread, spreadBps := computeSpread(bestBid, bestAsk, mid)
	bidsJSON, _ := json.Marshal(book.Bids)
	asksJSON, _ := json.Marshal(book.Asks)
	if err := s.Store.UpsertOrderbookLatest(ctx, &models.OrderbookLatest{
		TokenID:        tokenID,
		SnapshotTS:     now,
		BidsJSON:       bidsJSON,
		AsksJSON:       asksJSON,
		BestBid:        bestBid,
		BestAsk:        bestAsk,
		Mid:            mid,
		Source:         strPtr("rest"),
		DataAgeSeconds: 0,
		UpdatedAt:      now,
	}); err != nil {
		return err
	}
	if err := s.Store.UpsertMarketDataHealth(ctx, &models.MarketDataHealth{
		TokenID:          tokenID,
		WSConnected:      true,
		LastRESTTS:       &now,
		DataAgeSeconds:   0,
		Stale:            false,
		NeedsResync:      false,
		LastResyncTS:     &now,
		LastBookChangeTS: &now,
		Spread:           spread,
		SpreadBps:        spreadBps,
		Reason:           strPtr("book_resync"),
		UpdatedAt:        now,
	}); err != nil {
		return err
	}
	return s.Store.InsertRawRESTSnapshot(ctx, &models.RawRESTSnapshot{
		TokenID:      strPtr(tokenID),
		SnapshotType: "orderbook",
		FetchedAt:    now,
		Payload:      raw,
	})
}

func (s *CatalogSyncService) getBookWithRetry(ctx context.Context, tokenID string, maxRetry int) ([]byte, *clob.OrderBook, error) {
	if maxRetry < 0 {
		maxRetry = 0
	}
	var lastErr error
	for attempt := 0; attempt <= maxRetry; attempt++ {
		raw, book, err := s.Clob.GetBookRaw(ctx, tokenID)
		if err == nil {
			return raw, book, nil
		}
		lastErr = err
		if isBookNotFound(err) || ctx.Err() != nil {
			return nil, nil, err
		}
		backoff := time.Duration(400+attempt*400) * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(backoff):
		}
	}
	return nil, nil, lastErr
}

func uniqueTokenIDs(tokens []models.Token, limit int) []string {
	if limit <= 0 {
		limit = 200
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
		if len(out) >= limit {
			break
		}
	}
	return out
}

func topOrderPrice(orders []clob.Order) *float64 {
	if len(orders) == 0 {
		return nil
	}
	val := orders[0].Price.InexactFloat64()
	if val <= 0 {
		return nil
	}
	return &val
}

func computeMid(bid, ask *float64) *float64 {
	if bid == nil || ask == nil {
		return nil
	}
	mid := (*bid + *ask) / 2
	if math.IsNaN(mid) || math.IsInf(mid, 0) {
		return nil
	}
	return &mid
}

func computeSpread(bid, ask, mid *float64) (*float64, *float64) {
	if bid == nil || ask == nil {
		return nil, nil
	}
	spread := *ask - *bid
	if math.IsNaN(spread) || math.IsInf(spread, 0) {
		return nil, nil
	}
	if mid == nil || *mid == 0 {
		return &spread, nil
	}
	spreadBps := (spread / *mid) * 10000
	return &spread, &spreadBps
}

func isBookNotFound(err error) bool {
	var apiErr *clob.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return true
	}
	return false
}
func mapEventsPayload(events []polymarketgamma.Event, now time.Time) ([]models.Series, []models.Tag, []models.EventTag, []models.Market, []models.Token, []models.Event) {
	seriesByID := map[string]models.Series{}
	tagsByID := map[string]models.Tag{}
	eventTags := make([]models.EventTag, 0)
	markets := make([]models.Market, 0)
	tokens := make([]models.Token, 0)
	eventsOut := make([]models.Event, 0, len(events))

	for _, evt := range events {
		var seriesID *string
		for _, series := range evt.Series {
			if series.ID == "" {
				continue
			}
			seriesByID[series.ID] = models.Series{
				ID:                series.ID,
				Title:             series.Title,
				Slug:              strPtr(series.Slug),
				Image:             strPtr(series.Image),
				ExternalUpdatedAt: normalizedTimePtr(series.UpdatedAt),
				LastSeenAt:        now,
				RawJSON:           mustJSON(series),
			}
			seriesID = &series.ID
			break
		}

		eventsOut = append(eventsOut, models.Event{
			ID:                evt.ID,
			Slug:              evt.Slug,
			Title:             evt.Title,
			Description:       strPtr(evt.Description),
			Active:            evt.Active,
			Closed:            evt.Closed,
			NegRisk:           boolPtr(evt.NegRisk),
			StartTime:         pickTime(evt.StartDate, evt.StartTime),
			EndTime:           pickTime(evt.EndDate),
			SeriesID:          seriesID,
			ExternalCreatedAt: normalizedTimePtr(evt.CreatedAt),
			ExternalUpdatedAt: normalizedTimePtr(evt.UpdatedAt),
			LastSeenAt:        now,
			RawJSON:           mustJSON(evt),
		})

		for _, tag := range evt.Tags {
			if tag.ID == "" {
				continue
			}
			tagsByID[tag.ID] = models.Tag{
				ID:                tag.ID,
				Label:             tag.Label,
				Slug:              tag.Slug,
				ExternalUpdatedAt: normalizedTimePtr(tag.UpdatedAt),
				LastSeenAt:        now,
				RawJSON:           mustJSON(tag),
			}
			eventTags = append(eventTags, models.EventTag{
				EventID: evt.ID,
				TagID:   tag.ID,
			})
		}

		for _, m := range evt.Markets {
			market := models.Market{
				ID:                m.ID,
				EventID:           evt.ID,
				Slug:              strPtr(m.Slug),
				Question:          m.Question,
				ConditionID:       m.ConditionID,
				MarketAddress:     strPtr(m.MarketMakerAddress),
				TickSize:          decimal.NewFromFloat(m.OrderPriceMinTickSize),
				Volume:            decimalPtr(m.VolumeNum),
				Liquidity:         decimalPtr(m.LiquidityNum),
				Active:            m.Active,
				Closed:            m.Closed,
				NegRisk:           boolPtr(m.NegRiskOther),
				Status:            marketStatus(&m),
				ExternalCreatedAt: normalizedTimePtr(m.CreatedAt),
				ExternalUpdatedAt: normalizedTimePtr(m.UpdatedAt),
				LastSeenAt:        now,
				RawJSON:           mustJSON(m),
			}
			markets = append(markets, market)
			tokens = append(tokens, buildTokensFromMarket(&m, market.ExternalUpdatedAt, now)...)
		}
	}

	series := make([]models.Series, 0, len(seriesByID))
	for _, s := range seriesByID {
		series = append(series, s)
	}
	tags := make([]models.Tag, 0, len(tagsByID))
	for _, t := range tagsByID {
		tags = append(tags, t)
	}

	return series, tags, eventTags, markets, tokens, eventsOut
}

func (s *CatalogSyncService) filterMarketsAndTokens(ctx context.Context, markets []models.Market, tokens []models.Token) ([]models.Market, []models.Token, error) {
	if len(markets) == 0 {
		return markets, tokens, nil
	}
	seenCondition := map[string]string{}
	seenSlug := map[string]string{}
	filtered := make([]models.Market, 0, len(markets))
	conditionIDs := make([]string, 0)
	slugs := make([]string, 0)
	for _, market := range markets {
		if market.ConditionID != "" {
			if _, exists := seenCondition[market.ConditionID]; exists {
				continue
			}
			seenCondition[market.ConditionID] = market.ID
			conditionIDs = append(conditionIDs, market.ConditionID)
		}
		if market.Slug != nil && *market.Slug != "" {
			if _, exists := seenSlug[*market.Slug]; exists {
				continue
			}
			seenSlug[*market.Slug] = market.ID
			slugs = append(slugs, *market.Slug)
		}
		filtered = append(filtered, market)
	}

	existingByCondition, err := s.fetchMarketsByConditionIDs(ctx, conditionIDs)
	if err != nil {
		return nil, nil, err
	}
	existingBySlug, err := s.fetchMarketsBySlugs(ctx, slugs)
	if err != nil {
		return nil, nil, err
	}

	finalMarkets := make([]models.Market, 0, len(filtered))
	allowedMarketIDs := map[string]struct{}{}
	for _, market := range filtered {
		if market.ConditionID != "" {
			if existingID, ok := existingByCondition[market.ConditionID]; ok && existingID != market.ID {
				continue
			}
		}
		if market.Slug != nil && *market.Slug != "" {
			if existingID, ok := existingBySlug[*market.Slug]; ok && existingID != market.ID {
				continue
			}
		}
		finalMarkets = append(finalMarkets, market)
		allowedMarketIDs[market.ID] = struct{}{}
	}

	if len(allowedMarketIDs) == 0 {
		return finalMarkets, nil, nil
	}
	filteredTokens := make([]models.Token, 0, len(tokens))
	for _, token := range tokens {
		if _, ok := allowedMarketIDs[token.MarketID]; ok {
			filteredTokens = append(filteredTokens, token)
		}
	}
	return finalMarkets, filteredTokens, nil
}

func (s *CatalogSyncService) fetchMarketsByConditionIDs(ctx context.Context, conditionIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(conditionIDs) == 0 {
		return out, nil
	}
	for _, chunk := range chunkStrings(conditionIDs, 1000) {
		existing, err := s.Store.FindMarketsByConditionIDs(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for _, m := range existing {
			if m.ConditionID != "" {
				out[m.ConditionID] = m.ID
			}
		}
	}
	return out, nil
}

func (s *CatalogSyncService) fetchMarketsBySlugs(ctx context.Context, slugs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(slugs) == 0 {
		return out, nil
	}
	for _, chunk := range chunkStrings(slugs, 1000) {
		existing, err := s.Store.FindMarketsBySlugs(ctx, chunk)
		if err != nil {
			return nil, err
		}
		for _, m := range existing {
			if m.Slug != nil && *m.Slug != "" {
				out[*m.Slug] = m.ID
			}
		}
	}
	return out, nil
}

func chunkStrings(items []string, size int) [][]string {
	if size <= 0 || len(items) == 0 {
		return nil
	}
	if len(items) <= size {
		return [][]string{items}
	}
	chunks := make([][]string, 0, (len(items)/size)+1)
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

func buildTokensFromMarket(m *polymarketgamma.Market, updatedAt *time.Time, now time.Time) []models.Token {
	if m == nil {
		return nil
	}
	tokenIDs := parseTokenIDs(m.ClobTokenIDs)
	outcomes := []string(m.Outcomes)
	if len(tokenIDs) == 0 || len(outcomes) == 0 {
		return nil
	}
	tokens := make([]models.Token, 0, len(outcomes))
	for i, outcome := range outcomes {
		if i >= len(tokenIDs) {
			break
		}
		raw := map[string]string{
			"token_id":  tokenIDs[i],
			"market_id": m.ID,
			"outcome":   outcome,
		}
		tokens = append(tokens, models.Token{
			ID:                tokenIDs[i],
			MarketID:          m.ID,
			Outcome:           outcome,
			Side:              normalizeSide(outcome),
			ExternalUpdatedAt: updatedAt,
			LastSeenAt:        now,
			RawJSON:           mustJSON(raw),
		})
	}
	return tokens
}

func parseTokenIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err == nil {
			return ids
		}
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.Trim(strings.TrimSpace(p), "\"")
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func normalizeSide(outcome string) *string {
	switch strings.ToLower(strings.TrimSpace(outcome)) {
	case "yes":
		return strPtr("yes")
	case "no":
		return strPtr("no")
	default:
		return nil
	}
}

func normalizeLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	return limit
}

func normalizeMaxPages(maxPages int) int {
	if maxPages <= 0 {
		return 50
	}
	return maxPages
}

func pickTime(values ...polymarketgamma.NormalizedTime) *time.Time {
	for _, val := range values {
		if !val.IsZero() {
			t := val.Time()
			return &t
		}
	}
	return nil
}

func normalizedTimePtr(val polymarketgamma.NormalizedTime) *time.Time {
	if val.IsZero() {
		return nil
	}
	t := val.Time()
	return &t
}

func decimalPtr(value float64) *decimal.Decimal {
	if value == 0 {
		return nil
	}
	val := decimal.NewFromFloat(value)
	return &val
}

func marketStatus(m *polymarketgamma.Market) *string {
	if m == nil {
		return nil
	}
	if m.Closed {
		return strPtr("closed")
	}
	if m.Active {
		return strPtr("active")
	}
	return nil
}

func statsJSON(stats map[string]int) datatypes.JSON {
	if len(stats) == 0 {
		return datatypes.JSON([]byte("null"))
	}
	payload, err := json.Marshal(stats)
	if err != nil {
		return datatypes.JSON([]byte("null"))
	}
	return datatypes.JSON(payload)
}

func mustJSON(v any) datatypes.JSON {
	payload, err := json.Marshal(v)
	if err != nil {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(payload)
}

func strPtr(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}

func boolPtr(v bool) *bool {
	return &v
}
