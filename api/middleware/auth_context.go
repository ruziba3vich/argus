package middleware

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
	token_pkg "github.com/ruziba3vich/argus/internal/pkg/token"
)

func AuthContext(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {

		var (
			token    string
			authData = make(map[string]string)
		)

		// Extract token from query or headers
		if token = c.Query("token"); len(token) == 0 {
			if authHeader := c.GetHeader("Authorization"); len(authHeader) > 7 {
				fmt.Println("token auth : ", authHeader)
				if strings.Contains(authHeader, "Bearer ") {
					token = authHeader[7:] // Remove "Bearer " prefix
				} else {
					token = authHeader
				}
			}
		}

		// If token is missing, log it
		if token == "" {
			fmt.Println("Token not found")
			c.Next()
			return
		}

		// Parse the JWT token
		claims, err := token_pkg.ParseJwtToken(token, jwtSecret)
		if err != nil {
			fmt.Println("Failed to parse token:", err)
			c.Next()
			return
		}

		// Store claims in authData map
		if len(claims) != 0 {
			for key, value := range claims {
				if valStr, ok := value.(string); ok {
					authData[key] = valStr
				}
			}
			// Store auth data in Gin's context
			c.Set(RequestIDHeader, authData)
		}

		c.Next()
	}
}
