package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// WorkerClientParams содержит параметры для создания клиента Воркера.
type WorkerClientParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Logger    *zap.Logger
	Config    WorkerClientConfigProvider // Интерфейс для получения конфигурации
}

// WorkerClientConfigProvider определяет интерфейс для получения конфигурации клиента Воркера.
type WorkerClientConfigProvider interface {
	GetWorkerAddress() string
	GetGRPCClientTimeout() time.Duration // Используем общий таймаут
}

// NewWorkerServiceClient создает и возвращает новый клиент gRPC для сервиса Воркера.
func NewWorkerServiceClient(params WorkerClientParams) (pb.WorkerServiceClient, error) {
	params.Logger.Info("Попытка создания gRPC клиента для Воркера...",
		zap.String("адрес", params.Config.GetWorkerAddress()),
	)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(params.Config.GetWorkerAddress(), opts...)
	if err != nil {
		params.Logger.Error("Не удалось подключиться к gRPC серверу Воркера",
			zap.String("адрес", params.Config.GetWorkerAddress()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("не удалось подключиться к Воркеру (%s): %w", params.Config.GetWorkerAddress(), err)
	}

	params.Logger.Info("Успешно установлено gRPC соединение с Воркером",
		zap.String("адрес", params.Config.GetWorkerAddress()),
	)

	// Закрываем соединение при остановке Оркестратора
	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("Закрытие gRPC соединения с Воркером...")
			if err := conn.Close(); err != nil {
				params.Logger.Error("Ошибка при закрытии gRPC соединения с Воркером", zap.Error(err))
				return err
			}
			params.Logger.Info("gRPC соединение с Воркером успешно закрыто.")
			return nil
		},
	})

	client := pb.NewWorkerServiceClient(conn)
	return client, nil
}