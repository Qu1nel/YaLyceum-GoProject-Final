package shutdown

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Graceful выполняет корректное завершение работы приложения.
// Он слушает сигналы SIGINT и SIGTERM, отменяет главный контекст приложения
// и дает время (timeout) на завершение работы запущенных компонентов (HTTP, gRPC, БД).
func Graceful(
	mainCtx context.Context, 
	mainCancel context.CancelFunc, 
	log *zap.Logger,
	timeout time.Duration, 
	servers map[string]func(context.Context) error, 
	dbPool *pgxpool.Pool, 
	// grpcServer *grpc.Server, // Добавить позже
) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-quit:
			log.Info("Получен сигнал ОС для завершения работы", zap.String("сигнал", sig.String()))
			mainCancel() 
		case <-mainCtx.Done():
			log.Debug("Главный контекст приложения отменен, начинаем завершение...")
		}
	}()

	<-mainCtx.Done()

	log.Info("Начинаем корректное завершение работы...", zap.Duration("таймаут", timeout))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
	defer shutdownCancel() 

	var wg sync.WaitGroup

	// Останавливаем компоненты в обратном порядке их запуска (и по логике зависимостей)

	// 1. Остановка HTTP сервера
	if stopHTTPServer, ok := servers["http"]; ok {
		wg.Add(1)
		go func() {
			defer wg.Done()
			log.Debug("Остановка HTTP сервера...")
			if err := stopHTTPServer(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
				log.Error("Ошибка при остановке HTTP сервера", zap.Error(err))
			} else {
				log.Info("HTTP сервер успешно остановлен")
			}
		}()
	}

	// 2. Остановка gRPC сервера (когда появится)
	// if grpcServer != nil {
	//     wg.Add(1)
	//     go func() {
	//         defer wg.Done()
	//         log.Debug("Остановка gRPC сервера...")
	//         grpcServer.GracefulStop() // GracefulStop сам обрабатывает контекст
	//         log.Info("gRPC сервер успешно остановлен")
	//     }()
	// }

	// 3. Закрытие пула соединений к БД (после остановки серверов)
	// Ожидаем завершения остановки серверов перед закрытием пула
	allServersStopped := make(chan struct{})
	go func() {
		wg.Wait()
		close(allServersStopped)
	}()

	select {
	case <-allServersStopped:
		log.Debug("Все серверы остановлены, закрываем пул БД...")
		if dbPool != nil {
			dbPool.Close()
			log.Info("Пул соединений с БД успешно закрыт")
		}
	case <-shutdownCtx.Done():
		log.Error("Таймаут корректного завершения истек до остановки всех компонентов", zap.Error(shutdownCtx.Err()))
		if dbPool != nil {
			dbPool.Close()
			log.Warn("Пул соединений с БД закрыт принудительно из-за таймаута")
		}
	}

	log.Info("Корректное завершение работы завершено.")
}