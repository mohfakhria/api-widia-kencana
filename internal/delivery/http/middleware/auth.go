package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/mohfakhria/api-widia-kencana/internal/usecase/port/output"

	"github.com/gin-gonic/gin"
)

func AuthRequired(tokenSigner output.TokenSigner) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Missing or invalid Authorization header",
			})
			return
		}

		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := tokenSigner.ParseToken(c.Request.Context(), tokenStr)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid or expired token",
			})
			return
		}

		userID, err := strconv.ParseInt(claims.Subject, 10, 64)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"status":  "error",
				"message": "Invalid token subject",
			})
			return
		}

		c.Set("userID", claims.Subject)
		c.Set("userIDInt", userID)
		c.Set("name", claims.Name)
		c.Set("role", claims.Role)
		c.Next()
	}
}
