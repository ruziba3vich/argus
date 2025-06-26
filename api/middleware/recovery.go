package middleware

import (
	"fmt"
	"runtime/debug"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

func buildPanicMessage(c *gin.Context, err interface{}) string {
	var result strings.Builder

	result.WriteString(c.ClientIP())
	result.WriteString(" - ")
	result.WriteString(c.Request.Method)
	result.WriteString(" ")
	result.WriteString(c.Request.URL.RequestURI())
	result.WriteString(" PANIC DETECTED: ")
	result.WriteString(fmt.Sprintf("%v\n%s\n", err, debug.Stack()))

	return result.String()
}

func Recovery(l *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Log panic message with stack trace
				l.Error(buildPanicMessage(c, err))

				// Respond with 500 Internal Server Error
				c.AbortWithStatusJSON(500, gin.H{
					"message": "Internal Server Error",
				})
			}
		}()

		c.Next()
	}
}
