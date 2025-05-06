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
	// Можно добавить другие ошибки (логарифм от отриц. числа и т.д.)
)

type Calculator interface {
	Calculate(ctx context.Context, operation string, a, b float64) (float64, error)
}

// CalculatorService выполняет арифметические операции.
type CalculatorService struct {
	log  *zap.Logger
	cfg  *config.CalculationTimeConfig // Конфигурация времени операций
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
    var err error

    // Определяем результат и задержку в зависимости от операции
    switch operation {
    case "+":
        result = a + b
        delay = s.cfg.Addition
    case "-":
        // Пока считаем бинарным, унарный добавим позже, если нужно будет отделять
        result = a - b
        delay = s.cfg.Subtraction
    case "*":
        result = a * b
        delay = s.cfg.Multiplication
    case "/":
        if b == 0 {
            err = ErrDivisionByZero
        } else {
            result = a / b
        }
        delay = s.cfg.Division
    case "^":
        result = math.Pow(a, b)
        delay = s.cfg.Exponentiation
    // case "neg": // Пример для унарного минуса
    //     result = -a
    //     delay = s.cfg.UnaryMinus
    default:
        err = fmt.Errorf("%w: %s", ErrUnknownOperator, operation)
        delay = 0 // Нет задержки для неизвестной операции
    }

    // Если была ошибка на этапе определения операции (деление на 0, неизв. оператор)
    if err != nil {
        s.log.Error("Ошибка при подготовке к вычислению",
            zap.String("operation", operation),
            zap.Float64("a", a),
            zap.Float64("b", b),
            zap.Error(err),
        )
        return 0, err
    }

    // Имитируем задержку вычисления
    s.log.Debug("Начало имитации вычисления",
        zap.String("operation", operation),
        zap.Duration("delay", delay),
    )

    // Используем select для возможности отмены операции через контекст
    select {
    case <-time.After(delay):
        // Задержка прошла успешно
        s.log.Debug("Имитация вычисления завершена",
            zap.String("operation", operation),
            zap.Float64("result", result),
        )
        return result, nil
    case <-ctx.Done():
        // Контекст был отменен (например, таймаут со стороны Оркестратора)
        s.log.Warn("Вычисление операции отменено контекстом",
             zap.String("operation", operation),
             zap.Error(ctx.Err()),
        )
        return 0, fmt.Errorf("вычисление отменено: %w", ctx.Err())
    }
}