package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/docs/agent_api"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/client"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/handler"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/middleware"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/repository"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/service"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/hasher"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/jwtauth"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/logger"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/postgres"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/shutdown"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	echoSwagger "github.com/swaggo/echo-swagger"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	// "google.golang.org/grpc"
)

// Адаптер для предоставления конфигурации gRPC клиента Оркестратора через интерфейс.
// Это нужно, чтобы NewOrchestratorServiceClient мог зависеть от интерфейса, а не конкретной структуры Config.
type orchestratorClientConfigAdapter struct {
    cfg *config.Config
}

func (a *orchestratorClientConfigAdapter) GetOrchestratorAddress() string {
    return a.cfg.OrchestratorClient.OrchestratorAddress
}
func (a *orchestratorClientConfigAdapter) GetGRPCClientTimeout() time.Duration {
    return a.cfg.OrchestratorClient.Timeout
}

// Run запускает приложение Agent с использованием Fx для управления зависимостями и жизненным циклом.
func Run() {
	// Создаем корневой контекст приложения, который будет отменен при получении сигнала завершения.
	appCtx, cancel := context.WithCancel(context.Background())
	// defer cancel() // Отмена контекста теперь происходит в Graceful

	// Инициализируем логгер до старта Fx, чтобы логировать возможные ошибки Fx и конфигурации.
	// Загружаем конфиг временно только для настройки логгера.
	tempCfg, err := config.Load()
	var log *zap.Logger // Объявляем логгер здесь, чтобы он был доступен в случае ошибки Fx
	if err != nil {
		// Если конфиг не загрузился, используем базовый логгер для вывода фатальной ошибки.
		log, _ = zap.NewProduction()
		log.Fatal("Не удалось загрузить начальную конфигурацию", zap.Error(err))
	} else {
		// Создаем логгер на основе конфигурации.
		log, err = logger.New(tempCfg.Logger.Level, tempCfg.AppEnv)
		if err != nil {
			// Если логгер не создался, используем базовый и выходим.
			log, _ = zap.NewProduction() // Используем любой логгер для вывода
			log.Fatal("Не удалось инициализировать логгер", zap.Error(err))
		}
	}
	// Отложенный вызов Sync для гарантированной записи буферизованных логов перед выходом.
	defer func() {
		if err := log.Sync(); err != nil {
			// Выводим ошибку синхронизации в stderr, так как логгер может быть уже недоступен
			fmt.Fprintf(os.Stderr, "Ошибка синхронизации логгера: %v\n", err)
		}
	}()

	// Создаем Fx приложение.
	fxApp := fx.New(
		// Заменяем стандартный логгер Fx на наш Zap-логгер.
		fx.Logger(NewFxLogger(log)),
		// Регистрируем провайдеры зависимостей.
		fx.Provide(
			// 1. Конфигурация (перезагружаем, чтобы она была в DI графе Fx)
			func() (*config.Config, error) {
				cfg, err := config.Load()
				if err != nil {
					// Логируем фатальную ошибку, Fx остановит запуск
					log.Fatal("Не удалось загрузить конфигурацию для DI", zap.Error(err))
					// Эта строка не будет достигнута, но нужна для компиляции
					return nil, err
				}
				// Проверка секрета JWT еще раз, т.к. она могла быть пропущена в development
				if cfg.JWT.Secret == "default-secret-key-please-change" || len(cfg.JWT.Secret) < 32 {
					log.Fatal("Критическая ошибка: JWT_SECRET не установлен или слишком короткий (требуется >= 32 символов). Установите переменную окружения.")
				}
				return cfg, nil
			},
            func(cfg *config.Config) client.OrchestratorClientConfigProvider { return &orchestratorClientConfigAdapter{cfg: cfg} },
			func() *zap.Logger { return log },
			func() hasher.PasswordHasher { return hasher.NewBcryptHasher(bcrypt.DefaultCost) },
			func(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*pgxpool.Pool, error) {
				var pool *pgxpool.Pool
				var err error
				// Используем главный контекст приложения для создания пула.
				// Это позволяет отменить попытку соединения, если приложение завершается.
				pool, err = postgres.NewPool(appCtx, cfg.Database.DSN, cfg.Database.PoolMaxConns, log)
				if err != nil {
					log.Error("Не удалось создать пул соединений с БД", zap.Error(err))
					// Возвращаем ошибку, Fx прервет запуск.
					return nil, err // Ошибка уже залогирована в NewPool
				}

				// Регистрируем хук Fx Lifecycle для корректного закрытия пула
				// при остановке приложения (OnStop).
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						// Используем фоновый контекст для закрытия, т.к. shutdownCtx может истечь.
						log.Info("Закрытие пула соединений с БД (через Fx Hook)...")
						pool.Close() // Закрытие пула - блокирующая операция.
						log.Info("Пул соединений с БД закрыт.")
						return nil
					},
				})
				return pool, nil
			},

			func(cfg *config.Config, log *zap.Logger) (*jwtauth.Manager, error) {
				manager, err := jwtauth.NewManager(cfg.JWT.Secret, cfg.JWT.TokenTTL)
				if err != nil {
					log.Fatal("Не удалось создать JWT менеджер", zap.Error(err))
					return nil, err // Fx остановит запуск
				}
				log.Info("JWT менеджер успешно создан", zap.Duration("token_ttl", cfg.JWT.TokenTTL))
				return manager, nil
			},
			middleware.JWTAuth,
            client.NewOrchestratorServiceClient, // gRPC Клиент Оркестратора
            repository.NewPgxUserRepository,
            service.NewAuthService,
            service.NewTaskService, // TaskService Агента
            handler.NewAuthHandler,
            handler.NewTaskHandler,
            NewEchoServer,
		),

		// Регистрируем Invoke функции, которые выполняют действия при старте/стопе.
		// Они обычно используются для запуска серверов, регистрации маршрутов и т.д.
		fx.Invoke(func(lc fx.Lifecycle, e *echo.Echo, cfg *config.Config, log *zap.Logger,
            pool *pgxpool.Pool,
            authHandler *handler.AuthHandler,
            taskHandler *handler.TaskHandler,
            jwtAuthMiddleware echo.MiddlewareFunc,
        ) {
			// --- Регистрация Маршрутов ---

			// Группа для API V1
			apiV1 := e.Group("/api/v1")

			// Публичные маршруты (не требуют JWT)
			authHandler.RegisterRoutes(apiV1) // Регистрирует /register и /login

			protectedGroup := apiV1.Group("")
			protectedGroup.Use(jwtAuthMiddleware)

			taskHandler.RegisterRoutes(protectedGroup)

			agent_api.SwaggerInfo.Title = "API Калькулятора Выражений - Agent" // Можно переопределить из main.go
            agent_api.SwaggerInfo.Description = "Документация API для Agent сервиса."
            agent_api.SwaggerInfo.Version = "1.0"
            agent_api.SwaggerInfo.Host = fmt.Sprintf("localhost:%s", cfg.Server.Port) // Используем порт из конфига
            agent_api.SwaggerInfo.BasePath = "/api/v1"
            agent_api.SwaggerInfo.Schemes = []string{"http", "https"}


            // Маршрут для Swagger UI
            // Swagger UI будет доступен по /swagger/index.html
            // (или просто /swagger/ если echo-swagger настроен так)
            e.GET("/swagger/*", echoSwagger.WrapHandler)
            log.Info("Swagger UI доступен по /swagger/index.html", zap.String("host", agent_api.SwaggerInfo.Host))

			// --- Запуск HTTP сервера и Graceful Shutdown ---
			// Создаем HTTP сервер стандартной библиотеки Go.
			// Echo инстанс используется как Handler.
			httpServer := &http.Server{
				Addr:    ":" + cfg.Server.Port, // Берем порт из конфига
				Handler: e,                     // Echo обрабатывает запросы
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  60 * time.Second,
			}

			// Регистрируем хуки Fx Lifecycle для управления HTTP сервером.
			lc.Append(fx.Hook{
				// OnStart выполняется при запуске приложения Fx.
				OnStart: func(ctx context.Context) error {
					log.Info("Запуск HTTP сервера Agent", zap.String("адрес", httpServer.Addr))
					// Запускаем сервер в отдельной горутине, чтобы не блокировать старт Fx.
					go func() {
						// ListenAndServe блокирует до ошибки или вызова Shutdown/Close.
						if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
							// Логируем фатальную ошибку, если сервер упал неожиданно.
							log.Fatal("HTTP сервер неожиданно завершил работу", zap.Error(err))
							// Можно также вызвать mainCancel() здесь, чтобы остановить все приложение.
							cancel() // Отменяем главный контекст, что приведет к shutdown
						}
					}()
					return nil // Возвращаем nil, т.к. сервер запущен в фоне.
				},
				// OnStop выполняется при остановке приложения Fx.
				OnStop: func(ctx context.Context) error {
					log.Info("Остановка HTTP сервера Agent...")
					// Используем контекст, предоставленный Fx (обычно с таймаутом).
					if err := httpServer.Shutdown(ctx); err != nil {
						log.Error("Ошибка при корректной остановке HTTP сервера", zap.Error(err))
						return err
					}
					log.Info("HTTP сервер Agent успешно остановлен.")
					return nil
				},
			})

			// Запускаем обработчик Graceful Shutdown в отдельной горутине.
			// Он будет ждать сигнала ОС и затем вызывать mainCancel().
			// Передаем функцию остановки HTTP сервера.
			serversToStop := map[string]func(context.Context) error{
				"http": httpServer.Shutdown,
				// Добавить gRPC сервер сюда, когда он появится
			}
			go shutdown.Graceful(appCtx, cancel, log, cfg.GracefulTimeout, serversToStop, pool /*, grpcServer */)

		}),
	)

	// Запускаем приложение Fx. Эта функция блокирует выполнение до тех пор,
	// пока приложение не будет остановлено (например, через сигнал ОС или ошибку).
	if err := fxApp.Start(appCtx); err != nil {
		// Эта ошибка возникает, если Fx не смог успешно запустить все компоненты (OnStart хуки).
		// Ошибки зависимостей (Provide) обычно логируются как Fatal выше.
		log.Error("Не удалось запустить Fx приложение", zap.Error(err))
		// Принудительно выходим, если старт не удался
		os.Exit(1)
	}

	// Ожидаем завершения Fx приложения (после получения сигнала и выполнения OnStop хуков).
	<-fxApp.Done()

	// Проверяем, была ли ошибка во время остановки приложения (в OnStop хуках).
	// Контекст отмены (context.Canceled) здесь не считается ошибкой.
	stopErr := fxApp.Err()
	if stopErr != nil && !errors.Is(stopErr, context.Canceled) {
		log.Error("Fx приложение завершилось с ошибкой во время остановки", zap.Error(stopErr))
		os.Exit(1) // Выходим с ошибкой, если остановка прошла некорректно
	}

	log.Info("Сервис Agent успешно завершил работу.")
}

