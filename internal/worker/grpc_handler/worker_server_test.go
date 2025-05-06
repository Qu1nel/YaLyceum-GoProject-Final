package grpc_handler

import (
	"context"
	"fmt"
	"testing"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/service"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/service/mocks"
	pb_worker "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestWorkerServer_CalculateOperation_Success(t *testing.T) {
	logger := zap.NewNop()
	mockCalcService := mocks.NewCalculatorServiceMock(t)
	grpcServer := NewWorkerServer(logger, mockCalcService)

	req := &pb_worker.CalculateOperationRequest{
		OperationId:     "op123",
		OperationSymbol: "+",
		OperandA:        10,
		OperandB:        5,
	}
	expectedResult := 15.0

	mockCalcService.On("Calculate",
		mock.Anything, // <--- ИЗМЕНЕНО: Используем mock.Anything для контекста
		req.OperationSymbol,
		req.OperandA,
		req.OperandB,
	).Return(expectedResult, nil).Once()

	res, err := grpcServer.CalculateOperation(context.Background(), req)

	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, req.OperationId, res.OperationId)
	assert.Equal(t, expectedResult, res.Result)
	assert.Empty(t, res.ErrorMessage)
	mockCalcService.AssertExpectations(t)
}

func TestWorkerServer_CalculateOperation_ServiceError_DivisionByZero(t *testing.T) {
	logger := zap.NewNop()
	mockCalcService := mocks.NewCalculatorServiceMock(t)
	grpcServer := NewWorkerServer(logger, mockCalcService)

	req := &pb_worker.CalculateOperationRequest{
		OperationId:     "op456",
		OperationSymbol: "/",
		OperandA:        10,
		OperandB:        0,
	}
	serviceErr := service.ErrDivisionByZero

	mockCalcService.On("Calculate",
		mock.Anything, // <--- ИЗМЕНЕНО
		req.OperationSymbol,
		req.OperandA,
		req.OperandB,
	).Return(0.0, serviceErr).Once()

	res, err := grpcServer.CalculateOperation(context.Background(), req)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), service.ErrDivisionByZero.Error())
	require.NotNil(t, res)
	assert.Equal(t, req.OperationId, res.OperationId)
	assert.Equal(t, serviceErr.Error(), res.ErrorMessage)
	assert.Equal(t, 0.0, res.Result)
	mockCalcService.AssertExpectations(t)
}

func TestWorkerServer_CalculateOperation_ServiceError_UnknownOperator(t *testing.T) {
	logger := zap.NewNop()
	mockCalcService := mocks.NewCalculatorServiceMock(t)
	grpcServer := NewWorkerServer(logger, mockCalcService)

	req := &pb_worker.CalculateOperationRequest{
		OperationId:     "op789",
		OperationSymbol: "%",
		OperandA:        10,
		OperandB:        2,
	}
	serviceErr := fmt.Errorf("%w: %s", service.ErrUnknownOperator, "%")

	mockCalcService.On("Calculate",
		mock.Anything, // <--- ИЗМЕНЕНО
		req.OperationSymbol,
		req.OperandA,
		req.OperandB,
	).Return(0.0, serviceErr).Once()

	res, err := grpcServer.CalculateOperation(context.Background(), req)

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.InvalidArgument, st.Code())
	assert.Contains(t, st.Message(), serviceErr.Error())
	require.NotNil(t, res)
	assert.Equal(t, req.OperationId, res.OperationId)
	assert.Equal(t, serviceErr.Error(), res.ErrorMessage)
	mockCalcService.AssertExpectations(t)
}


func TestWorkerServer_CalculateOperation_ServiceError_ContextCancelled(t *testing.T) {
	logger := zap.NewNop()
	mockCalcService := mocks.NewCalculatorServiceMock(t)
	grpcServer := NewWorkerServer(logger, mockCalcService)

	req := &pb_worker.CalculateOperationRequest{
		OperationId:     "op_cancel",
		OperationSymbol: "+",
		OperandA:        1,
		OperandB:        1,
	}
	serviceErr := fmt.Errorf("вычисление отменено: %w", context.DeadlineExceeded)

	mockCalcService.On("Calculate",
		mock.Anything, // <--- ИЗМЕНЕНО
		req.OperationSymbol,
		req.OperandA,
		req.OperandB,
	).Return(0.0, serviceErr).Once()

	res, err := grpcServer.CalculateOperation(context.Background(), req) // Используем context.Background() здесь для простоты

	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.DeadlineExceeded, st.Code())
	assert.Contains(t, st.Message(), serviceErr.Error())
	require.NotNil(t, res)
	assert.Equal(t, req.OperationId, res.OperationId)
	assert.Equal(t, serviceErr.Error(), res.ErrorMessage)
	mockCalcService.AssertExpectations(t)
}

func TestWorkerServer_CalculateOperation_InvalidRequest_EmptyID(t *testing.T) {
    logger := zap.NewNop()
	mockCalcService := mocks.NewCalculatorServiceMock(t)
	grpcServer := NewWorkerServer(logger, mockCalcService)

    req := &pb_worker.CalculateOperationRequest{
		OperationId:     "",
		OperationSymbol: "+",
		OperandA:        10,
		OperandB:        5,
	}

    _, err := grpcServer.CalculateOperation(context.Background(), req)
    require.Error(t, err)
    st, ok := status.FromError(err)
    require.True(t, ok)
    assert.Equal(t, codes.InvalidArgument, st.Code())
    assert.Contains(t, st.Message(), "operation_id и operation_symbol не могут быть пустыми")
    mockCalcService.AssertNotCalled(t, "Calculate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}

func TestWorkerServer_CalculateOperation_InvalidRequest_EmptySymbol(t *testing.T) {
    logger := zap.NewNop()
	mockCalcService := mocks.NewCalculatorServiceMock(t)
	grpcServer := NewWorkerServer(logger, mockCalcService)

    req := &pb_worker.CalculateOperationRequest{
		OperationId:     "op123",
		OperationSymbol: "",
		OperandA:        10,
		OperandB:        5,
	}

    _, err := grpcServer.CalculateOperation(context.Background(), req)
    require.Error(t, err)
    st, ok := status.FromError(err)
    require.True(t, ok)
    assert.Equal(t, codes.InvalidArgument, st.Code())
    assert.Contains(t, st.Message(), "operation_id и operation_symbol не могут быть пустыми")
    mockCalcService.AssertNotCalled(t, "Calculate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
}