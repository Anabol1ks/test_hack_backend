package auth

import (
	"net/http"
	"strings"
	"test_hack/internal/handlers"
	"test_hack/internal/response"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// AuthMiddleware проверяет валидность access токена
func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{
				Code:    "NO_AUTH_HEADER",
				Message: "Требуется авторизация",
			})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return handlers.AccessSecret, nil
		})

		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{
				Code:    "INVALID_TOKEN",
				Message: "Неверный или просроченный токен",
			})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{
				Code:    "INVALID_TOKEN_CLAIMS",
				Message: "Невозможно прочитать claims токена",
			})
			c.Abort()
			return
		}

		userID, ok := claims["user_id"].(float64)
		if !ok {
			c.JSON(http.StatusUnauthorized, response.ErrorResponse{
				Code:    "INVALID_USER_ID",
				Message: "Невозможно извлечь user_id",
			})
			c.Abort()
			return
		}

		c.Set("userID", uint(userID))
		c.Next()
	}
}
