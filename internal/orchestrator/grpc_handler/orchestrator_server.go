package grpc_handler

import (
	"context"

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
	// Здесь будут зависимости, например, от сервиса задач
	// taskService service.TaskService
}

// NewOrchestratorServer создает новый экземпляр gRPC сервера Оркестратора.
func NewOrchestratorServer(log *zap.Logger /*, taskService service.TaskService */) *OrchestratorServer {
	return &OrchestratorServer{
		log: log,
		// taskService: taskService,
	}
}

// SubmitExpression обрабатывает gRPC запрос на отправку выражения.
// Пока что только логирует и возвращает фейковый ID.
func (s *OrchestratorServer) SubmitExpression(ctx context.Context, req *pb.ExpressionRequest) (*pb.ExpressionResponse, error) {
	userID := req.GetUserId()
	expression := req.GetExpression()

	s.log.Info("Получен gRPC запрос SubmitExpression",
		zap.String("userID", userID),
		zap.String("expression", expression),
	)

	// Простая валидация входных данных
	if userID == "" {
		s.log.Warn("Пустой UserID в запросе SubmitExpression")
		// Возвращаем gRPC ошибку с кодом InvalidArgument
		return nil, status.Error(codes.InvalidArgument, "userID не может быть пустым")
	}
	// Проверка валидности UUID (если userID должен быть UUID)
	if _, err := uuid.Parse(userID); err != nil {
         s.log.Warn("Невалидный формат UserID в запросе SubmitExpression", zap.String("userID", userID), zap.Error(err))
         return nil, status.Errorf(codes.InvalidArgument, "невалидный формат userID: %v", err)
    }

	if expression == "" {
		s.log.Warn("Пустое выражение в запросе SubmitExpression", zap.String("userID", userID))
		return nil, status.Error(codes.InvalidArgument, "expression не может быть пустым")
	}

	// TODO: Позже здесь будет вызов сервиса для создания задачи в БД и запуска вычисления
	// taskID, err := s.taskService.CreateTask(ctx, userID, expression)
	// if err != nil {
	//     s.log.Error("Ошибка при создании задачи", zap.Error(err), zap.String("userID", userID))
	//     // Преобразование ошибки сервиса в gRPC ошибку
	//     return nil, status.Error(codes.Internal, "внутренняя ошибка сервера")
	// }

	// ЗАГЛУШКА: Генерируем фейковый ID задачи
	fakeTaskID := uuid.NewString()
	s.log.Info("Задача (заглушка) принята", zap.String("assigned_task_id", fakeTaskID))

	// Возвращаем успешный ответ с ID задачи
	return &pb.ExpressionResponse{TaskId: fakeTaskID}, nil
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