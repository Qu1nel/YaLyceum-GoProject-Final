package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

const (
	defaultMaxPoolSize = 10
	defaultConnTimeout = 5 * time.Second
)

func NewPool(ctx context.Context, dsn string, maxPoolSize int, logger *zap.Logger) (*pgxpool.Pool, error) {
	if maxPoolSize <= 0 {
		maxPoolSize = defaultMaxPoolSize
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("не удалось разобрать DSN для PostgreSQL: %w", err)
	}

	cfg.MaxConns = int32(maxPoolSize)
	cfg.ConnConfig.ConnectTimeout = defaultConnTimeout

	cfg.MaxConnIdleTime = 1 * time.Minute
	cfg.HealthCheckPeriod = 30 * time.Second

	logger.Info("Попытка подключения к PostgreSQL...",
		zap.String("host", cfg.ConnConfig.Host),
		zap.Uint16("port", cfg.ConnConfig.Port),
		zap.String("user", cfg.ConnConfig.User),
		zap.String("database", cfg.ConnConfig.Database),
		zap.Int32("max_pool_size", cfg.MaxConns),
	)

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("не удалось создать пул соединений PostgreSQL: %w", err)
	}

	// Делаем несколько попыток, т.к. БД может стартовать чуть дольше приложения
	const maxPingAttempts = 5
	var pingErr error
	for attempt := 1; attempt <= maxPingAttempts; attempt++ {
		pingCtx, cancel := context.WithTimeout(ctx, defaultConnTimeout)
		pingErr = pool.Ping(pingCtx)
		cancel()
		if pingErr == nil {
			break // Успешный пинг
		}
		logger.Warn("Не удалось проверить соединение с PostgreSQL",
			zap.Int("попытка", attempt),
			zap.Int("макс_попыток", maxPingAttempts),
			zap.Error(pingErr),
		)
		select {
		case <-time.After(time.Second * time.Duration(attempt)):
		case <-ctx.Done():
			pool.Close()
			return nil, fmt.Errorf("проверка соединения с PostgreSQL отменена: %w", ctx.Err())
		}
	}

	if pingErr != nil {
		pool.Close() // Закрываем пул, если пинг так и не прошел
		return nil, fmt.Errorf("не удалось подключиться к PostgreSQL после %d попыток: %w", maxPingAttempts, pingErr)
	}

	logger.Info("Успешное подключение к PostgreSQL")

	return pool, nil
}