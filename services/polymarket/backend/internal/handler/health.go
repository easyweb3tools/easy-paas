package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HealthHandler struct {
	DB *gorm.DB
}

func (h *HealthHandler) Register(r *gin.Engine) {
	r.GET("/healthz", h.health)
	r.GET("/readyz", h.ready)
}

// @Summary Health check
// @Tags health
// @Success 200 {object} map[string]string
// @Router /healthz [get]
func (h *HealthHandler) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// @Summary Readiness check
// @Tags health
// @Success 200 {object} map[string]string
// @Router /readyz [get]
func (h *HealthHandler) ready(c *gin.Context) {
	if h.DB == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_missing"})
		return
	}
	sqlDB, err := h.DB.DB()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_error"})
		return
	}
	if err := sqlDB.Ping(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "db_unreachable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}
