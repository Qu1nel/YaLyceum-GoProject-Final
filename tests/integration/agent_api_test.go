// Файл: tests/integration/agent_api_test.go
package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http" // Будем использовать для прямого вызова хендлера Агента, если понадобится, но лучше через HTTP клиент
	"strings"
	"testing"
	"time"

	// Типы из Agent для запросов/ответов (если они экспортируемы и нужны)
	// agent_handler "github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/handler"
	// agent_service "github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/service"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Структуры для ответов API, которые мы ожидаем
type RegisterResponse struct {
    Message string `json:"message"`
}
type LoginResponse struct {
    Token string `json:"token"`
}
type CalculateResponse struct {
    TaskID string `json:"task_id"`
}
type ErrorResponse struct {
    Error string `json:"error"`
}
type TaskDetailsResponse struct { // Упрощенная версия, т.к. полный TaskDetails в agent/service
    ID         string    `json:"id"`
    Expression string    `json:"expression"`
    Status     string    `json:"status"`
    Result     *float64  `json:"result,omitempty"`
    ErrorMsg   *string   `json:"error_message,omitempty"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}

type TaskListItemResponse struct {
	ID         string    `json:"id"`
	Expression string    `json:"expression"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"` // Для парсинга из JSON
}

// TestIntegration_RegisterLoginSubmitTask проверяет базовый сценарий
func TestIntegration_RegisterLoginSubmitTask(t *testing.T) {
	// TestMain уже всё настроил и запустил, testAgentBaseURL доступен
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")

	client := &http.Client{Timeout: 10 * time.Second}
	ctx := context.Background() // Для запросов

	// --- Шаг 1: Регистрация ---
	registerLogin := fmt.Sprintf("user_%d", time.Now().UnixNano()) // Уникальный логин
	registerPassword := "password123"
	registerPayload := map[string]string{"login": registerLogin, "password": registerPassword}
	registerBody, _ := json.Marshal(registerPayload)

	regReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/register", bytes.NewBuffer(registerBody))
	regReq.Header.Set("Content-Type", "application/json")
	
	regResp, err := client.Do(regReq)
	require.NoError(t, err, "Ошибка при запросе регистрации")
	defer regResp.Body.Close()
	assert.Equal(t, http.StatusOK, regResp.StatusCode, "Ожидался статус 200 OK при регистрации")

	var regResponseData RegisterResponse
	err = json.NewDecoder(regResp.Body).Decode(&regResponseData)
	require.NoError(t, err, "Ошибка декодирования ответа регистрации")
	assert.Equal(t, "Пользователь успешно зарегистрирован", regResponseData.Message)
	log.Printf("Интеграционный тест: Пользователь %s зарегистрирован.\n", registerLogin)


	// --- Шаг 2: Логин ---
	loginPayload := map[string]string{"login": registerLogin, "password": registerPassword}
	loginBody, _ := json.Marshal(loginPayload)

	loginReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := client.Do(loginReq)
	require.NoError(t, err, "Ошибка при запросе входа")
	defer loginResp.Body.Close()
	assert.Equal(t, http.StatusOK, loginResp.StatusCode, "Ожидался статус 200 OK при входе")

	var loginResponseData LoginResponse
	err = json.NewDecoder(loginResp.Body).Decode(&loginResponseData)
	require.NoError(t, err, "Ошибка декодирования ответа входа")
	require.NotEmpty(t, loginResponseData.Token, "Токен не должен быть пустым")
	jwtToken := loginResponseData.Token
	log.Printf("Интеграционный тест: Пользователь %s вошел, токен: %.10s...\n", registerLogin, jwtToken)


	// --- Шаг 3: Отправка выражения на вычисление ---
	expression := " (2 + 3) * 4 - 10 / 2 " // Выражение с пробелами, которые должны обработаться
	calcPayload := map[string]string{"expression": expression}
	calcBody, _ := json.Marshal(calcPayload)

	calcReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBody))
	calcReq.Header.Set("Content-Type", "application/json")
	calcReq.Header.Set("Authorization", "Bearer "+jwtToken)

	calcResp, err := client.Do(calcReq)
	require.NoError(t, err, "Ошибка при запросе /calculate")
	defer calcResp.Body.Close()

    // Читаем тело ответа для логирования, если не 202
    var calcResponseData CalculateResponse
    var errorResponseData ErrorResponse
    
    respBodyBytes, _ := io.ReadAll(calcResp.Body) // Читаем тело один раз
    // Восстанавливаем тело для последующего декодирования
    calcResp.Body = io.NopCloser(bytes.NewBuffer(respBodyBytes))


	if calcResp.StatusCode != http.StatusAccepted {
        // Попытаемся декодировать как ошибку
        if json.Unmarshal(respBodyBytes, &errorResponseData) == nil {
             t.Fatalf("Ожидался статус 202 Accepted при /calculate, получен %d. Тело ошибки: %s", calcResp.StatusCode, errorResponseData.Error)
        } else {
             t.Fatalf("Ожидался статус 202 Accepted при /calculate, получен %d. Тело: %s", calcResp.StatusCode, string(respBodyBytes))
        }
    }
	assert.Equal(t, http.StatusAccepted, calcResp.StatusCode, "Ожидался статус 202 Accepted при /calculate")

	err = json.NewDecoder(calcResp.Body).Decode(&calcResponseData) // Теперь декодируем из восстановленного тела
	require.NoError(t, err, "Ошибка декодирования ответа /calculate")
	require.NotEmpty(t, calcResponseData.TaskID, "TaskID не должен быть пустым")
	taskID := calcResponseData.TaskID
	log.Printf("Интеграционный тест: Задача %s для выражения '%s' отправлена.\n", taskID, expression)


	// --- Шаг 4: Проверка статуса задачи (ожидаем, что она быстро вычислится) ---
	// Даем немного времени на асинхронное вычисление
    // Сумма задержек: 10ms (сложение) + 10ms (умножение) + 10ms (деление) + 10ms (вычитание) = 40ms
    // Плюс накладные расходы gRPC и БД. 1 секунды должно хватить.
	time.Sleep(1 * time.Second)

	taskDetailsURL := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, taskID)
	detailsReq, _ := http.NewRequestWithContext(ctx, "GET", taskDetailsURL, nil)
	detailsReq.Header.Set("Authorization", "Bearer "+jwtToken)

	var taskDetails TaskDetailsResponse
	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		detailsResp, err := client.Do(detailsReq.Clone(ctx)) // Клонируем запрос для повторных попыток
		require.NoError(t, err, "Ошибка при запросе деталей задачи %s (попытка %d)", taskID, attempt)
		defer detailsResp.Body.Close()

        bodyBytes, _ := io.ReadAll(detailsResp.Body)
        detailsResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes)) // Восстанавливаем тело

		if detailsResp.StatusCode != http.StatusOK {
             var errResp ErrorResponse
             if json.Unmarshal(bodyBytes, &errResp) == nil {
                log.Printf("Попытка %d: Не удалось получить детали задачи %s, статус %d, ошибка: %s", attempt, taskID, detailsResp.StatusCode, errResp.Error)
             } else {
                log.Printf("Попытка %d: Не удалось получить детали задачи %s, статус %d, тело: %s", attempt, taskID, detailsResp.StatusCode, string(bodyBytes))
             }
             if attempt == maxAttempts {
                 t.Fatalf("Не удалось получить детали задачи %s после %d попыток, последний статус %d", taskID, maxAttempts, detailsResp.StatusCode)
             }
             time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // Увеличивающаяся задержка
             continue
        }

		err = json.NewDecoder(detailsResp.Body).Decode(&taskDetails)
		require.NoError(t, err, "Ошибка декодирования деталей задачи %s (попытка %d)", taskID, attempt)

		if taskDetails.Status == repository.StatusCompleted || taskDetails.Status == repository.StatusFailed {
			break // Задача завершена
		}

		log.Printf("Попытка %d: Задача %s все еще в статусе '%s', ждем...", attempt, taskID, taskDetails.Status)
		if attempt == maxAttempts {
			t.Fatalf("Задача %s не перешла в финальный статус после %d попыток (текущий статус: %s)", taskID, maxAttempts, taskDetails.Status)
		}
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond) // Увеличивающаяся задержка
	}

	log.Printf("Интеграционный тест: Детали задачи %s: Статус=%s, Результат=%v, Ошибка=%v\n",
		taskID, taskDetails.Status, taskDetails.Result, taskDetails.ErrorMsg)

	assert.Equal(t, repository.StatusCompleted, taskDetails.Status, "Ожидался статус 'completed'")
	require.NotNil(t, taskDetails.Result, "Результат не должен быть nil для завершенной задачи")
	assert.InDelta(t, 15.0, *taskDetails.Result, 0.00001, "Результат вычисления некорректен") // (2+3)*4 - 10/2 = 5*4 - 5 = 20 - 5 = 15
	assert.Nil(t, taskDetails.ErrorMsg, "Сообщение об ошибке должно быть nil для успешно завершенной задачи")
}


