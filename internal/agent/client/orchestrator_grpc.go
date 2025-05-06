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

// OrchestratorClientParams содержит параметры для создания клиента Оркестратора.
// Fx использует эту структуру для внедрения зависимостей.
type OrchestratorClientParams struct {
    fx.In
    Lifecycle fx.Lifecycle // Для управления жизненным циклом соединения
    Logger    *zap.Logger
    Config    OrchestratorClientConfigProvider // Интерфейс для получения конфигурации
}

// OrchestratorClientConfigProvider определяет интерфейс для получения конфигурации клиента.
// Это позволяет легко мокать конфигурацию в тестах.
type OrchestratorClientConfigProvider interface {
    GetOrchestratorAddress() string
    GetGRPCClientTimeout() time.Duration
}

// NewOrchestratorServiceClient создает и возвращает новый клиент gRPC для сервиса Оркестратора.
// Он также управляет жизненным циклом gRPC соединения.
func NewOrchestratorServiceClient(params OrchestratorClientParams) (pb.OrchestratorServiceClient, error) {
    params.Logger.Info("Попытка создания gRPC клиента для Оркестратора...",
        zap.String("адрес", params.Config.GetOrchestratorAddress()),
    )

    // Настройки для gRPC соединения
    // Для локальной разработки используем insecure (без TLS).
    // В production нужно будет настроить TLS.
    opts := []grpc.DialOption{
        grpc.WithTransportCredentials(insecure.NewCredentials()), // Отключаем TLS
        // grpc.WithBlock(), // Можно использовать WithBlock, чтобы Dial был синхронным и возвращал ошибку сразу, если сервер недоступен
        // Опции Keepalive для поддержания соединения (можно настроить позже)
        // grpc.WithKeepaliveParams(keepalive.ClientParameters{
        // 	Time:                10 * time.Second, // Отправлять пинг каждые 10 секунд, если нет активности
        // 	Timeout:             time.Second,    // Ждать ответа на пинг 1 секунду
        // 	PermitWithoutStream: true,           // Разрешить пинги даже если нет активных стримов
        // }),
    }

    // Создаем контекст с таймаутом для установки соединения
    // Это не таймаут для каждого вызова, а таймаут на сам Dial
    // dialCtx, cancelDial := context.WithTimeout(context.Background(), params.Config.GetGRPCClientTimeout()) // Можно использовать общий таймаут
    // defer cancelDial()

    // Устанавливаем соединение с gRPC сервером Оркестратора.
    // grpc.NewClient заменил grpc.Dial в новых версиях
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

    // Регистрируем хук Fx Lifecycle для корректного закрытия соединения
    // при остановке приложения Agent.
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

    // Создаем клиентский стаб на основе соединения.
    client := pb.NewOrchestratorServiceClient(conn)
    return client, nil
}