package grpc_handler

import (
	"context"
	"errors" // Для errors.Is

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/service"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker" // Убедись, что путь правильный
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type WorkerServer struct {
	pb.UnimplementedWorkerServiceServer
	log         *zap.Logger
	calcService service.Calculator
}

func NewWorkerServer(log *zap.Logger, calcService service.Calculator) *WorkerServer {
	return &WorkerServer{
		log:         log,
		calcService: calcService,
	}
}

func (s *WorkerServer) CalculateOperation(ctx context.Context, req *pb.CalculateOperationRequest) (*pb.CalculateOperationResponse, error) {
	operationID := req.GetOperationId()
	operationSymbol := req.GetOperationSymbol()
	operandA := req.GetOperandA()
	operandB := req.GetOperandB()

	s.log.Info("WorkerServer: получен gRPC запрос CalculateOperation",
		zap.String("operationID", operationID),
		zap.String("symbol", operationSymbol),
		zap.Float64("a", operandA),
		zap.Float64("b", operandB),
	)

	if operationID == "" || operationSymbol == "" {
		s.log.Warn("WorkerServer: невалидный запрос - пустой operation_id или operation_symbol", zap.String("opID", operationID), zap.String("opSymbol", operationSymbol))
		return nil, status.Error(codes.InvalidArgument, "operation_id и operation_symbol не могут быть пустыми")
	}

	// Вызов сервиса вычислений
	result, errService := s.calcService.Calculate(ctx, operationSymbol, operandA, operandB)
	s.log.Debug("WorkerServer: результат от CalculatorService", // Отладочный лог
		zap.String("operationID", operationID),
		zap.Float64("serviceResult", result), // result здесь может быть 0, если была ошибка
		zap.Error(errService),                // errService должен содержать ошибку, если она была
	)

	response := &pb.CalculateOperationResponse{
		OperationId: operationID,
		// Result и ErrorMessage будут установлены ниже
	}

	if errService != nil { // Если CalculatorService ВЕРНУЛ ОШИБКУ
		s.log.Warn("WorkerServer: CalculatorService вернул ошибку, формируем gRPC ошибку",
			zap.String("operationID", operationID),
			zap.String("symbol", operationSymbol),
			zap.Error(errService),
		)
		response.ErrorMessage = errService.Error() // Записываем текст ошибки в ответ

		// Определяем gRPC код ошибки на основе типа ошибки от сервиса
		if errors.Is(errService, service.ErrDivisionByZero) || errors.Is(errService, service.ErrUnknownOperator) {
			return response, status.Error(codes.InvalidArgument, response.ErrorMessage)
		}
		// Проверяем, не была ли операция отменена родительским контекстом (например, таймаут из Оркестратора)
		if errors.Is(errService, context.Canceled) || errors.Is(errService, context.DeadlineExceeded) {
			return response, status.Error(codes.DeadlineExceeded, response.ErrorMessage)
		}
		// Для всех остальных ошибок сервиса возвращаем Internal
		return response, status.Error(codes.Internal, response.ErrorMessage)
	}

	// Если CalculatorService НЕ ВЕРНУЛ ОШИБКУ (errService == nil)
	response.Result = result // Записываем результат
	s.log.Info("WorkerServer: операция успешно вычислена",
		zap.String("operationID", operationID),
		zap.String("symbol", operationSymbol),
		zap.Float64("result", result),
	)
	return response, nil // Возвращаем успешный ответ
}