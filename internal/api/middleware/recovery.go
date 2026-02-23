package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/GooferByte/kalpi/internal/models"
	"go.uber.org/zap"
)

// Recovery returns a Gin middleware that recovers from panics and returns a 500 response.
func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				logger.Error("panic recovered",
					zap.Any("error", err),
					zap.String("path", c.Request.URL.Path),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, models.APIResponse{
					Success: false,
					Error:   "internal server error",
				})
			}
		}()
		c.Next()
	}
}
