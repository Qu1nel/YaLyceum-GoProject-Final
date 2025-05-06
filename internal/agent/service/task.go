package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	pb_orchestrator "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
    ErrTaskNotFound = errors.New("задача не найдена или нет прав доступа")
)

// Task DTO для передачи из сервиса в хендлер (может отличаться от gRPC структур)
// Это позволит нам не зависеть от gRPC деталей в хендлерах, если мы захотим изменить транспорт.
type TaskListItem struct {
	ID         string    `json:"id"`
	Expression string    `json:"expression"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type TaskDetails struct {
	ID           string     `json:"id"`
	Expression   string     `json:"expression"`
	Status       string     `json:"status"`
	Result       *float64   `json:"result,omitempty"` // omitempty, если nil
	ErrorMessage *string    `json:"error_message,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}


// TaskService определяет интерфейс для операций с задачами.
type TaskService interface {
	// SubmitNewTask отправляет новое выражение Оркестратору.
	SubmitNewTask(ctx context.Context, userID, expression string) (taskID string, err error)
    // GetUserTasks получает список задач для пользователя.
    GetUserTasks(ctx context.Context, userID string) ([]TaskListItem, error)
    // GetTaskDetails получает детали конкретной задачи.
    GetTaskDetails(ctx context.Context, userID, taskID string) (*TaskDetails, error)
}

type taskService struct {
	log               *zap.Logger
	orchestratorClient pb_orchestrator.OrchestratorServiceClient
	grpcClientTimeout time.Duration
}

// NewTaskService создает новый экземпляр TaskService.
func NewTaskService(
	log *zap.Logger,
	orcClient pb_orchestrator.OrchestratorServiceClient,
	cfg *config.Config,
) TaskService {
	return &taskService{
		log:                log,
		orchestratorClient: orcClient,
		grpcClientTimeout:  cfg.OrchestratorClient.Timeout,
	}
}

// SubmitNewTask вызывает gRPC метод Оркестратора.
func (s *taskService) SubmitNewTask(ctx context.Context, userID, expression string) (string, error) {
	grpcCtx, cancel := context.WithTimeout(ctx, s.grpcClientTimeout)
	defer cancel()

	grpcReq := &pb_orchestrator.ExpressionRequest{
		UserId:     userID,
		Expression: expression,
	}

	grpcRes, err := s.orchestratorClient.SubmitExpression(grpcCtx, grpcReq)
	if err != nil {
		s.log.Error("Ошибка gRPC вызова SubmitExpression из TaskService", zap.Error(err))
        st, _ := status.FromError(err)
		return "", fmt.Errorf("ошибка сервиса вычислений: %s (код: %s)", st.Message(), st.Code())
	}
	return grpcRes.GetTaskId(), nil
}

// GetUserTasks вызывает gRPC метод Оркестратора.
func (s *taskService) GetUserTasks(ctx context.Context, userID string) ([]TaskListItem, error) {
    grpcCtx, cancel := context.WithTimeout(ctx, s.grpcClientTimeout)
    defer cancel()

    grpcReq := &pb_orchestrator.UserTasksRequest{UserId: userID}
    grpcRes, err := s.orchestratorClient.ListUserTasks(grpcCtx, grpcReq)
    if err != nil {
        s.log.Error("Ошибка gRPC вызова ListUserTasks из TaskService", zap.Error(err), zap.String("userID", userID))
        st, _ := status.FromError(err)
        return nil, fmt.Errorf("ошибка получения списка задач: %s (код: %s)", st.Message(), st.Code())
    }

    tasks := make([]TaskListItem, 0, len(grpcRes.GetTasks()))
    for _, pbTask := range grpcRes.GetTasks() {
        createdAt, _ := time.Parse(time.RFC3339Nano, pbTask.GetCreatedAt()) // Ошибку парсинга можно обработать
        tasks = append(tasks, TaskListItem{
            ID:         pbTask.GetId(),
            Expression: pbTask.GetExpression(),
            Status:     pbTask.GetStatus(),
            CreatedAt:  createdAt,
        })
    }
    return tasks, nil
}

// GetTaskDetails вызывает gRPC метод Оркестратора.
func (s *taskService) GetTaskDetails(ctx context.Context, userID, taskID string) (*TaskDetails, error) {
    grpcCtx, cancel := context.WithTimeout(ctx, s.grpcClientTimeout)
    defer cancel()

    grpcReq := &pb_orchestrator.TaskDetailsRequest{UserId: userID, TaskId: taskID}
    grpcRes, err := s.orchestratorClient.GetTaskDetails(grpcCtx, grpcReq)
    if err != nil {
        s.log.Error("Ошибка gRPC вызова GetTaskDetails из TaskService",
            zap.Error(err),
            zap.String("userID", userID),
            zap.String("taskID", taskID),
        )
        st, _ := status.FromError(err)
        // Специально обрабатываем NotFound, чтобы вернуть его в хендлер
        if st.Code() == codes.NotFound {
             return nil, fmt.Errorf("%w: %s", ErrTaskNotFound, st.Message()) // Обернем нашу ошибку
        }
        return nil, fmt.Errorf("ошибка получения деталей задачи: %s (код: %s)", st.Message(), st.Code())
    }

    createdAt, _ := time.Parse(time.RFC3339Nano, grpcRes.GetCreatedAt())
    updatedAt, _ := time.Parse(time.RFC3339Nano, grpcRes.GetUpdatedAt())

    details := &TaskDetails{
        ID:         grpcRes.GetId(),
        Expression: grpcRes.GetExpression(),
        Status:     grpcRes.GetStatus(),
        CreatedAt:  createdAt,
        UpdatedAt:  updatedAt,
    }
    // Результат и ошибка могут быть nil в Task из репозитория, но в gRPC это просто пустые значения,
    // если их не установить. Proto3 по умолчанию для float64 это 0.0, для string - пустая строка.
    // Мы сделали Result и ErrorMessage в Task DTO указателями, чтобы передать null.
    if grpcRes.GetStatus() == repository.StatusCompleted {
        resCopy := grpcRes.GetResult() // Копируем значение
        details.Result = &resCopy
    }
    if grpcRes.GetStatus() == repository.StatusFailed && grpcRes.GetErrorMessage() != "" {
        errMsgCopy := grpcRes.GetErrorMessage()
        details.ErrorMessage = &errMsgCopy
    }

    return details, nil
}