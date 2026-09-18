package utils

import (
	"github.com/gin-gonic/gin"
	"net/http"
)

func RespondList[T any](c *gin.Context, items []T) {
	if items == nil {
		items = []T{}
	}
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": len(items),
	})
}

func RespondOne[T any](c *gin.Context, item T) {
	c.JSON(http.StatusOK, gin.H{"data": item})
}
