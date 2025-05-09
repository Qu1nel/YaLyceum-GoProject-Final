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

// WorkerServer реализует gRPC сервис для Воркера.
type WorkerServer struct {
	pb.UnimplementedWorkerServiceServer // Для обратной совместимости
	log                                 *zap.Logger
	calcService                         service.Calculator
}

// NewWorkerServer создает новый экземпляр WorkerServer.
func NewWorkerServer(log *zap.Logger, calcService service.Calculator) *WorkerServer {
	return &WorkerServer{
		log:         log,
		calcService: calcService,
	}
}

// CalculateOperation обрабатывает gRPC запрос на вычисление операции.
func (s *WorkerServer) CalculateOperation(ctx context.Context, req *pb.CalculateOperationRequest) (*pb.CalculateOperationResponse, error) {
	// Логируем основные параметры запроса для отладки и мониторинга.
	s.log.Debug("WorkerServer: получен запрос CalculateOperation",
		zap.String("operationID", req.GetOperationId()),
		zap.String("symbol", req.GetOperationSymbol()),
		zap.Float64("operandA", req.GetOperandA()),
		zap.Float64("operandB", req.GetOperandB()),
		// TODO: Логировать req.GetArguments() когда они будут использоваться
	)

	if req.GetOperationId() == "" || req.GetOperationSymbol() == "" {
		s.log.Warn("WorkerServer: невалидный запрос - пустой ID или символ операции")
		return nil, status.Error(codes.InvalidArgument, "operation_id и operation_symbol обязательны")
	}

	// Передаем управление сервисному слою для выполнения бизнес-логики.
	result, serviceErr := s.calcService.Calculate(ctx, req.GetOperationSymbol(), req.GetOperandA(), req.GetOperandB() /*, req.GetArguments()... */)
	// TODO: передать req.GetArguments() когда они будут использоваться в CalculatorService

	response := &pb.CalculateOperationResponse{OperationId: req.GetOperationId()}

	if serviceErr != nil {
		s.log.Warn("WorkerServer: сервис вычислений вернул ошибку",
			zap.String("operationID", req.GetOperationId()),
			zap.Error(serviceErr),
		)
		response.ErrorMessage = serviceErr.Error() // Записываем сообщение об ошибке для клиента

		// Маппим ошибки сервиса на gRPC статусы
		if errors.Is(serviceErr, service.ErrDivisionByZero) ||
			errors.Is(serviceErr, service.ErrUnknownOperator) /* || errors.Is(serviceErr, service.ErrInvalidArgumentsForFunc) */ {
			// TODO: добавить обработку других специфичных ошибок, например, для функций
			return response, status.Error(codes.InvalidArgument, response.ErrorMessage)
		}
		if errors.Is(serviceErr, context.Canceled) || errors.Is(serviceErr, context.DeadlineExceeded) {
			return response, status.Error(codes.DeadlineExceeded, "операция отменена или превышен таймаут")
		}
		// Для всех остальных (неожиданных) ошибок сервиса возвращаем Internal.
		return response, status.Error(codes.Internal, "внутренняя ошибка сервера при вычислении")
	}

	response.Result = result
	s.log.Info("WorkerServer: операция успешно вычислена",
		zap.String("operationID", req.GetOperationId()),
		zap.Float64("result", result),
	)
	return response, nil
}
