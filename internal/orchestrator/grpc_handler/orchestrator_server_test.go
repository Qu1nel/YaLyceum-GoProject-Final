package grpc_handler

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	repo_mocks "github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository/mocks"
	service_mocks "github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/service/mocks"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func setupOrchestratorServerTest(t *testing.T) (*OrchestratorServer, *repo_mocks.TaskRepositoryMock, *service_mocks.ExpressionEvaluatorMock) {
	logger := zap.NewNop()
	mockTaskRepo := repo_mocks.NewTaskRepositoryMock(t)
	mockEvaluator := service_mocks.NewExpressionEvaluatorMock(t)
	server := NewOrchestratorServer(logger, mockTaskRepo, mockEvaluator)
	return server, mockTaskRepo, mockEvaluator
}

func TestOrchestratorServer_GetTaskDetails_Success(t *testing.T) {
	server, mockTaskRepo, _ := setupOrchestratorServerTest(t)
	ctx := context.Background()

	taskID := uuid.New()
	userID := uuid.New()

	createdAt := time.Now().Add(-time.Hour).UTC().Truncate(time.Microsecond)
	updatedAt := time.Now().UTC().Truncate(time.Microsecond)
	resultVal := 123.45

	mockRepoTask := &repository.Task{
		ID:           taskID,
		UserID:       userID,
		Expression:   "test_expr",
		Status:       repository.StatusCompleted,
		Result:       &resultVal,
		ErrorMessage: nil,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}

	mockTaskRepo.On("GetTaskByID", mock.Anything, taskID).Return(mockRepoTask, nil).Once()

	req := &pb.TaskDetailsRequest{TaskId: taskID.String(), UserId: userID.String()}
	res, err := server.GetTaskDetails(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, taskID.String(), res.Id)
	assert.Equal(t, "test_expr", res.Expression)
	assert.Equal(t, repository.StatusCompleted, res.Status)
	assert.InDelta(t, resultVal, res.Result, 0.00001)
	assert.Empty(t, res.ErrorMessage)

	assert.Equal(t, createdAt.Format(time.RFC3339Nano), res.CreatedAt)
	assert.Equal(t, updatedAt.Format(time.RFC3339Nano), res.UpdatedAt)
	mockTaskRepo.AssertExpectations(t)
}

func TestOrchestratorServer_GetTaskDetails_NotFound(t *testing.T) {
	server, mockTaskRepo, _ := setupOrchestratorServerTest(t)
	ctx := context.Background()
	taskID := uuid.New()
	userID := uuid.New()

	mockTaskRepo.On("GetTaskByID", mock.Anything, taskID).Return(nil, repository.ErrTaskNotFound).Once()

	req := &pb.TaskDetailsRequest{TaskId: taskID.String(), UserId: userID.String()}
	_, err := server.GetTaskDetails(ctx, req)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "задача с ID")
	assert.Contains(t, st.Message(), "не найдена")
	mockTaskRepo.AssertExpectations(t)
}

func TestOrchestratorServer_GetTaskDetails_Forbidden(t *testing.T) {
	server, mockTaskRepo, _ := setupOrchestratorServerTest(t)
	ctx := context.Background()

	taskID := uuid.New()
	ownerUserID := uuid.New()
	requestingUserID := uuid.New()

	mockRepoTask := &repository.Task{ID: taskID, UserID: ownerUserID}
	mockTaskRepo.On("GetTaskByID", mock.Anything, taskID).Return(mockRepoTask, nil).Once()

	req := &pb.TaskDetailsRequest{TaskId: taskID.String(), UserId: requestingUserID.String()}
	_, err := server.GetTaskDetails(ctx, req)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.NotFound, st.Code())
	assert.Contains(t, st.Message(), "(или нет прав доступа)")
	mockTaskRepo.AssertExpectations(t)
}

func TestOrchestratorServer_ListUserTasks_Success(t *testing.T) {
	server, mockTaskRepo, _ := setupOrchestratorServerTest(t)
	ctx := context.Background()
	userID := uuid.New()

	createdAt1 := time.Now().Add(-2 * time.Hour).UTC().Truncate(time.Microsecond)
	createdAt2 := time.Now().Add(-1 * time.Hour).UTC().Truncate(time.Microsecond)

	mockRepoTasks := []repository.Task{
		{ID: uuid.New(), UserID: userID, Expression: "1+1", Status: repository.StatusCompleted, CreatedAt: createdAt1, UpdatedAt: createdAt1},
		{ID: uuid.New(), UserID: userID, Expression: "2*2", Status: repository.StatusPending, CreatedAt: createdAt2, UpdatedAt: createdAt2},
	}
	mockTaskRepo.On("GetTasksByUserID", mock.Anything, userID).Return(mockRepoTasks, nil).Once()

	req := &pb.UserTasksRequest{UserId: userID.String()}
	res, err := server.ListUserTasks(ctx, req)

	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Tasks, 2)
	assert.Equal(t, mockRepoTasks[0].ID.String(), res.Tasks[0].Id)
	assert.Equal(t, mockRepoTasks[0].Expression, res.Tasks[0].Expression)
	assert.Equal(t, mockRepoTasks[0].Status, res.Tasks[0].Status)
	assert.Equal(t, createdAt1.Format(time.RFC3339Nano), res.Tasks[0].CreatedAt)

	assert.Equal(t, mockRepoTasks[1].ID.String(), res.Tasks[1].Id)
	assert.Equal(t, mockRepoTasks[1].Expression, res.Tasks[1].Expression)
	assert.Equal(t, mockRepoTasks[1].Status, res.Tasks[1].Status)
	assert.Equal(t, createdAt2.Format(time.RFC3339Nano), res.Tasks[1].CreatedAt)

	mockTaskRepo.AssertExpectations(t)
}

func TestOrchestratorServer_ListUserTasks_RepoError(t *testing.T) {
	server, mockTaskRepo, _ := setupOrchestratorServerTest(t)
	ctx := context.Background()
	userID := uuid.New()
	repoErr := errors.New("ошибка бд при получении списка")

	mockTaskRepo.On("GetTasksByUserID", mock.Anything, userID).Return(nil, repoErr).Once()

	req := &pb.UserTasksRequest{UserId: userID.String()}
	_, err := server.ListUserTasks(ctx, req)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
	mockTaskRepo.AssertExpectations(t)
}
