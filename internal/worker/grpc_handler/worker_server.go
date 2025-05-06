package grpc_handler

import (
	"context"
	"errors"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/service"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WorkerServer реализует интерфейс pb.WorkerServiceServer.
type WorkerServer struct {
	pb.UnimplementedWorkerServiceServer // Встраиваем для совместимости
	log          *zap.Logger
	calcService  *service.CalculatorService // Зависимость от сервиса вычислений
}

// NewWorkerServer создает новый gRPC сервер Воркера.
func NewWorkerServer(log *zap.Logger, calcService *service.CalculatorService) *WorkerServer {
	return &WorkerServer{
		log:         log,
		calcService: calcService,
	}
}

// CalculateOperation обрабатывает запрос на вычисление операции.
func (s *WorkerServer) CalculateOperation(ctx context.Context, req *pb.CalculateOperationRequest) (*pb.CalculateOperationResponse, error) {
	operationID := req.GetOperationId()
	operationSymbol := req.GetOperationSymbol()
	operandA := req.GetOperandA()
	operandB := req.GetOperandB() // Может не использоваться для унарных

	s.log.Info("Получен gRPC запрос CalculateOperation",
		zap.String("operationID", operationID),
		zap.String("symbol", operationSymbol),
		zap.Float64("a", operandA),
		zap.Float64("b", operandB),
	)

	// Валидация (базовая)
	if operationID == "" || operationSymbol == "" {
		return nil, status.Error(codes.InvalidArgument, "operation_id и operation_symbol не могут быть пустыми")
	}

	// Вызов сервиса вычислений
	result, err := s.calcService.Calculate(ctx, operationSymbol, operandA, operandB)

	// Формируем ответ
	response := &pb.CalculateOperationResponse{
		OperationId: operationID,
		// Заполним поля ниже в зависимости от ошибки
	}

	if err != nil {
		s.log.Warn("Ошибка при вычислении операции",
			zap.String("operationID", operationID),
			zap.String("symbol", operationSymbol),
			zap.Error(err),
		)
		// Преобразуем ошибку сервиса в сообщение для gRPC ответа
        // и установим соответствующий gRPC код, если возможно
        response.ErrorMessage = err.Error() // Сохраняем текст ошибки
        // Можно проверять конкретные ошибки для кодов gRPC
        if errors.Is(err, service.ErrDivisionByZero) || errors.Is(err, service.ErrUnknownOperator) {
             // Деление на ноль или неизвестный оператор - ошибка в аргументах
             return response, status.Error(codes.InvalidArgument, response.ErrorMessage)
        }
        if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
            // Ошибка таймаута или отмены
            return response, status.Error(codes.DeadlineExceeded, response.ErrorMessage)
        }
        // Другие ошибки считаем внутренними
		return response, status.Error(codes.Internal, response.ErrorMessage)
	}

    // Ошибки нет, записываем результат
	response.Result = result
	s.log.Info("Операция успешно вычислена",
		zap.String("operationID", operationID),
		zap.String("symbol", operationSymbol),
		zap.Float64("result", result),
	)

	return response, nil // Возвращаем результат и nil ошибку gRPC
}