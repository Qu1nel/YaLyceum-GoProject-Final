package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

var (
	ErrUserNotFound       = errors.New("пользователь не найден")
	ErrLoginAlreadyExists = errors.New("пользователь с таким логином уже существует")
	ErrDatabase           = errors.New("ошибка базы данных")
)

const pgUniqueViolationCode = "23505"

type User struct {
	ID           uuid.UUID
	Login        string
	PasswordHash string
	CreatedAt    time.Time
}

type UserRepository interface {
	CreateUser(ctx context.Context, login, passwordHash string) (uuid.UUID, error)
	GetUserByLogin(ctx context.Context, login string) (*User, error)
}

type pgxUserRepository struct {
	pool *pgxpool.Pool
	log  *zap.Logger
}

func NewPgxUserRepository(pool *pgxpool.Pool, log *zap.Logger) UserRepository {
	return &pgxUserRepository{pool: pool, log: log}
}

func (r *pgxUserRepository) CreateUser(ctx context.Context, login, passwordHash string) (uuid.UUID, error) {
	query := `INSERT INTO users (login, password_hash) VALUES ($1, $2) RETURNING id`
	var userID uuid.UUID

	err := r.pool.QueryRow(ctx, query, login, passwordHash).Scan(&userID)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolationCode {

			if strings.Contains(pgErr.ConstraintName, "login") {
				r.log.Warn("Попытка создать пользователя с существующим логином (репозиторий)", zap.String("login", login))
				return uuid.Nil, ErrLoginAlreadyExists
			}
		}
		r.log.Error("Не удалось создать пользователя в БД (репозиторий)", zap.Error(err), zap.String("login", login))
		return uuid.Nil, fmt.Errorf("%w: не удалось вставить пользователя: %v", ErrDatabase, err)
	}

	r.log.Info("Пользователь успешно создан (репозиторий)", zap.String("login", login), zap.Stringer("userID", userID))
	return userID, nil
}

func (r *pgxUserRepository) GetUserByLogin(ctx context.Context, login string) (*User, error) {
	query := `SELECT id, login, password_hash, created_at FROM users WHERE login = $1`
	var user User

	err := r.pool.QueryRow(ctx, query, login).Scan(
		&user.ID,
		&user.Login,
		&user.PasswordHash,
		&user.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.Debug("Пользователь не найден по логину", zap.String("login", login))
			return nil, ErrUserNotFound
		}
		r.log.Error("Не удалось получить пользователя по логину из БД", zap.Error(err), zap.String("login", login))
		return nil, fmt.Errorf("%w: не удалось запросить пользователя: %v", ErrDatabase, err)
	}

	return &user, nil
}
