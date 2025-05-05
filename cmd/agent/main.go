package main

import (
	"fmt"
	"os"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/app"
)

// @title Expression Calculator API - Agent
// @version 1.0
// @description Это API Агента для сервиса Калькулятора Выражений. Он обрабатывает аутентификацию и пересылает запросы на вычисление.
// @contact.name API Support
// @contact.url http://www.example.com/support
// @contact.email support@example.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:8080
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите "Bearer" пробел и затем JWT токен.
func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Неперехваченная паника: %v\n", r)
			// Можно добавить вывод стектрейса runtime/debug.Stack()
			os.Exit(1)
		}
	}()

	app.Run()
}