package main

import (
	"fmt"
	"os"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/app"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Неперехваченная паника в Оркестраторе: %v\n", r)
			os.Exit(1)
		}
	}()

	app.Run()
}