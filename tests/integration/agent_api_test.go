package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository" // Используется для констант статусов
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RegisterResponse структура ответа для регистрации.
type RegisterResponse struct {
	Message string `json:"message"`
}

// LoginResponse структура ответа для входа.
type LoginResponse struct {
	Token string `json:"token"`
}

// CalculateResponse структура ответа для запроса на вычисление.
type CalculateResponse struct {
	TaskID string `json:"task_id"`
}

// ErrorResponse структура для ошибок API.
type ErrorResponse struct {
	Error string `json:"error"`
}

// TaskDetailsResponse структура для деталей задачи.
type TaskDetailsResponse struct {
	ID         string    `json:"id"`
	Expression string    `json:"expression"`
	Status     string    `json:"status"`
	Result     *float64  `json:"result,omitempty"` // Указатель, чтобы JSON мог быть nil
	ErrorMsg   *string   `json:"error_message,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TaskListItemResponse структура для элемента в списке задач.
type TaskListItemResponse struct {
	ID         string    `json:"id"`
	Expression string    `json:"expression"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// TestIntegration_RegisterLoginSubmitTask проверяет полный цикл: регистрация, логин, отправка задачи, проверка результата.
func TestIntegration_RegisterLoginSubmitTask(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 15 * time.Second} // Увеличен таймаут для клиента
	ctx := context.Background()

	// Шаг 1: Регистрация
	registerLogin := fmt.Sprintf("user_%d", time.Now().UnixNano())
	registerPassword := "password123"
	registerPayload := map[string]string{"login": registerLogin, "password": registerPassword}
	registerBody, _ := json.Marshal(registerPayload)

	regReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/register", bytes.NewBuffer(registerBody))
	regReq.Header.Set("Content-Type", "application/json")

	regResp, err := client.Do(regReq)
	require.NoError(t, err, "Ошибка при запросе регистрации")
	defer regResp.Body.Close()
	assert.Equal(t, http.StatusOK, regResp.StatusCode, "Регистрация: ожидался статус 200 OK")

	var regData RegisterResponse
	require.NoError(t, json.NewDecoder(regResp.Body).Decode(&regData), "Регистрация: ошибка декодирования ответа")
	assert.Equal(t, "Пользователь успешно зарегистрирован", regData.Message)
	log.Printf("Интеграционный тест: Пользователь %s зарегистрирован.\n", registerLogin)

	// Шаг 2: Логин
	loginPayload := map[string]string{"login": registerLogin, "password": registerPassword}
	loginBody, _ := json.Marshal(loginPayload)
	loginReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")

	loginResp, err := client.Do(loginReq)
	require.NoError(t, err, "Ошибка при запросе входа")
	defer loginResp.Body.Close()
	assert.Equal(t, http.StatusOK, loginResp.StatusCode, "Вход: ожидался статус 200 OK")

	var loginData LoginResponse
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&loginData), "Вход: ошибка декодирования ответа")
	require.NotEmpty(t, loginData.Token, "Токен не должен быть пустым")
	jwtToken := loginData.Token
	log.Printf("Интеграционный тест: Пользователь %s вошел, токен получен.\n", registerLogin)

	// Шаг 3: Отправка выражения на вычисление
	expression := " (2 + 3) * 4 - 10 / 2 " // Ожидаемый результат: 15
	calcPayload := map[string]string{"expression": expression}
	calcBody, _ := json.Marshal(calcPayload)
	calcReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBody))
	calcReq.Header.Set("Content-Type", "application/json")
	calcReq.Header.Set("Authorization", "Bearer "+jwtToken)

	calcResp, err := client.Do(calcReq)
	require.NoError(t, err, "Ошибка при запросе /calculate")

	// Читаем тело ответа для возможного логирования и последующего декодирования
	respBodyBytes, readErr := io.ReadAll(calcResp.Body)
	require.NoError(t, readErr, "Не удалось прочитать тело ответа /calculate")
	calcResp.Body.Close()                                        // Закрываем оригинальное тело
	calcResp.Body = io.NopCloser(bytes.NewBuffer(respBodyBytes)) // Восстанавливаем для декодера

	if calcResp.StatusCode != http.StatusAccepted {
		var errResp ErrorResponse
		if json.Unmarshal(respBodyBytes, &errResp) == nil {
			t.Fatalf("/calculate: ожидался статус 202, получен %d. Ошибка: %s", calcResp.StatusCode, errResp.Error)
		}
		t.Fatalf("/calculate: ожидался статус 202, получен %d. Тело: %s", calcResp.StatusCode, string(respBodyBytes))
	}

	var calcData CalculateResponse
	require.NoError(t, json.NewDecoder(calcResp.Body).Decode(&calcData), "/calculate: ошибка декодирования ответа")
	require.NotEmpty(t, calcData.TaskID, "TaskID не должен быть пустым")
	taskID := calcData.TaskID
	log.Printf("Интеграционный тест: Задача %s для '%s' отправлена.\n", taskID, expression)

	// Шаг 4: Проверка статуса задачи
	taskDetailsURL := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, taskID)
	detailsReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, taskDetailsURL, nil)
	detailsReq.Header.Set("Authorization", "Bearer "+jwtToken)

	var taskDetails TaskDetailsResponse
	const maxAttempts = 7 // Увеличено число попыток
	const initialDelay = 200 * time.Millisecond
	currentDelay := initialDelay

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		detailsResp, detailsErr := client.Do(detailsReq.Clone(ctx)) // Клонируем запрос
		require.NoError(t, detailsErr, "Детали задачи (попытка %d): ошибка запроса", attempt)

		bodyBytes, _ := io.ReadAll(detailsResp.Body)
		detailsResp.Body.Close()
		detailsResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		if detailsResp.StatusCode != http.StatusOK {
			var errResp ErrorResponse
			json.Unmarshal(bodyBytes, &errResp) // Игнорируем ошибку анмаршалинга, если тело не JSON
			log.Printf("Детали задачи (попытка %d): статус %d, ошибка: %s, тело: %s", attempt, detailsResp.StatusCode, errResp.Error, string(bodyBytes))
			if attempt == maxAttempts {
				t.Fatalf("Не удалось получить детали задачи %s после %d попыток, последний статус %d", taskID, maxAttempts, detailsResp.StatusCode)
			}
			time.Sleep(currentDelay)
			currentDelay *= 2 // Экспоненциальная задержка
			continue
		}

		require.NoError(t, json.NewDecoder(detailsResp.Body).Decode(&taskDetails), "Детали задачи (попытка %d): ошибка декодирования", attempt)

		if taskDetails.Status == repository.StatusCompleted || taskDetails.Status == repository.StatusFailed {
			break // Задача достигла финального статуса
		}

		log.Printf("Детали задачи (попытка %d): статус '%s', ждем...", attempt, taskDetails.Status)
		if attempt == maxAttempts {
			t.Fatalf("Задача %s не перешла в финальный статус (статус: %s)", taskID, taskDetails.Status)
		}
		time.Sleep(currentDelay)
		currentDelay *= 2
	}

	log.Printf("Интеграционный тест: Детали задачи %s: Статус=%s, Результат=%v, Ошибка=%v\n",
		taskID, taskDetails.Status, taskDetails.Result, taskDetails.ErrorMsg)

	assert.Equal(t, repository.StatusCompleted, taskDetails.Status, "Ожидался статус 'completed'")
	require.NotNil(t, taskDetails.Result, "Результат не должен быть nil")
	assert.InDelta(t, 15.0, *taskDetails.Result, 0.00001, "Результат вычисления некорректен")
	assert.Nil(t, taskDetails.ErrorMsg, "Сообщение об ошибке должно быть nil")
}

