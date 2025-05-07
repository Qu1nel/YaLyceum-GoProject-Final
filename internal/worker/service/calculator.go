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
		if b == 0.0 { // Явная проверка на ноль
			s.log.Warn("CalculatorService: обнаружено деление на ноль", zap.Float64("a", a))
			calcErr = ErrDivisionByZero // Устанавливаем ошибку
			// result остается 0.0 по умолчанию для float64
		} else {
			result = a / b
		}
		delay = s.cfg.Division
	case "^":
		result = math.Pow(a, b)
		delay = s.cfg.Exponentiation
	case "neg": // Обработка унарного минуса, который мы посылаем из Evaluator
		result = -a // b игнорируется
        // Используем задержку, аналогичную вычитанию, или свою
		delay = s.cfg.Subtraction // Или можно добавить TIME_UNARY_MINUS_MS в конфиг
	default:
		s.log.Warn("CalculatorService: неизвестный оператор", zap.String("operation", operation))
		calcErr = fmt.Errorf("%w: '%s'", ErrUnknownOperator, operation)
		delay = 0 // Нет смысла в задержке для неизвестной операции
	}

	// Если ошибка была определена на этапе выбора операции (деление на ноль, неизвестный оператор)
	if calcErr != nil {
		s.log.Error("CalculatorService: ошибка определена до имитации задержки, возвращаем ошибку",
			zap.String("operation", operation),
			zap.Error(calcErr),
		)
		return 0, calcErr // Возвращаем 0 (или другое значение по умолчанию) и ошибку
	}

	// Если ошибки не было, имитируем задержку вычисления
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
		return result, nil // Успешное вычисление
	case <-ctx.Done():
		// Контекст был отменен (например, таймаут со стороны вызывающего кода или Оркестратора)
		s.log.Warn("CalculatorService: вычисление операции отменено родительским контекстом",
			zap.String("operation", operation),
			zap.Error(ctx.Err()),
		)
		return 0, fmt.Errorf("вычисление операции '%s' отменено: %w", operation, ctx.Err())
	}
}