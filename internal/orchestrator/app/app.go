package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/client"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/grpc_handler"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/repository"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/service"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/logger"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/postgres"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/shutdown"
	pb_orchestrator "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Адаптер для конфигурации клиента Воркера
type workerClientConfigAdapter struct {
	cfg *config.Config
}
func (a *workerClientConfigAdapter) GetWorkerAddress() string {
	return a.cfg.WorkerClient.WorkerAddress
}
func (a *workerClientConfigAdapter) GetGRPCClientTimeout() time.Duration {
	return a.cfg.WorkerClient.Timeout
}


// Run запускает приложение Оркестратор.
func Run() {
	appCtx, cancel := context.WithCancel(context.Background())

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
		fx.Logger(NewFxLogger(log)),
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
			// Адаптер конфигурации для клиента Воркера
			func(cfg *config.Config) client.WorkerClientConfigProvider {
				return &workerClientConfigAdapter{cfg: cfg}
			},
			// 2. Логгер
			func() *zap.Logger {
				return log
			},
			// 3. Пул соединений к БД
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

			// 3.5 Репозиторий Задач
			// *pgxpool.Pool реализует repository.DBPoolIface
			func(pool *pgxpool.Pool, log *zap.Logger) repository.TaskRepository {
				return repository.NewPgxTaskRepository(pool, log)
			},

			// 3.6 gRPC Клиент Воркера
			client.NewWorkerServiceClient,

			// Сервис Вычислений
			service.NewExpressionEvaluator,

			// 4. gRPC Хендлер (Сервер Оркестратора)
			grpc_handler.NewOrchestratorServer,

			// 5. gRPC Сервер (стандартный)
			func(log *zap.Logger) *grpc.Server {
				srv := grpc.NewServer()
				log.Info("Создан инстанс gRPC сервера")
				return srv
			},
		),
		fx.Invoke(func(lc fx.Lifecycle,
			grpcServer *grpc.Server,
			orchestratorHandler *grpc_handler.OrchestratorServer,
			cfg *config.Config,
			log *zap.Logger,
			pool *pgxpool.Pool, // Для Graceful Shutdown
		) {
			pb_orchestrator.RegisterOrchestratorServiceServer(grpcServer, orchestratorHandler)
			log.Info("gRPC обработчик Оркестратора зарегистрирован")

			grpcAddr := ":" + cfg.GRPCServer.Port
			listener, err := net.Listen("tcp", grpcAddr)
			if err != nil {
				log.Fatal("Не удалось начать слушать порт для gRPC", zap.String("адрес", grpcAddr), zap.Error(err))
			}

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					log.Info("Запуск gRPC сервера Оркестратора", zap.String("адрес", grpcAddr))
					go func() {
						if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
							log.Fatal("gRPC сервер Оркестратора неожиданно завершил работу", zap.Error(err))
							cancel()
						}
					}()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					log.Info("Остановка gRPC сервера Оркестратора...")
					grpcServer.GracefulStop()
					log.Info("gRPC сервер Оркестратора успешно остановлен.")
					return nil
				},
			})

			serversToStop := map[string]func(context.Context) error{
				"grpc": func(ctx context.Context) error {
					done := make(chan struct{})
					go func() {
						grpcServer.GracefulStop()
						close(done)
					}()
					select {
					case <-done:
						return nil
					case <-ctx.Done():
						log.Error("Таймаут при остановке gRPC сервера", zap.Error(ctx.Err()))
						grpcServer.Stop()
						return ctx.Err()
					}
				},
			}
			go shutdown.Graceful(appCtx, cancel, log, cfg.GracefulTimeout, serversToStop, pool)
		}),
	)

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

type FxLogger struct {
	log *zap.Logger
}
func NewFxLogger(log *zap.Logger) *FxLogger {
	return &FxLogger{log: log.WithOptions(zap.AddCallerSkip(1))}
}
func (l *FxLogger) Printf(format string, args ...interface{}) {
	l.log.Info(fmt.Sprintf(format, args...))
}