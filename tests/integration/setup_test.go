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
	testOrchestratorGRPCPort string = "50071" // Другие порты для избежания конфликтов
	testWorkerGRPCPort       string = "50072"
	testAgentHTTPPort        string = "8095" // Еще один уникальный порт для агента в тестах
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	log.Println("Инициализация тестового окружения...")

	// 1. Запуск PostgreSQL
	pgContainer, dsn, err := setupPostgres(ctx)
	if err != nil { log.Fatalf("Не удалось запустить PostgreSQL контейнер: %v", err) }
	testPostgresDSN = dsn
	log.Printf("PostgreSQL контейнер запущен, DSN: %s\n", testPostgresDSN)

	// 2. Миграции
	if err := runMigrations(testPostgresDSN); err != nil { log.Fatalf("Не удалось применить миграции: %v", err) }
	log.Println("Миграции успешно применены")

	// 3. Установка ПЕРЕМЕННЫХ ОКРУЖЕНИЯ (делаем это ДО любых вызовов config.Load())
	os.Setenv("APP_ENV", "test")
	os.Setenv("LOG_LEVEL", "debug")
	os.Setenv("GRACEFUL_TIMEOUT", "1s") // Короткий таймаут для тестов
    os.Setenv("JWT_SECRET", "another-super-secret-key-for-testing-long-enough-32chars")
    os.Setenv("JWT_TOKEN_TTL", "5m")
	os.Setenv("POSTGRES_DSN", testPostgresDSN)
	os.Setenv("DB_POOL_MAX_CONNS", "5")

	// Переменные для Оркестратора
	os.Setenv("ORCHESTRATOR_GRPC_PORT", testOrchestratorGRPCPort)
	os.Setenv("WORKER_GRPC_ADDRESS", fmt.Sprintf("localhost:%s", testWorkerGRPCPort))
    os.Setenv("GRPC_CLIENT_TIMEOUT", "2s") // Этот таймаут используется и в Agent, и в Orchestrator для их клиентов

	// Переменные для Воркера
	os.Setenv("WORKER_GRPC_PORT", testWorkerGRPCPort)
    os.Setenv("TIME_ADDITION_MS", "10ms") // Убедись, что эти переменные читаются в worker/config.Load()
    os.Setenv("TIME_SUBTRACTION_MS", "10ms")
    os.Setenv("TIME_MULTIPLICATION_MS", "10ms")
    os.Setenv("TIME_DIVISION_MS", "10ms")
    os.Setenv("TIME_EXPONENTIATION_MS", "10ms")

	// Переменные для Агента
	os.Setenv("AGENT_HTTP_PORT", testAgentHTTPPort)
	os.Setenv("ORCHESTRATOR_GRPC_ADDRESS", fmt.Sprintf("localhost:%s", testOrchestratorGRPCPort))

	// Запускаем сервисы
    orchestratorReady := make(chan struct{})
    workerReady := make(chan struct{})
    agentReady := make(chan struct{})

	go func() {
		defer func() { if r := recover(); r != nil { log.Printf("Паника в тестовом Оркестраторе: %v\n", r) }; close(orchestratorReady) }()
		log.Println("Запуск тестового Оркестратора...")
		// config.Load() будет вызван снова внутри app.Run() через Fx, но переменные окружения уже должны быть установлены
		orchestrator_app.Run()
		log.Println("Тестовый Оркестратор завершил работу")
	}()

	go func() {
		defer func() { if r := recover(); r != nil { log.Printf("Паника в тестовом Воркере: %v\n", r) }; close(workerReady) }()
		log.Println("Запуск тестового Воркера...")
		worker_app.Run()
		log.Println("Тестовый Воркер завершил работу")
	}()

    // Даем немного времени Оркестратору и Воркеру на старт, прежде чем Агент попытается к ним подключиться
	time.Sleep(1 * time.Second) // Можно уменьшить, если они быстро стартуют

	go func() {
		defer func() { if r := recover(); r != nil { log.Printf("Паника в тестовом Агенте: %v\n", r) }; close(agentReady) }()
		log.Println("Запуск тестового Агента...")
		agent_app.Run()
		log.Println("Тестовый Агент завершил работу")
	}()

	time.Sleep(1 * time.Second) // Даем время Агенту запуститься
	testAgentBaseURL = fmt.Sprintf("http://localhost:%s/api/v1", testAgentHTTPPort)
	log.Printf("Тестовый Агент запущен, URL: %s\n", testAgentBaseURL)

	exitCode := m.Run()

	log.Println("Завершение работы тестовых сервисов (отмена контекста)...")
	cancel()
	time.Sleep(3 * time.Second) // Увеличим немного время на корректное завершение

	log.Println("Остановка PostgreSQL контейнера...")
	if err := pgContainer.Terminate(context.Background()); err != nil {
		log.Printf("Не удалось остановить PostgreSQL контейнер: %v", err)
	}
	log.Println("PostgreSQL контейнер остановлен.")
	log.Println("Тестовое окружение остановлено.")
	os.Exit(exitCode)
}

