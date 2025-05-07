package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/config" // Убедись, что путь правильный
	"go.uber.org/zap"
)

var (
	ErrDivisionByZero  = errors.New("деление на ноль")
	ErrUnknownOperator = errors.New("неизвестный оператор")
	// ErrCalculationCancelled = errors.New("вычисление отменено") // Можно добавить свою ошибку для отмены
)

// Calculator определяет интерфейс для сервиса вычислений.
type Calculator interface {
	Calculate(ctx context.Context, operation string, a, b float64) (float64, error)
}

// CalculatorService выполняет арифметические операции.
type CalculatorService struct {
	log *zap.Logger
	cfg *config.CalculationTimeConfig
}

// NewCalculatorService создает новый сервис вычислений.
func NewCalculatorService(log *zap.Logger, cfg *config.Config) Calculator {
	return &CalculatorService{
		log: log,
		cfg: &cfg.CalculationTime,
	}
}

// Calculate выполняет операцию и имитирует задержку.
func (s *CalculatorService) Calculate(ctx context.Context, operation string, a, b float64) (float64, error) {
	var result float64
	var delay time.Duration
	var calcErr error // Локальная переменная для ошибки внутри switch

	s.log.Debug("CalculatorService: начало вычисления операции",
		zap.String("operation", operation),
		zap.Float64("a", a),
		zap.Float64("b", b),
	)

	switch operation {
	case "+":
		result = a + b
		delay = s.cfg.Addition
	case "-":
		result = a - b
		delay = s.cfg.Subtraction
	case "*":
		result = a * b
		delay = s.cfg.Multiplication
	case "/":
		if b == 0.0 {
			s.log.Info("CalculatorService: обнаружено деление на ноль")
			calcErr = ErrDivisionByZero
		} else {
			result = a / b
		}
		delay = s.cfg.Division
	case "^":
		result = math.Pow(a, b)
		delay = s.cfg.Exponentiation
	case "neg":
		result = -a
		delay = s.cfg.Subtraction 
	default:
		s.log.Warn("CalculatorService: неизвестный оператор", zap.String("operation", operation))
		calcErr = fmt.Errorf("%w: '%s'", ErrUnknownOperator, operation)
		delay = 0 
	}

	if calcErr != nil {
		s.log.Error("CalculatorService: ошибка определена до имитации задержки, возвращаем ошибку",
			zap.String("operation", operation),
			zap.Error(calcErr),
		)
		return 0, calcErr
	}

	s.log.Debug("CalculatorService: начало имитации задержки вычисления",
		zap.String("operation", operation),
		zap.Duration("delay", delay),
	)

	select {
	case <-time.After(delay):
		s.log.Debug("CalculatorService: имитация задержки вычисления завершена, возвращаем результат",
			zap.String("operation", operation),
			zap.Float64("result", result),
		)
		return result, nil
	case <-ctx.Done():
		s.log.Warn("CalculatorService: вычисление операции отменено родительским контекстом",
			zap.String("operation", operation),
			zap.Error(ctx.Err()),
		)
		return 0, fmt.Errorf("вычисление операции '%s' отменено: %w", operation, ctx.Err())
	}
}