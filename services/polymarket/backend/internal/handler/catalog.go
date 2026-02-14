package handler

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
	"go.uber.org/zap"

	"polymarket/internal/models"
	"polymarket/internal/paas"
	"polymarket/internal/repository"
	"polymarket/internal/service"
)

type CatalogHandler struct {
	Service      *service.CatalogSyncService
	QueryService *service.CatalogQueryService
	Logger       *zap.Logger
}

func (h *CatalogHandler) Register(r *gin.Engine) {
	group := r.Group("/api/catalog")
	group.POST("/sync", h.syncCatalog)
	group.GET("/sync-state", h.listSyncState)
	group.GET("/events", h.listEvents)
	group.GET("/markets", h.listMarkets)
	group.GET("/tokens", h.listTokens)
	group.GET("/markets/realtime", h.getMarketRealtime)
	group.GET("/events/realtime", h.getEventRealtime)
}

// @Summary Run catalog sync
// @Tags catalog
// @Param scope query string false "sync scope (events|markets|series|tags|all)"
// @Param limit query int false "page size"
// @Param max_pages query int false "max pages"
// @Param resume query bool false "resume from cursor"
// @Param tag_id query int false "tag id"
// @Param closed query string false "open|closed"
// @Param book_max_assets query int false "max assets for /book resync"
// @Param book_batch_size query int false "batch size for /book resync"
// @Param book_sleep_per_batch query string false "sleep per batch (e.g. 2s)"
// @Success 200 {object} apiResponse
// @Router /api/catalog/sync [post]
func (h *CatalogHandler) syncCatalog(c *gin.Context) {
	if h.Service == nil {
		Error(c, http.StatusInternalServerError, "service unavailable", nil)
		return
	}
	scope := strings.TrimSpace(c.Query("scope"))
	limit := intQuery(c, "limit", 0)
	maxPages := intQuery(c, "max_pages", 0)
	resume := boolQueryDefault(c, "resume", true)
	tagID := intQueryPtr(c, "tag_id")
	closed := parseClosed(c.Query("closed"))
	bookMaxAssets := intQuery(c, "book_max_assets", 0)
	bookBatchSize := intQuery(c, "book_batch_size", 0)
	bookSleepPerBatch := durationQuery(c, "book_sleep_per_batch")

	result, err := h.Service.Sync(c.Request.Context(), service.SyncOptions{
		Scope:             scope,
		Limit:             limit,
		MaxPages:          maxPages,
		Resume:            resume,
		TagID:             tagID,
		Closed:            closed,
		BookMaxAssets:     bookMaxAssets,
		BookBatchSize:     bookBatchSize,
		BookSleepPerBatch: bookSleepPerBatch,
	})
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("catalog sync failed", zap.Error(err))
		}
		paas.LogBestEffort(c, "polymarket_catalog_sync_failed", "warn", map[string]any{
			"error": err.Error(),
		})
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	paas.LogBestEffort(c, "polymarket_catalog_sync_ok", "info", map[string]any{
		"scope":   result.Scope,
		"pages":   result.Pages,
		"events":  result.Events,
		"markets": result.Markets,
		"tokens":  result.Tokens,
		"series":  result.Series,
		"tags":    result.Tags,
	})
	Ok(c, result, nil)
}

// @Summary List sync states
// @Tags catalog
// @Success 200 {object} apiResponse
// @Router /api/catalog/sync-state [get]
func (h *CatalogHandler) listSyncState(c *gin.Context) {
	if h.Service == nil || h.Service.Store == nil {
		Error(c, http.StatusInternalServerError, "service unavailable", nil)
		return
	}
	states, err := h.Service.Store.ListSyncStates(c.Request.Context())
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("list sync state failed", zap.Error(err))
		}
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, states, nil)
}

