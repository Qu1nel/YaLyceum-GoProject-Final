package repository

import (
	"context"
	"errors"
	"regexp"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5" // Для pgxmock.ErrNoRows
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestPgxTaskRepository_CreateTask тестирует метод CreateTask
func TestPgxTaskRepository_CreateTask(t *testing.T) {
	mock, err := pgxmock.NewPool() // Возвращает PgxPoolIface, совместимый с DBPoolIface
	require.NoError(t, err, "Ошибка создания мок-пула")
	defer mock.Close()

	logger := zap.NewNop()
	repo := NewPgxTaskRepository(mock, logger) // Передаем мок

	userID := uuid.New()
	expression := "2+2"
	expectedTaskID := uuid.New()

	// Используем QueryMatcherRegexp для гибкости
	// Важно: pgxmock по умолчанию использует QueryMatcherEqual, если не указать другое.
	// Если SQL содержит переносы строк или отличается пробелами, лучше использовать QueryMatcherRegexp.
	// Для простых запросов иногда достаточно и QueryMatcherEqual.
	// regexp.QuoteMeta экранирует все специальные символы регулярных выражений в строке SQL.
	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO tasks (user_id, expression, status)
            VALUES ($1, $2, $3)
            RETURNING id`)).
		WithArgs(userID, expression, StatusPending).
		WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(expectedTaskID))

	taskID, err := repo.CreateTask(context.Background(), userID, expression)

	require.NoError(t, err, "CreateTask не должен возвращать ошибку")
	assert.Equal(t, expectedTaskID, taskID, "Возвращенный taskID не совпадает с ожидаемым")
	assert.NoError(t, mock.ExpectationsWereMet(), "Не все ожидания мок-пула были выполнены")
}

func TestPgxTaskRepository_CreateTask_DBError(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	logger := zap.NewNop()
	repo := NewPgxTaskRepository(mock, logger)

	userID := uuid.New()
	expression := "3*3"
	dbError := errors.New("какая-то ошибка бд")

	mock.ExpectQuery(regexp.QuoteMeta(`INSERT INTO tasks (user_id, expression, status)
            VALUES ($1, $2, $3)
            RETURNING id`)).
		WithArgs(userID, expression, StatusPending).
		WillReturnError(dbError)

	taskID, err := repo.CreateTask(context.Background(), userID, expression)

	require.Error(t, err, "CreateTask должен вернуть ошибку")
	assert.True(t, errors.Is(err, ErrDatabase), "Ошибка должна быть обернута в ErrDatabase")
	assert.Contains(t, err.Error(), dbError.Error(), "Сообщение об ошибке должно содержать оригинальную ошибку БД")
	assert.Equal(t, uuid.Nil, taskID, "TaskID должен быть Nil при ошибке")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPgxTaskRepository_GetTaskByID(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()
    repo := NewPgxTaskRepository(mock, zap.NewNop())

    taskID := uuid.New()
    userID := uuid.New()
    // Truncate для избежания проблем с точностью наносекунд при сравнении
    now := time.Now().Truncate(time.Microsecond)
    expectedTask := &Task{
        ID:           taskID,
        UserID:       userID,
        Expression:   "10-5",
        Status:       StatusCompleted,
        Result:       floatPtr(5.0),
        ErrorMessage: nil,
        CreatedAt:    now.Add(-time.Hour),
        UpdatedAt:    now,
    }

    rows := pgxmock.NewRows([]string{"id", "user_id", "expression", "status", "result", "error_message", "created_at", "updated_at"}).
        AddRow(expectedTask.ID, expectedTask.UserID, expectedTask.Expression, expectedTask.Status,
               expectedTask.Result, expectedTask.ErrorMessage, expectedTask.CreatedAt, expectedTask.UpdatedAt)

    mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, expression, status, result, error_message, created_at, updated_at
        FROM tasks
        WHERE id = $1`)).
        WithArgs(taskID).
        WillReturnRows(rows)

    task, err := repo.GetTaskByID(context.Background(), taskID)
    require.NoError(t, err)
    require.NotNil(t, task)
    assert.Equal(t, expectedTask.ID, task.ID)
    assert.Equal(t, expectedTask.UserID, task.UserID)
    assert.Equal(t, expectedTask.Expression, task.Expression)
    assert.Equal(t, expectedTask.Status, task.Status)
    assert.EqualValues(t, expectedTask.Result, task.Result)
    assert.EqualValues(t, expectedTask.ErrorMessage, task.ErrorMessage)
    // Используем WithinDuration для более надежного сравнения времени
    assert.WithinDuration(t, expectedTask.CreatedAt, task.CreatedAt, time.Second, "CreatedAt не совпадает")
    assert.WithinDuration(t, expectedTask.UpdatedAt, task.UpdatedAt, time.Second, "UpdatedAt не совпадает")

    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPgxTaskRepository_GetTaskByID_NotFound(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()
    repo := NewPgxTaskRepository(mock, zap.NewNop())
    taskID := uuid.New()

    mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, expression, status, result, error_message, created_at, updated_at
        FROM tasks
        WHERE id = $1`)).
        WithArgs(taskID).
        WillReturnError(pgx.ErrNoRows) // pgxmock.ErrNoRows это алиас на pgx.ErrNoRows

    task, err := repo.GetTaskByID(context.Background(), taskID)
    require.Error(t, err)
    assert.ErrorIs(t, err, ErrTaskNotFound)
    assert.Nil(t, task)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPgxTaskRepository_GetTasksByUserID(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()
    repo := NewPgxTaskRepository(mock, zap.NewNop())

    userID := uuid.New()
    // Truncate время для консистентности
    ts1 := time.Now().Add(-2*time.Hour).Truncate(time.Microsecond)
    ts2 := time.Now().Add(-time.Hour).Truncate(time.Microsecond)

    expectedTasks := []Task{
        {ID: uuid.New(), UserID: userID, Expression: "1+1", Status: StatusCompleted, Result: floatPtr(2.0), CreatedAt: ts1, UpdatedAt: ts1},
        {ID: uuid.New(), UserID: userID, Expression: "2*2", Status: StatusProcessing, CreatedAt: ts2, UpdatedAt: ts2},
    }

    rows := pgxmock.NewRows([]string{"id", "user_id", "expression", "status", "result", "error_message", "created_at", "updated_at"})
    for _, taskData := range expectedTasks {
        rows.AddRow(taskData.ID, taskData.UserID, taskData.Expression, taskData.Status, taskData.Result, taskData.ErrorMessage, taskData.CreatedAt, taskData.UpdatedAt)
    }

    mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, expression, status, result, error_message, created_at, updated_at
        FROM tasks
        WHERE user_id = $1
        ORDER BY created_at DESC`)).
        WithArgs(userID).
        WillReturnRows(rows)

    tasks, err := repo.GetTasksByUserID(context.Background(), userID)
    require.NoError(t, err)
    require.Len(t, tasks, len(expectedTasks))
    for i := range tasks {
        assert.Equal(t, expectedTasks[i].ID, tasks[i].ID)
        assert.Equal(t, expectedTasks[i].Expression, tasks[i].Expression)
        // Можно добавить больше проверок полей, если нужно
    }
    assert.NoError(t, mock.ExpectationsWereMet())
}


