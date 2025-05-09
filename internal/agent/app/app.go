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
)

type orchestratorClientConfigAdapter struct {
	cfg *config.Config
}

func (a *orchestratorClientConfigAdapter) GetOrchestratorAddress() string {
	return a.cfg.OrchestratorClient.OrchestratorAddress
}
func (a *orchestratorClientConfigAdapter) GetGRPCClientTimeout() time.Duration {
	return a.cfg.OrchestratorClient.Timeout
}

func Run() {

	appCtx, cancel := context.WithCancel(context.Background())

	tempCfg, err := config.Load()
	var log *zap.Logger
	if err != nil {

		log, _ = zap.NewProduction()
		log.Fatal("Не удалось загрузить начальную конфигурацию", zap.Error(err))
	} else {

		log, err = logger.New(tempCfg.Logger.Level, tempCfg.AppEnv)
		if err != nil {

			log, _ = zap.NewProduction()
			log.Fatal("Не удалось инициализировать логгер", zap.Error(err))
		}
	}

	defer func() {
		if err := log.Sync(); err != nil {

			fmt.Fprintf(os.Stderr, "Ошибка синхронизации логгера: %v\n", err)
		}
	}()

	fxApp := fx.New(

		fx.Logger(NewFxLogger(log)),

		fx.Provide(

			func() (*config.Config, error) {
				cfg, err := config.Load()
				if err != nil {

					log.Fatal("Не удалось загрузить конфигурацию для DI", zap.Error(err))

					return nil, err
				}

				if cfg.JWT.Secret == "default-secret-key-please-change" || len(cfg.JWT.Secret) < 32 {
					log.Fatal("Критическая ошибка: JWT_SECRET не установлен или слишком короткий (требуется >= 32 символов). Установите переменную окружения.")
				}
				return cfg, nil
			},
			func(cfg *config.Config) client.OrchestratorClientConfigProvider {
				return &orchestratorClientConfigAdapter{cfg: cfg}
			},
			func() *zap.Logger { return log },
			func() hasher.PasswordHasher { return hasher.NewBcryptHasher(bcrypt.DefaultCost) },
			func(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*pgxpool.Pool, error) {
				var pool *pgxpool.Pool
				var err error

				pool, err = postgres.NewPool(appCtx, cfg.Database.DSN, cfg.Database.PoolMaxConns, log)
				if err != nil {
					log.Error("Не удалось создать пул соединений с БД", zap.Error(err))

					return nil, err
				}

				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {

						log.Info("Закрытие пула соединений с БД (через Fx Hook)...")
						pool.Close()
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
					return nil, err
				}
				log.Info("JWT менеджер успешно создан", zap.Duration("token_ttl", cfg.JWT.TokenTTL))
				return manager, nil
			},
			middleware.JWTAuth,
			client.NewOrchestratorServiceClient,
			repository.NewPgxUserRepository,
			service.NewAuthService,
			service.NewTaskService,
			handler.NewAuthHandler,
			handler.NewTaskHandler,
			NewEchoServer,
		),

		fx.Invoke(func(lc fx.Lifecycle, e *echo.Echo, cfg *config.Config, log *zap.Logger,
			pool *pgxpool.Pool,
			authHandler *handler.AuthHandler,
			taskHandler *handler.TaskHandler,
			jwtAuthMiddleware echo.MiddlewareFunc,
		) {

			apiV1 := e.Group("/api/v1")

			authHandler.RegisterRoutes(apiV1)

			protectedGroup := apiV1.Group("")
			protectedGroup.Use(jwtAuthMiddleware)

			taskHandler.RegisterRoutes(protectedGroup)

			agent_api.SwaggerInfo.Title = "API Калькулятора Выражений - Agent"
			agent_api.SwaggerInfo.Description = "Документация API для Agent сервиса."
			agent_api.SwaggerInfo.Version = "1.0"
			agent_api.SwaggerInfo.Host = fmt.Sprintf("localhost:%s", cfg.Server.Port)
			agent_api.SwaggerInfo.BasePath = "/api/v1"
			agent_api.SwaggerInfo.Schemes = []string{"http", "https"}

			e.GET("/swagger/*", echoSwagger.WrapHandler)
			log.Info("Swagger UI доступен по /swagger/index.html", zap.String("host", agent_api.SwaggerInfo.Host))

			httpServer := &http.Server{
				Addr:         ":" + cfg.Server.Port,
				Handler:      e,
				ReadTimeout:  5 * time.Second,
				WriteTimeout: 10 * time.Second,
				IdleTimeout:  60 * time.Second,
			}

			lc.Append(fx.Hook{

				OnStart: func(ctx context.Context) error {
					log.Info("Запуск HTTP сервера Agent", zap.String("адрес", httpServer.Addr))

					go func() {

						if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {

							log.Fatal("HTTP сервер неожиданно завершил работу", zap.Error(err))

							cancel()
						}
					}()
					return nil
				},

				OnStop: func(ctx context.Context) error {
					log.Info("Остановка HTTP сервера Agent...")

					if err := httpServer.Shutdown(ctx); err != nil {
						log.Error("Ошибка при корректной остановке HTTP сервера", zap.Error(err))
						return err
					}
					log.Info("HTTP сервер Agent успешно остановлен.")
					return nil
				},
			})

			serversToStop := map[string]func(context.Context) error{
				"http": httpServer.Shutdown,
			}
			go shutdown.Graceful(appCtx, cancel, log, cfg.GracefulTimeout, serversToStop, pool /*, grpcServer */)

		}),
	)

	if err := fxApp.Start(appCtx); err != nil {

		log.Error("Не удалось запустить Fx приложение", zap.Error(err))

		os.Exit(1)
	}

	<-fxApp.Done()

	stopErr := fxApp.Err()
	if stopErr != nil && !errors.Is(stopErr, context.Canceled) {
		log.Error("Fx приложение завершилось с ошибкой во время остановки", zap.Error(stopErr))
		os.Exit(1)
	}

	log.Info("Сервис Agent успешно завершил работу.")
}

