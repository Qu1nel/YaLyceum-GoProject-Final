package grpc_handler

import (
	"context"
	"errors"
	"fmt"
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
)

type OrchestratorServer struct {
	pb.UnimplementedOrchestratorServiceServer
	log         *zap.Logger
	taskRepo    repository.TaskRepository
    evaluator   *service.ExpressionEvaluator
}


func NewOrchestratorServer(
    log *zap.Logger,
    taskRepo repository.TaskRepository,
    evaluator *service.ExpressionEvaluator,
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
	astRootNode := program.Node() // Получаем корневой узел AST

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

    // === АСИНХРОННЫЙ ЗАПУСК ВЫЧИСЛЕНИЯ ===
    go s.startEvaluation(taskID, userID, expression, astRootNode)

	// === Шаг 4: Возвращаем ID задачи Агенту ===
	return &pb.ExpressionResponse{TaskId: taskID.String()}, nil
}

func (s *OrchestratorServer) startEvaluation(taskID uuid.UUID, userID uuid.UUID, originalExpr string, rootNode ast.Node) {
    // Создаем новый контекст для этой задачи, независимый от gRPC запроса, но с таймаутом
    // Таймаут на все вычисление задачи (например, 1 минута)
    evalCtx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
    defer cancel()

    s.log.Info("Запуск асинхронного вычисления задачи",
        zap.Stringer("taskID", taskID),
        zap.Stringer("userID", userID),
        zap.String("expression", originalExpr),
    )

    // 1. Обновить статус задачи на "processing"
    err := s.taskRepo.UpdateTaskStatus(evalCtx, taskID, repository.StatusProcessing)
    if err != nil {
        s.log.Error("Не удалось обновить статус задачи на processing",
            zap.Stringer("taskID", taskID),
            zap.Error(err),
        )
        // Если не удалось даже обновить статус, дальнейшее вычисление бессмысленно,
        // но ошибку уже записать некуда, кроме логов.
        // Можно попытаться записать ошибку в задачу, но это может снова не удастся.
        _ = s.taskRepo.SetTaskError(context.Background(), taskID, fmt.Sprintf("Внутренняя ошибка: не удалось начать обработку: %v", err))
        return
    }

    // 2. Вызвать рекурсивный вычислитель
    s.log.Debug("Начало рекурсивного вычисления AST", zap.Stringer("taskID", taskID))
    result, evalErr := s.evaluator.Evaluate(evalCtx, rootNode)
    s.log.Debug("Рекурсивное вычисление AST завершено", zap.Stringer("taskID", taskID), zap.Error(evalErr))


    // 3. Обработать результат
    if evalErr != nil {
        s.log.Warn("Ошибка вычисления выражения для задачи",
            zap.Stringer("taskID", taskID),
            zap.Error(evalErr),
        )
        // Обновить статус на "failed", записать ошибку
        // Используем новый фоновый контекст для обновления БД, т.к. evalCtx мог истечь
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
        // Обновить статус на "completed", записать результат
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