func TestIntegration_TasksAPI_ListAndGetOwnTasks(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 10 * time.Second}
	ctx := context.Background()

	// --- Шаг 1: Регистрация и Логин Пользователя A ---
	userALogin := fmt.Sprintf("userA_%d", time.Now().UnixNano()%1000000)
	userAPassword := "passwordA"

	// Регистрация
	regPayloadA := map[string]string{"login": userALogin, "password": userAPassword}
	regBodyA, _ := json.Marshal(regPayloadA)
	regReqA, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/register", bytes.NewBuffer(regBodyA))
	regReqA.Header.Set("Content-Type", "application/json")
	regRespA, err := client.Do(regReqA)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, regRespA.StatusCode, "Регистрация пользователя A должна вернуть 200")
	regRespA.Body.Close()

	// Логин
	loginPayloadA := map[string]string{"login": userALogin, "password": userAPassword}
	loginBodyA, _ := json.Marshal(loginPayloadA)
	loginReqA, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/login", bytes.NewBuffer(loginBodyA))
	loginReqA.Header.Set("Content-Type", "application/json")
	loginRespA, err := client.Do(loginReqA)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loginRespA.StatusCode, "Логин пользователя A должен вернуть 200")
	var loginDataA LoginResponse
	err = json.NewDecoder(loginRespA.Body).Decode(&loginDataA)
	require.NoError(t, err)
	loginRespA.Body.Close()
	tokenA := loginDataA.Token
	require.NotEmpty(t, tokenA)
	log.Printf("Интеграционный тест (ListOwnTasks): Пользователь %s вошел, токен получен.\n", userALogin)

	// --- Шаг 2: Создание нескольких задач Пользователем A ---
	expressions := []string{"10+20", "100/10", "2^3"}
	expectedResults := []float64{30.0, 10.0, 8.0}
	taskIDs := make([]string, len(expressions))

	for i, expr := range expressions {
		calcPayload := map[string]string{"expression": expr}
		calcBody, _ := json.Marshal(calcPayload)
		calcReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBody))
		calcReq.Header.Set("Content-Type", "application/json")
		calcReq.Header.Set("Authorization", "Bearer "+tokenA)
		calcResp, err := client.Do(calcReq)
		require.NoError(t, err)
		
		var calcData CalculateResponse
        bodyBytes, _ := io.ReadAll(calcResp.Body)
        calcResp.Body.Close() // Закрываем тело здесь
        calcResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		require.Equal(t, http.StatusAccepted, calcResp.StatusCode, "Создание задачи для '%s' должно вернуть 202", expr)
		err = json.NewDecoder(calcResp.Body).Decode(&calcData)
		require.NoError(t, err)
		taskIDs[i] = calcData.TaskID
		log.Printf("Интеграционный тест (ListOwnTasks): Задача %s для '%s' создана пользователем A.\n", taskIDs[i], expr)
	}

	// Даем время на асинхронное вычисление всех задач
	// Суммарная задержка для всех операций по 10ms. Плюс gRPC, БД.
	// 3 задачи * (несколько операций + накладные расходы)
	// Увеличим до 3 секунд для надежности
	log.Println("Интеграционный тест (ListOwnTasks): Ожидание вычисления задач...")
	time.Sleep(3 * time.Second)

	// --- Шаг 3: Получение списка задач Пользователя A ---
	tasksReq, _ := http.NewRequestWithContext(ctx, "GET", testAgentBaseURL+"/tasks", nil)
	tasksReq.Header.Set("Authorization", "Bearer "+tokenA)
	tasksResp, err := client.Do(tasksReq)
	require.NoError(t, err)
	defer tasksResp.Body.Close()
	require.Equal(t, http.StatusOK, tasksResp.StatusCode, "GET /tasks должен вернуть 200 OK")

	var userATasks []TaskListItemResponse // Используем нашу структуру для ответа
	err = json.NewDecoder(tasksResp.Body).Decode(&userATasks)
	require.NoError(t, err, "Ошибка декодирования списка задач")
	
	log.Printf("Интеграционный тест (ListOwnTasks): Получено %d задач для пользователя A.\n", len(userATasks))
	assert.Len(t, userATasks, len(expressions), "Количество полученных задач не совпадает с количеством созданных")

	// Проверяем, что все созданные ID задач присутствуют в списке
	// и что их статус completed (так как выражения простые и должны быстро вычислиться)
	returnedTaskIDs := make(map[string]bool)
	for _, task := range userATasks {
		returnedTaskIDs[task.ID] = true
		assert.Contains(t, expressions, task.Expression, "Выражение задачи не найдено среди оригинальных")
		assert.Equal(t, repository.StatusCompleted, task.Status, "Статус задачи %s должен быть 'completed'", task.ID)
	}
	for _, originalID := range taskIDs {
		assert.True(t, returnedTaskIDs[originalID], "Созданная задача с ID %s не найдена в списке", originalID)
	}

	// --- Шаг 4: Получение деталей одной из задач Пользователя A ---
	if len(taskIDs) > 0 {
		firstTaskID := taskIDs[0]
		firstExpression := expressions[0]
		firstExpectedResult := expectedResults[0]

		detailsURL := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, firstTaskID)
		detailsReq, _ := http.NewRequestWithContext(ctx, "GET", detailsURL, nil)
		detailsReq.Header.Set("Authorization", "Bearer "+tokenA)
		detailsResp, err := client.Do(detailsReq)
		require.NoError(t, err)
		defer detailsResp.Body.Close()
		require.Equal(t, http.StatusOK, detailsResp.StatusCode, "GET /tasks/{id} должен вернуть 200 OK для своей задачи")

		var taskDetails TaskDetailsResponse
		err = json.NewDecoder(detailsResp.Body).Decode(&taskDetails)
		require.NoError(t, err, "Ошибка декодирования деталей задачи")

		assert.Equal(t, firstTaskID, taskDetails.ID)
		assert.Equal(t, firstExpression, taskDetails.Expression)
		assert.Equal(t, repository.StatusCompleted, taskDetails.Status)
		require.NotNil(t, taskDetails.Result, "Результат не должен быть nil")
		assert.InDelta(t, firstExpectedResult, *taskDetails.Result, 0.00001)
		assert.Nil(t, taskDetails.ErrorMsg, "Сообщение об ошибке должно быть nil")
		log.Printf("Интеграционный тест (ListOwnTasks): Детали задачи %s успешно получены.\n", firstTaskID)
	}
}


