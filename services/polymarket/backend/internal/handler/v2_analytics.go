package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polymarket/internal/repository"
)

type V2AnalyticsHandler struct {
	Repo repository.Repository
}

func (h *V2AnalyticsHandler) Register(r *gin.Engine) {
	group := r.Group("/api/v2/analytics")
	group.GET("/overview", h.overview)
	group.GET("/by-strategy", h.byStrategy)
	group.GET("/failures", h.failures)
	group.GET("/daily", h.daily)
	group.GET("/strategy/:name/daily", h.strategyDaily)
	group.GET("/strategy/:name/attribution", h.attribution)
	group.GET("/drawdown", h.drawdown)
	group.GET("/correlation", h.correlation)
	group.GET("/ratios", h.ratios)
}

func (h *V2AnalyticsHandler) overview(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	row, err := h.Repo.AnalyticsOverview(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, row, nil)
}

func (h *V2AnalyticsHandler) byStrategy(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	rows, err := h.Repo.AnalyticsByStrategy(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, rows, nil)
}

func (h *V2AnalyticsHandler) failures(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	rows, err := h.Repo.AnalyticsFailures(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, rows, nil)
}

func (h *V2AnalyticsHandler) daily(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 365)
	offset := intQuery(c, "offset", 0)
	since, until := timeRangeFromQuery(c)
	var strategyName *string
	if v := strings.TrimSpace(c.Query("strategy_name")); v != "" {
		strategyName = &v
	}
	rows, err := h.Repo.ListStrategyDailyStats(c.Request.Context(), repository.ListDailyStatsParams{
		Limit:        limit,
		Offset:       offset,
		StrategyName: strategyName,
		Since:        since,
		Until:        until,
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, rows, paginationMeta(limit, offset, int64(len(rows))))
}

func (h *V2AnalyticsHandler) strategyDaily(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		Error(c, http.StatusBadRequest, "invalid strategy name", nil)
		return
	}
	limit := intQuery(c, "limit", 365)
	offset := intQuery(c, "offset", 0)
	since, until := timeRangeFromQuery(c)
	rows, err := h.Repo.ListStrategyDailyStats(c.Request.Context(), repository.ListDailyStatsParams{
		Limit:        limit,
		Offset:       offset,
		StrategyName: &name,
		Since:        since,
		Until:        until,
	})
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, rows, paginationMeta(limit, offset, int64(len(rows))))
}

func (h *V2AnalyticsHandler) attribution(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	name := strings.TrimSpace(c.Param("name"))
	if name == "" {
		Error(c, http.StatusBadRequest, "invalid strategy name", nil)
		return
	}
	since, until := timeRangeFromQuery(c)
	row, err := h.Repo.AttributionByStrategy(c.Request.Context(), name, since, until)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, row, nil)
}

func (h *V2AnalyticsHandler) drawdown(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	row, err := h.Repo.PortfolioDrawdown(c.Request.Context())
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, row, nil)
}

func (h *V2AnalyticsHandler) correlation(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	since, until := timeRangeFromQuery(c)
	rows, err := h.Repo.StrategyCorrelation(c.Request.Context(), since, until)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, rows, nil)
}

func (h *V2AnalyticsHandler) ratios(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	since, until := timeRangeFromQuery(c)
	row, err := h.Repo.PerformanceRatios(c.Request.Context(), since, until)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, row, nil)
}

func timeRangeFromQuery(c *gin.Context) (*time.Time, *time.Time) {
	var since *time.Time
	var until *time.Time
	if raw := strings.TrimSpace(c.Query("since")); raw != "" {
		if ts, err := time.Parse(time.RFC3339, raw); err == nil {
			t := ts.UTC()
			since = &t
		}
	}
	if raw := strings.TrimSpace(c.Query("until")); raw != "" {
		if ts, err := time.Parse(time.RFC3339, raw); err == nil {
			t := ts.UTC()
			until = &t
		}
	}
	return since, until
}
