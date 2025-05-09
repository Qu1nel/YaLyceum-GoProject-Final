package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	AppEnv             string           `mapstructure:"APP_ENV"`
	Server             ServerConfig     `mapstructure:",squash"`
	Database           DatabaseConfig   `mapstructure:",squash"`
	JWT                JWTConfig        `mapstructure:",squash"`
	Logger             LoggerConfig     `mapstructure:",squash"`
	GracefulTimeout    time.Duration    `mapstructure:"GRACEFUL_TIMEOUT"`
	OrchestratorClient GRPCClientConfig `mapstructure:",squash"`
}

type ServerConfig struct {
	Port string `mapstructure:"AGENT_HTTP_PORT"`
}

type DatabaseConfig struct {
	DSN          string `mapstructure:"POSTGRES_DSN"`
	PoolMaxConns int    `mapstructure:"DB_POOL_MAX_CONNS"`
}

type JWTConfig struct {
	Secret   string        `mapstructure:"JWT_SECRET"`
	TokenTTL time.Duration `mapstructure:"JWT_TOKEN_TTL"`
}

type LoggerConfig struct {
	Level string `mapstructure:"LOG_LEVEL"`
}

type GRPCClientConfig struct {
	OrchestratorAddress string        `mapstructure:"ORCHESTRATOR_GRPC_ADDRESS"`
	Timeout             time.Duration `mapstructure:"GRPC_CLIENT_TIMEOUT"`
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

	v.SetDefault("AGENT_HTTP_PORT", "8080")
	v.SetDefault("JWT_SECRET", "default_jwt_secret_please_change_32_chars_long")
	v.SetDefault("JWT_TOKEN_TTL", "1h")
	v.SetDefault("ORCHESTRATOR_GRPC_ADDRESS", "orchestrator_default:50051")
	v.SetDefault("GRPC_CLIENT_TIMEOUT", "5s")

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
			log.Printf("Конфигурация Агента загружена из файла .env (APP_ENV=%s)", appEnv)
		}
	} else {
		log.Println("APP_ENV=test (Агент), файл .env не читается.")
	}

	var cfg Config

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("ошибка разбора конфигурации Агента: %w", err)
	}

	if cfg.Server.Port == "" {
		return nil, fmt.Errorf("AGENT_HTTP_PORT (из env или default) не установлен")
	}
	if cfg.Database.DSN == "" || (os.Getenv("APP_ENV") == "test" && cfg.Database.DSN == v.GetString("POSTGRES_DSN") && os.Getenv("POSTGRES_DSN") != cfg.Database.DSN) {

		return nil, fmt.Errorf("POSTGRES_DSN для Агента не установлен или равен дефолтному в тесте (текущий: '%s', ожидался из env: '%s')", cfg.Database.DSN, os.Getenv("POSTGRES_DSN"))
	}
	if cfg.JWT.Secret == "" || (len(cfg.JWT.Secret) < 32 && cfg.AppEnv != "test") {
		if cfg.JWT.Secret == v.GetString("JWT_SECRET") && os.Getenv("APP_ENV") == "test" && os.Getenv("JWT_SECRET") != cfg.JWT.Secret {
			return nil, fmt.Errorf("JWT_SECRET для Агента не установлен или равен дефолтному в тесте (текущий: '%s', ожидался из env: '%s')", cfg.JWT.Secret, os.Getenv("JWT_SECRET"))
		}
		if len(cfg.JWT.Secret) < 32 && cfg.AppEnv != "test" {
			return nil, fmt.Errorf("JWT_SECRET должен быть не менее 32 символов (текущая длина: %d)", len(cfg.JWT.Secret))
		}
	}
	if cfg.JWT.TokenTTL <= 0 {
		return nil, fmt.Errorf("JWT_TOKEN_TTL должен быть положительной длительностью")
	}
	if cfg.OrchestratorClient.OrchestratorAddress == "" || (os.Getenv("APP_ENV") == "test" && cfg.OrchestratorClient.OrchestratorAddress == v.GetString("ORCHESTRATOR_GRPC_ADDRESS") && os.Getenv("ORCHESTRATOR_GRPC_ADDRESS") != cfg.OrchestratorClient.OrchestratorAddress) {
		return nil, fmt.Errorf("ORCHESTRATOR_GRPC_ADDRESS для Агента не установлен или равен дефолтному в тесте (текущий: '%s', ожидался из env: '%s')", cfg.OrchestratorClient.OrchestratorAddress, os.Getenv("ORCHESTRATOR_GRPC_ADDRESS"))
	}
	if cfg.OrchestratorClient.Timeout <= 0 {
		return nil, fmt.Errorf("GRPC_CLIENT_TIMEOUT для клиента Оркестратора должен быть положительным")
	}

	return &cfg, nil
}
