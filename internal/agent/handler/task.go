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

// CalculateRequest определяет тело запроса для вычисления выражения.
type CalculateRequest struct {
	Expression string `json:"expression" validate:"required" example:"(2+2)*4"` // Математическое выражение для вычисления
}

// CalculateResponse определяет тело ответа при успешном приеме задачи на вычисление.
type CalculateResponse struct {
	TaskID string `json:"task_id" example:"a1b2c3d4-e5f6-7890-1234-567890abcdef"` // Уникальный идентификатор созданной задачи
}

// TaskHandler обрабатывает HTTP запросы, связанные с задачами вычисления.
type TaskHandler struct {
	log         *zap.Logger
	taskService service.TaskService
}

// NewTaskHandler создает новый TaskHandler.
func NewTaskHandler(log *zap.Logger, taskService service.TaskService) *TaskHandler {
	return &TaskHandler{
		log:         log,
		taskService: taskService,
	}
}

// Calculate godoc
// @Summary Отправить выражение на вычисление
// @Description Принимает арифметическое выражение от аутентифицированного пользователя, создает задачу и ставит ее в очередь на асинхронное вычисление.
// @Description В случае успеха возвращает ID созданной задачи. Статус задачи изначально будет "pending".
// @Tags Задачи
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param тело_запроса body CalculateRequest true "Объект с математическим выражением"
// @Success 202 {object} CalculateResponse "Запрос успешно принят, задача поставлена в очередь. Возвращается ID задачи."
// @Failure 400 {object} ErrorResponse "Ошибка валидации: неверный формат запроса, пустое или некорректное выражение."
// @Failure 401 {object} ErrorResponse "Ошибка аутентификации: JWT токен отсутствует, невалиден или истек."
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера при создании задачи или взаимодействии с другими сервисами."
// @Router /calculate [post]
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
        // Проверяем, не является ли это ошибкой InvalidArgument от Оркестратора (транслированной сервисом)
        // и возвращаем 400, если это так.
        // Это потребует от TaskService возвращать ошибку, которую можно так проверить,
        // или более специфичные типы ошибок. Пока оставляем 500 для простоты.
        // if strings.Contains(err.Error(), string(codes.InvalidArgument)) { // Грубая проверка
        //    return c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Ошибка в выражении: " + err.Error()})
        // }
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
	}

	h.log.Info("Задача успешно принята к обработке", zap.String("taskID", taskID), zap.String("userID", userID))
	return c.JSON(http.StatusAccepted, CalculateResponse{TaskID: taskID})
}

// GetTasks godoc
// @Summary Получить список задач пользователя
// @Description Возвращает список всех задач (с краткой информацией), созданных текущим аутентифицированным пользователем.
// @Description Задачи отсортированы по времени создания (сначала новые). Пагинация пока не реализована.
// @Tags Задачи
// @Security BearerAuth
// @Produce json
// @Success 200 {array} service.TaskListItem "Массив объектов с краткой информацией о задачах."
// @Failure 401 {object} ErrorResponse "Ошибка аутентификации: JWT токен отсутствует, невалиден или истек."
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера при получении списка задач."
// @Router /tasks [get]
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
        tasks = []service.TaskListItem{} // Гарантируем возврат пустого массива JSON `[]` вместо `null`
    }

    return c.JSON(http.StatusOK, tasks)
}

// GetTaskByID godoc
// @Summary Получить детали конкретной задачи
// @Description Возвращает полную информацию о задаче по её ID, если она принадлежит текущему аутентифицированному пользователю.
// @Tags Задачи
// @Security BearerAuth
// @Produce json
// @Param id path string true "ID Задачи (в формате UUID)" example:"a1b2c3d4-e5f6-7890-1234-567890abcdef"
// @Success 200 {object} service.TaskDetails "Объект с полной информацией о задаче."
// @Failure 400 {object} ErrorResponse "Невалидный формат ID задачи (не UUID)."
// @Failure 401 {object} ErrorResponse "Ошибка аутентификации: JWT токен отсутствует, невалиден или истек."
// @Failure 404 {object} ErrorResponse "Задача с указанным ID не найдена или не принадлежит текущему пользователю."
// @Failure 500 {object} ErrorResponse "Внутренняя ошибка сервера при получении деталей задачи."
// @Router /tasks/{id} [get]
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

// RegisterRoutes регистрирует маршруты для задач.
func (h *TaskHandler) RegisterRoutes(protectedGroup *echo.Group) {
	protectedGroup.POST("/calculate", h.Calculate)
	protectedGroup.GET("/tasks", h.GetTasks)
	protectedGroup.GET("/tasks/:id", h.GetTaskByID)
}