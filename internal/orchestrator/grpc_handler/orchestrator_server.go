package grpc_handler

import (
	"context"
	"fmt"

	"github.com/Knetic/govaluate"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// OrchestratorServer реализует интерфейс pb.OrchestratorServiceServer.
type OrchestratorServer struct {
	// Важно: Встраиваем Unimplemented для обратной совместимости!
	pb.UnimplementedOrchestratorServiceServer
	log *zap.Logger
	taskRepo repository.TaskRepository
	// Здесь будут зависимости, например, от сервиса задач
	// taskService service.TaskService
}

// NewOrchestratorServer создает новый экземпляр gRPC сервера Оркестратора.
func NewOrchestratorServer(log *zap.Logger, taskRepo repository.TaskRepository) *OrchestratorServer {
	return &OrchestratorServer{
		log:      log,
		taskRepo: taskRepo,
	}
}

func (s *OrchestratorServer) SubmitExpression(ctx context.Context, req *pb.ExpressionRequest) (*pb.ExpressionResponse, error) {
	userIDStr := req.GetUserId()
	expression := req.GetExpression()

	s.log.Info("Получен gRPC запрос SubmitExpression",
		zap.String("userID", userIDStr),
		zap.String("expression", expression),
	)

	// Валидация
	userID, err := uuid.Parse(userIDStr)
    if err != nil { /* ... обработка ошибки ... */ return nil, status.Errorf(codes.InvalidArgument, "невалидный формат userID: %v", err) }
    if expression == "" { /* ... обработка ошибки ... */ return nil, status.Error(codes.InvalidArgument, "expression не может быть пустым") }

    // ПРОВЕРКА ВЫРАЖЕНИЯ С GOVALUATE (перед сохранением)
    // Создаем объект выражения. NewEvaluableExpression пытается распарсить его.
    _, parseErr := govaluate.NewEvaluableExpression(expression)
    if parseErr != nil {
         // Если выражение некорректно с точки зрения govaluate,
         // мы можем сразу вернуть ошибку клиенту (Агенту).
         // Это отличается от предыдущего плана, где мы сохраняли задачу и потом парсили.
         // Такой подход кажется более логичным - не создавать задачу для заведомо невалидного выражения.
         s.log.Warn("Ошибка синтаксиса выражения (govaluate)",
             zap.String("expression", expression),
             zap.Error(parseErr),
         )
         // Возвращаем ошибку InvalidArgument клиенту
         return nil, status.Errorf(codes.InvalidArgument, "ошибка в синтаксисе выражения: %v", parseErr)
    }
    s.log.Info("Синтаксис выражения корректен (govaluate)", zap.String("expression", expression))

	// Создаем задачу в БД ТОЛЬКО ЕСЛИ выражение синтаксически верно
	taskID, err := s.taskRepo.CreateTask(ctx, userID, expression)
	if err != nil {
        s.log.Error("Ошибка при создании задачи в репозитории", zap.Error(err), zap.String("userID", userIDStr))
        return nil, status.Error(codes.Internal, "внутренняя ошибка сервера при создании задачи")
	}
	s.log.Info("Задача успешно создана и сохранена в БД", zap.String("task_id", taskID.String()))


    // TODO: Запустить АСИНХРОННУЮ обработку/вычисление этой задачи.
    // На данный момент вычисление ниже СИНХРОННОЕ и только для ДЕМОНСТРАЦИИ.
    // В реальной системе это должно уйти в отдельную горутину/сервис/воркер.

    // ----- Демонстрация вычисления (УБРАТЬ ПОЗЖЕ) -----
    evalExpression, _ := govaluate.NewEvaluableExpression(expression) // Ошибку уже проверяли
    result, evalErr := evalExpression.Evaluate(nil) // Evaluate без параметров
    if evalErr != nil {
        s.log.Error("Ошибка вычисления выражения (govaluate demo)",
            zap.String("task_id", taskID.String()),
            zap.Error(evalErr),
        )
        // TODO: Вызвать s.taskRepo.SetTaskError(...)
    } else {
        // govaluate может вернуть разные типы, приводим к float64, если возможно
        resultFloat, ok := result.(float64)
        if !ok {
             s.log.Error("Результат вычисления не является float64 (govaluate demo)",
                 zap.String("task_id", taskID.String()),
                 zap.Any("result_type", fmt.Sprintf("%T", result)),
                 zap.Any("result_value", result),
             )
             // TODO: Вызвать s.taskRepo.SetTaskError(...) с сообщением о неверном типе результата
        } else {
             s.log.Info("Выражение успешно вычислено (govaluate demo)",
                  zap.String("task_id", taskID.String()),
                  zap.Float64("result", resultFloat),
             )
            // TODO: Вызвать s.taskRepo.SetTaskResult(...)
        }
    }
    // ----- Конец Демонстрации вычисления -----


	// Возвращаем ID созданной задачи
	return &pb.ExpressionResponse{TaskId: taskID.String()}, nil
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