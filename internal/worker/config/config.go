package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

// Config определяет структуру конфигурации для Воркера.
type Config struct {
	AppEnv          string        `mapstructure:"APP_ENV"`
	GRPCServer      GRPCServerConfig `mapstructure:",squash"`
	Logger          LoggerConfig   `mapstructure:",squash"`
	GracefulTimeout time.Duration `mapstructure:"GRACEFUL_TIMEOUT"`
    CalculationTime CalculationTimeConfig `mapstructure:",squash"` // Добавили времена вычислений
}

// GRPCServerConfig содержит конфигурацию gRPC сервера Воркера.
type GRPCServerConfig struct {
	Port string `mapstructure:"WORKER_GRPC_PORT"` // Используем другой порт
}

// LoggerConfig содержит конфигурацию логгера.
type LoggerConfig struct {
	Level string `mapstructure:"LOG_LEVEL"`
}

// CalculationTimeConfig содержит время (задержку) для каждой операции.
// Используем строки для удобства задания в .env (e.g., "1s", "500ms").
type CalculationTimeConfig struct {
    Addition       time.Duration `mapstructure:"TIME_ADDITION_MS"`
    Subtraction    time.Duration `mapstructure:"TIME_SUBTRACTION_MS"`
    Multiplication time.Duration `mapstructure:"TIME_MULTIPLICATION_MS"`
    Division       time.Duration `mapstructure:"TIME_DIVISION_MS"`
    Exponentiation time.Duration `mapstructure:"TIME_EXPONENTIATION_MS"` // Для '^'
    // Добавить позже для функций и унарного минуса, если нужно
    // UnaryMinus     time.Duration `mapstructure:"TIME_UNARY_MINUS_MS"`
}


// Load загружает конфигурацию для Воркера.
func Load() (*Config, error) {
	// Установка значений по умолчанию
	viper.SetDefault("APP_ENV", "development")
	viper.SetDefault("WORKER_GRPC_PORT", "50052") // <-- Порт Воркера
	viper.SetDefault("LOG_LEVEL", "info")
	viper.SetDefault("GRACEFUL_TIMEOUT", 5*time.Second)
    // Времена операций по умолчанию (можно использовать ms или s)
    viper.SetDefault("TIME_ADDITION_MS", "200ms")
    viper.SetDefault("TIME_SUBTRACTION_MS", "200ms")
    viper.SetDefault("TIME_MULTIPLICATION_MS", "300ms")
    viper.SetDefault("TIME_DIVISION_MS", "400ms")
    viper.SetDefault("TIME_EXPONENTIATION_MS", "500ms")

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
    // Используем Hook для парсинга Duration из строки
    hook := viper.DecodeHook(
        mapstructure.ComposeDecodeHookFunc(
            mapstructure.StringToTimeDurationHookFunc(),
            mapstructure.StringToSliceHookFunc(","),
        ),
    )
	if err := viper.Unmarshal(&cfg, hook); err != nil {
		return nil, fmt.Errorf("ошибка разбора конфигурации Воркера: %w", err)
	}

	// Валидация
	if cfg.GRPCServer.Port == "" {
		return nil, fmt.Errorf("переменная окружения WORKER_GRPC_PORT должна быть установлена")
	}
    // Проверим, что времена распарсились корректно (больше 0)
    if cfg.CalculationTime.Addition <= 0 || cfg.CalculationTime.Subtraction <= 0 ||
       cfg.CalculationTime.Multiplication <= 0 || cfg.CalculationTime.Division <= 0 ||
       cfg.CalculationTime.Exponentiation <= 0 {
        return nil, fmt.Errorf("времена вычислений (TIME_..._MS) должны быть положительными длительностями")
    }


	return &cfg, nil
}