func setupPostgres(ctx context.Context) (*postgres.PostgresContainer, string, error) {
	// Используем стандартные имя пользователя/пароль/БД, которые заданы в модуле postgres
	// Это "test", "test", "test"
	pgContainer, err := postgres.Run(ctx,
		"postgres:15-alpine", // Имя образа
		postgres.WithDatabase("testdb_integration"),   // Можем указать другое имя БД для тестов
		postgres.WithUsername("testuser_int"),       // Другой пользователь
		postgres.WithPassword("testpassword_int"),   // Другой пароль
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2). // Ждем второго такого сообщения (после initdb)
				WithStartupTimeout(15*time.Second), // Немного увеличим таймаут на всякий случай
		),
	)
	if err != nil {
		return nil, "", fmt.Errorf("не удалось запустить postgres контейнер: %w", err)
	}

	// Получаем DSN для подключения к этому контейнеру
	// ConnectionString вернет URI, подходящий для database/sql и pgxpool.ParseConfig
	dsn, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		// Если ошибка, пытаемся остановить контейнер
		if termErr := pgContainer.Terminate(context.Background()); termErr != nil {
			log.Printf("Ошибка при попытке остановить pgContainer после ошибки ConnectionString: %v", termErr)
		}
		return nil, "", fmt.Errorf("не удалось получить ConnectionString: %w", err)
	}

	return pgContainer, dsn, nil
}

func runMigrations(dsn string) error {
	projectRoot, err := getProjectRoot()
	if err != nil {
		return fmt.Errorf("не удалось найти корень проекта для миграций: %w", err)
	}
	// Получаем абсолютный путь к директории миграций
	absMigrationsPath, err := filepath.Abs(filepath.Join(projectRoot, "migrations"))
	if err != nil {
		return fmt.Errorf("не удалось получить абсолютный путь к миграциям: %w", err)
	}

	// Формируем URL для migrate.
	// Для source "file", URL должен быть просто "file://<path>"
	// где <path> это путь к директории с миграциями.
	// filepath.ToSlash важен для Windows, чтобы заменить '\' на '/'.
	migrationsURL := "file://" + filepath.ToSlash(absMigrationsPath)

	// DSN для migrate (pgx5)
	migrateDSN := strings.Replace(dsn, "postgresql://", "pgx5://", 1)
    if !strings.HasPrefix(migrateDSN, "pgx5://") && strings.HasPrefix(dsn, "postgres://") {
         migrateDSN = strings.Replace(dsn, "postgres://", "pgx5://", 1)
    }

	log.Printf("DSN для миграций (migrate): %s", migrateDSN)
	log.Printf("URL миграций (сформированный): %s", migrationsURL)

	m, err := migrate.New(migrationsURL, migrateDSN)
	if err != nil {
		if urlErr, ok := err.(*url.Error); ok {
			log.Printf("Ошибка парсинга URL миграций: Op: %s, URL: %s, Err: %s", urlErr.Op, urlErr.URL, urlErr.Err)
		}
		return fmt.Errorf("ошибка создания экземпляра migrate: %w (DSN: %s, URL: %s)", err, migrateDSN, migrationsURL)
	}

	log.Println("Попытка применить миграции (m.Up)...")
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Printf("Ошибка применения миграций (m.Up): %v", err) // Более детальный лог
		// Попробуем получить ошибки источника и БД для большей детализации
		var srcErrText, dbErrText string
		srcErr, dbErr := m.Close() // m.Close() возвращает две ошибки
		if srcErr != nil {
			srcErrText = srcErr.Error()
			log.Printf("Ошибка закрытия источника миграций (после ошибки Up): %v", srcErr)
		}
		if dbErr != nil {
			dbErrText = dbErr.Error()
			log.Printf("Ошибка закрытия соединения БД миграций (после ошибки Up): %v", dbErr)
		}
		return fmt.Errorf("ошибка применения миграций (m.Up): %w. SourceErr: %s, DBErr: %s", err, srcErrText, dbErrText)
	}

	// Закрываем соединение с БД и источник миграций
	srcErrClose, dbErrClose := m.Close() // Переименовал переменные
	if srcErrClose != nil {
		return fmt.Errorf("ошибка закрытия источника миграций: %w", srcErrClose)
	}
	if dbErrClose != nil {
		return fmt.Errorf("ошибка закрытия соединения БД миграций: %w", dbErrClose)
	}
	log.Println("Миграции успешно применены (runMigrations)")
	return nil
}

func getProjectRoot() (string, error) {
    // ... (код getProjectRoot остается таким же) ...
    dir, err := os.Getwd()
    if err != nil {
        return "", err
    }
    // Ищем go.mod, поднимаясь вверх по дереву каталогов
    for {
        if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
            return dir, nil // Нашли go.mod, это корень проекта
        }
        parent := filepath.Dir(dir)
        if parent == dir {
            // Дошли до корня файловой системы и не нашли go.mod
            return "", errors.New("не удалось найти go.mod в текущем или родительских каталогах")
        }
        dir = parent
    }
}
