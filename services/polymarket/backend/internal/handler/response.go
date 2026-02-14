package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type apiResponse struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    any            `json:"data,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

func Ok(c *gin.Context, data any, meta map[string]any) {
	c.JSON(http.StatusOK, apiResponse{
		Code:    0,
		Message: "ok",
		Data:    data,
		Meta:    meta,
	})
}

func Error(c *gin.Context, status int, message string, meta map[string]any) {
	c.JSON(status, apiResponse{
		Code:    status,
		Message: message,
		Meta:    meta,
	})
}
