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

	s.log.Debug("WorkerServer: получен запрос CalculateOperation",
		zap.String("operationID", req.GetOperationId()),
		zap.String("symbol", req.GetOperationSymbol()),
		zap.Float64("operandA", req.GetOperandA()),
		zap.Float64("operandB", req.GetOperandB()),
	)

	if req.GetOperationId() == "" || req.GetOperationSymbol() == "" {
		s.log.Warn("WorkerServer: невалидный запрос - пустой ID или символ операции")
		return nil, status.Error(codes.InvalidArgument, "operation_id и operation_symbol обязательны")
	}

	result, serviceErr := s.calcService.Calculate(ctx, req.GetOperationSymbol(), req.GetOperandA(), req.GetOperandB())

	response := &pb.CalculateOperationResponse{OperationId: req.GetOperationId()}

	if serviceErr != nil {
		s.log.Warn("WorkerServer: сервис вычислений вернул ошибку",
			zap.String("operationID", req.GetOperationId()),
			zap.Error(serviceErr),
		)
		response.ErrorMessage = serviceErr.Error()

		if errors.Is(serviceErr, service.ErrDivisionByZero) ||
			errors.Is(serviceErr, service.ErrUnknownOperator) {

			return response, status.Error(codes.InvalidArgument, response.ErrorMessage)
		}
		if errors.Is(serviceErr, context.Canceled) || errors.Is(serviceErr, context.DeadlineExceeded) {
			return response, status.Error(codes.DeadlineExceeded, "операция отменена или превышен таймаут")
		}

		return response, status.Error(codes.Internal, "внутренняя ошибка сервера при вычислении")
	}

	response.Result = result
	s.log.Info("WorkerServer: операция успешно вычислена",
		zap.String("operationID", req.GetOperationId()),
		zap.Float64("result", result),
	)
	return response, nil
}
