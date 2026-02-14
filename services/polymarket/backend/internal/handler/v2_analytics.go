package handler

import (
	"net/http"

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
