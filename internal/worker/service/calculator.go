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
)

type Calculator interface {
	Calculate(ctx context.Context, operation string, a, b float64) (float64, error)
}

type calculatorService struct {
	log *zap.Logger
	cfg *config.CalculationTimeConfig
}

func NewCalculatorService(log *zap.Logger, cfg *config.Config) Calculator {
	return &calculatorService{
		log: log,
		cfg: &cfg.CalculationTime,
	}
}

func (s *calculatorService) Calculate(ctx context.Context, operation string, a, b float64) (float64, error) {

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
		return 0, calcErr
	}

	s.log.Debug("CalculatorService: имитация задержки", zap.Duration("delay", delay))
	select {
	case <-time.After(delay):
		s.log.Debug("CalculatorService: вычисление завершено", zap.Float64("result", result))
		return result, nil
	case <-ctx.Done():
		s.log.Warn("CalculatorService: вычисление отменено контекстом", zap.Error(ctx.Err()))
		return 0, fmt.Errorf("вычисление '%s' отменено: %w", operation, ctx.Err())
	}
}
