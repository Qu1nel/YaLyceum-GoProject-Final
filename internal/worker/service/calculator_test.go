package service_test // Используем _test суффикс

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/service" // Импортируем пакет под тестом
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// testCalcTimeConfig создает конфигурацию времени с минимальными задержками для тестов.
func testCalcTimeConfig() config.CalculationTimeConfig {
	return config.CalculationTimeConfig{
		Addition:       1 * time.Millisecond,
		Subtraction:    1 * time.Millisecond,
		Multiplication: 1 * time.Millisecond,
		Division:       1 * time.Millisecond,
		Exponentiation: 1 * time.Millisecond,
	}
}

func TestCalculatorService_Calculate_BasicOperations(t *testing.T) {
	logger := zap.NewNop()
	testCfg := &config.Config{CalculationTime: testCalcTimeConfig()}
	calcService := service.NewCalculatorService(logger, testCfg)
	ctx := context.Background()

	testCases := []struct {
		name       string
		operation  string
		a, b       float64
		want       float64
		wantErrIs  error  // Ожидаем, что ошибка будет этим типом (через errors.Is)
		wantErrMsg string // Опционально, часть сообщения об ошибке для assert.Contains
	}{
		{name: "Сложение", operation: "+", a: 10, b: 5, want: 15.0},
		{name: "Вычитание", operation: "-", a: 10, b: 5, want: 5.0},
		{name: "Умножение", operation: "*", a: 10, b: 5, want: 50.0},
		{name: "Деление", operation: "/", a: 10, b: 2, want: 5.0},
		{name: "Возведение в степень", operation: "^", a: 2, b: 3, want: 8.0},
		{name: "Унарный минус (neg)", operation: "neg", a: 7, b: 0, want: -7.0}, // b игнорируется
		{name: "Деление на ноль", operation: "/", a: 10, b: 0, wantErrIs: service.ErrDivisionByZero},
		{name: "Неизвестный оператор", operation: "%", a: 10, b: 5, wantErrIs: service.ErrUnknownOperator, wantErrMsg: "неизвестный оператор: '%'"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := calcService.Calculate(ctx, tc.operation, tc.a, tc.b)

			if tc.wantErrIs != nil {
				require.Error(t, err, "Ожидалась ошибка")
				assert.True(t, errors.Is(err, tc.wantErrIs), "Неверный тип ошибки: получено %v, ожидался тип %v", err, tc.wantErrIs)
				if tc.wantErrMsg != "" {
					assert.Contains(t, err.Error(), tc.wantErrMsg, "Сообщение об ошибке не содержит ожидаемый текст")
				}
			} else {
				require.NoError(t, err, "Не ожидалась ошибка")
				assert.InDelta(t, tc.want, got, 0.00001, "Результат не совпадает с ожидаемым")
			}
		})
	}
}

func TestCalculatorService_Calculate_ContextCancelled(t *testing.T) {
	logger := zap.NewNop()
	testCfg := &config.Config{CalculationTime: testCalcTimeConfig()}
	// Устанавливаем большую задержку, чтобы успеть отменить контекст
	testCfg.CalculationTime.Addition = 100 * time.Millisecond
	calcService := service.NewCalculatorService(logger, testCfg)

	// Создаем контекст, который будет отменен через 10ms
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel() // Важно вызвать cancel, чтобы освободить ресурсы, связанные с WithTimeout

	_, err := calcService.Calculate(ctx, "+", 1, 1)

	require.Error(t, err, "Ожидалась ошибка из-за отмены контекста")
	// Проверяем, что ошибка действительно связана с отменой контекста
	assert.True(t, errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled),
		"Ожидалась ошибка context.DeadlineExceeded или context.Canceled, получено: %v", err)
	assert.Contains(t, err.Error(), "отменено", "Сообщение об ошибке должно указывать на отмену")
}
