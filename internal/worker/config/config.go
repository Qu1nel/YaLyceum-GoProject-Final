package config

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
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
				log.Println("Worker Config: .env файл не найден, используются переменные окружения/дефолты.")
			} else {

				log.Printf("Worker Config: предупреждение при чтении .env: %v (игнорируется)\n", err)
			}
		} else {
			log.Printf("Worker Config: конфигурация загружена из .env (APP_ENV=%s)\n", appEnv)
		}
	} else {
		log.Println("Worker Config: APP_ENV=test, .env файл не читается.")
	}

	var cfg Config

	hook := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
	))
	if err := v.Unmarshal(&cfg, hook); err != nil {
		return nil, fmt.Errorf("worker config: ошибка разбора конфигурации: %w", err)
	}

	if cfg.GRPCServer.Port == "" {
		return nil, errors.New("worker config: WORKER_GRPC_PORT не может быть пустым")
	}
	if cfg.CalculationTime.Addition <= 0 {

		log.Printf("Worker Config: TIME_ADDITION_MS имеет нетипичное значение: %s", cfg.CalculationTime.Addition)
	}
	if cfg.GracefulTimeout <= 0 {
		return nil, errors.New("worker config: GRACEFUL_TIMEOUT должен быть положительным")
	}

	return &cfg, nil
}
