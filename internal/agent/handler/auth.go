package handler

import (
	"errors"
	"net/http"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/repository"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/service"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type RegisterRequest struct {
	Login    string `json:"login" example:"newuser123"`
	Password string `json:"password" example:"P@$$wOrd123"`
}

type LoginRequest struct {
	Login    string `json:"login" example:"user123"`
	Password string `json:"password" example:"password"`
}

type LoginResponse struct {
	Token string `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE2..."`
}

type ErrorResponse struct {
	Error string `json:"error" example:"Сообщение об ошибке"`
}

type AuthHandler struct {
	authService service.AuthService
	log         *zap.Logger
}

func NewAuthHandler(authService service.AuthService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, log: log}
}

func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest
	if err := c.Bind(&req); err != nil {
		h.log.Warn("Не удалось привязать тело запроса регистрации", zap.Error(err))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Неверное тело запроса"})
	}

	_, err := h.authService.Register(c.Request().Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidLoginFormat), errors.Is(err, service.ErrInvalidPasswordFormat):
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case errors.Is(err, repository.ErrLoginAlreadyExists):
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		default:
			h.log.Error("Ошибка при регистрации пользователя (хендлер)", zap.Error(err), zap.String("login", req.Login))
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Ошибка регистрации"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Пользователь успешно зарегистрирован"})
}

func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest
	if err := c.Bind(&req); err != nil {
		h.log.Warn("Не удалось привязать тело запроса входа", zap.Error(err))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Неверное тело запроса"})
	}

	userID, token, err := h.authService.Login(c.Request().Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
		default:
			h.log.Error("Ошибка при входе пользователя (хендлер)", zap.Error(err), zap.String("login", req.Login))
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Ошибка входа"})
		}
	}

	h.log.Info("Успешный вход пользователя, возвращаем токен", zap.String("login", req.Login), zap.String("userID", userID))
	return c.JSON(http.StatusOK, LoginResponse{Token: token})
}

func (h *AuthHandler) RegisterRoutes(apiGroup *echo.Group) {
	apiGroup.POST("/register", h.Register)
	apiGroup.POST("/login", h.Login)
}