func NewEchoServer(log *zap.Logger, cfg *config.Config) *echo.Echo {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.Use(echomiddleware.RequestID())

	e.Use(RequestZapLogger(log))

	e.Use(echomiddleware.RecoverWithConfig(echomiddleware.RecoverConfig{
		LogErrorFunc: func(c echo.Context, err error, stack []byte) error {

			log.Error("Перехвачена паника",
				zap.String("request_id", c.Response().Header().Get(echo.HeaderXRequestID)),
				zap.Error(err),
				zap.ByteString("stack", stack),
			)
			return err
		},
	}))

	e.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{

		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	return e
}

func RequestZapLogger(log *zap.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			req := c.Request()
			res := c.Response()
			latency := time.Since(start)

			fields := []zap.Field{
				zap.String("request_id", res.Header().Get(echo.HeaderXRequestID)),
				zap.String("method", req.Method),
				zap.String("uri", req.RequestURI),
				zap.String("remote_ip", c.RealIP()),
				zap.Int("status", res.Status),
				zap.Duration("latency", latency),
				zap.Int64("response_size", res.Size),
				zap.String("user_agent", req.UserAgent()),
			}

			if err != nil {

				var httpError *echo.HTTPError
				if !errors.As(err, &httpError) {
					fields = append(fields, zap.NamedError("handler_error", err))
				}
			}

			logFunc := log.Info
			if res.Status >= 500 {
				logFunc = log.Error
			} else if res.Status >= 400 {
				logFunc = log.Warn
			}

			logFunc("Обработан входящий HTTP запрос", fields...)

			return err
		}
	}
}

type FxLogger struct {
	log *zap.Logger
}

func NewFxLogger(log *zap.Logger) *FxLogger {

	return &FxLogger{log: log.WithOptions(zap.AddCallerSkip(1))}
}

func (l *FxLogger) Printf(format string, args ...interface{}) {

	l.log.Info(fmt.Sprintf(format, args...))
}
