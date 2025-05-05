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
	Login    string `json:"login"`    // validate:"required,alphanum,min=3,max=30"
	Password string `json:"password"` // validate:"required,min=6"
}

type LoginRequest struct {
	Login    string `json:"login"`    // validate:"required"
	Password string `json:"password"` // validate:"required"
}

type LoginResponse struct {
	Token string `json:"token"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type AuthHandler struct {
	authService service.AuthService
	log         *zap.Logger        
}

func NewAuthHandler(authService service.AuthService, log *zap.Logger) *AuthHandler {
	return &AuthHandler{authService: authService, log: log}
}

// Register godoc
// @Summary Регистрация нового пользователя
// @Description Создает новый аккаунт пользователя.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Данные для регистрации"
// @Success 200 {object} map[string]string "message: Пользователь успешно зарегистрирован"
// @Failure 400 {object} ErrorResponse "Неверный формат запроса или данных"
// @Failure 409 {object} ErrorResponse "Логин уже существует"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/register [post]
func (h *AuthHandler) Register(c echo.Context) error {
	var req RegisterRequest

	if err := c.Bind(&req); err != nil {
		h.log.Warn("Не удалось привязать тело запроса регистрации", zap.Error(err))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Неверное тело запроса"})
	}

	// TODO: Добавить валидацию полей req с помощью библиотеки validator, если нужно.
	// Например:
	// if err := c.Validate(&req); err != nil {
	//     return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
	// }

	_, err := h.authService.Register(c.Request().Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidLoginFormat), errors.Is(err, service.ErrInvalidPasswordFormat):
			return c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
		case errors.Is(err, repository.ErrLoginAlreadyExists):
			return c.JSON(http.StatusConflict, ErrorResponse{Error: err.Error()})
		default:
			h.log.Error("Ошибка при регистрации пользователя", zap.Error(err), zap.String("login", req.Login))
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Ошибка регистрации"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "Пользователь успешно зарегистрирован"})
}

// Login godoc
// @Summary Вход пользователя
// @Description Аутентифицирует пользователя и возвращает JWT токен.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Данные для входа"
// @Success 200 {object} LoginResponse "Успешный вход"
// @Failure 400 {object} ErrorResponse "Неверный формат запроса"
// @Failure 401 {object} ErrorResponse "Неверный логин или пароль"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	var req LoginRequest

	if err := c.Bind(&req); err != nil {
		h.log.Warn("Не удалось привязать тело запроса входа", zap.Error(err))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Неверное тело запроса"})
	}

	// TODO: Валидация полей req, если необходимо.

	userID, token, err := h.authService.Login(c.Request().Context(), req.Login, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidCredentials):
			return c.JSON(http.StatusUnauthorized, ErrorResponse{Error: err.Error()})
		default:
			h.log.Error("Ошибка при входе пользователя", zap.Error(err), zap.String("login", req.Login))
			return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Ошибка входа"})
		}
	}

	h.log.Info("Успешный вход пользователя, возвращаем токен (заглушка)", zap.String("login", req.Login), zap.String("userID", userID))
	return c.JSON(http.StatusOK, LoginResponse{Token: token})
}

// RegisterRoutes регистрирует маршруты для эндпоинтов аутентификации
// в переданной группе маршрутов Echo (например, /api/v1).
func (h *AuthHandler) RegisterRoutes(apiGroup *echo.Group) {
	apiGroup.POST("/register", h.Register)
	apiGroup.POST("/login", h.Login)
}