// TestIntegration_TasksAPI_ListAndGetOwnTasks проверяет получение списка задач и доступ к конкретной задаче.
func TestIntegration_TasksAPI_ListAndGetOwnTasks(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 15 * time.Second}
	ctx := context.Background()

	// Пользователь A
	userALogin := fmt.Sprintf("userA_list_%d", time.Now().UnixNano()%1000000)
	tokenA := registerAndLoginUser(t, client, ctx, userALogin, "passwordA")
	log.Printf("ListOwnTasks: Пользователь A (%s) вошел.\n", userALogin)

	// Создание задач Пользователем A
	expressions := []string{"10+20", "100/10", "2^3"}
	expectedResults := []float64{30.0, 10.0, 8.0}
	taskIDsA := make([]string, len(expressions))

	for i, expr := range expressions {
		calcPayload := map[string]string{"expression": expr}
		calcBody, _ := json.Marshal(calcPayload)
		calcReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBody))
		calcReq.Header.Set("Content-Type", "application/json")
		calcReq.Header.Set("Authorization", "Bearer "+tokenA)

		calcResp, err := client.Do(calcReq)
		require.NoError(t, err)

		var calcData CalculateResponse
		bodyBytes, _ := io.ReadAll(calcResp.Body)
		calcResp.Body.Close()
		calcResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		require.Equal(t, http.StatusAccepted, calcResp.StatusCode, "Создание задачи для '%s' должно вернуть 202", expr)
		require.NoError(t, json.NewDecoder(calcResp.Body).Decode(&calcData))
		taskIDsA[i] = calcData.TaskID
	}
	log.Printf("ListOwnTasks: Пользователем A создано %d задач.\n", len(taskIDsA))
	time.Sleep(3 * time.Second) // Ожидание вычисления

	// Получение списка задач Пользователя A
	tasksReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, testAgentBaseURL+"/tasks", nil)
	tasksReq.Header.Set("Authorization", "Bearer "+tokenA)
	tasksResp, err := client.Do(tasksReq)
	require.NoError(t, err)
	defer tasksResp.Body.Close()
	require.Equal(t, http.StatusOK, tasksResp.StatusCode, "GET /tasks должен вернуть 200")

	var userATasks []TaskListItemResponse
	require.NoError(t, json.NewDecoder(tasksResp.Body).Decode(&userATasks), "Ошибка декодирования списка задач")
	assert.Len(t, userATasks, len(expressions), "Количество полученных задач не совпадает")

	returnedTaskIDs := make(map[string]bool)
	for _, task := range userATasks {
		returnedTaskIDs[task.ID] = true
		assert.Contains(t, expressions, task.Expression)
		assert.Equal(t, repository.StatusCompleted, task.Status, "Статус задачи %s должен быть 'completed'", task.ID)
	}
	for _, originalID := range taskIDsA {
		assert.True(t, returnedTaskIDs[originalID], "Задача %s не найдена в списке", originalID)
	}

	// Получение деталей одной из задач Пользователя A
	if len(taskIDsA) > 0 {
		detailsURL := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, taskIDsA[0])
		detailsReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, detailsURL, nil)
		detailsReq.Header.Set("Authorization", "Bearer "+tokenA)
		detailsResp, err := client.Do(detailsReq)
		require.NoError(t, err)
		defer detailsResp.Body.Close()
		require.Equal(t, http.StatusOK, detailsResp.StatusCode, "GET /tasks/{id} должен вернуть 200")

		var taskDetails TaskDetailsResponse
		require.NoError(t, json.NewDecoder(detailsResp.Body).Decode(&taskDetails))
		assert.Equal(t, taskIDsA[0], taskDetails.ID)
		assert.Equal(t, expressions[0], taskDetails.Expression)
		assert.Equal(t, repository.StatusCompleted, taskDetails.Status)
		require.NotNil(t, taskDetails.Result)
		assert.InDelta(t, expectedResults[0], *taskDetails.Result, 0.00001)
	}
}

