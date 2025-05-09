package grpc_handler_test // Используем _test суффикс для тестового пакета

import (
	"context"
	"errors" // Для errors.Is в тестах
	"fmt"
	"testing"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/grpc_handler" // Пакет под тестом
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/service"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/service/mocks" // Моки
	pb_worker "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// newTestServer создает сервер с моком для тестов.
func newTestServer(t *testing.T) (*grpc_handler.WorkerServer, *mocks.CalculatorServiceMock) {
	t.Helper()
	logger := zap.NewNop() // Не выводим логи в тестах
	mockCalcService := mocks.NewCalculatorServiceMock(t)
	grpcServer := grpc_handler.NewWorkerServer(logger, mockCalcService)
	return grpcServer, mockCalcService
}

func TestWorkerServer_CalculateOperation_Success(t *testing.T) {
	grpcServer, mockCalcService := newTestServer(t)
	req := &pb_worker.CalculateOperationRequest{
		OperationId: "op123", OperationSymbol: "+", OperandA: 10, OperandB: 5,
	}
	expectedResult := 15.0

	// Ожидаем вызов Calculate с любым контекстом и заданными аргументами.
	mockCalcService.On("Calculate", mock.Anything, req.OperationSymbol, req.OperandA, req.OperandB).Return(expectedResult, nil).Once()

	res, err := grpcServer.CalculateOperation(context.Background(), req)

	require.NoError(t, err, "Ошибки быть не должно")
	require.NotNil(t, res, "Ответ не должен быть nil")
	assert.Equal(t, req.OperationId, res.OperationId, "ID операции в ответе")
	assert.Equal(t, expectedResult, res.Result, "Результат вычисления")
	assert.Empty(t, res.ErrorMessage, "Сообщение об ошибке должно быть пустым")
	mockCalcService.AssertExpectations(t) // Проверяем, что мок был вызван как ожидалось
}

func TestWorkerServer_CalculateOperation_ServiceError(t *testing.T) {
	testCases := []struct {
		name             string
		symbol           string
		a, b             float64
		serviceErr       error
		expectedGRPCCode codes.Code
		expectedMsgPart  string
	}{
		{"Деление на ноль", "/", 10, 0, service.ErrDivisionByZero, codes.InvalidArgument, service.ErrDivisionByZero.Error()},
		{"Неизвестный оператор", "%", 10, 2, fmt.Errorf("%w: %%", service.ErrUnknownOperator), codes.InvalidArgument, service.ErrUnknownOperator.Error()},
		{"Отмена контекста", "+", 1, 1, context.DeadlineExceeded, codes.DeadlineExceeded, "операция отменена или превышен таймаут"},
		{"Другая ошибка сервиса", "*", 2, 2, errors.New("неожиданная ошибка"), codes.Internal, "внутренняя ошибка сервера"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			grpcServer, mockCalcService := newTestServer(t)
			req := &pb_worker.CalculateOperationRequest{
				OperationId: "op_err", OperationSymbol: tc.symbol, OperandA: tc.a, OperandB: tc.b,
			}

			mockCalcService.On("Calculate", mock.Anything, req.OperationSymbol, req.OperandA, req.OperandB).
				Return(0.0, tc.serviceErr).Once()

			res, err := grpcServer.CalculateOperation(context.Background(), req)

			require.Error(t, err, "Должна быть ошибка gRPC")
			st, ok := status.FromError(err)
			require.True(t, ok, "Ошибка должна быть типа status.Status")
			assert.Equal(t, tc.expectedGRPCCode, st.Code(), "Неверный gRPC код ошибки")
			assert.Contains(t, st.Message(), tc.expectedMsgPart, "Сообщение об ошибке gRPC")

			require.NotNil(t, res, "Тело ответа (с ошибкой) не должно быть nil")
			assert.Equal(t, req.OperationId, res.OperationId, "ID операции в ответе")
			assert.Equal(t, tc.serviceErr.Error(), res.ErrorMessage, "ErrorMessage в теле ответа")
			assert.Zero(t, res.Result, "Результат при ошибке должен быть 0")
			mockCalcService.AssertExpectations(t)
		})
	}
}

func TestWorkerServer_CalculateOperation_InvalidRequest(t *testing.T) {
	grpcServer, mockCalcService := newTestServer(t)
	testCases := []struct {
		name    string
		req     *pb_worker.CalculateOperationRequest
		wantErr string
	}{
		{"Пустой ID операции", &pb_worker.CalculateOperationRequest{OperationSymbol: "+"}, "operation_id и operation_symbol обязательны"},
		{"Пустой символ операции", &pb_worker.CalculateOperationRequest{OperationId: "id1"}, "operation_id и operation_symbol обязательны"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := grpcServer.CalculateOperation(context.Background(), tc.req)

			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok)
			assert.Equal(t, codes.InvalidArgument, st.Code())
			assert.Contains(t, st.Message(), tc.wantErr)
			// Убедимся, что сервис вычислений не вызывался
			mockCalcService.AssertNotCalled(t, "Calculate", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
		})
	}
}
