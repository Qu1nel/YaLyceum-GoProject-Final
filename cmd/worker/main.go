package main

import (
	"fmt"
	"os"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/app"
)

func main() {
	// Базовый обработчик паник.
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Критическая ошибка (паника) в Worker main: %v\n", r)
			os.Exit(1)
		}
	}()

	// Запуск основного приложения Worker.
	app.Run()
}
