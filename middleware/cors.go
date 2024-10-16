package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Allow specific origin
		c.Writer.Header().Set("Access-Control-Allow-Origin", "http://localhost:9000")
		// Allow methods
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
		// Allow headers
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		// Expose headers
		c.Writer.Header().Set("Access-Control-Expose-Headers", "Content-Length")
		// Allow credentials
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		// Handle preflight OPTIONS request
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}
