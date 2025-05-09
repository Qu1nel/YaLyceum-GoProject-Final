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
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // Драйвер для pgx/v5
	_ "github.com/golang-migrate/migrate/v4/source/file"     // Поддержка миграций из файлов
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testAgentBaseURL         string
	testPostgresDSN          string
	testOrchestratorGRPCPort = "50217" // Уникальные порты для тестовых сервисов
	testWorkerGRPCPort       = "50218"
	testAgentHTTPPort        = "8043"
)

// TestMain настраивает и запускает полное интеграционное тестовое окружение.
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

	// Запускаем сервисы в отдельных горутинах.
	// Каналы используются для сигнализации о завершении, но здесь не для синхронизации старта.
	go runTestService("Оркестратор", orchestrator_app.Run)
	go runTestService("Воркер", worker_app.Run)

	// Небольшая пауза, чтобы Оркестратор и Воркер успели инициализироваться перед Агентом.
	// В реальных системах лучше использовать health checks или другие механизмы ожидания.
	time.Sleep(2 * time.Second)

	go runTestService("Агент", agent_app.Run)

	// Еще одна пауза, чтобы Агент успел запуститься.
	time.Sleep(1 * time.Second)
	testAgentBaseURL = fmt.Sprintf("http://localhost:%s/api/v1", testAgentHTTPPort)
	log.Printf("Тестовый Агент должен быть доступен по URL: %s", testAgentBaseURL)

	exitCode := m.Run() // Запускаем тесты

	log.Println("Завершение работы тестовых сервисов (отмена контекста)...")
	cancel()              // Сигнал всем сервисам на завершение
	time.Sleep(2 * time.Second) // Даем время на корректное завершение

	log.Println("Остановка PostgreSQL контейнера...")
	if err := pgContainer.Terminate(context.Background()); err != nil {
		log.Printf("Не удалось остановить PostgreSQL контейнер: %v", err)
	}
	log.Println("PostgreSQL контейнер остановлен.")
	log.Println("Тестовое окружение остановлено.")
	os.Exit(exitCode)
}

// runTestService запускает сервис в горутине и обрабатывает паники.
func runTestService(name string, runApp func()) {
	defer func() {
		if r := recover(); r != nil {
			// Паника в сервисе не должна останавливать TestMain, но мы ее логируем.
			log.Printf("Критическая ошибка (паника) в тестовом сервисе '%s': %v", name, r)
		}
	}()
	log.Printf("Запуск тестового сервиса '%s'...", name)
	runApp()
	log.Printf("Тестовый сервис '%s' завершил работу.", name)
}

// setupTestEnvironmentVariables устанавливает переменные окружения для тестовых сервисов.
func setupTestEnvironmentVariables() {
	os.Setenv("APP_ENV", "test")
	os.Setenv("LOG_LEVEL", "debug") // Используем debug для тестов, чтобы видеть больше деталей
	os.Setenv("GRACEFUL_TIMEOUT", "1s")
	os.Setenv("JWT_SECRET", "test-jwt-secret-key-must-be-at-least-32-bytes-long")
	os.Setenv("JWT_TOKEN_TTL", "5m")
	os.Setenv("POSTGRES_DSN", testPostgresDSN)
	os.Setenv("DB_POOL_MAX_CONNS", "5")

	os.Setenv("ORCHESTRATOR_GRPC_PORT", testOrchestratorGRPCPort)
	os.Setenv("WORKER_GRPC_ADDRESS", fmt.Sprintf("localhost:%s", testWorkerGRPCPort))
	os.Setenv("GRPC_CLIENT_TIMEOUT", "3s") // Таймаут для gRPC клиентов

	os.Setenv("WORKER_GRPC_PORT", testWorkerGRPCPort)
	// Устанавливаем короткие задержки для операций в Воркере для ускорения тестов
	os.Setenv("TIME_ADDITION_MS", "10ms")
	os.Setenv("TIME_SUBTRACTION_MS", "10ms")
	os.Setenv("TIME_MULTIPLICATION_MS", "10ms")
	os.Setenv("TIME_DIVISION_MS", "10ms")
	os.Setenv("TIME_EXPONENTIATION_MS", "10ms")

	os.Setenv("AGENT_HTTP_PORT", testAgentHTTPPort)
	os.Setenv("ORCHESTRATOR_GRPC_ADDRESS", fmt.Sprintf("localhost:%s", testOrchestratorGRPCPort))
	log.Println("Переменные окружения для тестов установлены.")
}

// setupPostgres запускает Docker контейнер с PostgreSQL для тестов.
func setupPostgres(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine",
		postgres.WithDatabase("testdb_integration"),
		postgres.WithUsername("testuser_int"),
		postgres.WithPassword("testpassword_int"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2). // Ожидаем два таких сообщения для надежности
				WithStartupTimeout(20*time.Second), // Увеличен таймаут на запуск PG
		),
	)
	if err != nil {
		return nil, "", fmt.Errorf("запуск postgres контейнера: %w", err)
	}

	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		// Пытаемся остановить контейнер, если не смогли получить DSN
		if termErr := pgContainer.Terminate(context.Background()); termErr != nil {
			log.Printf("Ошибка остановки pgContainer после ошибки ConnectionString: %v", termErr)
		}
		return nil, "", fmt.Errorf("получение ConnectionString: %w", err)
	}
	return pgContainer, dsn, nil
}

// runMigrations применяет миграции базы данных.
func runMigrations(dsn string) error {
	projectRoot, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("поиск корня проекта: %w", err)
	}

	migrationsPath := filepath.Join(projectRoot, "tests/migrations") 
	migrationsURL := "file://" + filepath.ToSlash(migrationsPath)

	// Адаптируем DSN для golang-migrate (ожидает pgx5://)
	migrateDSN := strings.Replace(dsn, "postgresql://", "pgx5://", 1)
	if !strings.HasPrefix(migrateDSN, "pgx5://") && strings.HasPrefix(dsn, "postgres://") {
		migrateDSN = strings.Replace(dsn, "postgres://", "pgx5://", 1)
	} else if !strings.HasPrefix(migrateDSN, "pgx5://") { // Общий случай, если DSN вообще другой
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
		// Логируем ошибки закрытия, если они есть, но основная ошибка - от m.Up()
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

// getProjectRoot находит корневую директорию проекта (где лежит go.mod).
func getProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	// Идем вверх по дереву каталогов, пока не найдем go.mod
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil // go.mod найден
		}
		parent := filepath.Dir(dir)
		if parent == dir { // Дошли до корня файловой системы
			return "", errors.New("не удалось найти go.mod")
		}
		dir = parent
	}
}