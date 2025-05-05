package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config определяет структуру конфигурации приложения.
type Config struct {
	AppEnv          string         `mapstructure:"APP_ENV"`
	Server          ServerConfig   `mapstructure:",squash"`
	Database        DatabaseConfig `mapstructure:",squash"`
	JWT             JWTConfig      `mapstructure:",squash"`
	Logger          LoggerConfig   `mapstructure:",squash"`
	GracefulTimeout time.Duration  `mapstructure:"GRACEFUL_TIMEOUT"`
}

type ServerConfig struct {
	Port string `mapstructure:"AGENT_HTTP_PORT"`
}

type DatabaseConfig struct {
	DSN          string `mapstructure:"POSTGRES_DSN"`
	PoolMaxConns int    `mapstructure:"DB_POOL_MAX_CONNS"`
}

type JWTConfig struct {
	Secret string `mapstructure:"JWT_SECRET"`
	// TokenTTL time.Duration `mapstructure:"JWT_TOKEN_TTL"` // позже для времени жизни токена
}

type LoggerConfig struct {
	Level string `mapstructure:"LOG_LEVEL"`
}

func Load() (*Config, error) {
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("AGENT_HTTP_PORT", "8080")
	viper.SetDefault("POSTGRES_DSN", "postgres://user:password@localhost:5432/calculator_db?sslmode=disable") // DSN по умолчанию для локального запуска без Docker
	viper.SetDefault("JWT_SECRET", "default-secret-key-please-change")
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("DB_POOL_MAX_CONNS", 10)
	viper.SetDefault("GRACEFUL_TIMEOUT", 5*time.Second) // Таймаут для graceful shutdown

	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AllowEmptyEnv(true)

	viper.SetConfigName(".env")
	viper.SetConfigType("env")
	viper.AddConfigPath(".")
	_ = viper.ReadInConfig()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("ошибка разбора конфигурации: %w", err)
	}

	if cfg.JWT.Secret == "default-secret-key-please-change" || len(cfg.JWT.Secret) < 32 {
		if cfg.AppEnv == "production" {
			return nil, fmt.Errorf("переменная окружения JWT_SECRET должна быть установлена и иметь длину не менее 32 символов в production")
		}
		// В development просто предупреждаем (но Fx все равно упадет при старте)
		fmt.Println("ПРЕДУПРЕЖДЕНИЕ: JWT_SECRET не установлен или слишком короткий. Используется значение по умолчанию.")
	}
	if cfg.Database.DSN == "" {
		return nil, fmt.Errorf("переменная окружения POSTGRES_DSN должна быть установлена")
	}

	return &cfg, nil
}