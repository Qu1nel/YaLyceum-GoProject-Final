package integration

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	agent_app "github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/app"
	orchestrator_app "github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/app"
	worker_app "github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/app"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testAgentBaseURL         string
	testPostgresDSN          string
	testOrchestratorGRPCPort = "50217"
	testWorkerGRPCPort       = "50218"
	testAgentHTTPPort        = "8043"
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	log.Println("Инициализация тестового окружения...")

	pgContainer, dsn, err := setupPostgres(ctx)
	if err != nil {
		log.Fatalf("Не удалось запустить PostgreSQL контейнер: %v", err)
	}
	testPostgresDSN = dsn
	log.Printf("PostgreSQL контейнер запущен, DSN: %s", testPostgresDSN)

	if err := runMigrations(testPostgresDSN); err != nil {
		log.Fatalf("Не удалось применить миграции: %v", err)
	}
	log.Println("Миграции успешно применены.")

	setupTestEnvironmentVariables()

	go runTestService("Оркестратор", orchestrator_app.Run)
	go runTestService("Воркер", worker_app.Run)

	time.Sleep(2 * time.Second)

	go runTestService("Агент", agent_app.Run)

	time.Sleep(1 * time.Second)
	testAgentBaseURL = fmt.Sprintf("http://localhost:%s/api/v1", testAgentHTTPPort)
	log.Printf("Тестовый Агент должен быть доступен по URL: %s", testAgentBaseURL)

	exitCode := m.Run()

	log.Println("Завершение работы тестовых сервисов (отмена контекста)...")
	cancel()
	time.Sleep(2 * time.Second)

	log.Println("Остановка PostgreSQL контейнера...")
	if err := pgContainer.Terminate(context.Background()); err != nil {
		log.Printf("Не удалось остановить PostgreSQL контейнер: %v", err)
	}
	log.Println("PostgreSQL контейнер остановлен.")
	log.Println("Тестовое окружение остановлено.")
	os.Exit(exitCode)
}

func runTestService(name string, runApp func()) {
	defer func() {
		if r := recover(); r != nil {

			log.Printf("Критическая ошибка (паника) в тестовом сервисе '%s': %v", name, r)
		}
	}()
	log.Printf("Запуск тестового сервиса '%s'...", name)
	runApp()
	log.Printf("Тестовый сервис '%s' завершил работу.", name)
}

func setupTestEnvironmentVariables() {
	os.Setenv("APP_ENV", "test")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("GRACEFUL_TIMEOUT", "1s")
	os.Setenv("JWT_SECRET", "test-jwt-secret-key-must-be-at-least-32-bytes-long")
	os.Setenv("JWT_TOKEN_TTL", "5m")
	os.Setenv("POSTGRES_DSN", testPostgresDSN)
	os.Setenv("DB_POOL_MAX_CONNS", "5")

	os.Setenv("ORCHESTRATOR_GRPC_PORT", testOrchestratorGRPCPort)
	os.Setenv("WORKER_GRPC_ADDRESS", fmt.Sprintf("localhost:%s", testWorkerGRPCPort))
	os.Setenv("GRPC_CLIENT_TIMEOUT", "3s")

	os.Setenv("WORKER_GRPC_PORT", testWorkerGRPCPort)

	os.Setenv("TIME_ADDITION_MS", "10ms")
	os.Setenv("TIME_SUBTRACTION_MS", "10ms")
	os.Setenv("TIME_MULTIPLICATION_MS", "10ms")
	os.Setenv("TIME_DIVISION_MS", "10ms")
	os.Setenv("TIME_EXPONENTIATION_MS", "10ms")

	os.Setenv("AGENT_HTTP_PORT", testAgentHTTPPort)
	os.Setenv("ORCHESTRATOR_GRPC_ADDRESS", fmt.Sprintf("localhost:%s", testOrchestratorGRPCPort))
	log.Println("Переменные окружения для тестов установлены.")
}

func setupPostgres(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb_integration"),
		postgres.WithUsername("testuser_int"),
		postgres.WithPassword("testpassword_int"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(20*time.Second),
		),
	)
	if err != nil {
		return nil, "", fmt.Errorf("запуск postgres контейнера: %w", err)
	}

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {

		if termErr := pgContainer.Terminate(context.Background()); termErr != nil {
			log.Printf("Ошибка остановки pgContainer после ошибки ConnectionString: %v", termErr)
		}
		return nil, "", fmt.Errorf("получение ConnectionString: %w", err)
	}
	return pgContainer, dsn, nil
}

func runMigrations(dsn string) error {
	projectRoot, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("поиск корня проекта: %w", err)
	}

	migrationsPath := filepath.Join(projectRoot, "tests/migrations")
	migrationsURL := "file://" + filepath.ToSlash(migrationsPath)

	migrateDSN := strings.Replace(dsn, "postgresql://", "pgx5://", 1)
	if !strings.HasPrefix(migrateDSN, "pgx5://") && strings.HasPrefix(dsn, "postgres://") {
		migrateDSN = strings.Replace(dsn, "postgres://", "pgx5://", 1)
	} else if !strings.HasPrefix(migrateDSN, "pgx5://") {
		parsedURL, parseErr := url.Parse(dsn)
		if parseErr != nil {
			return fmt.Errorf("ошибка парсинга DSN для миграций (%s): %w", dsn, parseErr)
		}
		parsedURL.Scheme = "pgx5"
		migrateDSN = parsedURL.String()
	}

	log.Printf("Применение миграций из: %s к БД: %s (скрыт пароль)", migrationsURL, strings.Split(migrateDSN, "@")[0])

	m, err := migrate.New(migrationsURL, migrateDSN)
	if err != nil {
		return fmt.Errorf("создание экземпляра migrate (URL: %s): %w", migrationsURL, err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {

		srcErr, dbErr := m.Close()
		errMsg := fmt.Sprintf("применение миграций (m.Up): %v", err)
		if srcErr != nil {
			errMsg += fmt.Sprintf("; ошибка закрытия источника: %v", srcErr)
		}
		if dbErr != nil {
			errMsg += fmt.Sprintf("; ошибка закрытия БД: %v", dbErr)
		}
		return errors.New(errMsg)
	}

	srcErr, dbErr := m.Close()
	if srcErr != nil {
		return fmt.Errorf("ошибка закрытия источника миграций: %w", srcErr)
	}
	if dbErr != nil {
		return fmt.Errorf("ошибка закрытия соединения БД миграций: %w", dbErr)
	}
	return nil
}

func getProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("не удалось найти go.mod")
		}
		dir = parent
	}
}
