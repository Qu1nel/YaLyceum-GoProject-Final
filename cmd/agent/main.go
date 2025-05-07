package main

import (
	"fmt"
	"os"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/app"
)

// @title API Агента для Калькулятора Выражений
// @version 1.0.0
// @description Этот сервис является точкой входа для пользователей Калькулятора Выражений. Он отвечает за аутентификацию, авторизацию и прием задач на вычисление, которые затем передаются в Оркестратор.
// @contact.name Ivan Kovach (Qu1nel)
// @contact.url https://github.com/Qu1nel
// @contact.email covach.qn@gmail.com
// @license.name MIT License
// @license.url https://github.com/Qu1nel/YaLyceum-GoProject-Final/blob/main/LICENSE
// @host localhost:8080
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT токен авторизации. Формат: "Bearer <токен>"
// @description Пример: "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
// @externalDocs.description Дополнительная документация по проекту (README)
// @externalDocs.url https://github.com/Qu1nel/YaLyceum-GoProject-Final/blob/main/README.md
// @tag.name Аутентификация
// @tag.description Эндпоинты для регистрации и входа пользователей
// @tag.name Задачи
// @tag.description Эндпоинты для управления задачами вычисления выражений
func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Неперехваченная паника в Agent: %v\n", r)
			os.Exit(1)
		}
	}()

	app.Run()
}