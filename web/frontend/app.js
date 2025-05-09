document.addEventListener("DOMContentLoaded", () => {
  const API_BASE_URL = "http://localhost:8080/api/v1"; 

  const messagesDiv = document.getElementById("messages");

  const authSection = document.getElementById("authSection");
  const registerFormContainer = document.getElementById("registerFormContainer");
  const loginFormContainer = document.getElementById("loginFormContainer");
  const registerForm = document.getElementById("registerForm");
  const loginForm = document.getElementById("loginForm");
  const showLoginLink = document.getElementById("showLoginLink");
  const showRegisterLink = document.getElementById("showRegisterLink");

  const calculatorSection = document.getElementById("calculatorSection");
  const currentUserLoginSpan = document.getElementById("currentUserLogin");
  const logoutButton = document.getElementById("logoutButton");
  const expressionForm = document.getElementById("expressionForm");
  const expressionInput = document.getElementById("expressionInput");

  const refreshTasksButton = document.getElementById("refreshTasksButton");
  const taskListUl = document.getElementById("taskList");
  const taskDetailsViewDiv = document.getElementById("taskDetailsView");

  const themeToggleButton = document.getElementById("themeToggleButton");
  const themeIconMoon = document.getElementById("themeIconMoon");
  const themeIconSun = document.getElementById("themeIconSun");
  const bodyElement = document.body;

  let jwtToken = localStorage.getItem("jwtToken");
  let currentUserLogin = localStorage.getItem("userLogin");
  let currentTheme = localStorage.getItem("theme") || "light";

  function applyTheme(theme) {
    bodyElement.setAttribute("data-theme", theme);
    localStorage.setItem("theme", theme);
    updateThemeIcon(theme);
  }

  function updateThemeIcon(theme) {
    if (theme === "dark") {
      if (themeIconMoon) themeIconMoon.classList.add("hidden");
      if (themeIconSun) themeIconSun.classList.remove("hidden");
    } else {
      if (themeIconMoon) themeIconMoon.classList.remove("hidden");
      if (themeIconSun) themeIconSun.classList.add("hidden");
    }
  }

  if (themeToggleButton) {
    themeToggleButton.addEventListener("click", () => {
      currentTheme = bodyElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
      applyTheme(currentTheme);
    });
  }

  function showMessage(message, type = "success") {
    if (!messagesDiv) {
        console.warn("Элемент для сообщений (messagesDiv) не найден.");
        return;
    }
    messagesDiv.textContent = message;
    messagesDiv.classList.remove('success', 'error', 'hidden');
    messagesDiv.classList.add(type);
    
    setTimeout(() => {
      messagesDiv.classList.add("hidden");
    }, 5000);
  }

  async function apiRequest(endpoint, method = "GET", body = null, requiresAuth = false) {
    const headers = { "Content-Type": "application/json" };
    if (requiresAuth && jwtToken) {
      headers["Authorization"] = `Bearer ${jwtToken}`;
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
      const responseData = response.status !== 204 ? await response.json().catch(() => ({})) : {};


      if (!response.ok) {
        const errorMessage = responseData.error || responseData.message || `Ошибка: ${response.status} ${response.statusText}`;
        throw new Error(errorMessage);
      }
      return responseData; 
    } catch (error) {
      console.error(`API ошибка для ${method} ${endpoint}:`, error);
      showMessage(error.message || "Произошла сетевая ошибка или ошибка сервера", "error");
      throw error; 
    }
  }

  function updateAuthState() {
    if (jwtToken && currentUserLogin) {
      if (authSection) authSection.classList.add("hidden");
      if (calculatorSection) calculatorSection.classList.remove("hidden");
      if (currentUserLoginSpan) currentUserLoginSpan.textContent = currentUserLogin;
      fetchTasks(); 
    } else {
      if (authSection) authSection.classList.remove("hidden");
      if (calculatorSection) calculatorSection.classList.add("hidden");
      if (currentUserLoginSpan) currentUserLoginSpan.textContent = "";
      if (taskListUl) taskListUl.innerHTML = ""; 
      if (taskDetailsViewDiv) {
        taskDetailsViewDiv.classList.add("hidden");
        taskDetailsViewDiv.innerHTML = "";
      }
    }
  }
  
  if (showLoginLink) {
    showLoginLink.addEventListener("click", (e) => {
      e.preventDefault();
      if (registerFormContainer) registerFormContainer.classList.add("hidden");
      if (loginFormContainer) loginFormContainer.classList.remove("hidden");
      if (messagesDiv) messagesDiv.classList.add("hidden");
    });
  }

  if (showRegisterLink) {
    showRegisterLink.addEventListener("click", (e) => {
      e.preventDefault();
      if (loginFormContainer) loginFormContainer.classList.add("hidden");
      if (registerFormContainer) registerFormContainer.classList.remove("hidden");
      if (messagesDiv) messagesDiv.classList.add("hidden");
    });
  }

  if (registerForm) {
    registerForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      const loginInput = document.getElementById("registerLogin");
      const passwordInput = document.getElementById("registerPassword");
      if (!loginInput || !passwordInput) return;

      const login = loginInput.value;
      const password = passwordInput.value;
      try {
        await apiRequest("/register", "POST", { login, password });
        showMessage("Регистрация прошла успешно! Теперь вы можете войти.", "success");
        registerForm.reset();
        if (showLoginLink) showLoginLink.click();
      } catch (error) {
      }
    });
  }

  if (loginForm) {
    loginForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      const loginInput = document.getElementById("loginLogin");
      const passwordInput = document.getElementById("loginPassword");
      if (!loginInput || !passwordInput) return;

      const login = loginInput.value;
      const password = passwordInput.value;
      try {
        const data = await apiRequest("/login", "POST", { login, password });
        jwtToken = data.token;
        currentUserLogin = login; 
        localStorage.setItem("jwtToken", jwtToken);
        localStorage.setItem("userLogin", currentUserLogin);
        showMessage("Вход выполнен успешно!", "success");
        loginForm.reset();
        updateAuthState();
      } catch (error) {
      }
    });
  }

  if (logoutButton) {
    logoutButton.addEventListener("click", () => {
      jwtToken = null;
      currentUserLogin = null;
      localStorage.removeItem("jwtToken");
      localStorage.removeItem("userLogin");
      showMessage("Вы успешно вышли из системы.", "success");
      updateAuthState();
    });
  }

  if (expressionForm) {
    expressionForm.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (!expressionInput) return;

      const expression = expressionInput.value;
      if (!expression.trim()) {
        showMessage("Выражение не может быть пустым.", "error");
        return;
      }
      try {
        const data = await apiRequest("/calculate", "POST", { expression }, true);
        showMessage(`Задача ${data.task_id ? data.task_id.substring(0,8)+'...' : ''} отправлена на вычисление.`, "success");
        expressionForm.reset();
        fetchTasks(); 
      } catch (error) {
      }
    });
  }

  async function fetchTasks() {
    if (!jwtToken || !taskListUl) return;
    try {
      const tasks = await apiRequest("/tasks", "GET", null, true);
      renderTasks(tasks);
    } catch (error) {
    }
  }

  function renderTasks(tasks) {
    if (!taskListUl) return;
    taskListUl.innerHTML = ""; 
    if (!tasks || tasks.length === 0) {
      const li = document.createElement('li');
      li.textContent = 'У вас пока нет задач.';
      li.classList.add('no-tasks');
      taskListUl.appendChild(li);
      return;
    }
    tasks.sort((a, b) => new Date(b.created_at) - new Date(a.created_at));

    tasks.forEach((task) => {
      const li = document.createElement("li");
      li.dataset.taskId = task.id; 

      const exprSpan = document.createElement("span");
      exprSpan.className = "task-expression";
      exprSpan.textContent = task.expression;

      const statusSpan = document.createElement("span");
      statusSpan.className = "task-status " + task.status.toLowerCase(); 
      statusSpan.textContent = translateStatus(task.status);

      li.appendChild(exprSpan);
      li.appendChild(statusSpan);

      li.addEventListener("click", () => fetchTaskDetails(task.id));
      taskListUl.appendChild(li);
    });
  }

  function translateStatus(status) {
    const statuses = {
      pending: "В ожидании",
      processing: "В обработке",
      completed: "Завершено",
      failed: "Ошибка",
    };
    return statuses[status.toLowerCase()] || status;
  }

  async function fetchTaskDetails(taskId) {
    if (!jwtToken || !taskDetailsViewDiv) return;
    
    taskDetailsViewDiv.innerHTML = "<p>Загрузка деталей задачи...</p>";
    taskDetailsViewDiv.classList.remove("hidden");


    try {
      const task = await apiRequest(`/tasks/${taskId}`, "GET", null, true);
      renderTaskDetails(task);
    } catch (error) {
      taskDetailsViewDiv.innerHTML = `<p class="error-message">Не удалось загрузить детали задачи: ${
        error.message || "Неизвестная ошибка"
      }</p>`;
    }
  }

  function renderTaskDetails(task) {
    if (!taskDetailsViewDiv) return;

    let resultHtml = "Недоступен";
    if (task.status === "completed" && task.result !== null && task.result !== undefined) {
      resultHtml = `<strong>${task.result}</strong>`;
    } else if (task.status === "failed" && task.error_message) {
      resultHtml = `<span class="error-text">${task.error_message}</span>`; 
    } else if (task.status === "processing") {
      resultHtml = "Вычисляется...";
    } else if (task.status === "pending") {
      resultHtml = "В очереди...";
    }

    taskDetailsViewDiv.innerHTML = `
            <h3>Детали Задачи <span class="task-id-detail">#${task.id.substring(0, 8)}...</span></h3>
            <p><strong>Выражение:</strong> ${task.expression}</p>
            <p><strong>Статус:</strong> <span class="task-status ${task.status.toLowerCase()}">${translateStatus(
      task.status
    )}</span></p>
            <p><strong>Результат:</strong> ${resultHtml}</p>
            <p><strong>Создана:</strong> ${new Date(task.created_at).toLocaleString("ru-RU", { dateStyle: 'short', timeStyle: 'short' })}</p>
            <p><strong>Обновлена:</strong> ${new Date(task.updated_at).toLocaleString("ru-RU", { dateStyle: 'short', timeStyle: 'short' })}</p>
        `;
    taskDetailsViewDiv.classList.remove("hidden");
  }

  if (refreshTasksButton) {
    refreshTasksButton.addEventListener("click", fetchTasks);
  }

  applyTheme(currentTheme);
  updateAuthState();
});