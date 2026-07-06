package dto

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Data    any    `json:"data"`
}

type APIErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func Success(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, APIResponse{
		Status:  "ok",
		Message: message,
		Data:    data,
	})
}

func Error(c *gin.Context, statusCode int, message string) {
	c.JSON(statusCode, APIErrorResponse{
		Status:  "error",
		Message: message,
	})
}
