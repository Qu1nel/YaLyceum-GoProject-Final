package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	pb_worker "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker" // Убедись, что путь правильный
	"github.com/expr-lang/expr/ast"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrUnsupportedNodeType = errors.New("неподдерживаемый тип узла AST")
	ErrEvaluationTimeout   = errors.New("превышен таймаут вычисления выражения")
	// ErrWorkerCalculation = errors.New("ошибка вычисления на воркере") // Можно определить общую ошибку
)

type Evaluator interface {
	Evaluate(ctx context.Context, node ast.Node) (float64, error)
}

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
	select {
	case <-ctx.Done():
		return 0, fmt.Errorf("вычисление узла отменено перед обработкой: %w", ctx.Err())
	default:
	}

	switch n := node.(type) {
	case *ast.IntegerNode:
		return float64(n.Value), nil
	case *ast.FloatNode:
		return n.Value, nil
	case *ast.UnaryNode:
		operandVal, err := e.Evaluate(ctx, n.Node)
		if err != nil {
			return 0, fmt.Errorf("ошибка вычисления операнда для унарной операции '%s': %w", n.Operator, err)
		}
		if n.Operator == "-" {
			return e.callWorker(ctx, "neg", operandVal, 0) // Используем "neg" для унарного минуса
		}
		e.log.Error("Неподдерживаемый унарный оператор", zap.String("operator", n.Operator))
		return 0, fmt.Errorf("%w: унарный оператор '%s'", ErrUnsupportedNodeType, n.Operator)
	case *ast.BinaryNode:
		opSymbol := n.Operator
		// Обработка оператора возведения в степень "**" из expr как "^" для нашего воркера
		if opSymbol == "**" {
			opSymbol = "^"
		}


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

		// Важно: читаем из errChan ПОСЛЕ wg.Wait(), чтобы гарантировать, что горутины завершились.
		for i := 0; i < 2; i++ { // Проверяем обе возможные ошибки
            if evalErr := <-errChan; evalErr != nil {
                 e.log.Debug("Ошибка от дочернего узла при вычислении бинарной операции", zap.Error(evalErr))
                 return 0, evalErr // Возвращаем первую встреченную ошибку от дочерних узлов
            }
        }


		leftVal := <-leftChan
		rightVal := <-rightChan

		e.log.Debug("Вызов callWorker для бинарной операции",
			zap.String("operator", opSymbol),
			zap.Float64("left", leftVal),
			zap.Float64("right", rightVal),
		)
		result, workerErr := e.callWorker(ctx, opSymbol, leftVal, rightVal)
		e.log.Debug("Результат от callWorker для бинарной операции",
			zap.String("operator", opSymbol),
			zap.Float64("result", result),
			zap.Error(workerErr),
		)
		return result, workerErr
     case *ast.CallNode: // Функции, например, sqrt(x) или log(a,b)
        // Получаем имя функции
        // Callee - это узел, который вызывается, обычно это IdentifierNode с именем функции
        funcIdentNode, ok := n.Callee.(*ast.IdentifierNode)
        if !ok {
            // Это может произойти, если вызывается не просто имя, а что-то более сложное,
            // например, (func(){...})() или obj.method(). Пока это не поддерживаем.
            e.log.Error("Узел функции CallNode имеет Callee не типа IdentifierNode",
                zap.Any("callee_type", fmt.Sprintf("%T", n.Callee)),
            )
            return 0, fmt.Errorf("%w: неподдерживаемый тип вызываемого объекта в CallNode (%T)", ErrUnsupportedNodeType, n.Callee)
        }
        funcName := funcIdentNode.Value

        e.log.Info("Обнаружен вызов функции (пока не реализовано)", // Изменил с Warn на Info
            zap.String("function_name", funcName),
            zap.Int("arg_count", len(n.Arguments)),
        )

        // TODO: Реализовать поддержку функций:
        // 1. Рекурсивно вычислить каждый аргумент в n.Arguments:
        //    argsResults := make([]float64, len(n.Arguments))
        //    for i, argNode := range n.Arguments {
        //        argVal, err := e.Evaluate(ctx, argNode)
        //        if err != nil {
        //            return 0, fmt.Errorf("ошибка вычисления аргумента %d для функции '%s': %w", i+1, funcName, err)
        //        }
        //        argsResults[i] = argVal
        //    }
        // 2. Сформировать OperationSymbol для воркера (например, само имя функции "sqrt", "log")
        // 3. Вызвать воркер e.callWorker(ctx, funcName, argsResults[0], argsResults[1]... )
        //    (callWorker нужно будет адаптировать для функций с разным числом аргументов, или иметь разные методы в воркере)

        // Пока возвращаем ошибку, что функции не поддерживаются
        return 0, fmt.Errorf("%w: вызов функции '%s(%d арг.)' пока не реализован", ErrUnsupportedNodeType, funcName, len(n.Arguments))

	// Другие типы узлов, которые может вернуть expr, но мы их пока не обрабатываем
	case *ast.NilNode, *ast.IdentifierNode, *ast.BoolNode, *ast.StringNode,
         *ast.MemberNode, *ast.SliceNode, *ast.ArrayNode, *ast.MapNode,
         *ast.ConditionalNode, *ast.BuiltinNode,
         *ast.PointerNode, *ast.ConstantNode:
		e.log.Error("Неподдерживаемый тип узла AST в Evaluate", zap.Any("type", fmt.Sprintf("%T", n)))
		return 0, fmt.Errorf("%w: %T", ErrUnsupportedNodeType, n)
	default:
		e.log.Error("Неизвестный тип узла AST в Evaluate", zap.Any("type", fmt.Sprintf("%T", n)))
		return 0, fmt.Errorf("%w: неизвестный тип %T", ErrUnsupportedNodeType, n)
	}
}

