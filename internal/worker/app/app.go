package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/logger"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/pkg/shutdown"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/config"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/grpc_handler"
	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/worker/service"
	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

// Run запускает приложение Воркер.
func Run() {
	appCtx, cancel := context.WithCancel(context.Background())
	// defer cancel() // Управляется через Graceful

	// Инициализация логгера (аналогично Agent/Orchestrator)
	tempCfg, err := config.Load()
	var log *zap.Logger
	if err != nil {
		log, _ = zap.NewProduction()
		log.Fatal("Не удалось загрузить начальную конфигурацию Воркера", zap.Error(err))
	} else {
		log, err = logger.New(tempCfg.Logger.Level, tempCfg.AppEnv)
		if err != nil {
			log, _ = zap.NewProduction()
			log.Fatal("Не удалось инициализировать логгер Воркера", zap.Error(err))
		}
	}
	defer func() {
		if err := log.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "Ошибка синхронизации логгера Воркера: %v\n", err)
		}
	}()

	fxApp := fx.New(
		fx.Logger(NewFxLogger(log)),
		fx.Provide(
			// 1. Конфигурация
			func() (*config.Config, error) {
				cfg, err := config.Load()
				if err != nil {
					log.Fatal("Не удалось загрузить конфигурацию Воркера для DI", zap.Error(err))
					return nil, err
				}
				return cfg, nil
			},
			// 2. Логгер
			func() *zap.Logger {
				return log
			},
            // 3. Сервис Вычислений
            // Fx передаст *zap.Logger, *config.Config
            service.NewCalculatorService,

            // 4. gRPC Хендлер (Сервер)
            // Fx передаст *zap.Logger, *service.CalculatorService
            grpc_handler.NewWorkerServer,

            // 5. gRPC Сервер (стандартный)
            func(log *zap.Logger) *grpc.Server {
                 srv := grpc.NewServer()
                 log.Info("Создан инстанс gRPC сервера Воркера")
                 return srv
            },
		),
		fx.Invoke(func(lc fx.Lifecycle,
            grpcServer *grpc.Server, // Запрашиваем инстанс gRPC сервера
            workerHandler *grpc_handler.WorkerServer, // Запрашиваем наш обработчик
            cfg *config.Config,
            log *zap.Logger,
        ) {
            // Регистрируем наш обработчик в gRPC сервере
            pb.RegisterWorkerServiceServer(grpcServer, workerHandler)
            log.Info("gRPC обработчик Воркера зарегистрирован")

            // Определяем адрес для gRPC сервера
            grpcAddr := ":" + cfg.GRPCServer.Port

            // Создаем листенер для gRPC сервера
            listener, err := net.Listen("tcp", grpcAddr)
            if err != nil {
                log.Fatal("Не удалось начать слушать порт для gRPC Воркера", zap.String("адрес", grpcAddr), zap.Error(err))
            }

            // Регистрируем хуки Fx Lifecycle для gRPC сервера
            lc.Append(fx.Hook{
                OnStart: func(ctx context.Context) error {
                    log.Info("Запуск gRPC сервера Воркера", zap.String("адрес", grpcAddr))
                    go func() {
                        if err := grpcServer.Serve(listener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
                            log.Fatal("gRPC сервер Воркера неожиданно завершил работу", zap.Error(err))
                            cancel()
                        }
                    }()
                    return nil
                },
                OnStop: func(ctx context.Context) error {
                    log.Info("Остановка gRPC сервера Воркера...")
                    grpcServer.GracefulStop()
                    log.Info("gRPC сервер Воркера успешно остановлен.")
                    return nil
                },
            })

            // Запускаем Graceful Shutdown для Воркера (без БД)
            serversToStop := map[string]func(context.Context) error{
            	"grpc": func(ctx context.Context) error {
                    done := make(chan struct{})
                    go func() {
                        grpcServer.GracefulStop()
                        close(done)
                    }()
                    select {
                    case <-done: return nil
                    case <-ctx.Done():
                       log.Error("Таймаут при остановке gRPC сервера Воркера", zap.Error(ctx.Err()))
                       grpcServer.Stop()
                       return ctx.Err()
                    }
            	},
            }
            // Пул БД равен nil, т.к. Воркер не работает с БД напрямую
            go shutdown.Graceful(appCtx, cancel, log, cfg.GracefulTimeout, serversToStop, nil)
        }),
	)

	// Запуск Fx приложения
	if err := fxApp.Start(appCtx); err != nil {
		log.Error("Не удалось запустить Fx приложение Воркера", zap.Error(err))
		os.Exit(1)
	}

	<-fxApp.Done()

	stopErr := fxApp.Err()
	if stopErr != nil && !errors.Is(stopErr, context.Canceled) {
		log.Error("Fx приложение Воркера завершилось с ошибкой во время остановки", zap.Error(stopErr))
		os.Exit(1)
	}

	log.Info("Сервис Воркер успешно завершил работу.")
}

// FxLogger
type FxLogger struct { log *zap.Logger }
func NewFxLogger(log *zap.Logger) *FxLogger { return &FxLogger{log: log.WithOptions(zap.AddCallerSkip(1))} }
func (l *FxLogger) Printf(format string, args ...interface{}) { l.log.Info(fmt.Sprintf(format, args...)) }