package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	pb_worker "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"
	"github.com/expr-lang/expr/ast"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
    ErrUnsupportedNodeType = errors.New("неподдерживаемый тип узла AST")
    ErrEvaluationTimeout   = errors.New("превышен таймаут вычисления выражения")
)

// ExpressionEvaluator отвечает за рекурсивное вычисление AST.
type ExpressionEvaluator struct {
    log          *zap.Logger
    workerClient pb_worker.WorkerServiceClient
    // Можно добавить таймаут на одну операцию, если он не передается извне
    // operationTimeout time.Duration
}

// NewExpressionEvaluator создает новый вычислитель.
func NewExpressionEvaluator(log *zap.Logger, workerClient pb_worker.WorkerServiceClient) *ExpressionEvaluator {
    return &ExpressionEvaluator{
        log:          log,
        workerClient: workerClient,
    }
}

// Evaluate рекурсивно вычисляет значение узла AST.
// ctx - контекст для управления временем выполнения и отменой.
func (e *ExpressionEvaluator) Evaluate(ctx context.Context, node ast.Node) (float64, error) {
    // Проверяем, не отменен ли контекст перед началом обработки узла
    select {
    case <-ctx.Done():
        return 0, fmt.Errorf("вычисление узла отменено: %w", ctx.Err())
    default:
        // Продолжаем, если контекст не отменен
    }

    switch n := node.(type) {
    case *ast.NilNode:
         e.log.Error("Обнаружен NilNode в AST, это неожиданно")
         return 0, fmt.Errorf("%w: nil node", ErrUnsupportedNodeType)
    case *ast.IdentifierNode:
        // Переменные/идентификаторы пока не поддерживаем в простом калькуляторе
        e.log.Error("IdentifierNode не поддерживается", zap.String("value", n.Value))
        return 0, fmt.Errorf("%w: идентификатор '%s'", ErrUnsupportedNodeType, n.Value)
    case *ast.IntegerNode:
        return float64(n.Value), nil
    case *ast.FloatNode:
        return n.Value, nil
    case *ast.BoolNode:
        // Булевы значения пока не обрабатываем как числа
        e.log.Error("BoolNode не поддерживается", zap.Bool("value", n.Value))
        return 0, fmt.Errorf("%w: булево значение", ErrUnsupportedNodeType)
    case *ast.StringNode:
        // Строки не обрабатываем как числа
        e.log.Error("StringNode не поддерживается", zap.String("value", n.Value))
        return 0, fmt.Errorf("%w: строковое значение", ErrUnsupportedNodeType)

    case *ast.UnaryNode: // Например, -5
        operandVal, err := e.Evaluate(ctx, n.Node)
        if err != nil {
            return 0, fmt.Errorf("ошибка вычисления операнда для унарной операции '%s': %w", n.Operator, err)
        }
        // Для унарного минуса можно либо отправить специальный символ Воркеру,
        // либо обработать здесь, если Воркер ожидает два операнда для минуса.
        // Пока предположим, что унарный минус - это 0 - X.
        // Или отправляем "neg" и один операнд.
        // Для простоты, пока делаем его как операцию "neg" с одним операндом.
        if n.Operator == "-" { // Унарный минус
            return e.callWorker(ctx, "neg", operandVal, 0) // Второй операнд 0 или игнорируется
        }
        return 0, fmt.Errorf("%w: унарный оператор '%s'", ErrUnsupportedNodeType, n.Operator)

    case *ast.BinaryNode: // Например, a + b
        opSymbol := n.Operator

        // Каналы для получения результатов от параллельных вычислений дочерних узлов
        leftChan := make(chan float64, 1)
        rightChan := make(chan float64, 1)
        errChan := make(chan error, 2) // Буферизованный на 2 возможные ошибки

        var wg sync.WaitGroup
        wg.Add(2) // Два дочерних узла

        // Вычисляем левый операнд в горутине
        go func() {
            defer wg.Done()
            // Передаем контекст дальше
            val, err := e.Evaluate(ctx, n.Left)
            if err != nil {
                errChan <- fmt.Errorf("левый операнд для '%s': %w", opSymbol, err)
                return
            }
            leftChan <- val
        }()

        // Вычисляем правый операнд в горутине
        go func() {
            defer wg.Done()
            // Передаем контекст дальше
            val, err := e.Evaluate(ctx, n.Right)
            if err != nil {
                errChan <- fmt.Errorf("правый операнд для '%s': %w", opSymbol, err)
                return
            }
            rightChan <- val
        }()

        // Ожидаем завершения обеих горутин
        wg.Wait()
        close(leftChan)  // Закрываем каналы после WaitGroup, чтобы сигнализировать range
        close(rightChan) // (хотя здесь мы читаем только по одному значению)
        close(errChan)

        // Проверяем ошибки от дочерних вычислений
        // Берем первую ошибку, если она есть
        if firstErr := <-errChan; firstErr != nil {
             // Если одна из веток завершилась ошибкой, остальные ошибки из errChan не так важны.
             // Важно не блокироваться на чтении из каналов результатов.
             return 0, firstErr
        }
         // Если была еще одна ошибка (маловероятно при правильной логике выше, но для полноты)
        if secondErr := <-errChan; secondErr != nil {
             return 0, secondErr
        }


        // Получаем результаты (если не было ошибок)
        // Эти чтения не должны блокироваться, если ошибок не было и wg.Wait() завершился
        leftVal := <-leftChan
        rightVal := <-rightChan

        // Вызываем Воркер для выполнения операции
        return e.callWorker(ctx, opSymbol, leftVal, rightVal)

    case *ast.CallNode: // Функции, например, sqrt(x) или log(a,b)
        // TODO: Реализовать поддержку функций позже
        //e.log.Error("CallNode (функции) пока не поддерживаются", zap.Any("node_name", n.Node.(*ast.IdentifierNode).Value))
        //return 0, fmt.Errorf("%w: функции типа '%s'", ErrUnsupportedNodeType, n.Node.(*ast.IdentifierNode).Value)
		return 0, fmt.Errorf("TMP")

    default:
        e.log.Error("Неизвестный тип узла AST", zap.Any("type", fmt.Sprintf("%T", n)))
        return 0, fmt.Errorf("%w: %T", ErrUnsupportedNodeType, n)
    }
}

