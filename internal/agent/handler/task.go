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
	Expression string `json:"expression" validate:"required" example:"(2+2)*4"`
}

type CalculateResponse struct {
	TaskID string `json:"task_id" example:"a1b2c3d4-e5f6-7890-1234-567890abcdef"`
}

type TaskHandler struct {
	log         *zap.Logger
	taskService service.TaskService
}

func NewTaskHandler(log *zap.Logger, taskService service.TaskService) *TaskHandler {
	return &TaskHandler{
		log:         log,
		taskService: taskService,
	}
}

func (h *TaskHandler) Calculate(c echo.Context) error {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		h.log.Error("Не удалось получить UserID из контекста в /calculate")
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Внутренняя ошибка сервера"})
	}

	h.log.Info("Получен запрос на вычисление", zap.String("userID", userID))

	var req CalculateRequest
	if err := c.Bind(&req); err != nil {
		h.log.Warn("Не удалось привязать тело запроса /calculate", zap.Error(err), zap.String("userID", userID))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Неверное тело запроса"})
	}

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

		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	h.log.Info("Задача успешно принята к обработке", zap.String("taskID", taskID), zap.String("userID", userID))
	return c.JSON(http.StatusAccepted, CalculateResponse{TaskID: taskID})
}

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

	if tasks == nil {
		tasks = []service.TaskListItem{}
	}

	return c.JSON(http.StatusOK, tasks)
}

func (h *TaskHandler) GetTaskByID(c echo.Context) error {
	userID, ok := middleware.GetUserIDFromContext(c)
	if !ok {
		h.log.Error("Не удалось получить UserID из контекста в /tasks/{id}")
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Внутренняя ошибка сервера"})
	}

	taskIDStr := c.Param("id")
	if _, err := uuid.Parse(taskIDStr); err != nil {
		h.log.Warn("Запрос деталей задачи с невалидным форматом ID", zap.String("taskID_str", taskIDStr), zap.Error(err))
		return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Невалидный формат ID задачи"})
	}

	h.log.Info("Запрос деталей задачи", zap.String("userID", userID), zap.String("taskID", taskIDStr))
	taskDetails, err := h.taskService.GetTaskDetails(c.Request().Context(), userID, taskIDStr)
	if err != nil {
		h.log.Warn("Ошибка от TaskService при GetTaskDetails", zap.Error(err), zap.String("userID", userID), zap.String("taskID", taskIDStr))
		if errors.Is(err, service.ErrTaskNotFound) {
			return c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
		}
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	return c.JSON(http.StatusOK, taskDetails)
}

func (h *TaskHandler) RegisterRoutes(protectedGroup *echo.Group) {
	protectedGroup.POST("/calculate", h.Calculate)
	protectedGroup.GET("/tasks", h.GetTasks)
	protectedGroup.GET("/tasks/:id", h.GetTaskByID)
}
