package grpc_handler

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/service"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type OrchestratorServer struct {
	pb.UnimplementedOrchestratorServiceServer
	log       *zap.Logger
	taskRepo  repository.TaskRepository
	evaluator service.Evaluator
}

func NewOrchestratorServer(
	log *zap.Logger,
	taskRepo repository.TaskRepository,
	evaluator service.Evaluator,
) *OrchestratorServer {
	return &OrchestratorServer{
		log:       log,
		taskRepo:  taskRepo,
		evaluator: evaluator,
	}
}

func (s *OrchestratorServer) SubmitExpression(ctx context.Context, req *pb.ExpressionRequest) (*pb.ExpressionResponse, error) {
	userIDStr := req.GetUserId()
	expression := req.GetExpression()

	s.log.Info("Получен gRPC запрос SubmitExpression",
		zap.String("userID", userIDStr),
		zap.String("expression", expression),
	)

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.log.Warn("Невалидный формат UserID", zap.String("userID", userIDStr), zap.Error(err))
		return nil, status.Errorf(codes.InvalidArgument, "невалидный формат userID: %v", err)
	}
	if expression == "" {
		s.log.Warn("Пустое выражение", zap.String("userID", userIDStr))
		return nil, status.Error(codes.InvalidArgument, "expression не может быть пустым")
	}

	program, compileErr := expr.Compile(expression)
	if compileErr != nil {

		s.log.Warn("Ошибка компиляции/парсинга выражения (expr)",
			zap.String("expression", expression),
			zap.Error(compileErr),
		)

		return nil, status.Errorf(codes.InvalidArgument, "ошибка в выражении: %s", compileErr.Error())
	}
	s.log.Info("Выражение успешно скомпилировано и распарсено в AST (expr)", zap.String("expression", expression))
	astRootNode := program.Node()

	taskID, err := s.taskRepo.CreateTask(ctx, userID, expression)
	if err != nil {
		s.log.Error("Ошибка при создании задачи в репозитории", zap.Error(err))

		if errors.Is(err, repository.ErrDatabase) {
			return nil, status.Error(codes.Internal, "внутренняя ошибка сервера при создании задачи")
		}

		return nil, status.Errorf(codes.Unknown, "неизвестная ошибка при создании задачи: %v", err)
	}
	s.log.Info("Задача успешно создана", zap.String("taskID", taskID.String()))

	s.log.Info("Планируется запуск асинхронного вычисления",
		zap.String("taskID", taskID.String()),
		zap.Any("ast_root_type", fmt.Sprintf("%T", astRootNode)),
	)

	go s.startEvaluation(taskID, userID, expression, astRootNode)

	return &pb.ExpressionResponse{TaskId: taskID.String()}, nil
}

func (s *OrchestratorServer) startEvaluation(taskID uuid.UUID, userID uuid.UUID, originalExpr string, rootNode ast.Node) {

	evalCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()

	s.log.Info("Запуск асинхронного вычисления задачи",
		zap.Stringer("taskID", taskID),
		zap.Stringer("userID", userID),
		zap.String("expression", originalExpr),
	)

	err := s.taskRepo.UpdateTaskStatus(evalCtx, taskID, repository.StatusProcessing)
	if err != nil {
		s.log.Error("Не удалось обновить статус задачи на processing",
			zap.Stringer("taskID", taskID),
			zap.Error(err),
		)

		_ = s.taskRepo.SetTaskError(context.Background(), taskID, fmt.Sprintf("Внутренняя ошибка: не удалось начать обработку: %v", err))
		return
	}

	s.log.Debug("Начало рекурсивного вычисления AST", zap.Stringer("taskID", taskID))
	result, evalErr := s.evaluator.Evaluate(evalCtx, rootNode)
	s.log.Debug("Рекурсивное вычисление AST завершено", zap.Stringer("taskID", taskID), zap.Float64("result_before_check", result), zap.Error(evalErr))

	if evalErr == nil {
		if math.IsInf(result, 0) || math.IsNaN(result) {
			errorMsg := "ошибка вычисления: результат является бесконечностью или не числом (возможно, деление на ноль)"
			if math.IsInf(result, 1) {
				errorMsg = "ошибка вычисления: результат +бесконечность (вероятно, деление на ноль)"
			} else if math.IsInf(result, -1) {
				errorMsg = "ошибка вычисления: результат -бесконечность"
			} else if math.IsNaN(result) {
				errorMsg = "ошибка вычисления: результат не является числом (NaN)"
			}
			s.log.Warn("Результат вычисления является Inf или NaN",
				zap.Stringer("taskID", taskID),
				zap.Float64("result", result),
			)
			evalErr = errors.New(errorMsg)
		}
	}

	if evalErr != nil {
		s.log.Warn("Ошибка вычисления выражения для задачи",
			zap.Stringer("taskID", taskID),
			zap.Error(evalErr),
		)

		dbUpdateCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dbCancel()
		if updateErr := s.taskRepo.SetTaskError(dbUpdateCtx, taskID, evalErr.Error()); updateErr != nil {
			s.log.Error("Не удалось обновить задачу с ошибкой вычисления",
				zap.Stringer("taskID", taskID),
				zap.Error(updateErr),
			)
		}
	} else {
		s.log.Info("Выражение успешно вычислено для задачи",
			zap.Stringer("taskID", taskID),
			zap.Float64("result", result),
		)
		dbUpdateCtx, dbCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dbCancel()
		if updateErr := s.taskRepo.SetTaskResult(dbUpdateCtx, taskID, result); updateErr != nil {
			s.log.Error("Не удалось обновить задачу с результатом вычисления",
				zap.Stringer("taskID", taskID),
				zap.Error(updateErr),
			)
		}
	}
	s.log.Info("Асинхронное вычисление задачи завершено", zap.Stringer("taskID", taskID))
}