// NewEchoServer создает и настраивает инстанс Echo.
func NewEchoServer(log *zap.Logger, cfg *config.Config) *echo.Echo {
	e := echo.New()
	e.HideBanner = true // Скрываем баннер Echo при старте
	e.HidePort = true   // Не выводим порт (логгер Fx сделает это)

	// --- Middleware ---
	// RequestID добавляет уникальный ID к каждому запросу/ответу.
	e.Use(echomiddleware.RequestID())

	// Кастомный логгер запросов с использованием Zap.
	e.Use(RequestZapLogger(log))

	// Recover перехватывает паники в обработчиках и возвращает 500 ошибку.
	e.Use(echomiddleware.RecoverWithConfig(echomiddleware.RecoverConfig{ // Исправлено на RecoverWithConfig
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {
			// Логируем панику с ID запроса и стектрейсом.
			log.Error("Перехвачена паника",
				zap.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				zap.Error(err),
				zap.ByteString("stack", stack),
			)
			return err // Возвращаем ошибку, чтобы Echo отправил 500 Internal Server Error
		},
		// DisablePrintStack: true, // Мы логируем стек сами
	}))

	// CORS middleware для разрешения кросс-доменных запросов (важно для фронтенда).
	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		// В production окружении нужно ограничить AllowOrigins!
		AllowOrigins: []string{"*"}, // Разрешить все источники для разработки
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization}, // Разрешить нужные заголовки
	}))

	return e
}

