package handler

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"polymarket/internal/repository"
)

type V2JournalHandler struct {
	Repo repository.Repository
}

func (h *V2JournalHandler) Register(r *gin.Engine) {
	g := r.Group("/api/v2/journal")
	g.GET("", h.list)
	g.GET("/:execution_plan_id", h.get)
	g.PUT("/:execution_plan_id/notes", h.putNotes)
}

func (h *V2JournalHandler) list(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	limit := intQuery(c, "limit", 50)
	offset := intQuery(c, "offset", 0)
	var strategyName *string
	if v := strings.TrimSpace(c.Query("strategy_name")); v != "" {
		strategyName = &v
	}
	var outcome *string
	if v := strings.TrimSpace(c.Query("outcome")); v != "" {
		outcome = &v
	}
	var since *time.Time
	if v := strings.TrimSpace(c.Query("since")); v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			t := ts.UTC()
			since = &t
		}
	}
	var until *time.Time
	if v := strings.TrimSpace(c.Query("until")); v != "" {
		if ts, err := time.Parse(time.RFC3339, v); err == nil {
			t := ts.UTC()
			until = &t
		}
	}
	var tags []string
	if raw := strings.TrimSpace(c.Query("tags")); raw != "" {
		for _, v := range strings.Split(raw, ",") {
			tag := strings.TrimSpace(v)
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	params := repository.ListTradeJournalParams{
		Limit:        limit,
		Offset:       offset,
		StrategyName: strategyName,
		Outcome:      outcome,
		Since:        since,
		Until:        until,
		Tags:         tags,
		OrderBy:      "created_at",
		Asc:          boolPtr(false),
	}
	items, err := h.Repo.ListTradeJournals(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	total, err := h.Repo.CountTradeJournals(c.Request.Context(), params)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	Ok(c, items, paginationMeta(limit, offset, total))
}

func (h *V2JournalHandler) get(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	planID := uint64QueryParam(c, "execution_plan_id")
	if planID == 0 {
		Error(c, http.StatusBadRequest, "invalid execution_plan_id", nil)
		return
	}
	item, err := h.Repo.GetTradeJournalByPlanID(c.Request.Context(), planID)
	if err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	if item == nil {
		Error(c, http.StatusNotFound, "journal not found", nil)
		return
	}
	Ok(c, item, nil)
}

type putJournalNotesRequest struct {
	Notes      string   `json:"notes"`
	Tags       []string `json:"tags"`
	ReviewedAt *string  `json:"reviewed_at"`
}

func (h *V2JournalHandler) putNotes(c *gin.Context) {
	if h.Repo == nil {
		Error(c, http.StatusInternalServerError, "repo unavailable", nil)
		return
	}
	planID := uint64QueryParam(c, "execution_plan_id")
	if planID == 0 {
		Error(c, http.StatusBadRequest, "invalid execution_plan_id", nil)
		return
	}
	var req putJournalNotesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Error(c, http.StatusBadRequest, "invalid body", nil)
		return
	}
	var reviewedAt *time.Time
	if req.ReviewedAt != nil && strings.TrimSpace(*req.ReviewedAt) != "" {
		ts, err := time.Parse(time.RFC3339, strings.TrimSpace(*req.ReviewedAt))
		if err != nil {
			Error(c, http.StatusBadRequest, "invalid reviewed_at", nil)
			return
		}
		t := ts.UTC()
		reviewedAt = &t
	}
	tagsRaw, _ := json.Marshal(req.Tags)
	if err := h.Repo.UpdateTradeJournalNotes(c.Request.Context(), planID, req.Notes, tagsRaw, reviewedAt); err != nil {
		Error(c, http.StatusBadGateway, err.Error(), nil)
		return
	}
	item, _ := h.Repo.GetTradeJournalByPlanID(c.Request.Context(), planID)
	Ok(c, item, nil)
}