func TestIntegration_SubmitExpressionThatFailsAtEvaluation(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 10 * time.Second}
	ctx := context.Background()

	// 1. Регистрация и логин
	timestampSuffix := time.Now().UnixNano() % 100000
	registerLogin := fmt.Sprintf("evalfail_usr_%d", timestampSuffix)
	if len(registerLogin) > 30 {
		registerLogin = registerLogin[:30]
	}
	registerPassword := "password123"

	regPayload := map[string]string{"login": registerLogin, "password": registerPassword}
	regBody, _ := json.Marshal(regPayload)
	regReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/register", bytes.NewBuffer(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := client.Do(regReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, regResp.StatusCode, "Регистрация должна вернуть 200")
	regResp.Body.Close()

	loginPayload := map[string]string{"login": registerLogin, "password": registerPassword}
	loginBody, _ := json.Marshal(loginPayload)
	loginReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, loginResp.StatusCode, "Логин должен вернуть 200")
	var loginData LoginResponse
	err = json.NewDecoder(loginResp.Body).Decode(&loginData)
	require.NoError(t, err)
	loginResp.Body.Close()
	jwtToken := loginData.Token
	require.NotEmpty(t, jwtToken)

	// 2. Отправляем выражение, которое expr распарсит, но наш Evaluator не сможет вычислить
	// Например, унарный плюс, который мы не обрабатываем, или неизвестная функция.
	// "2 +++ 3" expr может интерпретировать как "2+3".
	// Давай используем выражение, которое точно вызовет ошибку в нашем ExpressionEvaluator,
	// например, идентификатор (переменную), которую мы не поддерживаем.
	// ИЛИ выражение, которое приведет к ошибке в Воркере, например, деление на ноль.
	// expression := "a + 1" // Это вызовет ошибку парсинга в expr
	expression := "1 / 0" // Это должно вызвать ошибку вычисления в Воркере

	calcPayload := map[string]string{"expression": expression}
	calcBody, _ := json.Marshal(calcPayload)

	calcReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBody))
	calcReq.Header.Set("Content-Type", "application/json")
	calcReq.Header.Set("Authorization", "Bearer "+jwtToken)

	calcResp, err := client.Do(calcReq)
	require.NoError(t, err, "Ошибка при запросе /calculate")
	defer calcResp.Body.Close()

	// Ожидаем 202, так как парсинг "1/0" в expr пройдет успешно
	require.Equal(t, http.StatusAccepted, calcResp.StatusCode, "Ожидался статус 202 Accepted при /calculate")

	var calcResponseData CalculateResponse
	err = json.NewDecoder(calcResp.Body).Decode(&calcResponseData)
	require.NoError(t, err, "Ошибка декодирования ответа /calculate")
	require.NotEmpty(t, calcResponseData.TaskID, "TaskID не должен быть пустым")
	taskID := calcResponseData.TaskID
	log.Printf("Интеграционный тест (ошибка вычисления): Задача %s для '%s' отправлена.\n", taskID, expression)

	// 3. Проверяем статус задачи, ожидаем 'failed' и сообщение об ошибке
	time.Sleep(1 * time.Second) // Даем время на асинхронное вычисление

	taskDetailsURL := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, taskID)
	detailsReq, _ := http.NewRequestWithContext(ctx, "GET", taskDetailsURL, nil)
	detailsReq.Header.Set("Authorization", "Bearer "+jwtToken)

	var taskDetails TaskDetailsResponse
	maxAttempts := 5
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		detailsResp, errAttempt := client.Do(detailsReq.Clone(ctx))
		require.NoError(t, errAttempt)
		
		bodyBytes, _ := io.ReadAll(detailsResp.Body)
        detailsResp.Body.Close() // Закрываем здесь
        detailsResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))


		if detailsResp.StatusCode != http.StatusOK {
			t.Fatalf("Ожидался статус 200 OK при запросе деталей задачи, получен %d. Тело: %s", detailsResp.StatusCode, string(bodyBytes))
		}

		errAttempt = json.NewDecoder(detailsResp.Body).Decode(&taskDetails)
		require.NoError(t, errAttempt)

		if taskDetails.Status == repository.StatusFailed {
			break
		}
		if attempt == maxAttempts {
			t.Fatalf("Задача %s не перешла в статус 'failed' после %d попыток (текущий статус: %s)", taskID, maxAttempts, taskDetails.Status)
		}
		log.Printf("Попытка %d: Задача %s (ошибка вычисления) в статусе '%s', ждем...", attempt, taskID, taskDetails.Status)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}

	log.Printf("Интеграционный тест (ошибка вычисления): Детали задачи %s: Статус=%s, Ошибка=%v\n",
		taskID, taskDetails.Status, taskDetails.ErrorMsg)

	assert.Equal(t, repository.StatusFailed, taskDetails.Status, "Ожидался статус 'failed'")
	require.NotNil(t, taskDetails.ErrorMsg, "Сообщение об ошибке не должно быть nil для задачи со статусом failed")
	// Ожидаем ошибку, связанную с делением на ноль от воркера
	assert.Contains(t, *taskDetails.ErrorMsg, "деление на ноль", "Ожидалось сообщение об ошибке деления на ноль")
	assert.Nil(t, taskDetails.Result, "Результат должен быть nil для задачи со статусом failed")
}