// callWorker отправляет операцию на вычисление Воркеру.
func (e *ExpressionEvaluator) callWorker(ctx context.Context, opSymbol string, a, b float64) (float64, error) {
    operationID := uuid.NewString() // Генерируем ID для каждой операции
    e.log.Debug("Отправка операции Воркеру",
        zap.String("operationID", operationID),
        zap.String("symbol", opSymbol),
        zap.Float64("a", a),
        zap.Float64("b", b),
    )

    // TODO: Получить таймаут для операции из конфига или передать в Evaluate
    opCtx, cancel := context.WithTimeout(ctx, 5*time.Second) // Пример таймаута на операцию
    defer cancel()

    req := &pb_worker.CalculateOperationRequest{
        OperationId:     operationID,
        OperationSymbol: opSymbol,
        OperandA:        a,
        OperandB:        b,
    }

    res, err := e.workerClient.CalculateOperation(opCtx, req)
    if err != nil {
        e.log.Error("Ошибка gRPC вызова Воркера",
            zap.String("operationID", operationID),
            zap.Error(err),
        )
        // Проверяем, не является ли это ошибкой отмены/таймаута
        if s, ok := status.FromError(err); ok {
            if s.Code() == codes.DeadlineExceeded {
                return 0, fmt.Errorf("таймаут операции '%s': %w", opSymbol, ErrEvaluationTimeout)
            }
        }
        return 0, fmt.Errorf("ошибка воркера при операции '%s': %w", opSymbol, err)
    }

    if res.ErrorMessage != "" {
        e.log.Warn("Воркер вернул ошибку для операции",
            zap.String("operationID", operationID),
            zap.String("symbol", opSymbol),
            zap.String("workerError", res.ErrorMessage),
        )
        // Преобразуем ошибку воркера в стандартную ошибку
        return 0, fmt.Errorf("ошибка вычисления операции '%s': %s", opSymbol, res.ErrorMessage)
    }

    e.log.Debug("Операция успешно выполнена Воркером",
        zap.String("operationID", operationID),
        zap.Float64("result", res.Result),
    )
    return res.Result, nil
}