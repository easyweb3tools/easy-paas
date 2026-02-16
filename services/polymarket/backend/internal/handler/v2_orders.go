package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"polymarket/internal/repository"
	"polymarket/internal/service"
)

type V2OrderHandler struct {
	Repo     repository.Repository
	Executor *service.CLOBExecutor
}

func (h *V2OrderHandler) Register(r *gin.Engine) {
	o := r.Group("/api/v2/orders")
	o.GET("", h.list)
	o.GET("/:id", h.get)
	o.POST("/:id/cancel", h.cancel)

	e := r.Group("/api/v2/executions")
	e.POST("/:id/submit", h.submitPlan)
}

func (h *V2OrderHandler) list(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	var status *string
	if v := strings.TrimSpace(c.Query("status")); v != "" {
		status = &v
	}
	var tokenID *string
	if v := strings.TrimSpace(c.Query("token_id")); v != "" {
		tokenID = &v
	}
	var planID *uint64
	if v := strings.TrimSpace(c.Query("plan_id")); v != "" {
		if id := parseUint64(v); id > 0 {
			planID = &id
		}
	}
	params := repository.ListOrdersParams{
		Limit:   limit,
		Offset:  offset,
		Status:  status,
		PlanID:  planID,
		TokenID: tokenID,
		OrderBy: "created_at",
		Asc:     boolPtr(false),
	}
	items, err := h.Repo.ListOrders(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	total, err := h.Repo.CountOrders(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, paginationMeta(limit, offset, total))
}

func (h *V2OrderHandler) get(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	item, err := h.Repo.GetOrderByID(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		Error(c, http.StatusNotFound, "order not found", nil)
		return
	}
	Ok(c, item, nil)
}

func (h *V2OrderHandler) cancel(c *gin.Context) {
	if h.Executor == nil {
		Error(c, http.StatusServiceUnavailable, "executor unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	if err := h.Executor.CancelOrder(c.Request.Context(), id); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	item, _ := h.Repo.GetOrderByID(c.Request.Context(), id)
	Ok(c, item, nil)
}

func (h *V2OrderHandler) submitPlan(c *gin.Context) {
	if h.Executor == nil {
		Error(c, http.StatusServiceUnavailable, "executor unavailable", nil)
		return
	}
	id := uint64QueryParam(c, "id")
	if id == 0 {
		Error(c, http.StatusBadRequest, "invalid id", nil)
		return
	}
	out, err := h.Executor.SubmitPlan(c.Request.Context(), id)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if out == nil {
		Error(c, http.StatusNotFound, "plan not found", nil)
		return
	}
	Ok(c, out, nil)
}

func parseUint64(v string) uint64 {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0
	}
	var out uint64
	for i := 0; i < len(v); i++ {
		ch := v[i]
		if ch < '0' || ch > '9' {
			return 0
		}
		out = out*10 + uint64(ch-'0')
	}
	return out
}
