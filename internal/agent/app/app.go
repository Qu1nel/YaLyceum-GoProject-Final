package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/handler"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/repository"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/agent/service"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/hasher"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/logger"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/postgres"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/shutdown"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
)

// Run запускает приложение Agent с использованием Fx для управления зависимостями и жизненным циклом.
func Run() {
	appCtx, cancel := context.WithCancel(context.Background())

	// Инициализируем логгер до старта Fx, чтобы логировать возможные ошибки Fx и конфигурации.
	// Загружаем конфиг временно только для настройки логгера.
	tempCfg, err := config.Load()
	var log *zap.Logger // Объявляем логгер здесь, чтобы он был доступен в случае ошибки Fx
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
			func() *zap.Logger {
				return log
			},
			func() hasher.PasswordHasher {
				return hasher.NewBcryptHasher(bcrypt.DefaultCost)
			},
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

			repository.NewPgxUserRepository,
			service.NewAuthService,
			handler.NewAuthHandler,
			NewEchoServer,
		),

		fx.Invoke(func(lc fx.Lifecycle, e *echo.Echo, cfg *config.Config, log *zap.Logger,
			pool *pgxpool.Pool, // Нужен для Graceful Shutdown
			authHandler *handler.AuthHandler, // Нужен для регистрации маршрутов
		) {
			apiGroup := e.Group("/api/v1")
			authHandler.RegisterRoutes(apiGroup)
			httpServer := &http.Server{
				Addr:    ":" + cfg.Server.Port, 
				Handler: e,                     
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
				fields = append(fields, zap.NamedError("error", err))
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