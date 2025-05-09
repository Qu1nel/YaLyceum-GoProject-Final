package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/config"
	"go.uber.org/zap"
)

var (
	ErrDivisionByZero  = errors.New("деление на ноль")
	ErrUnknownOperator = errors.New("неизвестный оператор")
	// TODO: Добавить ошибки для функций, например, ErrSqrtNegative, ErrLogNonPositive
)

// Calculator определяет интерфейс для сервиса вычислений.
type Calculator interface {
	// TODO: Когда будут добавлены функции с переменным числом аргументов,
	// сигнатуру Calculate нужно будет изменить или добавить новый метод.
	// Например: Calculate(ctx context.Context, operation string, operands ...float64) (float64, error)
	Calculate(ctx context.Context, operation string, a, b float64) (float64, error)
}

// calculatorService выполняет арифметические операции с имитацией задержки.
type calculatorService struct {
	log *zap.Logger
	cfg *config.CalculationTimeConfig // Конфигурация времени задержек
}

// NewCalculatorService создает новый сервис вычислений.
func NewCalculatorService(log *zap.Logger, cfg *config.Config) Calculator {
	return &calculatorService{
		log: log,
		cfg: &cfg.CalculationTime, // Сохраняем указатель на подструктуру конфига
	}
}

// Calculate выполняет указанную арифметическую операцию.
func (s *calculatorService) Calculate(ctx context.Context, operation string, a, b float64 /*, args ...float64 */) (float64, error) {
	// TODO: Обработать args для функций, когда они будут добавлены.
	s.log.Debug("CalculatorService: начало вычисления",
		zap.String("operation", operation),
		zap.Float64("a", a),
		zap.Float64("b", b),
	)

	var result float64
	var delay time.Duration
	var calcErr error

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
			s.log.Warn("CalculatorService: попытка деления на ноль", zap.Float64("a", a))
			calcErr = ErrDivisionByZero
		} else {
			result = a / b
		}
		delay = s.cfg.Division
	case "^": // Возведение в степень
		result = math.Pow(a, b)
		delay = s.cfg.Exponentiation
	case "neg": // Унарный минус
		result = -a               // 'b' игнорируется для "neg"
		delay = s.cfg.Subtraction // Используем задержку вычитания или свою
	// TODO: Добавить case для "sqrt", "log", "abs", "sin", "cos" и т.д.
	// case "sqrt":
	//    if a < 0 { calcErr = errors.New("аргумент sqrt не может быть отрицательным") }
	//    else { result = math.Sqrt(a) }
	//    delay = s.cfg.Exponentiation // Пример, нужна своя задержка
	default:
		s.log.Warn("CalculatorService: неизвестный оператор", zap.String("operation", operation))
		calcErr = fmt.Errorf("%w: '%s'", ErrUnknownOperator, operation)
		delay = 0 // Для неизвестных операций задержки нет
	}

	// Если ошибка произошла на этапе определения операции (например, деление на ноль),
	// возвращаем ее сразу, не имитируя задержку.
	if calcErr != nil {
		return 0, calcErr
	}

	// Имитация задержки вычисления.
	// Позволяет проверить отмену контекста.
	s.log.Debug("CalculatorService: имитация задержки", zap.Duration("delay", delay))
	select {
	case <-time.After(delay):
		s.log.Debug("CalculatorService: вычисление завершено", zap.Float64("result", result))
		return result, nil
	case <-ctx.Done(): // Если родительский контекст отменен (например, таймаут)
		s.log.Warn("CalculatorService: вычисление отменено контекстом", zap.Error(ctx.Err()))
		return 0, fmt.Errorf("вычисление '%s' отменено: %w", operation, ctx.Err())
	}
}
