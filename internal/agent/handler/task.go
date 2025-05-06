package handler

import (
	"net/http"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/middleware"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"

	"context"

	"github.com/labstack/echo/v4"
	"go.uber.org/zap"
	"google.golang.org/grpc/status"
)

type CalculateRequest struct {
	Expression string `json:"expression" validate:"required"`
}

type CalculateResponse struct {
	TaskID string `json:"task_id"`
}

type TaskHandler struct {
	log               *zap.Logger
	orchestratorClient pb.OrchestratorServiceClient // gRPC клиент
	grpcClientTimeout time.Duration                // Таймаут для gRPC вызовов
}


// NewTaskHandler теперь принимает OrchestratorServiceClient и конфигурацию.
func NewTaskHandler(
	log *zap.Logger,
	orcClient pb.OrchestratorServiceClient,
	cfg *config.Config, // Получаем весь конфиг, чтобы взять таймаут
) *TaskHandler {
	return &TaskHandler{
		log:                log,
		orchestratorClient: orcClient,
		grpcClientTimeout:  cfg.OrchestratorClient.Timeout, // Берем таймаут из конфига
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

	// Создаем контекст с таймаутом для gRPC вызова
	ctx, cancel := context.WithTimeout(c.Request().Context(), h.grpcClientTimeout)
	defer cancel()

	// Вызов gRPC метода Оркестратора
	grpcReq := &pb.ExpressionRequest{
		UserId:     userID,
		Expression: req.Expression,
	}

	grpcRes, err := h.orchestratorClient.SubmitExpression(ctx, grpcReq)
	if err != nil {
		h.log.Error("Ошибка при вызове gRPC SubmitExpression Оркестратора",
			zap.String("userID", userID),
			zap.String("expression", req.Expression),
			zap.Error(err),
		)
		// Преобразуем gRPC ошибку в HTTP ошибку
		st, _ := status.FromError(err) // Игнорируем ok, т.к. даже если это не gRPC ошибка, код будет Unknown
		// Можно добавить более детальную обработку кодов gRPC (Unavailable, DeadlineExceeded и т.д.)
		return c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Ошибка взаимодействия с сервисом вычислений: " + st.Message()})
	}

	h.log.Info("Выражение успешно отправлено в Оркестратор",
		zap.String("userID", userID),
		zap.String("expression", req.Expression),
		zap.String("returned_task_id", grpcRes.GetTaskId()),
	)

	return c.JSON(http.StatusAccepted, CalculateResponse{TaskID: grpcRes.GetTaskId()})
}

// RegisterRoutes регистрирует маршруты для задач в защищенной группе.
func (h *TaskHandler) RegisterRoutes(protectedGroup *echo.Group) {
	protectedGroup.POST("/calculate", h.Calculate)
	// protectedGroup.GET("/tasks", h.GetTasks)
	// protectedGroup.GET("/tasks/:id", h.GetTaskByID)
}