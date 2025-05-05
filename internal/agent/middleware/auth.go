package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/jwtauth"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// Использование кастомного типа предотвращает коллизии ключей.
type contextKey string

const UserIDKey contextKey = "userID"

type ErrorResponse struct {
	Error string `json:"error"`
}

// JWTAuth создает Echo middleware для проверки JWT токена.
// Оно зависит от JWT менеджера и логгера.
func JWTAuth(jwtManager *jwtauth.Manager, log *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get(echo.HeaderAuthorization)
			if authHeader == "" {
				log.Warn("Отсутствует заголовок Authorization")
				return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Отсутствует токен авторизации"})
			}

			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				log.Warn("Неверный формат заголовка Authorization", zap.String("header", authHeader))
				return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Неверный формат токена авторизации"})
			}

			tokenString := parts[1]
			if tokenString == "" {
				log.Warn("Пустой токен в заголовке Authorization")
				return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Пустой токен авторизации"})
			}

			userID, err := jwtManager.Verify(tokenString)
			if err != nil {
				log.Warn("Ошибка проверки JWT токена", zap.Error(err))
				return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: "Невалидный или истекший токен авторизации"})
			}

			reqCtx := context.WithValue(c.Request().Context(), UserIDKey, userID)
			c.SetRequest(c.Request().WithContext(reqCtx))

			log.Debug("JWT токен успешно проверен", zap.String("userID", userID))
			return next(c)
		}
	}
}

// GetUserIDFromContext извлекает UserID из контекста Echo.
// Возвращает UserID и true, если ID найден и имеет тип string, иначе пустую строку и false.
// Эту хелпер-функцию будут использовать хендлеры защищенных маршрутов.
func GetUserIDFromContext(c echo.Context) (string, bool) {
	userID, ok := c.Request().Context().Value(UserIDKey).(string)
	return userID, ok
}