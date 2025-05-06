package handler

import (
	"errors"
	"net/http"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/middleware"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/service"
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
	log         *zap.Logger
	taskService service.TaskService // <-- Зависимость от сервиса задач
}


func NewTaskHandler(
	log *zap.Logger,
	taskService service.TaskService, // <-- Новая зависимость
) *TaskHandler {
	return &TaskHandler{
		log:         log,
		taskService: taskService,
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

	taskID, err := h.taskService.SubmitNewTask(c.Request().Context(), userID, req.Expression)
	if err != nil {
		h.log.Error("Ошибка от TaskService при SubmitNewTask", zap.Error(err), zap.String("userID", userID))
		// Здесь можно добавить более специфичную обработку ошибок от сервиса, если нужно
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()}) // Передаем ошибку от сервиса
	}

	h.log.Info("Задача успешно принята к обработке", zap.String("taskID", taskID), zap.String("userID", userID))

	return c.JSON(http.StatusAccepted, CalculateResponse{TaskID: taskID})
}

// GetTasks godoc
// @Summary Получить список задач пользователя
// @Description Возвращает список всех задач, созданных текущим аутентифицированным пользователем.
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Success 200 {array} service.TaskListItem "Список задач"
// @Failure 401 {object} ErrorResponse "Ошибка аутентификации"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/tasks [get]
func (h *TaskHandler) GetTasks(c echo.Context) error {
    userID, ok := middleware.GetUserIDFromContext(c)
    if !ok {
        h.log.Error("Не удалось получить UserID из контекста в /tasks")
        return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Внутренняя ошибка сервера"})
    }

    h.log.Info("Запрос списка задач для пользователя", zap.String("userID", userID))
    tasks, err := h.taskService.GetUserTasks(c.Request().Context(), userID)
    if err != nil {
        h.log.Error("Ошибка от TaskService при GetUserTasks", zap.Error(err), zap.String("userID", userID))
        return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
    }

    if tasks == nil { // Сервис может вернуть nil, если задач нет (или []TaskListItem{})
        tasks = []service.TaskListItem{} // Возвращаем пустой массив JSON, а не null
    }

    return c.JSON(http.StatusOK, tasks)
}

// GetTaskByID godoc
// @Summary Получить детали конкретной задачи
// @Description Возвращает детали задачи по её ID, если она принадлежит текущему пользователю.
// @Tags Tasks
// @Security BearerAuth
// @Produce json
// @Param id path string true "ID Задачи (UUID)"
// @Success 200 {object} service.TaskDetails "Детали задачи"
// @Failure 400 {object} ErrorResponse "Невалидный ID задачи"
// @Failure 401 {object} ErrorResponse "Ошибка аутентификации"
// @Failure 404 {object} ErrorResponse "Задача не найдена или нет прав доступа"
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера"
// @Router /api/v1/tasks/{id} [get]
func (h *TaskHandler) GetTaskByID(c echo.Context) error {
    userID, ok := middleware.GetUserIDFromContext(c)
    if !ok {
        h.log.Error("Не удалось получить UserID из контекста в /tasks/{id}")
        return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Внутренняя ошибка сервера"})
    }

    taskIDStr := c.Param("id")
    if _, err := uuid.Parse(taskIDStr); err != nil { // Валидация формата UUID
        return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Невалидный формат ID задачи"})
    }

    h.log.Info("Запрос деталей задачи", zap.String("userID", userID), zap.String("taskID", taskIDStr))
    taskDetails, err := h.taskService.GetTaskDetails(c.Request().Context(), userID, taskIDStr)
    if err != nil {
        h.log.Error("Ошибка от TaskService при GetTaskDetails", zap.Error(err), zap.String("userID", userID), zap.String("taskID", taskIDStr))
        if errors.Is(err, service.ErrTaskNotFound) { // Проверяем нашу кастомную ошибку
            return c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
        }
        return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
    }

    return c.JSON(http.StatusOK, taskDetails)
}


// RegisterRoutes теперь регистрирует все маршруты для задач.
func (h *TaskHandler) RegisterRoutes(protectedGroup *echo.Group) {
	protectedGroup.POST("/calculate", h.Calculate)
	protectedGroup.GET("/tasks", h.GetTasks)
	protectedGroup.GET("/tasks/:id", h.GetTaskByID)
}