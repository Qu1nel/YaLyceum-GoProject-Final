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

type TaskListItem struct {
	ID         string    `json:"id"`
	Expression string    `json:"expression"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type TaskDetails struct {
	ID           string    `json:"id"`
	Expression   string    `json:"expression"`
	Status       string    `json:"status"`
	Result       *float64  `json:"result,omitempty"`
	ErrorMessage *string   `json:"error_message,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type TaskService interface {
	SubmitNewTask(ctx context.Context, userID, expression string) (taskID string, err error)

	GetUserTasks(ctx context.Context, userID string) ([]TaskListItem, error)

	GetTaskDetails(ctx context.Context, userID, taskID string) (*TaskDetails, error)
}

type taskService struct {
	log                *zap.Logger
	orchestratorClient pb_orchestrator.OrchestratorServiceClient
	grpcClientTimeout  time.Duration
}

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

		return "", fmt.Errorf("ошибка сервиса вычислений: %w", err)
	}
	return grpcRes.GetTaskId(), nil
}

func (s *taskService) GetUserTasks(ctx context.Context, userID string) ([]TaskListItem, error) {
	grpcCtx, cancel := context.WithTimeout(ctx, s.grpcClientTimeout)
	defer cancel()

	grpcReq := &pb_orchestrator.UserTasksRequest{UserId: userID}
	grpcRes, err := s.orchestratorClient.ListUserTasks(grpcCtx, grpcReq)
	if err != nil {
		s.log.Error("Ошибка gRPC вызова ListUserTasks из TaskService", zap.Error(err), zap.String("userID", userID))
		return nil, fmt.Errorf("ошибка получения списка задач: %w", err)
	}
	tasks := make([]TaskListItem, 0, len(grpcRes.GetTasks()))
	for _, pbTask := range grpcRes.GetTasks() {
		createdAt, pErr := time.Parse(time.RFC3339Nano, pbTask.GetCreatedAt())
		if pErr != nil {
			s.log.Warn("Не удалось распарсить CreatedAt из gRPC ответа", zap.Error(pErr), zap.String("value", pbTask.GetCreatedAt()))
		}
		tasks = append(tasks, TaskListItem{
			ID:         pbTask.GetId(),
			Expression: pbTask.GetExpression(),
			Status:     pbTask.GetStatus(),
			CreatedAt:  createdAt,
		})
	}
	return tasks, nil
}

func (s *taskService) GetTaskDetails(ctx context.Context, userID, taskID string) (*TaskDetails, error) {
	grpcCtx, cancel := context.WithTimeout(ctx, s.grpcClientTimeout)
	defer cancel()

	grpcReq := &pb_orchestrator.TaskDetailsRequest{UserId: userID, TaskId: taskID}
	grpcRes, err := s.orchestratorClient.GetTaskDetails(grpcCtx, grpcReq)
	if err != nil {
		s.log.Error("Ошибка gRPC вызова GetTaskDetails из TaskService", zap.Error(err), zap.String("userID", userID), zap.String("taskID", taskID))
		st, ok := status.FromError(err)
		if ok && st.Code() == codes.NotFound {

			return nil, fmt.Errorf("%w: %w", ErrTaskNotFound, err)
		}

		return nil, fmt.Errorf("ошибка получения деталей задачи: %w", err)
	}

	createdAt, cErr := time.Parse(time.RFC3339Nano, grpcRes.GetCreatedAt())
	if cErr != nil {
		s.log.Warn("Не удалось распарсить CreatedAt для деталей задачи", zap.Error(cErr), zap.String("value", grpcRes.GetCreatedAt()))
	}
	updatedAt, uErr := time.Parse(time.RFC3339Nano, grpcRes.GetUpdatedAt())
	if uErr != nil {
		s.log.Warn("Не удалось распарсить UpdatedAt для деталей задачи", zap.Error(uErr), zap.String("value", grpcRes.GetUpdatedAt()))
	}

	details := &TaskDetails{
		ID:         grpcRes.GetId(),
		Expression: grpcRes.GetExpression(),
		Status:     grpcRes.GetStatus(),
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
	if grpcRes.GetStatus() == repository.StatusCompleted {
		resCopy := grpcRes.GetResult()
		details.Result = &resCopy
	}
	if grpcRes.GetStatus() == repository.StatusFailed && grpcRes.GetErrorMessage() != "" {
		errMsgCopy := grpcRes.GetErrorMessage()
		details.ErrorMessage = &errMsgCopy
	}
	return details, nil
}