// TestIntegration_SubmitExpressionThatFailsAtEvaluation проверяет обработку задачи, которая падает при вычислении.
func TestIntegration_SubmitExpressionThatFailsAtEvaluation(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 10 * time.Second}
	ctx := context.Background()

	userLogin := fmt.Sprintf("evalfail_usr_%d", time.Now().UnixNano()%100000)
	jwtToken := registerAndLoginUser(t, client, ctx, userLogin, "password123")

	expression := "1 / 0" // Деление на ноль
	calcPayload := map[string]string{"expression": expression}
	calcBody, _ := json.Marshal(calcPayload)
	calcReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBody))
	calcReq.Header.Set("Content-Type", "application/json")
	calcReq.Header.Set("Authorization", "Bearer "+jwtToken)

	calcResp, err := client.Do(calcReq)
	require.NoError(t, err)
	defer calcResp.Body.Close()
	require.Equal(t, http.StatusAccepted, calcResp.StatusCode, "/calculate с '1/0' должен вернуть 202")

	var calcData CalculateResponse
	require.NoError(t, json.NewDecoder(calcResp.Body).Decode(&calcData))
	taskID := calcData.TaskID
	log.Printf("Ошибка вычисления: Задача %s для '%s' отправлена.\n", taskID, expression)

	time.Sleep(1500 * time.Millisecond) // Ожидание

	taskDetailsURL := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, taskID)
	detailsReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, taskDetailsURL, nil)
	detailsReq.Header.Set("Authorization", "Bearer "+jwtToken)

	var taskDetails TaskDetailsResponse
	const maxAttempts = 5
	currentDelay := 200 * time.Millisecond

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		detailsResp, detailsErr := client.Do(detailsReq.Clone(ctx))
		require.NoError(t, detailsErr)

		bodyBytes, _ := io.ReadAll(detailsResp.Body)
		detailsResp.Body.Close()
		detailsResp.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		require.Equal(t, http.StatusOK, detailsResp.StatusCode, "Детали задачи (ошибка): попытка %d, статус %d, тело: %s", attempt, detailsResp.StatusCode, string(bodyBytes))
		require.NoError(t, json.NewDecoder(detailsResp.Body).Decode(&taskDetails))

		if taskDetails.Status == repository.StatusFailed {
			break
		}
		if attempt == maxAttempts {
			t.Fatalf("Задача %s не перешла в 'failed' (статус: %s)", taskID, taskDetails.Status)
		}
		log.Printf("Ошибка вычисления: Задача %s в статусе '%s', ждем...", taskID, taskDetails.Status)
		time.Sleep(currentDelay)
		currentDelay *= 2
	}

	assert.Equal(t, repository.StatusFailed, taskDetails.Status, "Ожидался статус 'failed'")
	require.NotNil(t, taskDetails.ErrorMsg, "Сообщение об ошибке не должно быть nil")
	assert.Contains(t, strings.ToLower(*taskDetails.ErrorMsg), "деление на ноль", "Ожидалось сообщение об ошибке деления на ноль")
	assert.Nil(t, taskDetails.Result, "Результат должен быть nil для задачи 'failed'")
}