func TestIntegration_TasksAPI_AccessControls(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 10 * time.Second}
	ctx := context.Background()

	// --- Шаг 1: Создание и логин Пользователя A ---
	userALogin := fmt.Sprintf("userA_access_%d", time.Now().UnixNano()%1000000)
	userAPassword := "passwordA"
	tokenA := registerAndLoginUser(t, client, ctx, userALogin, userAPassword)
	log.Printf("Интеграционный тест (AccessControls): Пользователь A (%s) вошел.\n", userALogin)

	// --- Шаг 2: Пользователь A создает задачу ---
	exprA := "100+200"
	calcPayloadA := map[string]string{"expression": exprA}
	calcBodyA, _ := json.Marshal(calcPayloadA)
	calcReqA, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBodyA))
	calcReqA.Header.Set("Content-Type", "application/json")
	calcReqA.Header.Set("Authorization", "Bearer "+tokenA)
	calcRespA, err := client.Do(calcReqA)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, calcRespA.StatusCode, "Создание задачи пользователем A должно вернуть 202")
	var calcDataA CalculateResponse
	err = json.NewDecoder(calcRespA.Body).Decode(&calcDataA)
	require.NoError(t, err)
	calcRespA.Body.Close()
	taskAID := calcDataA.TaskID
	require.NotEmpty(t, taskAID)
	log.Printf("Интеграционный тест (AccessControls): Пользователь A создал задачу %s.\n", taskAID)

	// Даем время на вычисление задачи A
	time.Sleep(1 * time.Second)

	// --- Шаг 3: Создание и логин Пользователя B ---
	userBLogin := fmt.Sprintf("userB_access_%d", time.Now().UnixNano()%1000000)
	userBPassword := "passwordB"
	tokenB := registerAndLoginUser(t, client, ctx, userBLogin, userBPassword)
	log.Printf("Интеграционный тест (AccessControls): Пользователь B (%s) вошел.\n", userBLogin)

	// --- Шаг 4: Пользователь B пытается получить детали задачи Пользователя A ---
	detailsURL_A_by_B := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, taskAID)
	detailsReq_A_by_B, _ := http.NewRequestWithContext(ctx, "GET", detailsURL_A_by_B, nil)
	detailsReq_A_by_B.Header.Set("Authorization", "Bearer "+tokenB) // Используем токен B
	detailsResp_A_by_B, err := client.Do(detailsReq_A_by_B)
	require.NoError(t, err)
	defer detailsResp_A_by_B.Body.Close()

	// Ожидаем 404 Not Found, так как пользователь B не должен видеть задачу A
	assert.Equal(t, http.StatusNotFound, detailsResp_A_by_B.StatusCode, "Пользователь B не должен иметь доступ к задаче пользователя A (ожидаем 404)")
	var errorDataB ErrorResponse
	err = json.NewDecoder(detailsResp_A_by_B.Body).Decode(&errorDataB)
	if err == nil { // Если удалось распарсить тело ошибки
		log.Printf("Интеграционный тест (AccessControls): Ошибка при доступе B к задаче A: %s\n", errorDataB.Error)
		assert.Contains(t, errorDataB.Error, "не найдена", "Сообщение об ошибке должно указывать на 'не найдена'")
	} else {
		log.Printf("Интеграционный тест (AccessControls): Не удалось распарсить тело ошибки при доступе B к задаче A (статус %d)\n", detailsResp_A_by_B.StatusCode)
	}


	// --- Шаг 5: Пользователь B запрашивает свой список задач (должен быть пустым) ---
	tasksReqB, _ := http.NewRequestWithContext(ctx, "GET", testAgentBaseURL+"/tasks", nil)
	tasksReqB.Header.Set("Authorization", "Bearer "+tokenB)
	tasksRespB, err := client.Do(tasksReqB)
	require.NoError(t, err)
	defer tasksRespB.Body.Close()
	require.Equal(t, http.StatusOK, tasksRespB.StatusCode, "GET /tasks для пользователя B должен вернуть 200 OK")

	var userBTasks []TaskListItemResponse
	err = json.NewDecoder(tasksRespB.Body).Decode(&userBTasks)
	require.NoError(t, err, "Ошибка декодирования списка задач пользователя B")
	assert.Empty(t, userBTasks, "Список задач пользователя B должен быть пустым")
	log.Printf("Интеграционный тест (AccessControls): Список задач пользователя B пуст, как и ожидалось.\n")

	// --- Шаг 6: Пользователь A запрашивает детали своей задачи (должен получить) ---
	detailsURL_A_by_A := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, taskAID)
	detailsReq_A_by_A, _ := http.NewRequestWithContext(ctx, "GET", detailsURL_A_by_A, nil)
	detailsReq_A_by_A.Header.Set("Authorization", "Bearer "+tokenA) // Используем токен A
	detailsResp_A_by_A, err := client.Do(detailsReq_A_by_A)
	require.NoError(t, err)
	defer detailsResp_A_by_A.Body.Close()
	assert.Equal(t, http.StatusOK, detailsResp_A_by_A.StatusCode, "Пользователь A должен иметь доступ к своей задаче")
	log.Printf("Интеграционный тест (AccessControls): Пользователь A успешно получил детали своей задачи %s.\n", taskAID)
}

