package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
	allowedOriginsStr := os.Getenv("ALLOWED_ORIGINS")
	allowedOrigins := strings.Split(allowedOriginsStr, ",")

	for i, origin := range allowedOrigins {
		allowedOrigins[i] = strings.TrimSpace(origin)
	}

	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		// Validate origin
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				c.Writer.Header().Set("Access-Control-Allow-Origin", origin)
				break
			}
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