// TestIntegration_TasksAPI_AccessControls проверяет контроль доступа к задачам.
func TestIntegration_TasksAPI_AccessControls(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 10 * time.Second}
	ctx := context.Background()

	// Пользователь A
	userALogin := fmt.Sprintf("userA_access_%d", time.Now().UnixNano()%1000000)
	tokenA := registerAndLoginUser(t, client, ctx, userALogin, "passwordA")

	// Пользователь A создает задачу
	calcPayloadA := map[string]string{"expression": "1+1"}
	calcBodyA, _ := json.Marshal(calcPayloadA)
	calcReqA, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBodyA))
	calcReqA.Header.Set("Content-Type", "application/json")
	calcReqA.Header.Set("Authorization", "Bearer "+tokenA)
	calcRespA, err := client.Do(calcReqA)
	require.NoError(t, err)
	require.Equal(t, http.StatusAccepted, calcRespA.StatusCode)
	var calcDataA CalculateResponse
	require.NoError(t, json.NewDecoder(calcRespA.Body).Decode(&calcDataA))
	calcRespA.Body.Close()
	taskAID := calcDataA.TaskID
	time.Sleep(1 * time.Second) // Ожидание

	// Пользователь B
	userBLogin := fmt.Sprintf("userB_access_%d", time.Now().UnixNano()%1000000)
	tokenB := registerAndLoginUser(t, client, ctx, userBLogin, "passwordB")

	// Пользователь B пытается получить детали задачи Пользователя A
	detailsURL_A_by_B := fmt.Sprintf("%s/tasks/%s", testAgentBaseURL, taskAID)
	detailsReq_A_by_B, _ := http.NewRequestWithContext(ctx, http.MethodGet, detailsURL_A_by_B, nil)
	detailsReq_A_by_B.Header.Set("Authorization", "Bearer "+tokenB)
	detailsResp_A_by_B, err := client.Do(detailsReq_A_by_B)
	require.NoError(t, err)
	defer detailsResp_A_by_B.Body.Close()
	assert.Equal(t, http.StatusNotFound, detailsResp_A_by_B.StatusCode, "Пользователь B не должен иметь доступ к задаче A")

	var errorDataB ErrorResponse
	if json.NewDecoder(detailsResp_A_by_B.Body).Decode(&errorDataB) == nil {
		assert.Contains(t, errorDataB.Error, "не найдена", "Сообщение об ошибке 'не найдена'")
	}

	// Пользователь B запрашивает свой список задач (пустой)
	tasksReqB, _ := http.NewRequestWithContext(ctx, http.MethodGet, testAgentBaseURL+"/tasks", nil)
	tasksReqB.Header.Set("Authorization", "Bearer "+tokenB)
	tasksRespB, err := client.Do(tasksReqB)
	require.NoError(t, err)
	defer tasksRespB.Body.Close()
	require.Equal(t, http.StatusOK, tasksRespB.StatusCode)
	var userBTasks []TaskListItemResponse
	require.NoError(t, json.NewDecoder(tasksRespB.Body).Decode(&userBTasks))
	assert.Empty(t, userBTasks, "Список задач пользователя B должен быть пустым")

	// Пользователь A запрашивает детали своей задачи (успех)
	detailsReq_A_by_A, _ := http.NewRequestWithContext(ctx, http.MethodGet, detailsURL_A_by_B, nil) // Используем тот же URL
	detailsReq_A_by_A.Header.Set("Authorization", "Bearer "+tokenA)
	detailsResp_A_by_A, err := client.Do(detailsReq_A_by_A)
	require.NoError(t, err)
	defer detailsResp_A_by_A.Body.Close()
	assert.Equal(t, http.StatusOK, detailsResp_A_by_A.StatusCode, "Пользователь A должен иметь доступ к своей задаче")
}

