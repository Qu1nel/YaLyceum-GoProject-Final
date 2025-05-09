package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"go.uber.org/zap"
)

const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
)

type Task struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Expression   string
	Status       string
	Result       *float64
	ErrorMessage *string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

var (
	ErrTaskNotFound = errors.New("задача не найдена")
	ErrDatabase     = errors.New("ошибка базы данных")
)

type TaskRepository interface {
	CreateTask(ctx context.Context, userID uuid.UUID, expression string) (uuid.UUID, error)
	GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error)
	GetTasksByUserID(ctx context.Context, userID uuid.UUID) ([]Task, error)
	UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error
	SetTaskResult(ctx context.Context, taskID uuid.UUID, result float64) error
	SetTaskError(ctx context.Context, taskID uuid.UUID, errorMessage string) error
}

type pgxTaskRepository struct {
	db  DBPoolIface
	log *zap.Logger
}

func NewPgxTaskRepository(db DBPoolIface, log *zap.Logger) TaskRepository {
	return &pgxTaskRepository{db: db, log: log}
}

func (r *pgxTaskRepository) CreateTask(ctx context.Context, userID uuid.UUID, expression string) (uuid.UUID, error) {
	query := `
        INSERT INTO tasks (user_id, expression, status)
        VALUES ($1, $2, $3)
        RETURNING id
    `
	var taskID uuid.UUID
	err := r.db.QueryRow(ctx, query, userID, expression, StatusPending).Scan(&taskID)
	if err != nil {
		r.log.Error("Не удалось создать задачу в БД",
			zap.Stringer("userID", userID),
			zap.String("expression", expression),
			zap.Error(err),
		)
		return uuid.Nil, fmt.Errorf("%w: не удалось вставить задачу: %v", ErrDatabase, err)
	}
	r.log.Info("Задача успешно создана в БД",
		zap.Stringer("taskID", taskID),
		zap.Stringer("userID", userID),
	)
	return taskID, nil
}

func (r *pgxTaskRepository) GetTaskByID(ctx context.Context, taskID uuid.UUID) (*Task, error) {
	query := `
        SELECT id, user_id, expression, status, result, error_message, created_at, updated_at
        FROM tasks
        WHERE id = $1
    `
	var t Task
	err := r.db.QueryRow(ctx, query, taskID).Scan(
		&t.ID, &t.UserID, &t.Expression, &t.Status,
		&t.Result, &t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrTaskNotFound
		}
		r.log.Error("Ошибка получения задачи по ID из БД", zap.Stringer("taskID", taskID), zap.Error(err))
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	return &t, nil
}

func (r *pgxTaskRepository) GetTasksByUserID(ctx context.Context, userID uuid.UUID) ([]Task, error) {
	query := `
        SELECT id, user_id, expression, status, result, error_message, created_at, updated_at
        FROM tasks
        WHERE user_id = $1
        ORDER BY created_at DESC
    `
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		r.log.Error("Ошибка получения задач по UserID из БД", zap.Stringer("userID", userID), zap.Error(err))
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(
			&t.ID, &t.UserID, &t.Expression, &t.Status,
			&t.Result, &t.ErrorMessage, &t.CreatedAt, &t.UpdatedAt,
		); err != nil {
			r.log.Error("Ошибка сканирования строки задачи", zap.Stringer("userID", userID), zap.Error(err))
			return nil, fmt.Errorf("%w: ошибка сканирования: %v", ErrDatabase, err)
		}
		tasks = append(tasks, t)
	}

	if err = rows.Err(); err != nil {
		r.log.Error("Ошибка после итерации по строкам задач", zap.Stringer("userID", userID), zap.Error(err))
		return nil, fmt.Errorf("%w: ошибка итерации: %v", ErrDatabase, err)
	}

	return tasks, nil
}

func (r *pgxTaskRepository) UpdateTaskStatus(ctx context.Context, taskID uuid.UUID, status string) error {
	query := `UPDATE tasks SET status = $1, updated_at = NOW() WHERE id = $2`
	commandTag, err := r.db.Exec(ctx, query, status, taskID)
	if err != nil {
		r.log.Error("Ошибка обновления статуса задачи", zap.Stringer("taskID", taskID), zap.String("status", status), zap.Error(err))
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}
	r.log.Info("Статус задачи обновлен", zap.Stringer("taskID", taskID), zap.String("new_status", status))
	return nil
}

func (r *pgxTaskRepository) SetTaskResult(ctx context.Context, taskID uuid.UUID, result float64) error {
	query := `UPDATE tasks SET status = $1, result = $2, error_message = NULL, updated_at = NOW() WHERE id = $3`
	commandTag, err := r.db.Exec(ctx, query, StatusCompleted, result, taskID)
	if err != nil {
		r.log.Error("Ошибка установки результата задачи", zap.Stringer("taskID", taskID), zap.Float64("result", result), zap.Error(err))
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}
	r.log.Info("Результат задачи установлен", zap.Stringer("taskID", taskID), zap.Float64("result", result))
	return nil
}

func (r *pgxTaskRepository) SetTaskError(ctx context.Context, taskID uuid.UUID, errorMessage string) error {
	query := `UPDATE tasks SET status = $1, error_message = $2, result = NULL, updated_at = NOW() WHERE id = $3`
	commandTag, err := r.db.Exec(ctx, query, StatusFailed, errorMessage, taskID)
	if err != nil {
		r.log.Error("Ошибка установки ошибки задачи", zap.Stringer("taskID", taskID), zap.String("errorMessage", errorMessage), zap.Error(err))
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	if commandTag.RowsAffected() == 0 {
		return ErrTaskNotFound
	}
	r.log.Info("Ошибка задачи установлена", zap.Stringer("taskID", taskID), zap.String("errorMessage", errorMessage))
	return nil
}
