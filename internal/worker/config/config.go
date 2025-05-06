package config

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure" // Нужен для парсинга Duration
	"github.com/spf13/viper"
)

type Config struct {
	AppEnv          string                `mapstructure:"APP_ENV"`
	GRPCServer      GRPCServerConfig      `mapstructure:",squash"`
	Logger          LoggerConfig          `mapstructure:",squash"`
	GracefulTimeout time.Duration         `mapstructure:"GRACEFUL_TIMEOUT"`
	CalculationTime CalculationTimeConfig `mapstructure:",squash"`
}

type GRPCServerConfig struct {
	Port string `mapstructure:"WORKER_GRPC_PORT"`
}

type LoggerConfig struct {
	Level string `mapstructure:"LOG_LEVEL"`
}

type CalculationTimeConfig struct {
	Addition       time.Duration `mapstructure:"TIME_ADDITION_MS"`
	Subtraction    time.Duration `mapstructure:"TIME_SUBTRACTION_MS"`
	Multiplication time.Duration `mapstructure:"TIME_MULTIPLICATION_MS"`
	Division       time.Duration `mapstructure:"TIME_DIVISION_MS"`
	Exponentiation time.Duration `mapstructure:"TIME_EXPONENTIATION_MS"`
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	v.SetDefault("APP_ENV", "development")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("GRACEFUL_TIMEOUT", "5s")
	v.SetDefault("WORKER_GRPC_PORT", "50052")
	v.SetDefault("TIME_ADDITION_MS", "200ms")
	v.SetDefault("TIME_SUBTRACTION_MS", "200ms")
	v.SetDefault("TIME_MULTIPLICATION_MS", "300ms")
	v.SetDefault("TIME_DIVISION_MS", "400ms")
	v.SetDefault("TIME_EXPONENTIATION_MS", "500ms")

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
			log.Printf("Конфигурация Воркера загружена из файла .env (APP_ENV=%s)", appEnv)
		}
	} else {
		log.Println("APP_ENV=test (Воркер), файл .env не читается.")
	}

	var cfg Config
	hook := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		// mapstructure.StringToSliceHookFunc(","), // Не нужен для Duration
	))
	if err := v.Unmarshal(&cfg, hook); err != nil {
		return nil, fmt.Errorf("ошибка разбора конфигурации Воркера: %w", err)
	}

	// Валидация
	if cfg.GRPCServer.Port == "" || (os.Getenv("APP_ENV") == "test" && cfg.GRPCServer.Port == v.GetString("WORKER_GRPC_PORT") && os.Getenv("WORKER_GRPC_PORT") != cfg.GRPCServer.Port) {
		return nil, fmt.Errorf("WORKER_GRPC_PORT: ожидалось '%s' из env, получено '%s'", os.Getenv("WORKER_GRPC_PORT"), cfg.GRPCServer.Port)
	}
	if cfg.CalculationTime.Addition <= 0 || (os.Getenv("APP_ENV") == "test" && cfg.CalculationTime.Addition.String() == v.GetString("TIME_ADDITION_MS") && os.Getenv("TIME_ADDITION_MS") != cfg.CalculationTime.Addition.String()) {
		return nil, fmt.Errorf("TIME_ADDITION_MS для Воркера не установлен корректно (текущий: %s, из viper default: %s, из env: %s)", cfg.CalculationTime.Addition, v.GetString("TIME_ADDITION_MS"), os.Getenv("TIME_ADDITION_MS"))
	}
	// ... аналогичные проверки для других времен операций ...
	if cfg.GracefulTimeout <=0 {
		return nil, fmt.Errorf("GRACEFUL_TIMEOUT должен быть положительным")
	}


	return &cfg, nil
}