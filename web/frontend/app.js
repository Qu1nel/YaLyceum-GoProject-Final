document.addEventListener('DOMContentLoaded', () => {
    // URL нашего API
    const API_BASE_URL = 'http://localhost:8080/api/v1'; // Убедись, что порт совпадает с Agent

    // Элементы DOM
    const messagesDiv = document.getElementById('messages');

    const authSection = document.getElementById('authSection');
    const registerFormContainer = document.getElementById('registerFormContainer');
    const loginFormContainer = document.getElementById('loginFormContainer');
    const registerForm = document.getElementById('registerForm');
    const loginForm = document.getElementById('loginForm');
    const showLoginLink = document.getElementById('showLoginLink');
    const showRegisterLink = document.getElementById('showRegisterLink');

    const calculatorSection = document.getElementById('calculatorSection');
    const currentUserLoginSpan = document.getElementById('currentUserLogin');
    const logoutButton = document.getElementById('logoutButton');
    const expressionForm = document.getElementById('expressionForm');
    const expressionInput = document.getElementById('expressionInput');

    const refreshTasksButton = document.getElementById('refreshTasksButton');
    const taskListUl = document.getElementById('taskList');
    const taskDetailsViewDiv = document.getElementById('taskDetailsView');

    let jwtToken = localStorage.getItem('jwtToken');
    let currentUserID = localStorage.getItem('userID'); // Мы пока не получаем userID при логине, но можем хранить логин
    let currentUserLogin = localStorage.getItem('userLogin');

    // --- Функции для отображения сообщений ---
    function showMessage(message, type = 'success') {
        messagesDiv.textContent = message;
        messagesDiv.className = type; // 'success' или 'error'
        messagesDiv.classList.remove('hidden');
        setTimeout(() => { // Скрываем сообщение через 5 секунд
            messagesDiv.classList.add('hidden');
            messagesDiv.textContent = '';
            messagesDiv.className = '';
        }, 5000);
    }

    // --- Функции для работы с API ---
    async function apiRequest(endpoint, method = 'GET', body = null, requiresAuth = false) {
        const headers = { 'Content-Type': 'application/json' };
        if (requiresAuth && jwtToken) {
            headers['Authorization'] = `Bearer ${jwtToken}`;
        }

        const config = {
            method: method,
            headers: headers,
        };

        if (body) {
            config.body = JSON.stringify(body);
        }

        try {
            const response = await fetch(`${API_BASE_URL}${endpoint}`, config);
            const responseData = await response.json().catch(() => ({})); // Попытка парсить JSON, или пустой объект при ошибке

            if (!response.ok) {
                // Используем сообщение из ответа API, если оно есть, иначе стандартное
                const errorMessage = responseData.error || `Ошибка: ${response.status} ${response.statusText}`;
                throw new Error(errorMessage);
            }
            return responseData; // Для GET и успешных POST/PUT
        } catch (error) {
            console.error(`API ошибка для ${method} ${endpoint}:`, error);
            showMessage(error.message || 'Произошла сетевая ошибка или ошибка сервера', 'error');
            throw error; // Перебрасываем ошибку для обработки в вызывающем коде
        }
    }

    // --- Логика Аутентификации ---
    function updateAuthState() {
        if (jwtToken && currentUserLogin) {
            authSection.classList.add('hidden');
            calculatorSection.classList.remove('hidden');
            currentUserLoginSpan.textContent = currentUserLogin;
            fetchTasks(); // Загружаем задачи при входе
        } else {
            authSection.classList.remove('hidden');
            calculatorSection.classList.add('hidden');
            currentUserLoginSpan.textContent = '';
            taskListUl.innerHTML = ''; // Очищаем список задач
            taskDetailsViewDiv.classList.add('hidden');
            taskDetailsViewDiv.innerHTML = '';
        }
    }

    showLoginLink.addEventListener('click', (e) => {
        e.preventDefault();
        registerFormContainer.classList.add('hidden');
        loginFormContainer.classList.remove('hidden');
        messagesDiv.classList.add('hidden');
    });

    showRegisterLink.addEventListener('click', (e) => {
        e.preventDefault();
        loginFormContainer.classList.add('hidden');
        registerFormContainer.classList.remove('hidden');
        messagesDiv.classList.add('hidden');
    });

    registerForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const login = document.getElementById('registerLogin').value;
        const password = document.getElementById('registerPassword').value;
        try {
            await apiRequest('/register', 'POST', { login, password });
            showMessage('Регистрация прошла успешно! Теперь вы можете войти.', 'success');
            registerForm.reset();
            // Автоматически переключаем на форму входа
            showLoginLink.click();
        } catch (error) {
            // Сообщение об ошибке уже показано apiRequest
        }
    });

    loginForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const login = document.getElementById('loginLogin').value;
        const password = document.getElementById('loginPassword').value;
        try {
            const data = await apiRequest('/login', 'POST', { login, password });
            jwtToken = data.token;
            currentUserLogin = login; // Сохраняем логин для отображения
            localStorage.setItem('jwtToken', jwtToken);
            localStorage.setItem('userLogin', currentUserLogin);
            // UserID из токена мы не извлекаем на клиенте, это делает бекенд
            showMessage('Вход выполнен успешно!', 'success');
            loginForm.reset();
            updateAuthState();
        } catch (error) {
            // Сообщение об ошибке уже показано apiRequest
        }
    });

    logoutButton.addEventListener('click', () => {
        jwtToken = null;
        currentUserLogin = null;
        localStorage.removeItem('jwtToken');
        localStorage.removeItem('userLogin');
        showMessage('Вы успешно вышли из системы.', 'success');
        updateAuthState();
    });

    // --- Логика Калькулятора и Задач ---
    expressionForm.addEventListener('submit', async (e) => {
        e.preventDefault();
        const expression = expressionInput.value;
        if (!expression.trim()) {
            showMessage('Выражение не может быть пустым.', 'error');
            return;
        }
        try {
            const data = await apiRequest('/calculate', 'POST', { expression }, true);
            showMessage(`Задача ${data.task_id} отправлена на вычисление.`, 'success');
            expressionForm.reset();
            fetchTasks(); // Обновляем список задач
        } catch (error) {
            // Сообщение об ошибке уже показано apiRequest
        }
    });

    async function fetchTasks() {
        if (!jwtToken) return;
        try {
            const tasks = await apiRequest('/tasks', 'GET', null, true);
            renderTasks(tasks);
        } catch (error) {
            // Сообщение об ошибке уже показано apiRequest
        }
    }

    function renderTasks(tasks) {
        taskListUl.innerHTML = ''; // Очищаем старый список
        if (!tasks || tasks.length === 0) {
            taskListUl.innerHTML = '<li>У вас пока нет задач.</li>';
            return;
        }
        tasks.forEach(task => {
            const li = document.createElement('li');
            li.dataset.taskId = task.id; // Сохраняем ID для получения деталей

            const exprSpan = document.createElement('span');
            exprSpan.className = 'task-expression';
            exprSpan.textContent = task.expression;

            const statusSpan = document.createElement('span');
            statusSpan.className = 'task-status ' + task.status.toLowerCase(); // Для стилизации
            statusSpan.textContent = translateStatus(task.status);

            li.appendChild(exprSpan);
            li.appendChild(statusSpan);
            
            li.addEventListener('click', () => fetchTaskDetails(task.id));
            taskListUl.appendChild(li);
        });
    }
    
    function translateStatus(status) {
        const statuses = {
            pending: 'В ожидании',
            processing: 'В обработке',
            completed: 'Завершено',
            failed: 'Ошибка'
        };
        return statuses[status.toLowerCase()] || status;
    }

    async function fetchTaskDetails(taskId) {
        if (!jwtToken) return;
        taskDetailsViewDiv.classList.add('hidden'); // Скрываем детали перед новым запросом
        taskDetailsViewDiv.innerHTML = '<p>Загрузка деталей задачи...</p>';
        taskDetailsViewDiv.classList.remove('hidden');

        try {
            const task = await apiRequest(`/tasks/${taskId}`, 'GET', null, true);
            renderTaskDetails(task);
        } catch (error) {
            taskDetailsViewDiv.innerHTML = `<p class="error-message">Не удалось загрузить детали задачи: ${error.message || 'Неизвестная ошибка'}</p>`;
        }
    }

    function renderTaskDetails(task) {
        let resultHtml = 'Недоступен';
        if (task.status === 'completed' && task.result !== null && task.result !== undefined) {
            resultHtml = `<strong>${task.result}</strong>`;
        } else if (task.status === 'failed' && task.error_message) {
            resultHtml = `<span style="color:red;">${task.error_message}</span>`;
        } else if (task.status === 'processing') {
            resultHtml = 'Вычисляется...';
        } else if (task.status === 'pending') {
            resultHtml = 'В очереди...';
        }

        taskDetailsViewDiv.innerHTML = `
            <h3>Детали Задачи #${task.id.substring(0, 8)}...</h3>
            <p><strong>Выражение:</strong> ${task.expression}</p>
            <p><strong>Статус:</strong> <span class="task-status ${task.status.toLowerCase()}">${translateStatus(task.status)}</span></p>
            <p><strong>Результат:</strong> ${resultHtml}</p>
            <p><strong>Создана:</strong> ${new Date(task.created_at).toLocaleString('ru-RU')}</p>
            <p><strong>Обновлена:</strong> ${new Date(task.updated_at).toLocaleString('ru-RU')}</p>
        `;
        taskDetailsViewDiv.classList.remove('hidden');
    }

    refreshTasksButton.addEventListener('click', fetchTasks);

    // Инициализация состояния UI при загрузке страницы
    updateAuthState();
});