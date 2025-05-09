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

func Run() {
	appCtx, cancel := context.WithCancel(context.Background())

	tempCfg, err := config.Load()
	var log *zap.Logger
	if err != nil {
		log, _ = zap.NewProduction()
		log.Fatal("Worker: не удалось загрузить начальную конфигурацию", zap.Error(err))
	} else {
		log, err = logger.New(tempCfg.Logger.Level, tempCfg.AppEnv)
		if err != nil {
			log, _ = zap.NewProduction()
			log.Fatal("Worker: не удалось инициализировать логгер", zap.Error(err))
		}
	}
	defer func() {
		if syncErr := log.Sync(); syncErr != nil {
			fmt.Fprintf(os.Stderr, "Worker: ошибка синхронизации логгера: %v\n", syncErr)
		}
	}()

	fxApp := fx.New(
		fx.Logger(NewFxLogger(log)),
		fx.Provide(
			func() (*config.Config, error) {
				cfg, loadErr := config.Load()
				if loadErr != nil {
					log.Fatal("Worker: не удалось загрузить конфигурацию для DI", zap.Error(loadErr))
					return nil, loadErr
				}
				return cfg, nil
			},
			func() *zap.Logger { return log },
			service.NewCalculatorService,
			grpc_handler.NewWorkerServer,
			func(l *zap.Logger) *grpc.Server {

				srv := grpc.NewServer()
				l.Info("Worker: создан инстанс gRPC сервера")
				return srv
			},
		),
		fx.Invoke(func(lc fx.Lifecycle,
			grpcServer *grpc.Server,
			workerHandler *grpc_handler.WorkerServer,
			cfg *config.Config,
			l *zap.Logger,
		) {
			pb.RegisterWorkerServiceServer(grpcServer, workerHandler)
			l.Info("Worker: gRPC обработчик зарегистрирован")

			grpcAddr := ":" + cfg.GRPCServer.Port
			listener, listenErr := net.Listen("tcp", grpcAddr)
			if listenErr != nil {
				l.Fatal("Worker: не удалось начать слушать порт для gRPC", zap.String("адрес", grpcAddr), zap.Error(listenErr))
			}

			lc.Append(fx.Hook{
				OnStart: func(startCtx context.Context) error {
					l.Info("Worker: запуск gRPC сервера", zap.String("адрес", grpcAddr))
					go func() {
						if serveErr := grpcServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, grpc.ErrServerStopped) {
							l.Fatal("Worker: gRPC сервер неожиданно завершил работу", zap.Error(serveErr))
							cancel()
						}
					}()
					return nil
				},
				OnStop: func(stopCtx context.Context) error {
					l.Info("Worker: остановка gRPC сервера...")

					done := make(chan struct{})
					go func() {
						grpcServer.GracefulStop()
						close(done)
					}()

					select {
					case <-done:
						l.Info("Worker: gRPC сервер успешно остановлен (GracefulStop).")
					case <-stopCtx.Done():
						l.Error("Worker: таймаут при корректной остановке gRPC сервера, принудительная остановка.", zap.Error(stopCtx.Err()))
						grpcServer.Stop()
						return stopCtx.Err()
					}
					return nil
				},
			})

			go shutdown.Graceful(appCtx, cancel, l, cfg.GracefulTimeout, nil, nil)
		}),
	)

	if startErr := fxApp.Start(appCtx); startErr != nil {
		log.Error("Worker: не удалось запустить Fx приложение", zap.Error(startErr))
		os.Exit(1)
	}

	<-fxApp.Done()

	if stopErr := fxApp.Err(); stopErr != nil && !errors.Is(stopErr, context.Canceled) {
		log.Error("Worker: Fx приложение завершилось с ошибкой при остановке", zap.Error(stopErr))
		os.Exit(1)
	}
	log.Info("Worker: сервис успешно завершил работу.")
}

type FxLogger struct{ log *zap.Logger }

func NewFxLogger(log *zap.Logger) *FxLogger {
	return &FxLogger{log: log.WithOptions(zap.AddCallerSkip(1))}
}

func (l *FxLogger) Printf(format string, args ...interface{}) {
	l.log.Info(fmt.Sprintf(format, args...))
}
