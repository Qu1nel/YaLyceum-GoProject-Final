package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/grpc_handler"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/logger"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/postgres"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/shutdown"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Run запускает приложение Оркестратор.
func Run() {
	appCtx, cancel := context.WithCancel(context.Background())
	// defer cancel() // Управляется через Graceful

	// Инициализация логгера (аналогично Agent)
	tempCfg, err := config.Load()
	var log *zap.Logger
	if err != nil {
		log, _ = zap.NewProduction()
		log.Fatal("Не удалось загрузить начальную конфигурацию Оркестратора", zap.Error(err))
	} else {
		log, err = logger.New(tempCfg.Logger.Level, tempCfg.AppEnv)
		if err != nil {
			log, _ = zap.NewProduction()
			log.Fatal("Не удалось инициализировать логгер Оркестратора", zap.Error(err))
		}
	}
	defer func() {
		if err := log.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка синхронизации логгера Оркестратора: %v\n", err)
		}
	}()

	fxApp := fx.New(
		fx.Logger(NewFxLogger(log)), // Используем наш логгер для Fx
		fx.Provide(
			// 1. Конфигурация
			func() (*config.Config, error) {
				cfg, err := config.Load()
				if err != nil {
					log.Fatal("Не удалось загрузить конфигурацию Оркестратора для DI", zap.Error(err))
					return nil, err
				}
				return cfg, nil
			},
			// 2. Логгер
			func() *zap.Logger {
				return log
			},
			// 3. Пул соединений к БД (пока не используется хендлером, но нужен для запуска)
			func(lc fx.Lifecycle, cfg *config.Config, log *zap.Logger) (*pgxpool.Pool, error) {
				pool, err := postgres.NewPool(appCtx, cfg.Database.DSN, cfg.Database.PoolMaxConns, log)
				if err != nil {
					log.Error("Не удалось создать пул соединений с БД для Оркестратора", zap.Error(err))
					return nil, err
				}
				lc.Append(fx.Hook{
					OnStop: func(ctx context.Context) error {
						log.Info("Закрытие пула соединений с БД Оркестратора (через Fx Hook)...")
						pool.Close()
						log.Info("Пул соединений с БД Оркестратора закрыт.")
						return nil
					},
				})
				return pool, nil
			},
			// 3.5 Репозиторий Задач <-- Добавляем провайдер
            // Fx передаст *pgxpool.Pool, *zap.Logger
            repository.NewPgxTaskRepository,

            // 4. gRPC Хендлер (Сервер)
            // Теперь NewOrchestratorServer будет принимать TaskRepository
            grpc_handler.NewOrchestratorServer,

            // 5. gRPC Сервер (стандартный)
            // Эта функция создает сам инстанс grpc.Server
            func(log *zap.Logger) *grpc.Server {
                 // Можно добавить опции, например, интерцепторы для логирования/метрик/panic recovery
                 // unaryInterceptor := grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer(
                 //     grpc_zap.UnaryServerInterceptor(log),
                 //     // Добавить другие интерцепторы
                 // ))
                 // srv := grpc.NewServer(unaryInterceptor)
                 srv := grpc.NewServer()
                 log.Info("Создан инстанс gRPC сервера")
                 return srv
            },

            // ... Добавить репозитории и сервисы позже ...
		),
		fx.Invoke(func(lc fx.Lifecycle,
            grpcServer *grpc.Server, // Запрашиваем инстанс gRPC сервера
            orchestratorHandler *grpc_handler.OrchestratorServer, // Запрашиваем наш обработчик
            cfg *config.Config,
            log *zap.Logger,
            pool *pgxpool.Pool, // Для Graceful Shutdown
        ) {
            // Регистрируем наш обработчик в gRPC сервере
            pb.RegisterOrchestratorServiceServer(grpcServer, orchestratorHandler)
            log.Info("gRPC обработчик Оркестратора зарегистрирован")

            // Можно включить gRPC Reflection для дебаггинга (например, с grpcurl)
            // reflection.Register(grpcServer)
            // log.Info("gRPC Reflection зарегистрирован")

            // Определяем адрес для gRPC сервера
            grpcAddr := ":" + cfg.GRPCServer.Port

            // Создаем листенер для gRPC сервера
            listener, err := net.Listen("tcp", grpcAddr)
            if err != nil {
                // Не смогли занять порт - фатальная ошибка
                log.Fatal("Не удалось начать слушать порт для gRPC", zap.String("адрес", grpcAddr), zap.Error(err))
            }

            // Регистрируем хуки Fx Lifecycle для gRPC сервера
            lc.Append(fx.Hook{
                OnStart: func(ctx context.Context) error {
                    log.Info("Запуск gRPC сервера Оркестратора", zap.String("адрес", grpcAddr))
                    // Запускаем gRPC сервер в отдельной горутине
                    go func() {
                        if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
                            log.Fatal("gRPC сервер Оркестратора неожиданно завершил работу", zap.Error(err))
                            // Можно отменить главный контекст
                            cancel()
                        }
                    }()
                    return nil
                },
                OnStop: func(ctx context.Context) error {
                    log.Info("Остановка gRPC сервера Оркестратора...")
                    // Используем GracefulStop, он дождется завершения текущих запросов
                    // (в пределах контекста/таймаута)
                    grpcServer.GracefulStop()
                    log.Info("gRPC сервер Оркестратора успешно остановлен.")
                    // listener.Close() не нужен, Serve() его закроет
                    return nil
                },
            })

            // Запускаем Graceful Shutdown для Оркестратора
            // Передаем функцию остановки gRPC сервера
            serversToStop := map[string]func(context.Context) error{
            	// Ключ может быть любым, используем "grpc" для ясности
            	"grpc": func(ctx context.Context) error {
            		// GracefulStop() сам обрабатывает контекст/таймаут,
            		// но обернем на всякий случай
                    done := make(chan struct{})
                    go func() {
                        grpcServer.GracefulStop()
                        close(done)
                    }()
                    select {
                    case <-done:
                       return nil // Успешно остановлен
                    case <-ctx.Done():
                       log.Error("Таймаут при остановке gRPC сервера", zap.Error(ctx.Err()))
                       grpcServer.Stop() // Принудительная остановка, если таймаут истек
                       return ctx.Err()
                    }
            	},
            }
            go shutdown.Graceful(appCtx, cancel, log, cfg.GracefulTimeout, serversToStop, pool)
        }),
	)

	// Запуск Fx приложения
	if err := fxApp.Start(appCtx); err != nil {
		log.Error("Не удалось запустить Fx приложение Оркестратора", zap.Error(err))
		os.Exit(1)
	}

	<-fxApp.Done()

	stopErr := fxApp.Err()
	if stopErr != nil && !errors.Is(stopErr, context.Canceled) {
		log.Error("Fx приложение Оркестратора завершилось с ошибкой во время остановки", zap.Error(stopErr))
		os.Exit(1)
	}

	log.Info("Сервис Оркестратор успешно завершил работу.")
}

// FxLogger (можно скопировать из Agent или вынести в общий пакет)
type FxLogger struct {
	log *zap.Logger
}
func NewFxLogger(log *zap.Logger) *FxLogger {
	return &FxLogger{log: log.WithOptions(zap.AddCallerSkip(1))}
}
func (l *FxLogger) Printf(format string, args ...interface{}) {
	l.log.Info(fmt.Sprintf(format, args...))
}