// Вспомогательная функция для регистрации и логина пользователя
func registerAndLoginUser(t *testing.T, client *http.Client, ctx context.Context, login, password string) string {
	t.Helper() // Помечаем как хелпер-функцию

	// Регистрация
	regPayload := map[string]string{"login": login, "password": password}
	regBody, _ := json.Marshal(regPayload)
	regReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/register", bytes.NewBuffer(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := client.Do(regReq)
	require.NoError(t, err, "Ошибка регистрации в хелпере для %s", login)
    
    regBodyBytes, _ := io.ReadAll(regResp.Body)
    regResp.Body.Close()
    regResp.Body = io.NopCloser(bytes.NewBuffer(regBodyBytes))
	if regResp.StatusCode != http.StatusOK {
        var errResp ErrorResponse
        if json.Unmarshal(regBodyBytes, &errResp) == nil {
            // Проверяем, не является ли это ошибкой "логин уже существует" (409)
            if regResp.StatusCode == http.StatusConflict && strings.Contains(errResp.Error, "уже существует") {
                log.Printf("Хелпер: Пользователь %s уже существует, пропускаем регистрацию.\n", login)
            } else {
                t.Fatalf("Регистрация в хелпере для %s не удалась: статус %d, ошибка: %s", login, regResp.StatusCode, errResp.Error)
            }
        } else {
            t.Fatalf("Регистрация в хелпере для %s не удалась: статус %d, тело: %s", login, regResp.StatusCode, string(regBodyBytes))
        }
    }


	// Логин
	loginPayload := map[string]string{"login": login, "password": password}
	loginBody, _ := json.Marshal(loginPayload)
	loginReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	require.NoError(t, err, "Ошибка логина в хелпере для %s", login)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode, "Логин в хелпере для %s должен вернуть 200 OK", login)
	var loginData LoginResponse
	err = json.NewDecoder(loginResp.Body).Decode(&loginData)
	require.NoError(t, err, "Ошибка декодирования ответа логина в хелпере для %s", login)
	require.NotEmpty(t, loginData.Token, "Токен не должен быть пустым в хелпере для %s", login)
	return loginData.Token
}