// @Summary List events
// @Tags catalog
// @Param limit query int false "limit"
// @Param offset query int false "offset"
// @Param active query bool false "active"
// @Param closed query bool false "closed"
// @Param slug query string false "slug"
// @Param title query string false "title"
// @Param order_by query string false "order by field"
// @Param ascending query bool false "ascending"
// @Success 200 {object} apiResponse
// @Router /api/catalog/events [get]
func (h *CatalogHandler) listEvents(c *gin.Context) {
	if h.QueryService == nil || h.QueryService.Repo == nil {
		Error(c, http.StatusInternalServerError, "service unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	active := boolQueryPtr(c, "active")
	closed := boolQueryPtr(c, "closed")
	slug := strQueryPtr(c, "slug")
	title := strQueryPtr(c, "title")
	orderBy := parseOrder(c.Query("order_by"), map[string]string{
		"external_updated_at": "external_updated_at",
		"last_seen_at":        "last_seen_at",
		"title":               "title",
		"end_time":            "end_time",
	})
	asc := boolQueryPtr(c, "ascending")

	result, err := h.QueryService.ListEvents(c.Request.Context(), repository.ListEventsParams{
		Limit:   limit,
		Offset:  offset,
		Active:  active,
		Closed:  closed,
		Slug:    slug,
		Title:   title,
		OrderBy: orderBy,
		Asc:     asc,
	})
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("list events failed", zap.Error(err))
		}
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	meta := paginationMeta(limit, offset, result.Total)
	Ok(c, result.Items, meta)
}

// @Summary List markets
// @Tags catalog
// @Param limit query int false "limit"
// @Param offset query int false "offset"
// @Param active query bool false "active"
// @Param closed query bool false "closed"
// @Param event_id query string false "event id"
// @Param slug query string false "slug"
// @Param question query string false "question contains"
// @Param order_by query string false "order by field"
// @Param ascending query bool false "ascending"
// @Success 200 {object} apiResponse
// @Router /api/catalog/markets [get]
func (h *CatalogHandler) listMarkets(c *gin.Context) {
	if h.QueryService == nil || h.QueryService.Repo == nil {
		Error(c, http.StatusInternalServerError, "service unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	active := boolQueryPtr(c, "active")
	closed := boolQueryPtr(c, "closed")
	eventID := strQueryPtr(c, "event_id")
	slug := strQueryPtr(c, "slug")
	question := strQueryPtr(c, "question")
	orderBy := parseOrder(c.Query("order_by"), map[string]string{
		"external_updated_at": "external_updated_at",
		"last_seen_at":        "last_seen_at",
		"question":            "question",
		"volume":              "volume",
		"liquidity":           "liquidity",
	})
	asc := boolQueryPtr(c, "ascending")

	result, err := h.QueryService.ListMarkets(c.Request.Context(), repository.ListMarketsParams{
		Limit:    limit,
		Offset:   offset,
		Active:   active,
		Closed:   closed,
		EventID:  eventID,
		Slug:     slug,
		Question: question,
		OrderBy:  orderBy,
		Asc:      asc,
	})
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("list markets failed", zap.Error(err))
		}
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	meta := paginationMeta(limit, offset, result.Total)
	Ok(c, result.Items, meta)
}

// @Summary List tokens
// @Tags catalog
// @Param limit query int false "limit"
// @Param offset query int false "offset"
// @Param market_id query string false "market id"
// @Param outcome query string false "outcome"
// @Param side query string false "side"
// @Param order_by query string false "order by field"
// @Param ascending query bool false "ascending"
// @Success 200 {object} apiResponse
// @Router /api/catalog/tokens [get]
func (h *CatalogHandler) listTokens(c *gin.Context) {
	if h.QueryService == nil || h.QueryService.Repo == nil {
		Error(c, http.StatusInternalServerError, "service unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 100)
	offset := intQuery(c, "offset", 0)
	marketID := strQueryPtr(c, "market_id")
	outcome := strQueryPtr(c, "outcome")
	side := strQueryPtr(c, "side")
	orderBy := parseOrder(c.Query("order_by"), map[string]string{
		"external_updated_at": "external_updated_at",
		"last_seen_at":        "last_seen_at",
		"outcome":             "outcome",
	})
	asc := boolQueryPtr(c, "ascending")

	result, err := h.QueryService.ListTokens(c.Request.Context(), repository.ListTokensParams{
		Limit:    limit,
		Offset:   offset,
		MarketID: marketID,
		Outcome:  outcome,
		Side:     side,
		OrderBy:  orderBy,
		Asc:      asc,
	})
	if err != nil {
		if h.Logger != nil {
			h.Logger.Warn("list tokens failed", zap.Error(err))
		}
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	meta := paginationMeta(limit, offset, result.Total)
	Ok(c, result.Items, meta)
}

type realtimeToken struct {
	TokenID          string     `json:"token_id"`
	Outcome          string     `json:"outcome"`
	Side             *string    `json:"side"`
	MarketID         string     `json:"market_id"`
	MarketQuestion   string     `json:"market_question"`
	BestBid          *float64   `json:"best_bid"`
	BestAsk          *float64   `json:"best_ask"`
	Mid              *float64   `json:"mid"`
	SpreadBps        *float64   `json:"spread_bps"`
	PriceJumpBps     *float64   `json:"price_jump_bps"`
	LastTradePrice   *float64   `json:"last_trade_price"`
	LastTradeTS      *time.Time `json:"last_trade_ts"`
	LastBookChangeTS *time.Time `json:"last_book_change_ts"`
}

type realtimeMarketResponse struct {
	MarketID string          `json:"market_id"`
	Slug     *string         `json:"slug"`
	Question string          `json:"question"`
	Tokens   []realtimeToken `json:"tokens"`
}

// @Summary Get realtime data for a market by slug
// @Tags catalog
// @Param slug query string true "market slug"
// @Success 200 {object} apiResponse
// @Router /api/catalog/markets/realtime [get]
func (h *CatalogHandler) getMarketRealtime(c *gin.Context) {
	if h.QueryService == nil || h.QueryService.Repo == nil {
		Error(c, http.StatusInternalServerError, "service unavailable", nil)
		return
	}
	slug := strings.TrimSpace(c.Query("slug"))
	if slug == "" {
		Error(c, http.StatusBadRequest, "slug required", nil)
		return
	}
	market, err := h.QueryService.Repo.GetMarketBySlug(c.Request.Context(), slug)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if market == nil {
		Error(c, http.StatusNotFound, "market not found", nil)
		return
	}
	tokens, err := h.QueryService.Repo.ListTokensByMarketIDs(c.Request.Context(), []string{market.ID})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	tokenIDs := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token.ID != "" {
			tokenIDs = append(tokenIDs, token.ID)
		}
	}
	healthRows, _ := h.QueryService.Repo.ListMarketDataHealthByTokenIDs(c.Request.Context(), tokenIDs)
	bookRows, _ := h.QueryService.Repo.ListOrderbookLatestByTokenIDs(c.Request.Context(), tokenIDs)
	tradeRows, _ := h.QueryService.Repo.ListLastTradePricesByTokenIDs(c.Request.Context(), tokenIDs)

	healthByID := map[string]models.MarketDataHealth{}
	for _, row := range healthRows {
		healthByID[row.TokenID] = row
	}
	bookByID := map[string]models.OrderbookLatest{}
	for _, row := range bookRows {
		bookByID[row.TokenID] = row
	}
	tradeByID := map[string]models.LastTradePrice{}
	for _, row := range tradeRows {
		tradeByID[row.TokenID] = row
	}

	resp := realtimeMarketResponse{
		MarketID: market.ID,
		Slug:     market.Slug,
		Question: market.Question,
		Tokens:   make([]realtimeToken, 0, len(tokens)),
	}
	for _, token := range tokens {
		health := healthByID[token.ID]
		book := bookByID[token.ID]
		trade := tradeByID[token.ID]
		var lastTradePrice *float64
		var lastTradeTS *time.Time
		if trade.Price != 0 {
			val := trade.Price
			lastTradePrice = &val
			lastTradeTS = trade.TradeTS
		}
		resp.Tokens = append(resp.Tokens, realtimeToken{
			TokenID:          token.ID,
			Outcome:          token.Outcome,
			Side:             token.Side,
			MarketID:         token.MarketID,
			MarketQuestion:   market.Question,
			BestBid:          book.BestBid,
			BestAsk:          book.BestAsk,
			Mid:              book.Mid,
			SpreadBps:        health.SpreadBps,
			PriceJumpBps:     health.PriceJumpBps,
			LastTradePrice:   lastTradePrice,
			LastTradeTS:      lastTradeTS,
			LastBookChangeTS: health.LastBookChangeTS,
		})
	}
	Ok(c, resp, nil)
}

type realtimeEventResponse struct {
	EventID string          `json:"event_id"`
	Slug    string          `json:"slug"`
	Title   string          `json:"title"`
	Tokens  []realtimeToken `json:"tokens"`
}

// @Summary Get realtime data for an event by slug
// @Tags catalog
// @Param slug query string true "event slug"
// @Success 200 {object} apiResponse
// @Router /api/catalog/events/realtime [get]
func (h *CatalogHandler) getEventRealtime(c *gin.Context) {
	if h.QueryService == nil || h.QueryService.Repo == nil {
		Error(c, http.StatusInternalServerError, "service unavailable", nil)
		return
	}
	slug := strings.TrimSpace(c.Query("slug"))
	if slug == "" {
		Error(c, http.StatusBadRequest, "slug required", nil)
		return
	}
	event, err := h.QueryService.Repo.GetEventBySlug(c.Request.Context(), slug)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if event == nil {
		Error(c, http.StatusNotFound, "event not found", nil)
		return
	}
	markets, err := h.QueryService.Repo.ListMarketsByEventID(c.Request.Context(), event.ID)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	marketByID := map[string]models.Market{}
	for _, market := range markets {
		marketByID[market.ID] = market
	}
	marketIDs := make([]string, 0, len(markets))
	for _, market := range markets {
		marketIDs = append(marketIDs, market.ID)
	}
	tokens, err := h.QueryService.Repo.ListTokensByMarketIDs(c.Request.Context(), marketIDs)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	tokenIDs := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token.ID != "" {
			tokenIDs = append(tokenIDs, token.ID)
		}
	}
	healthRows, _ := h.QueryService.Repo.ListMarketDataHealthByTokenIDs(c.Request.Context(), tokenIDs)
	bookRows, _ := h.QueryService.Repo.ListOrderbookLatestByTokenIDs(c.Request.Context(), tokenIDs)
	tradeRows, _ := h.QueryService.Repo.ListLastTradePricesByTokenIDs(c.Request.Context(), tokenIDs)

	healthByID := map[string]models.MarketDataHealth{}
	for _, row := range healthRows {
		healthByID[row.TokenID] = row
	}
	bookByID := map[string]models.OrderbookLatest{}
	for _, row := range bookRows {
		bookByID[row.TokenID] = row
	}
	tradeByID := map[string]models.LastTradePrice{}
	for _, row := range tradeRows {
		tradeByID[row.TokenID] = row
	}

	resp := realtimeEventResponse{
		EventID: event.ID,
		Slug:    event.Slug,
		Title:   event.Title,
		Tokens:  make([]realtimeToken, 0, len(tokens)),
	}
	for _, token := range tokens {
		health := healthByID[token.ID]
		book := bookByID[token.ID]
		trade := tradeByID[token.ID]
		var lastTradePrice *float64
		var lastTradeTS *time.Time
		if trade.Price != 0 {
			val := trade.Price
			lastTradePrice = &val
			lastTradeTS = trade.TradeTS
		}
		market := marketByID[token.MarketID]
		resp.Tokens = append(resp.Tokens, realtimeToken{
			TokenID:          token.ID,
			Outcome:          token.Outcome,
			Side:             token.Side,
			MarketID:         token.MarketID,
			MarketQuestion:   market.Question,
			BestBid:          book.BestBid,
			BestAsk:          book.BestAsk,
			Mid:              book.Mid,
			SpreadBps:        health.SpreadBps,
			PriceJumpBps:     health.PriceJumpBps,
			LastTradePrice:   lastTradePrice,
			LastTradeTS:      lastTradeTS,
			LastBookChangeTS: health.LastBookChangeTS,
		})
	}
	Ok(c, resp, nil)
}

func intQuery(c *gin.Context, key string, def int) int {
	if val := c.Query(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return def
}

func intQueryPtr(c *gin.Context, key string) *int {
	if val := c.Query(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return &i
		}
	}
	return nil
}

func boolQueryDefault(c *gin.Context, key string, def bool) bool {
	if val := c.Query(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return def
}

func parseClosed(value string) *bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "open":
		v := false
		return &v
	case "closed":
		v := true
		return &v
	default:
		return nil
	}
}

func boolQueryPtr(c *gin.Context, key string) *bool {
	if val := c.Query(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return &b
		}
	}
	return nil
}

func strQueryPtr(c *gin.Context, key string) *string {
	if val := strings.TrimSpace(c.Query(key)); val != "" {
		return &val
	}
	return nil
}

func durationQuery(c *gin.Context, key string) time.Duration {
	if val := strings.TrimSpace(c.Query(key)); val != "" {
		if d, err := time.ParseDuration(val); err == nil {
			return d
		}
	}
	return 0
}

func parseOrder(value string, allow map[string]string) string {
	key := strings.TrimSpace(strings.ToLower(value))
	if key == "" {
		return ""
	}
	if mapped, ok := allow[key]; ok {
		return mapped
	}
	return ""
}

func paginationMeta(limit, offset int, total int64) map[string]any {
	if limit <= 0 {
		limit = 0
	}
	if offset < 0 {
		offset = 0
	}
	hasNext := int64(offset+limit) < total
	return map[string]any{
		"limit":    limit,
		"offset":   offset,
		"total":    total,
		"has_next": hasNext,
	}
}

func decimalQueryPtr(c *gin.Context, key string) *decimal.Decimal {
	if val := strings.TrimSpace(c.Query(key)); val != "" {
		if d, err := decimal.NewFromString(val); err == nil {
			return &d
		}
	}
	return nil
}

func cleanStrings(items []string) []string {
	out := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		val := strings.TrimSpace(item)
		if val == "" {
			continue
		}
		if _, ok := seen[val]; ok {
			continue
		}
		seen[val] = struct{}{}
		out = append(out, val)
	}
	return out
}