func TestPgxTaskRepository_UpdateTaskStatus(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()
    repo := NewPgxTaskRepository(mock, zap.NewNop())
    taskID := uuid.New()
    newStatus := StatusProcessing

    mock.ExpectExec(regexp.QuoteMeta(
        `UPDATE tasks SET status = $1, updated_at = NOW() WHERE id = $2`)).
        WithArgs(newStatus, taskID).
        WillReturnResult(pgxmock.NewResult("UPDATE", 1))

    err := repo.UpdateTaskStatus(context.Background(), taskID, newStatus)
    require.NoError(t, err)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPgxTaskRepository_UpdateTaskStatus_NotFound(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()
    repo := NewPgxTaskRepository(mock, zap.NewNop())
    taskID := uuid.New()
    newStatus := StatusProcessing

    mock.ExpectExec(regexp.QuoteMeta(
        `UPDATE tasks SET status = $1, updated_at = NOW() WHERE id = $2`)).
        WithArgs(newStatus, taskID).
        WillReturnResult(pgxmock.NewResult("UPDATE", 0)) // 0 rows affected

    err := repo.UpdateTaskStatus(context.Background(), taskID, newStatus)
    require.Error(t, err)
    assert.ErrorIs(t, err, ErrTaskNotFound)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func TestPgxTaskRepository_SetTaskResult(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()
    repo := NewPgxTaskRepository(mock, zap.NewNop())
    taskID := uuid.New()
    resultVal := 42.0

    mock.ExpectExec(regexp.QuoteMeta(
        `UPDATE tasks SET status = $1, result = $2, error_message = NULL, updated_at = NOW() WHERE id = $3`)).
        WithArgs(StatusCompleted, resultVal, taskID).
        WillReturnResult(pgxmock.NewResult("UPDATE", 1))

    err := repo.SetTaskResult(context.Background(), taskID, resultVal)
    require.NoError(t, err)
    assert.NoError(t, mock.ExpectationsWereMet())
}


func TestPgxTaskRepository_SetTaskError(t *testing.T) {
    mock, _ := pgxmock.NewPool()
    defer mock.Close()
    repo := NewPgxTaskRepository(mock, zap.NewNop())
    taskID := uuid.New()
    errMsg := "division by zero"

    mock.ExpectExec(regexp.QuoteMeta(
        `UPDATE tasks SET status = $1, error_message = $2, result = NULL, updated_at = NOW() WHERE id = $3`)).
        WithArgs(StatusFailed, errMsg, taskID).
        WillReturnResult(pgxmock.NewResult("UPDATE", 1))

    err := repo.SetTaskError(context.Background(), taskID, errMsg)
    require.NoError(t, err)
    assert.NoError(t, mock.ExpectationsWereMet())
}

func floatPtr(f float64) *float64 {
	return &f
}