func (e *ExpressionEvaluator) callWorker(ctx context.Context, opSymbol string, a, b float64) (float64, error) {
	operationID := uuid.NewString()
	e.log.Debug("Отправка операции Воркеру",
		zap.String("operationID", operationID),
		zap.String("symbol", opSymbol),
		zap.Float64("a", a),
		zap.Float64("b", b),
	)

	opCtx, cancel := context.WithTimeout(ctx, 5*time.Second) // Таймаут на конкретную операцию
	defer cancel()

	req := &pb_worker.CalculateOperationRequest{
		OperationId:     operationID,
		OperationSymbol: opSymbol,
		OperandA:        a,
		OperandB:        b,
	}

	res, grpcErr := e.workerClient.CalculateOperation(opCtx, req)
	e.log.Debug("Ответ от Воркера (сырой) в callWorker",
		zap.String("operationID", req.OperationId),
		zap.Any("response_body", res),
		zap.Error(grpcErr),
	)
    e.log.Debug("ExpressionEvaluator.callWorker: СЫРОЙ ОТВЕТ от workerClient.CalculateOperation",
		zap.String("operationID", req.OperationId),
		zap.String("opSymbol", opSymbol),
		zap.Any("response_body", res), // res может быть nil, если err != nil
		zap.Error(grpcErr),                // Это gRPC ошибка от клиента или статусная ошибка от сервера
	)

 	if grpcErr != nil { // Если есть gRPC ошибка
		e.log.Warn("ExpressionEvaluator.callWorker: Ошибка gRPC вызова Воркера", // Уточнил лог
			zap.String("operationID", req.OperationId),
			zap.String("symbol", opSymbol),
			zap.Error(grpcErr),
		)
		st, ok := status.FromError(grpcErr)
		if ok {
			if st.Code() == codes.InvalidArgument {
				return 0, errors.New(st.Message()) // Возвращаем только сообщение
			}
			if st.Code() == codes.DeadlineExceeded || errors.Is(opCtx.Err(), context.DeadlineExceeded) || errors.Is(opCtx.Err(), context.Canceled) {
				return 0, fmt.Errorf("таймаут/отмена операции '%s': %w", opSymbol, ErrEvaluationTimeout)
			}
			return 0, fmt.Errorf("gRPC ошибка от воркера (код %s) при операции '%s': %s", st.Code(), opSymbol, st.Message())
		}
		return 0, fmt.Errorf("ошибка связи с воркером при операции '%s': %w", opSymbol, grpcErr)
	}

	// Если gRPC вызов успешен (grpcErr == nil), но Воркер мог вернуть логическую ошибку в теле ответа
	// (Наш Воркер сейчас всегда возвращает gRPC статусную ошибку, так что этот блок может быть избыточен, но для полноты)
	if res != nil && res.ErrorMessage != "" {
		e.log.Warn("Воркер вернул ошибку в теле ответа (gRPC вызов успешен)",
			zap.String("operationID", req.OperationId),
			zap.String("symbol", opSymbol),
			zap.String("workerError", res.ErrorMessage),
		)
		return 0, errors.New(res.ErrorMessage)
	}
	
	if res == nil { // Очень маловероятно, если grpcErr == nil
        e.log.Error("Неожиданный nil ответ от воркера без ошибки gRPC", zap.String("operationID", req.OperationId))
        return 0, fmt.Errorf("неожиданный пустой ответ от воркера для операции '%s'", opSymbol)
    }


	e.log.Debug("Операция успешно выполнена Воркером",
		zap.String("operationID", req.OperationId),
		zap.Float64("result", res.Result),
	)
	return res.Result, nil
}