// RequestZapLogger создает middleware для логирования HTTP запросов с помощью Zap.
func RequestZapLogger(log *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now() // Засекаем время начала обработки

			// Выполняем следующий обработчик в цепочке
			err := next(c)

			// Логируем информацию о запросе после его выполнения.
			req := c.Request()
			res := c.Response()
			latency := time.Since(start) // Вычисляем время обработки

			// Создаем поля для лога Zap.
			fields := []zap.Field{
				zap.String("request_id", res.Header().Get(echo.HeaderXRequestID)), // ID запроса
				zap.String("method", req.Method),                                  // Метод (GET, POST, ...)
				zap.String("uri", req.RequestURI),                                 // URI запроса
				zap.String("remote_ip", c.RealIP()),                               // IP клиента
				zap.Int("status", res.Status),                                     // HTTP статус ответа
				zap.Duration("latency", latency),                                  // Время обработки
				zap.Int64("response_size", res.Size),                              // Размер ответа в байтах
				zap.String("user_agent", req.UserAgent()),                         // User-Agent клиента
			}

			// Добавляем поле ошибки, если она была возвращена обработчиком.
			if err != nil {
				// Проверяем, не является ли ошибка HTTP ошибкой Echo, чтобы не логировать ее дважды (Echo сам логирует HTTP ошибки)
				var httpError *echo.HTTPError
				if !errors.As(err, &httpError) { // Логируем только если это не стандартная HTTP ошибка Echo
					fields = append(fields, zap.NamedError("handler_error", err))
				}
			}

			// Выбираем уровень логирования в зависимости от статуса ответа.
			logFunc := log.Info // По умолчанию Info
			if res.Status >= 500 {
				logFunc = log.Error // Ошибки сервера - Error
			} else if res.Status >= 400 {
				logFunc = log.Warn // Ошибки клиента - Warn
			}

			// Пишем лог.
			logFunc("Обработан входящий HTTP запрос", fields...)

			// Возвращаем ошибку дальше, если она была.
			return err
		}
	}
}

// FxLogger адаптирует Zap логгер для использования с Fx (интерфейс fx.Printer).
type FxLogger struct {
	log *zap.Logger
}

// NewFxLogger создает новый FxLogger.
func NewFxLogger(log *zap.Logger) *FxLogger {
	// Используем .WithOptions(zap.AddCallerSkip(1)), чтобы Fx не отображался как вызывающий в логах.
	return &FxLogger{log: log.WithOptions(zap.AddCallerSkip(1))}
}

// Printf реализует интерфейс fx.Printer.
func (l *FxLogger) Printf(format string, args ...interface{}) {
	// Логируем сообщения Fx на уровне Info.
	l.log.Info(fmt.Sprintf(format, args...))
}