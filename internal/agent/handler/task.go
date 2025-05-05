package handler

import (
	"net/http"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
)

type CalculateRequest struct {
	Expression string `json:"expression" validate:"required"`
}

type CalculateResponse struct {
	TaskID string `json:"task_id"`
}

type TaskHandler struct {
	log *zap.Logger
}

func NewTaskHandler(log *zap.Logger /*, taskService service.TaskService */) *TaskHandler {
	return &TaskHandler{
		log: log,
		// taskService: taskService,
	}
}

// Calculate godoc
// @Summary Отправить выражение на вычисление
// @Description Принимает арифметическое выражение от аутентифицированного пользователя и ставит его в очередь на вычисление.
// @Tags Tasks
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body CalculateRequest true "Выражение для вычисления"
// @Success 202 {object} CalculateResponse "Запрос принят к обработке"
// @Failure 400 {object} ErrorResponse "Неверный формат запроса или выражения"
// @Failure 401 {object} ErrorResponse "Ошибка аутентификации (невалидный токен)"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/calculate [post]
func (h *TaskHandler) Calculate(c echo.Context) error {
	// 1. Извлекаем UserID из контекста, который был добавлен JWTAuth middleware.
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		// Эта ситуация не должна происходить, если middleware отработало корректно,
		// но лучше проверить на всякий случай.
		h.log.Error("Не удалось получить UserID из контекста в защищенном маршруте")
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Внутренняя ошибка сервера: отсутствует UserID"})
	}

	// Логируем полученный UserID
	h.log.Info("Получен запрос на вычисление", zap.String("userID", userID))

	// 2. Привязываем тело запроса
	var req CalculateRequest
	if err := c.Bind(&req); err != nil {
		h.log.Warn("Не удалось привязать тело запроса /calculate", zap.Error(err), zap.String("userID", userID))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Неверное тело запроса"})
	}

	// 3. TODO: Валидация выражения req.Expression (позже, возможно, на уровне сервиса Оркестратора)
	if req.Expression == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Поле 'expression' не может быть пустым"})
	}

	h.log.Info("Принято выражение от пользователя",
		zap.String("userID", userID),
		zap.String("expression", req.Expression),
	)

	// 4. TODO: Вызвать сервис (например, TaskService), который передаст задачу Оркестратору по gRPC.
	// taskID, err := h.taskService.CreateTask(c.Request().Context(), userID, req.Expression)
	// Обработка ошибок сервиса...

	// ЗАГЛУШКА: Генерируем фейковый ID задачи для ответа
	fakeTaskID := uuid.NewString()

	// 5. Возвращаем ответ 202 Accepted с ID задачи.
	return c.JSON(http.StatusAccepted, CalculateResponse{TaskID: fakeTaskID})
}

// RegisterRoutes регистрирует маршруты для задач в защищенной группе.
func (h *TaskHandler) RegisterRoutes(protectedGroup *echo.Group) {
	protectedGroup.POST("/calculate", h.Calculate)
	// protectedGroup.GET("/tasks", h.GetTasks)
	// protectedGroup.GET("/tasks/:id", h.GetTaskByID)
}