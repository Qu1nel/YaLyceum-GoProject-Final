package grpc_handler

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"
	pb_worker "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"

	"github.com/expr-lang/expr"
	"github.com/expr-lang/expr/ast"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type OrchestratorServer struct {
	pb.UnimplementedOrchestratorServiceServer
	log         *zap.Logger
	taskRepo    repository.TaskRepository
    workerClient pb_worker.WorkerServiceClient 
}


func NewOrchestratorServer(
    log *zap.Logger,
    taskRepo repository.TaskRepository,
    workerClient pb_worker.WorkerServiceClient, // <-- Новая зависимость
) *OrchestratorServer {
	return &OrchestratorServer{
		log:          log,
		taskRepo:     taskRepo,
        workerClient: workerClient, // <-- Сохранили зависимость
	}
}
func (s *OrchestratorServer) SubmitExpression(ctx context.Context, req *pb.ExpressionRequest) (*pb.ExpressionResponse, error) {
	userIDStr := req.GetUserId()
	expression := req.GetExpression()

	s.log.Info("Получен gRPC запрос SubmitExpression",
		zap.String("userID", userIDStr),
		zap.String("expression", expression),
	)

	// Валидация userID и expression
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		s.log.Warn("Невалидный формат UserID", zap.String("userID", userIDStr), zap.Error(err))
		return nil, status.Errorf(codes.InvalidArgument, "невалидный формат userID: %v", err)
	}
	if expression == "" {
		s.log.Warn("Пустое выражение", zap.String("userID", userIDStr))
		return nil, status.Error(codes.InvalidArgument, "expression не может быть пустым")
	}

	// === Шаг 1: Парсинг/Компиляция выражения с помощью expr.Compile ===
	program, compileErr := expr.Compile(expression)
	if compileErr != nil {
		// Ошибка парсинга или компиляции (например, синтаксическая или ошибка типов)
		s.log.Warn("Ошибка компиляции/парсинга выражения (expr)",
			zap.String("expression", expression),
			zap.Error(compileErr),
		)
		// Возвращаем ошибку InvalidArgument клиенту с текстом ошибки от expr
		return nil, status.Errorf(codes.InvalidArgument, "ошибка в выражении: %s", compileErr.Error())
	}
	s.log.Info("Выражение успешно скомпилировано и распарсено в AST (expr)", zap.String("expression", expression))
	astRootNode := program.Node // Получаем корневой узел AST

	// === Шаг 2: Создание задачи в БД ===
	taskID, err := s.taskRepo.CreateTask(ctx, userID, expression)
	if err != nil {
		s.log.Error("Ошибка при создании задачи в репозитории", zap.Error(err))
		// Определяем тип ошибки базы данных для более точного ответа gRPC
		if errors.Is(err, repository.ErrDatabase) { // Используем нашу общую ошибку БД
			return nil, status.Error(codes.Internal, "внутренняя ошибка сервера при создании задачи")
		}
		// Если это другая ошибка (хотя CreateTask пока возвращает только ErrDatabase)
		return nil, status.Errorf(codes.Unknown, "неизвестная ошибка при создании задачи: %v", err)
	}
	s.log.Info("Задача успешно создана", zap.String("taskID", taskID.String()))

	// === Шаг 3: Асинхронный запуск вычисления (TBD) ===
	s.log.Info("Планируется запуск асинхронного вычисления",
		zap.String("taskID", taskID.String()),
		zap.Any("ast_root_type", fmt.Sprintf("%T", astRootNode)),
	)

	// go s.startEvaluation(taskID, astRootNode) // Запустим позже

	// === Шаг 4: Возвращаем ID задачи Агенту ===
	return &pb.ExpressionResponse{TaskId: taskID.String()}, nil
}

func (s *OrchestratorServer) startEvaluation(taskID uuid.UUID, rootNode ast.Node) {
    // Создаем новый контекст для вычисления, возможно, с таймаутом
    //evalCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute) // Пример таймаута
    _, cancel := context.WithTimeout(context.Background(), 1*time.Minute) // Пример таймаута
    defer cancel()

    s.log.Info("Запуск вычисления задачи в горутине", zap.String("taskID", taskID.String()))

    // 1. Обновить статус задачи на "processing"
    // err := s.taskRepo.UpdateTaskStatus(evalCtx, taskID, repository.StatusProcessing)
    // if err != nil { ... обработать ошибку обновления статуса ...; return }

    // 2. Вызвать рекурсивный вычислитель
    // evaluator := NewExpressionEvaluator(s.log, s.workerClient) // Создать сервис-вычислитель
    // result, evalErr := evaluator.Evaluate(evalCtx, rootNode)

    // 3. Обработать результат
    // if evalErr != nil {
    //    // Обновить статус на "failed", записать ошибку
    //    s.taskRepo.SetTaskError(evalCtx, taskID, evalErr.Error())
    // } else {
    //    // Обновить статус на "completed", записать результат
    //    s.taskRepo.SetTaskResult(evalCtx, taskID, result)
    // }
    s.log.Warn("Логика вычисления в startEvaluation еще не реализована", zap.String("taskID", taskID.String())) // Временный лог
}


// GetTaskDetails (Заглушка)
func (s *OrchestratorServer) GetTaskDetails(ctx context.Context, req *pb.TaskDetailsRequest) (*pb.TaskDetailsResponse, error) {
    s.log.Warn("Метод GetTaskDetails еще не реализован", zap.String("userID", req.GetUserId()), zap.String("taskID", req.GetTaskId()))
    return nil, status.Errorf(codes.Unimplemented, "метод GetTaskDetails не реализован")
}

// ListUserTasks (Заглушка)
func (s *OrchestratorServer) ListUserTasks(ctx context.Context, req *pb.UserTasksRequest) (*pb.UserTasksResponse, error) {
    s.log.Warn("Метод ListUserTasks еще не реализован", zap.String("userID", req.GetUserId()))
    return nil, status.Errorf(codes.Unimplemented, "метод ListUserTasks не реализован")
}