// registerAndLoginUser вспомогательная функция для регистрации и логина.
func registerAndLoginUser(t *testing.T, client *http.Client, ctx context.Context, login, password string) string {
	t.Helper()

	// Регистрация
	regPayload := map[string]string{"login": login, "password": password}
	regBody, _ := json.Marshal(regPayload)
	regReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/register", bytes.NewBuffer(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regResp, err := client.Do(regReq)
	require.NoError(t, err, "Регистрация (хелпер) для %s: ошибка запроса", login)

	regBodyBytes, _ := io.ReadAll(regResp.Body)
	regResp.Body.Close()
	regResp.Body = io.NopCloser(bytes.NewBuffer(regBodyBytes))

	if regResp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(regBodyBytes, &errResp) == nil && regResp.StatusCode == http.StatusConflict && strings.Contains(errResp.Error, "уже существует") {
			log.Printf("Хелпер: Пользователь %s уже существует, пропускаем регистрацию.\n", login)
		} else {
			t.Fatalf("Регистрация (хелпер) для %s: статус %d, ошибка: %v, тело: %s", login, regResp.StatusCode, errResp.Error, string(regBodyBytes))
		}
	}

	// Логин
	loginPayload := map[string]string{"login": login, "password": password}
	loginBody, _ := json.Marshal(loginPayload)
	loginReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/login", bytes.NewBuffer(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginResp, err := client.Do(loginReq)
	require.NoError(t, err, "Логин (хелпер) для %s: ошибка запроса", login)
	defer loginResp.Body.Close()
	require.Equal(t, http.StatusOK, loginResp.StatusCode, "Логин (хелпер) для %s: ожидался 200 OK", login)

	var loginData LoginResponse
	require.NoError(t, json.NewDecoder(loginResp.Body).Decode(&loginData), "Логин (хелпер) для %s: ошибка декодирования", login)
	require.NotEmpty(t, loginData.Token, "Логин (хелпер) для %s: токен пуст", login)
	return loginData.Token
}

// TestIntegration_AuthErrors проверяет ошибки аутентификации.
func TestIntegration_AuthErrors(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()

	t.Run("CalculateWithoutToken", func(t *testing.T) {
		calcPayload := map[string]string{"expression": "1+1"}
		calcBody, _ := json.Marshal(calcPayload)
		calcReq, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/calculate", bytes.NewBuffer(calcBody))
		calcReq.Header.Set("Content-Type", "application/json")

		calcResp, err := client.Do(calcReq)
		require.NoError(t, err)
		defer calcResp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, calcResp.StatusCode, "/calculate без токена: ожидался 401")
	})

	t.Run("TasksWithInvalidToken", func(t *testing.T) {
		tasksReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, testAgentBaseURL+"/tasks", nil)
		tasksReq.Header.Set("Authorization", "Bearer an.invalid.token.here")

		tasksResp, err := client.Do(tasksReq)
		require.NoError(t, err)
		defer tasksResp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, tasksResp.StatusCode, "/tasks с невалидным токеном: ожидался 401")
	})
}

// TestIntegration_RegisterExistingLogin проверяет регистрацию с уже существующим логином.
func TestIntegration_RegisterExistingLogin(t *testing.T) {
	require.NotEmpty(t, testAgentBaseURL, "Базовый URL Агента не должен быть пустым")
	client := &http.Client{Timeout: 5 * time.Second}
	ctx := context.Background()

	login := fmt.Sprintf("existing_user_%d", time.Now().UnixNano()%1000000)
	payload := map[string]string{"login": login, "password": "password123"}
	body, _ := json.Marshal(payload)

	// Первая регистрация
	req1, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/register", bytes.NewBuffer(body))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := client.Do(req1)
	require.NoError(t, err)
	defer resp1.Body.Close()
	require.Equal(t, http.StatusOK, resp1.StatusCode, "Первая регистрация должна быть успешной")

	// Повторная регистрация
	req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, testAgentBaseURL+"/register", bytes.NewBuffer(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req2)
	require.NoError(t, err)
	defer resp2.Body.Close()
	assert.Equal(t, http.StatusConflict, resp2.StatusCode, "Повторная регистрация должна вернуть 409 Conflict")
}