func TestIntegration_AuthErrors(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()

	// Сценарий 1: Попытка доступа к /calculate без токена
	t.Run("CalculateWithoutToken", func(t *testing.T) {
		calcPayload := map[string]string{"expression": "1+1"}
		calcBody, _ := json.Marshal(calcPayload)
		calcReq, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBody))
		calcReq.Header.Set("Content-Type", "application/json")
		// НЕТ ЗАГОЛОВКА Authorization

		calcResp, err := client.Do(calcReq)
		require.NoError(t, err)
		defer calcResp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, calcResp.StatusCode, "Ожидался 401 Unauthorized без токена")

		log.Printf("Интеграционный тест (AuthErrors/NoToken): /calculate без токена вернул %d\n", calcResp.StatusCode)
	})

	// Сценарий 2: Попытка доступа к /tasks с невалидным токеном
	t.Run("TasksWithInvalidToken", func(t *testing.T) {
		tasksReq, _ := http.NewRequestWithContext(ctx, "GET", testAgentBaseURL+"/tasks", nil)
		tasksReq.Header.Set("Authorization", "Bearer an.invalid.token.here")

		tasksResp, err := client.Do(tasksReq)
		require.NoError(t, err)
		defer tasksResp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, tasksResp.StatusCode, "Ожидался 401 Unauthorized с невалидным токеном")
		
		log.Printf("Интеграционный тест (AuthErrors/InvalidToken): /tasks с невалидным токеном вернул %d\n", tasksResp.StatusCode)
	})
}

func TestIntegration_RegisterExistingLogin(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()

	login := fmt.Sprintf("existing_user_%d", time.Now().UnixNano()%1000000)
	password := "password123"
	payload := map[string]string{"login": login, "password": password}
	body, _ := json.Marshal(payload)

	// 1. Первая (успешная) регистрация
	req1, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/register", bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode, "Первая регистрация должна быть успешной")
	log.Printf("Интеграционный тест (RegisterExisting): Пользователь %s успешно зарегистрирован первый раз.\n", login)


	// 2. Повторная регистрация с тем же логином
	req2, _ := http.NewRequestWithContext(ctx, "POST", testAgentBaseURL+"/register", bytes.NewBuffer(body)) // Используем то же тело
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()

	assert.Equal(t, http.StatusConflict, resp2.StatusCode, "Повторная регистрация с тем же логином должна вернуть 409 Conflict")

	log.Printf("Интеграционный тест (RegisterExisting): Повторная регистрация пользователя %s вернула %d, как и ожидалось.\n", login, resp2.StatusCode)
}