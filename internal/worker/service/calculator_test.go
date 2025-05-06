package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/config" // Для CalculationTimeConfig
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Вспомогательная функция для создания конфигурации времени для тестов
func testCalcTimeConfig() *config.CalculationTimeConfig {
	return &config.CalculationTimeConfig{
		Addition:       1 * time.Millisecond, // Маленькие задержки для тестов
		Subtraction:    1 * time.Millisecond,
		Multiplication: 1 * time.Millisecond,
		Division:       1 * time.Millisecond,
		Exponentiation: 1 * time.Millisecond,
	}
}

func TestCalculatorService_Calculate(t *testing.T) {
	logger := zap.NewNop()
	// Создаем тестовую конфигурацию с минимальными задержками
	testConfig := &config.Config{CalculationTime: *testCalcTimeConfig()}
	calcService := NewCalculatorService(logger, testConfig)
	ctx := context.Background()

	testCases := []struct {
		name      string
		operation string
		a, b      float64
		want      float64
		wantErr   error // Ожидаемая ошибка (тип)
        wantMsg   string // Часть сообщения об ошибке
	}{
		{name: "Сложение", operation: "+", a: 10, b: 5, want: 15.0, wantErr: nil},
		{name: "Вычитание", operation: "-", a: 10, b: 5, want: 5.0, wantErr: nil},
		{name: "Умножение", operation: "*", a: 10, b: 5, want: 50.0, wantErr: nil},
		{name: "Деление", operation: "/", a: 10, b: 5, want: 2.0, wantErr: nil},
		{name: "Возведение в степень", operation: "^", a: 2, b: 3, want: 8.0, wantErr: nil},
		{name: "Деление на ноль", operation: "/", a: 10, b: 0, want: 0, wantErr: ErrDivisionByZero},
		{name: "Неизвестный оператор", operation: "%", a: 10, b: 5, want: 0, wantErr: ErrUnknownOperator, wantMsg: "неизвестный оператор: %"},
        // Можно добавить тесты для унарного минуса, когда он будет реализован в switch
        // {name: "Унарный минус (если реализован)", operation: "neg", a: 5, b: 0, want: -5.0, wantErr: nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := calcService.Calculate(ctx, tc.operation, tc.a, tc.b)

			if tc.wantErr != nil {
				require.Error(t, err, "Ожидалась ошибка")
				assert.True(t, errors.Is(err, tc.wantErr), "Тип ошибки не совпадает: получено %v, ожидалось %v", err, tc.wantErr)
                if tc.wantMsg != "" {
                    assert.Contains(t, err.Error(), tc.wantMsg, "Сообщение об ошибке не содержит ожидаемый текст")
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
	testConfig := &config.Config{CalculationTime: *testCalcTimeConfig()}
    // Увеличим задержку для этого теста, чтобы успеть отменить контекст
    testConfig.CalculationTime.Addition = 50 * time.Millisecond
	calcService := NewCalculatorService(logger, testConfig)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond) // Таймаут меньше задержки операции

    // Отменяем контекст чуть позже, но до завершения time.After(delay) в Calculate
    go func() {
        time.Sleep(5 * time.Millisecond)
        cancel()
    }()

	_, err := calcService.Calculate(ctx, "+", 1, 1)

	require.Error(t, err, "Ожидалась ошибка из-за отмены контекста")
    // Ошибка будет обернута
    assert.True(t, errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded),
        "Ожидалась ошибка context.Canceled или context.DeadlineExceeded, получено: %v", err)
}