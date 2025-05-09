package main

import (
	"fmt"
	"os"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/app"
)

func main() {
	// Базовый обработчик паник.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Критическая ошибка (паника) в Orchestrator main: %v\n", r)
			os.Exit(1)
		}
	}()

	// Запуск основного приложения Orchestrator.
	app.Run()
}
