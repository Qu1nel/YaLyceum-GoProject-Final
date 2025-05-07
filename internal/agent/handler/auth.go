package handler

import (
	"errors"
	"net/http"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/repository"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

// RegisterRequest определяет тело запроса для регистрации пользователя.
type RegisterRequest struct {
	Login    string `json:"login" example:"newuser123"`    // Логин пользователя, от 3 до 30 символов (буквы, цифры, '_')
	Password string `json:"password" example:"P@$$wOrd123"` // Пароль пользователя, минимум 6 символов
}

// LoginRequest определяет тело запроса для входа пользователя.
type LoginRequest struct {
	Login    string `json:"login" example:"user123"`
	Password string `json:"password" example:"password"`
}

// LoginResponse определяет тело ответа при успешном входе.
type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2..."` // JWT токен доступа
}

// ErrorResponse определяет стандартное тело ответа для ошибок API.
type ErrorResponse struct {
	Error string `json:"error" example:"Сообщение об ошибке"` // Текстовое описание ошибки
}

// AuthHandler обрабатывает HTTP запросы, связанные с аутентификацией.
type AuthHandler struct {
	authService service.AuthService
	log         *zap.Logger
}

// NewAuthHandler создает новый AuthHandler.
func NewAuthHandler(authService service.AuthService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, log: log}
}

// Register godoc
// @Summary Регистрация нового пользователя
// @Description Создает новый аккаунт пользователя с указанными логином и паролем.
// @Description Пароль будет сохранен в хешированном виде (bcrypt).
// @Tags Аутентификация
// @Accept json
// @Produce json
// @Param данные_регистрации body RegisterRequest true "Логин и пароль пользователя для регистрации"
// @Success 200 {object} map[string]string "Сообщение об успешной регистрации (например, {\"message\":\"Пользователь успешно зарегистрирован\"})"
// @Failure 400 {object} ErrorResponse "Ошибка валидации: неверный формат логина или пароля, или неверное тело запроса."
// @Failure 409 {object} ErrorResponse "Конфликт: пользователь с таким логином уже существует."
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера при попытке регистрации."
// @Router /register [post]
func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		h.log.Warn("Не удалось привязать тело запроса регистрации", zap.Error(err))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Неверное тело запроса"})
	}

	// Валидация требований к логину и паролю происходит в AuthService.
	// Здесь можно добавить валидацию формата запроса (например, JSON schema), если используется Echo validator.

	_, err := h.authService.Register(c.Request().Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidLoginFormat), errors.Is(err, service.ErrInvalidPasswordFormat):
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case errors.Is(err, repository.ErrLoginAlreadyExists): // Эта ошибка приходит из repository через service
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		default: // Включая service.ErrRegistrationFailed
			h.log.Error("Ошибка при регистрации пользователя (хендлер)", zap.Error(err), zap.String("login", req.Login))
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Ошибка регистрации"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Пользователь успешно зарегистрирован"})
}

// Login godoc
// @Summary Вход пользователя в систему
// @Description Аутентифицирует пользователя по логину и паролю и возвращает JWT токен доступа.
// @Tags Аутентификация
// @Accept json
// @Produce json
// @Param учетные_данные body LoginRequest true "Логин и пароль пользователя для входа"
// @Success 200 {object} LoginResponse "JWT токен для доступа к защищенным эндпоинтам"
// @Failure 400 {object} ErrorResponse "Неверный формат запроса."
// @Failure 401 {object} ErrorResponse "Ошибка аутентификации: неверный логин или пароль."
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера при попытке входа."
// @Router /login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		h.log.Warn("Не удалось привязать тело запроса входа", zap.Error(err))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Неверное тело запроса"})
	}

	// Валидация происходит в AuthService.

	userID, token, err := h.authService.Login(c.Request().Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
		default: // Включая service.ErrLoginFailed
			h.log.Error("Ошибка при входе пользователя (хендлер)", zap.Error(err), zap.String("login", req.Login))
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Ошибка входа"})
		}
	}

	h.log.Info("Успешный вход пользователя, возвращаем токен", zap.String("login", req.Login), zap.String("userID", userID))
	return c.JSON(http.StatusOK, LoginResponse{Token: token})
}

// RegisterRoutes регистрирует маршруты для эндпоинтов аутентификации.
func (h *AuthHandler) RegisterRoutes(apiGroup *echo.Group) {
	apiGroup.POST("/register", h.Register)
	apiGroup.POST("/login", h.Login)
}