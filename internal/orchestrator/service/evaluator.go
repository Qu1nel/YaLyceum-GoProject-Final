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

// Evaluator определяет интерфейс для сервиса вычисления выражений.
type Evaluator interface {
	Evaluate(ctx context.Context, node ast.Node) (float64, error)
}

// ExpressionEvaluator отвечает за рекурсивное вычисление AST.
// Он реализует интерфейс Evaluator.
type ExpressionEvaluator struct {
    log          *zap.Logger
    workerClient pb_worker.WorkerServiceClient
}

func NewExpressionEvaluator(log *zap.Logger, workerClient pb_worker.WorkerServiceClient) Evaluator {
    return &ExpressionEvaluator{
        log:          log,
        workerClient: workerClient,
    }
}

func (e *ExpressionEvaluator) Evaluate(ctx context.Context, node ast.Node) (float64, error) {
    // ... (код метода Evaluate без изменений) ...
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
        e.log.Error("IdentifierNode не поддерживается", zap.String("value", n.Value))
        return 0, fmt.Errorf("%w: идентификатор '%s'", ErrUnsupportedNodeType, n.Value)
    case *ast.IntegerNode:
        return float64(n.Value), nil
    case *ast.FloatNode:
        return n.Value, nil
    case *ast.BoolNode:
        e.log.Error("BoolNode не поддерживается", zap.Bool("value", n.Value))
        return 0, fmt.Errorf("%w: булево значение", ErrUnsupportedNodeType)
    case *ast.StringNode:
        e.log.Error("StringNode не поддерживается", zap.String("value", n.Value))
        return 0, fmt.Errorf("%w: строковое значение", ErrUnsupportedNodeType)
    case *ast.UnaryNode:
        operandVal, err := e.Evaluate(ctx, n.Node)
        if err != nil {
            return 0, fmt.Errorf("ошибка вычисления операнда для унарной операции '%s': %w", n.Operator, err)
        }
        if n.Operator == "-" {
            return e.callWorker(ctx, "neg", operandVal, 0)
        }
        return 0, fmt.Errorf("%w: унарный оператор '%s'", ErrUnsupportedNodeType, n.Operator)
    case *ast.BinaryNode:
        opSymbol := n.Operator
        leftChan := make(chan float64, 1)
        rightChan := make(chan float64, 1)
        errChan := make(chan error, 2)
        var wg sync.WaitGroup
        wg.Add(2)
        go func() {
            defer wg.Done()
            val, err := e.Evaluate(ctx, n.Left)
            if err != nil {
                errChan <- fmt.Errorf("левый операнд для '%s': %w", opSymbol, err)
                return
            }
            leftChan <- val
        }()
        go func() {
            defer wg.Done()
            val, err := e.Evaluate(ctx, n.Right)
            if err != nil {
                errChan <- fmt.Errorf("правый операнд для '%s': %w", opSymbol, err)
                return
            }
            rightChan <- val
        }()
        wg.Wait()
        close(leftChan)
        close(rightChan)
        close(errChan)
        if firstErr := <-errChan; firstErr != nil {
             return 0, firstErr
        }
        if secondErr := <-errChan; secondErr != nil { // Проверяем вторую ошибку, если первая была nil
             return 0, secondErr
        }
        leftVal := <-leftChan
        rightVal := <-rightChan
        return e.callWorker(ctx, opSymbol, leftVal, rightVal)
    case *ast.CallNode:
        // TODO: Реализовать поддержку функций позже
		return 0, fmt.Errorf("TMP") // Заглушка для функций
    default:
        e.log.Error("Неизвестный тип узла AST", zap.Any("type", fmt.Sprintf("%T", n)))
        return 0, fmt.Errorf("%w: %T", ErrUnsupportedNodeType, n)
    }
}

// callWorker отправляет операцию на вычисление Воркеру.
func (e *ExpressionEvaluator) callWorker(ctx context.Context, opSymbol string, a, b float64) (float64, error) {
    // ... (код метода callWorker без изменений) ...
    operationID := uuid.NewString()
    e.log.Debug("Отправка операции Воркеру",
        zap.String("operationID", operationID),
        zap.String("symbol", opSymbol),
        zap.Float64("a", a),
        zap.Float64("b", b),
    )
    // TODO: Получить таймаут для операции из конфига или передать в Evaluate
    opCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    req := &pb_worker.CalculateOperationRequest{
        OperationId:     operationID,
        OperationSymbol: opSymbol,
        OperandA:        a,
        OperandB:        b,
    }
    res, err := e.workerClient.CalculateOperation(opCtx, req)
    if err != nil {
        e.log.Error("Ошибка gRPC вызова Воркера", zap.String("operationID", operationID), zap.Error(err))
        if s, ok := status.FromError(err); ok {
            if s.Code() == codes.DeadlineExceeded {
                return 0, fmt.Errorf("таймаут операции '%s': %w", opSymbol, ErrEvaluationTimeout)
            }
        }
        return 0, fmt.Errorf("ошибка воркера при операции '%s': %w", opSymbol, err)
    }
    if res.ErrorMessage != "" {
        e.log.Warn("Воркер вернул ошибку для операции", zap.String("operationID", operationID), zap.String("symbol", opSymbol), zap.String("workerError", res.ErrorMessage))
        return 0, fmt.Errorf("ошибка вычисления операции '%s': %s", opSymbol, res.ErrorMessage)
    }
    e.log.Debug("Операция успешно выполнена Воркером", zap.String("operationID", operationID), zap.Float64("result", res.Result))
    return res.Result, nil
}