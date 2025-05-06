package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Определим статусы задач как константы для надежности
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

// Task модель для репозитория Оркестратора.
type Task struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Expression   string
	Status       string
	Result       *float64 // Используем указатель, чтобы различать 0 и NULL
	ErrorMessage *string  // Используем указатель для NULL
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Определим возможные ошибки репозитория задач
var (
	ErrTaskNotFound = errors.New("задача не найдена")
	// ErrDatabase используется из репозитория пользователей Agent, но можно определить и здесь
	ErrDatabase = errors.New("ошибка базы данных")
)

// TaskRepository определяет интерфейс для работы с данными задач.
type TaskRepository interface {
	// CreateTask создает новую задачу в БД и возвращает ее ID.
	CreateTask(ctx context.Context, userID uuid.UUID, expression string) (uuid.UUID, error)
	// GetTaskByID получает задачу по ее ID.
	GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
    // GetTasksByUserID получает список задач для пользователя (позже с пагинацией).
    GetTasksByUserID(ctx context.Context, userID uuid.UUID) ([]Task, error)
	// UpdateTaskStatus обновляет статус задачи.
	UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error
	// SetTaskResult обновляет результат и статус задачи на 'completed'.
	SetTaskResult(ctx context.Context, taskID uuid.UUID, result float64) error
	// SetTaskError обновляет сообщение об ошибке и статус задачи на 'failed'.
	SetTaskError(ctx context.Context, taskID uuid.UUID, errorMessage string) error
    // TODO: Добавить методы для выбора задач на обработку (например, со статусом 'pending')
}

// pgxTaskRepository реализует TaskRepository с использованием pgx.
type pgxTaskRepository struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

// NewPgxTaskRepository создает новый экземпляр pgxTaskRepository.
func NewPgxTaskRepository(pool *pgxpool.Pool, log *zap.Logger) TaskRepository {
	return &pgxTaskRepository{pool: pool, log: log}
}

// CreateTask создает новую задачу со статусом 'pending'.
func (r *pgxTaskRepository) CreateTask(ctx context.Context, userID uuid.UUID, expression string) (uuid.UUID, error) {
	query := `
        INSERT INTO tasks (user_id, expression, status)
        VALUES ($1, $2, $3)
        RETURNING id
    `
	var taskID uuid.UUID
	err := r.pool.QueryRow(ctx, query, userID, expression, StatusPending).Scan(&taskID)
	if err != nil {
		// TODO: Обработать возможную ошибку нарушения FOREIGN KEY (если user_id не существует)
		r.log.Error("Не удалось создать задачу в БД",
			zap.String("userID", userID.String()),
			zap.String("expression", expression),
			zap.Error(err),
		)
		return uuid.Nil, fmt.Errorf("%w: не удалось вставить задачу: %v", ErrDatabase, err)
	}
	r.log.Info("Задача успешно создана в БД",
		zap.String("taskID", taskID.String()),
		zap.String("userID", userID.String()),
	)
	return taskID, nil
}

// GetTaskByID (Заглушка, реализуем позже для GET /tasks/{id})
func (r *pgxTaskRepository) GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error) {
    r.log.Warn("Метод GetTaskByID еще не реализован", zap.String("taskID", taskID.String()))
    // TODO: Реализовать запрос SELECT ... WHERE id = $1
    return nil, errors.New("метод GetTaskByID не реализован")
}

// GetTasksByUserID (Заглушка, реализуем позже для GET /tasks)
func (r *pgxTaskRepository) GetTasksByUserID(ctx context.Context, userID uuid.UUID) ([]Task, error) {
    r.log.Warn("Метод GetTasksByUserID еще не реализован", zap.String("userID", userID.String()))
    // TODO: Реализовать запрос SELECT ... WHERE user_id = $1 ORDER BY created_at DESC
    return nil, errors.New("метод GetTasksByUserID не реализован")
}

// UpdateTaskStatus (Заглушка)
func (r *pgxTaskRepository) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
    r.log.Warn("Метод UpdateTaskStatus еще не реализован", zap.String("taskID", taskID.String()), zap.String("status", status))
    // TODO: Реализовать UPDATE tasks SET status = $2, updated_at = NOW() WHERE id = $1
    return errors.New("метод UpdateTaskStatus не реализован")
}

// SetTaskResult (Заглушка)
func (r *pgxTaskRepository) SetTaskResult(ctx context.Context, taskID uuid.UUID, result float64) error {
    r.log.Warn("Метод SetTaskResult еще не реализован", zap.String("taskID", taskID.String()), zap.Float64("result", result))
    // TODO: Реализовать UPDATE tasks SET status = $2, result = $3, error_message = NULL, updated_at = NOW() WHERE id = $1
    return errors.New("метод SetTaskResult не реализован")
}

// SetTaskError (Заглушка)
func (r *pgxTaskRepository) SetTaskError(ctx context.Context, taskID uuid.UUID, errorMessage string) error {
    r.log.Warn("Метод SetTaskError еще не реализован", zap.String("taskID", taskID.String()), zap.String("error", errorMessage))
    // TODO: Реализовать UPDATE tasks SET status = $2, error_message = $3, result = NULL, updated_at = NOW() WHERE id = $1
    return errors.New("метод SetTaskError не реализован")
}