package client

import (
	"context"
	"fmt"
	"time"

	pb "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/orchestrator"

	"go.uber.org/fx"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type OrchestratorClientParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Logger    *zap.Logger
	Config    OrchestratorClientConfigProvider
}

type OrchestratorClientConfigProvider interface {
	GetOrchestratorAddress() string
	GetGRPCClientTimeout() time.Duration
}

func NewOrchestratorServiceClient(params OrchestratorClientParams) (pb.OrchestratorServiceClient, error) {
	params.Logger.Info("Попытка создания gRPC клиента для Оркестратора...",
		zap.String("адрес", params.Config.GetOrchestratorAddress()),
	)

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.NewClient(params.Config.GetOrchestratorAddress(), opts...)
	if err != nil {
		params.Logger.Error("Не удалось подключиться к gRPC серверу Оркестратора",
			zap.String("адрес", params.Config.GetOrchestratorAddress()),
			zap.Error(err),
		)
		return nil, fmt.Errorf("не удалось подключиться к Оркестратору (%s): %w", params.Config.GetOrchestratorAddress(), err)
	}

	params.Logger.Info("Успешно установлено gRPC соединение с Оркестратором",
		zap.String("адрес", params.Config.GetOrchestratorAddress()),
	)

	params.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			params.Logger.Info("Закрытие gRPC соединения с Оркестратором...")
			if err := conn.Close(); err != nil {
				params.Logger.Error("Ошибка при закрытии gRPC соединения с Оркестратором", zap.Error(err))
				return err
			}
			params.Logger.Info("gRPC соединение с Оркестратором успешно закрыто.")
			return nil
		},
	})

	client := pb.NewOrchestratorServiceClient(conn)
	return client, nil
}
