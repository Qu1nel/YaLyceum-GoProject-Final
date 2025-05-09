package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/service/mocks"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func setupTaskServiceTest(t *testing.T) (*taskService, *mocks.OrchestratorServiceClientMock) {
	logger := zap.NewNop()
	mockOrcClient := mocks.NewOrchestratorServiceClientMock(t)
	cfg := &config.Config{
		OrchestratorClient: config.GRPCClientConfig{
			Timeout: 5 * time.Second,
		},
	}
	serviceInstance := NewTaskService(logger, mockOrcClient, cfg).(*taskService)
	return serviceInstance, mockOrcClient
}

func TestTaskService_SubmitNewTask_Success(t *testing.T) {
	ts, mockOrcClient := setupTaskServiceTest(t)
	ctx := context.Background()
	userID := uuid.New().String()
	expression := "2+2"
	expectedTaskID := uuid.New().String()

	mockOrcClient.On("SubmitExpression",
		mock.AnythingOfType("*context.timerCtx"),
		&pb.ExpressionRequest{UserId: userID, Expression: expression},
	).Return(&pb.ExpressionResponse{TaskId: expectedTaskID}, nil).Once()

	taskID, err := ts.SubmitNewTask(ctx, userID, expression)
	require.NoError(t, err)
	assert.Equal(t, expectedTaskID, taskID)
	mockOrcClient.AssertExpectations(t)
}

func TestTaskService_SubmitNewTask_gRPCError(t *testing.T) {
	ts, mockOrcClient := setupTaskServiceTest(t)
	ctx := context.Background()
	originalGrpcErr := status.Error(codes.Unavailable, "оркестратор недоступен")

	mockOrcClient.On("SubmitExpression",
		mock.AnythingOfType("*context.timerCtx"),
		mock.AnythingOfType("*orchestrator_grpc.ExpressionRequest"),
	).Return(nil, originalGrpcErr).Once()

	_, err := ts.SubmitNewTask(ctx, uuid.New().String(), "3*3")
	require.Error(t, err, "SubmitNewTask должен вернуть ошибку")

	assert.Contains(t, err.Error(), "ошибка сервиса вычислений: ", "Сообщение об ошибке должно начинаться с префикса сервиса")
	assert.Contains(t, err.Error(), originalGrpcErr.Error(), "Сообщение об ошибке должно содержать текст оригинальной gRPC ошибки")

	unwrappedErr := errors.Unwrap(err)
	require.NotNil(t, unwrappedErr, "Обернутая ошибка не должна быть nil")

	rpcStatus, ok := status.FromError(unwrappedErr)
	require.True(t, ok, "Развернутая ошибка должна быть конвертируема в gRPC статус")
	assert.Equal(t, codes.Unavailable, rpcStatus.Code(), "gRPC код ошибки не совпадает")
	assert.Equal(t, "оркестратор недоступен", rpcStatus.Message(), "Сообщение gRPC ошибки не совпадает")

	mockOrcClient.AssertExpectations(t)
}

func TestTaskService_GetUserTasks_Success(t *testing.T) {
	ts, mockOrcClient := setupTaskServiceTest(t)
	ctx := context.Background()
	userID := uuid.New().String()
	now := time.Now()

	nowStr := now.Format(time.RFC3339Nano)

	mockResponse := &pb.UserTasksResponse{
		Tasks: []*pb.TaskBrief{
			{Id: uuid.New().String(), Expression: "1+1", Status: "completed", CreatedAt: nowStr},
			{Id: uuid.New().String(), Expression: "2*2", Status: "pending", CreatedAt: nowStr},
		},
	}
	mockOrcClient.On("ListUserTasks",
		mock.AnythingOfType("*context.timerCtx"),
		&pb.UserTasksRequest{UserId: userID},
	).Return(mockResponse, nil).Once()

	tasks, err := ts.GetUserTasks(ctx, userID)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	assert.Equal(t, mockResponse.Tasks[0].Id, tasks[0].ID)
	assert.Equal(t, mockResponse.Tasks[1].Expression, tasks[1].Expression)

	assert.WithinDuration(t, now, tasks[0].CreatedAt, time.Second)
	mockOrcClient.AssertExpectations(t)
}

func TestTaskService_GetTaskDetails_Success(t *testing.T) {
	ts, mockOrcClient := setupTaskServiceTest(t)
	ctx := context.Background()
	userID := uuid.New().String()
	taskID := uuid.New().String()
	now := time.Now()
	nowStr := now.Format(time.RFC3339Nano)
	resultVal := 6.0

	mockResponse := &pb.TaskDetailsResponse{
		Id:         taskID,
		Expression: "2+2*2",
		Status:     "completed",
		Result:     resultVal,
		CreatedAt:  nowStr,
		UpdatedAt:  nowStr,
	}
	mockOrcClient.On("GetTaskDetails",
		mock.AnythingOfType("*context.timerCtx"),
		&pb.TaskDetailsRequest{UserId: userID, TaskId: taskID},
	).Return(mockResponse, nil).Once()

	details, err := ts.GetTaskDetails(ctx, userID, taskID)
	require.NoError(t, err)
	require.NotNil(t, details)
	assert.Equal(t, taskID, details.ID)
	assert.Equal(t, "completed", details.Status)
	require.NotNil(t, details.Result)
	assert.Equal(t, resultVal, *details.Result)
	assert.Nil(t, details.ErrorMessage)
	assert.WithinDuration(t, now, details.CreatedAt, time.Second)
	assert.WithinDuration(t, now, details.UpdatedAt, time.Second)
	mockOrcClient.AssertExpectations(t)
}

func TestTaskService_GetTaskDetails_NotFound(t *testing.T) {
	ts, mockOrcClient := setupTaskServiceTest(t)
	ctx := context.Background()
	userID := uuid.New().String()
	taskID := uuid.New().String()

	grpcErr := status.Error(codes.NotFound, "задача не найдена в оркестраторе")

	mockOrcClient.On("GetTaskDetails",
		mock.AnythingOfType("*context.timerCtx"),
		&pb.TaskDetailsRequest{UserId: userID, TaskId: taskID},
	).Return(nil, grpcErr).Once()

	_, err := ts.GetTaskDetails(ctx, userID, taskID)
	require.Error(t, err)

	assert.ErrorIs(t, err, ErrTaskNotFound, "Ошибка должна быть ErrTaskNotFound")

	assert.Contains(t, err.Error(), "задача не найдена в оркестраторе", "Сообщение должно содержать детали gRPC ошибки")
	mockOrcClient.AssertExpectations(t)
}
