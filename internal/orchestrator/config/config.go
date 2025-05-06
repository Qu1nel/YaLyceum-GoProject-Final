package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type GRPCClientConfig struct {
	WorkerAddress string        `mapstructure:"WORKER_GRPC_ADDRESS"`
	Timeout       time.Duration `mapstructure:"GRPC_CLIENT_TIMEOUT"` // Общий таймаут для gRPC клиентов
}

type Config struct {
	AppEnv          string        `mapstructure:"APP_ENV"`
	GRPCServer      GRPCServerConfig `mapstructure:",squash"`
	Database        DatabaseConfig `mapstructure:",squash"`
	Logger          LoggerConfig   `mapstructure:",squash"`
	GracefulTimeout time.Duration `mapstructure:"GRACEFUL_TIMEOUT"`
	WorkerClient    GRPCClientConfig `mapstructure:",squash"` // <-- Добавили конфигурацию клиента Воркера
}

// GRPCServerConfig содержит конфигурацию gRPC сервера.
type GRPCServerConfig struct {
	Port string `mapstructure:"ORCHESTRATOR_GRPC_PORT"`
}

// DatabaseConfig содержит конфигурацию подключения к базе данных.
// Можно вынести в общий internal/pkg/config, если будет много общих полей.
type DatabaseConfig struct {
	DSN          string `mapstructure:"POSTGRES_DSN"`
	PoolMaxConns int    `mapstructure:"DB_POOL_MAX_CONNS"`
}

// LoggerConfig содержит конфигурацию логгера.
type LoggerConfig struct {
	Level string `mapstructure:"LOG_LEVEL"`
}

// Load загружает конфигурацию для Оркестратора.
func Load() (*Config, error) {
	// Установка значений по умолчанию
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("ORCHESTRATOR_GRPC_PORT", "50051") // Порт gRPC сервера
	viper.SetDefault("POSTGRES_DSN", "postgres://user:password@postgres:5432/calculator_db?sslmode=disable") // Используем DSN из Agent по умолчанию
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("DB_POOL_MAX_CONNS", 10)
	viper.SetDefault("GRACEFUL_TIMEOUT", 5*time.Second)
	viper.SetDefault("WORKER_GRPC_ADDRESS", "worker:50052")
	viper.SetDefault("GRPC_CLIENT_TIMEOUT", "5s") 

	// Настройка чтения переменных окружения
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AllowEmptyEnv(true)

	// Чтение из .env (если есть)
	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".") // Ищем в корне проекта
	_ = viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("ошибка разбора конфигурации Оркестратора: %w", err)
	}

	// Валидация
	if cfg.GRPCServer.Port == "" {
		return nil, fmt.Errorf("переменная окружения ORCHESTRATOR_GRPC_PORT должна быть установлена")
	}
	if cfg.Database.DSN == "" {
		return nil, fmt.Errorf("переменная окружения POSTGRES_DSN должна быть установлена")
	}

	// ... валидация ORCHESTRATOR_GRPC_PORT, POSTGRES_DSN ...
	if cfg.WorkerClient.WorkerAddress == "" {
		return nil, fmt.Errorf("переменная окружения WORKER_GRPC_ADDRESS должна быть установлена")
	}
    if cfg.WorkerClient.Timeout <= 0 {
        return nil, fmt.Errorf("GRPC_CLIENT_TIMEOUT должен быть положительной длительностью")
    }


	return &cfg, nil
}