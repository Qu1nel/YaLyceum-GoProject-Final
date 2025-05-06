package grpc_handler

import (
	"context"

	"github.com/Knetic/govaluate"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"
	pb_worker "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"
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
    // ... (логирование, валидация, проверка парсером, создание задачи в БД) ...
    userID, _ := uuid.Parse(req.GetUserId()) // Ошибку парсинга уже обработали
    expression := req.GetExpression()

    // Проверка парсинга govaluate
    _, parseErr := govaluate.NewEvaluableExpression(expression)
    if parseErr != nil {
        s.log.Warn("Ошибка синтаксиса выражения (govaluate)", zap.Error(parseErr))
        return nil, status.Errorf(codes.InvalidArgument, "ошибка в синтаксисе выражения: %v", parseErr)
    }

    taskID, err := s.taskRepo.CreateTask(ctx, userID, expression)
    if err != nil {
        s.log.Error("Ошибка при создании задачи", zap.Error(err))
        return nil, status.Error(codes.Internal, "внутренняя ошибка сервера")
    }
    s.log.Info("Задача успешно создана", zap.String("taskID", taskID.String()))


    // ----- ЗАГЛУШКА: Вызов Воркера для первой операции (например, сложения) -----
    // В реальной системе это будет делаться асинхронно и на основе дерева выражения
    s.log.Info("Демонстрационный вызов Воркера для операции '+'", zap.String("taskID", taskID.String()))
    workerReq := &pb_worker.CalculateOperationRequest{
        OperationId:   uuid.NewString(), // Генерируем ID для операции
        OperationSymbol: "+",
        OperandA:      10, // Фейковые операнды
        OperandB:      5,
    }

    // Контекст для вызова воркера (можно использовать изначальный ctx или создать новый с таймаутом)
    // workerCtx, workerCancel := context.WithTimeout(context.Background(), 5*time.Second) // Пример с таймаутом
    // defer workerCancel()
    // Используем пока основной контекст запроса
    workerCtx := ctx

    workerRes, workerErr := s.workerClient.CalculateOperation(workerCtx, workerReq)
    if workerErr != nil {
         s.log.Error("Ошибка при вызове Воркера (демо)",
             zap.String("taskID", taskID.String()),
             zap.String("operationID", workerReq.OperationId),
             zap.Error(workerErr),
         )
        // TODO: Обработать ошибку Воркера (например, обновить статус задачи на failed)
    } else if workerRes.ErrorMessage != "" {
        s.log.Error("Воркер вернул ошибку (демо)",
             zap.String("taskID", taskID.String()),
             zap.String("operationID", workerReq.OperationId),
             zap.String("workerError", workerRes.ErrorMessage),
        )
        // TODO: Обработать ошибку Воркера
    } else {
        s.log.Info("Воркер успешно вернул результат (демо)",
             zap.String("taskID", taskID.String()),
             zap.String("operationID", workerReq.OperationId),
             zap.Float64("result", workerRes.Result),
        )
        // TODO: Использовать результат для дальнейших вычислений
    }
    // ----- Конец ЗАГЛУШКИ вызова Воркера -----


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