func (s *OrchestratorServer) GetTaskDetails(ctx context.Context, req *pb.TaskDetailsRequest) (*pb.TaskDetailsResponse, error) {
	taskIDStr := req.GetTaskId()
	requestingUserIDStr := req.GetUserId()

	s.log.Info("Получен gRPC запрос GetTaskDetails",
		zap.String("taskID", taskIDStr),
		zap.String("requestingUserID", requestingUserIDStr),
	)

	taskID, err := uuid.Parse(taskIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "невалидный формат taskID: %v", err)
	}
	requestingUserID, err := uuid.Parse(requestingUserIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "невалидный формат userID в запросе: %v", err)
	}

	task, err := s.taskRepo.GetTaskByID(ctx, taskID)
	if err != nil {
		if errors.Is(err, repository.ErrTaskNotFound) {
			s.log.Warn("Задача не найдена для GetTaskDetails", zap.Stringer("taskID", taskID))
			return nil, status.Errorf(codes.NotFound, "задача с ID %s не найдена", taskIDStr)
		}
		s.log.Error("Ошибка получения задачи из репозитория для GetTaskDetails", zap.Stringer("taskID", taskID), zap.Error(err))
		return nil, status.Error(codes.Internal, "внутренняя ошибка сервера")
	}

	if task.UserID != requestingUserID {
		s.log.Warn("Попытка доступа к чужой задаче",
			zap.Stringer("taskID", taskID),
			zap.Stringer("taskOwnerUserID", task.UserID),
			zap.Stringer("requestingUserID", requestingUserID),
		)

		return nil, status.Errorf(codes.NotFound, "задача с ID %s не найдена (или нет прав доступа)", taskIDStr)
	}

	response := &pb.TaskDetailsResponse{
		Id:         task.ID.String(),
		Expression: task.Expression,
		Status:     task.Status,
		CreatedAt:  timestamppb.New(task.CreatedAt).AsTime().Format(time.RFC3339Nano),
		UpdatedAt:  timestamppb.New(task.UpdatedAt).AsTime().Format(time.RFC3339Nano),
	}
	if task.Result != nil {
		response.Result = *task.Result
	}
	if task.ErrorMessage != nil {
		response.ErrorMessage = *task.ErrorMessage
	}

	return response, nil
}

func (s *OrchestratorServer) ListUserTasks(ctx context.Context, req *pb.UserTasksRequest) (*pb.UserTasksResponse, error) {
	userIDStr := req.GetUserId()
	s.log.Info("Получен gRPC запрос ListUserTasks", zap.String("userID", userIDStr))

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "невалидный формат userID: %v", err)
	}

	tasks, err := s.taskRepo.GetTasksByUserID(ctx, userID)
	if err != nil {
		s.log.Error("Ошибка получения списка задач из репозитория для ListUserTasks", zap.Stringer("userID", userID), zap.Error(err))
		return nil, status.Error(codes.Internal, "внутренняя ошибка сервера")
	}

	pbTasks := make([]*pb.TaskBrief, 0, len(tasks))
	for _, task := range tasks {
		pbTasks = append(pbTasks, &pb.TaskBrief{
			Id:         task.ID.String(),
			Expression: task.Expression,
			Status:     task.Status,
			CreatedAt:  timestamppb.New(task.CreatedAt).AsTime().Format(time.RFC3339Nano),
		})
	}

	return &pb.UserTasksResponse{Tasks: pbTasks}, nil
}
