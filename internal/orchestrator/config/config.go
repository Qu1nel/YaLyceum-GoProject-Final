package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
	// "github.com/mitchellh/mapstructure" // Не нужен, если нет time.Duration из строк
)

type GRPCClientConfig struct { // Конфиг для клиента Воркера
	WorkerAddress string        `mapstructure:"WORKER_GRPC_ADDRESS"`
	Timeout       time.Duration `mapstructure:"GRPC_CLIENT_TIMEOUT"`
}

type Config struct {
	AppEnv          string           `mapstructure:"APP_ENV"`
	GRPCServer      GRPCServerConfig `mapstructure:",squash"` // Конфиг своего gRPC сервера
	Database        DatabaseConfig   `mapstructure:",squash"`
	Logger          LoggerConfig     `mapstructure:",squash"`
	GracefulTimeout time.Duration    `mapstructure:"GRACEFUL_TIMEOUT"`
	WorkerClient    GRPCClientConfig `mapstructure:",squash"` // Конфиг клиента к Воркеру
}

type GRPCServerConfig struct {
	Port string `mapstructure:"ORCHESTRATOR_GRPC_PORT"`
}

type DatabaseConfig struct {
	DSN          string `mapstructure:"POSTGRES_DSN"`
	PoolMaxConns int    `mapstructure:"DB_POOL_MAX_CONNS"`
}

type LoggerConfig struct {
	Level string `mapstructure:"LOG_LEVEL"`
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("APP_ENV", "development")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("GRACEFUL_TIMEOUT", "5s")
	v.SetDefault("POSTGRES_DSN", "postgres://user_default:pass_default@postgres_host_default:5432/db_default?sslmode=disable")
	v.SetDefault("DB_POOL_MAX_CONNS", 10)
	v.SetDefault("GRPC_CLIENT_TIMEOUT", "5s") // Общий для клиента Воркера

	v.SetDefault("ORCHESTRATOR_GRPC_PORT", "50051")
	v.SetDefault("WORKER_GRPC_ADDRESS", "worker_default:50052")

	if appEnv := os.Getenv("APP_ENV"); appEnv != "test" {
		v.SetConfigName(".env")
		v.SetConfigType("env")
		v.AddConfigPath(".")
		if err := v.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				log.Println("Файл .env не найден, используются переменные окружения/дефолты.")
			} else {
				log.Printf("Предупреждение: ошибка чтения файла .env: %v (игнорируется)", err)
			}
		} else {
			log.Printf("Конфигурация Оркестратора загружена из файла .env (APP_ENV=%s)", appEnv)
		}
	} else {
		log.Println("APP_ENV=test (Оркестратор), файл .env не читается.")
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("ошибка разбора конфигурации Оркестратора: %w", err)
	}

	// Валидация
	if cfg.GRPCServer.Port == "" || (os.Getenv("APP_ENV") == "test" && cfg.GRPCServer.Port == v.GetString("ORCHESTRATOR_GRPC_PORT") && os.Getenv("ORCHESTRATOR_GRPC_PORT") != cfg.GRPCServer.Port) {
		return nil, fmt.Errorf("ORCHESTRATOR_GRPC_PORT: ожидалось '%s' из env, получено '%s'", os.Getenv("ORCHESTRATOR_GRPC_PORT"), cfg.GRPCServer.Port)
	}
	if cfg.Database.DSN == "" || (os.Getenv("APP_ENV") == "test" && cfg.Database.DSN == v.GetString("POSTGRES_DSN") && os.Getenv("POSTGRES_DSN") != cfg.Database.DSN )  {
		return nil, fmt.Errorf("POSTGRES_DSN для Оркестратора не установлен или равен дефолтному в тесте (текущий: '%s', ожидался из env: '%s')", cfg.Database.DSN, os.Getenv("POSTGRES_DSN"))
	}
	if cfg.WorkerClient.WorkerAddress == "" || (os.Getenv("APP_ENV") == "test" && cfg.WorkerClient.WorkerAddress == v.GetString("WORKER_GRPC_ADDRESS") && os.Getenv("WORKER_GRPC_ADDRESS") != cfg.WorkerClient.WorkerAddress) {
		return nil, fmt.Errorf("WORKER_GRPC_ADDRESS для Оркестратора не установлен или равен дефолтному в тесте (текущий: '%s', ожидался из env: '%s')", cfg.WorkerClient.WorkerAddress, os.Getenv("WORKER_GRPC_ADDRESS"))
	}
    if cfg.WorkerClient.Timeout <= 0 {
        return nil, fmt.Errorf("GRPC_CLIENT_TIMEOUT для клиента Воркера должен быть положительным")
    }
    if cfg.GracefulTimeout <= 0 {
        return nil, fmt.Errorf("GRACEFUL_TIMEOUT должен быть положительным")
    }

	return &cfg, nil
}