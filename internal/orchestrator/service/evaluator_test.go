package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Qu1nel/YaLyceum-GoProject-Final/internal/orchestrator/service/mocks"
	pb_worker "github.com/Qu1nel/YaLyceum-GoProject-Final/proto/gen/worker"
	"github.com/expr-lang/expr/ast"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap" // Импорт для grpc.CallOption
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Вспомогательная функция для создания мок-клиента и evaluator
func setupEvaluatorTest(t *testing.T) (*ExpressionEvaluator, *mocks.WorkerServiceClientMock) {
	logger := zap.NewNop()
	mockWorkerClient := mocks.NewWorkerServiceClientMock(t)
	evaluator := NewExpressionEvaluator(logger, mockWorkerClient)
	return evaluator, mockWorkerClient
}

// --- Тесты для callWorker ---
func TestExpressionEvaluator_callWorker_Success(t *testing.T) {
	evaluator, mockWorkerClient := setupEvaluatorTest(t)
	ctx := context.Background()

	opSymbol := "+"
	opA, opB := 10.0, 5.0
	expectedResult := 15.0

	// Ожидаем вызов CalculateOperation на моке
	// Аргументы: контекст, запрос, и variadic opts (мы передаем 0 опций)
	mockWorkerClient.On("CalculateOperation",
		mock.Anything, // Для контекста (любой контекст, т.к. он оборачивается)
		mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
			return req.OperationSymbol == opSymbol && req.OperandA == opA && req.OperandB == opB
		}),
		// mock.Anything, // Если бы мы передавали одну CallOption
		// Если мы не передаем CallOption в реальном коде, то и здесь не указываем лишние аргументы для opts.
		// Если mock.Mock.Called ожидает точное кол-во аргументов, то нужно соответствовать.
		// Либо можно сделать мок более гибким (но это сложнее в testify/mock для variadic).
		// Проще всего убедиться, что наш код вызывает с 0 CallOption, и мок ожидает 0 CallOption.
		// В сгенерированном моке вызов _m.Called(_ca...) передает opts как слайс.
		// Если opts пустой, то передается пустой слайс.
		// Попробуем без mock.Anything для opts, если мы не передаем опции.
	).Return(&pb_worker.CalculateOperationResponse{Result: expectedResult}, nil).Once()

	result, err := evaluator.callWorker(ctx, opSymbol, opA, opB)
	require.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_callWorker_WorkerError(t *testing.T) {
	evaluator, mockWorkerClient := setupEvaluatorTest(t)
	ctx := context.Background()
	workerErrMsg := "деление на ноль от воркера"

	mockWorkerClient.On("CalculateOperation", mock.Anything, mock.AnythingOfType("*worker_grpc.CalculateOperationRequest")). // Уточнили тип запроса
		Return(&pb_worker.CalculateOperationResponse{ErrorMessage: workerErrMsg}, nil).Once()

	_, err := evaluator.callWorker(ctx, "/", 10.0, 0.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка вычисления операции '/'")
	assert.Contains(t, err.Error(), workerErrMsg)
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_callWorker_gRPCError(t *testing.T) {
	evaluator, mockWorkerClient := setupEvaluatorTest(t)
	ctx := context.Background()
	grpcErr := status.Error(codes.Unavailable, "воркер недоступен")

	mockWorkerClient.On("CalculateOperation", mock.Anything, mock.AnythingOfType("*worker_grpc.CalculateOperationRequest")).
		Return(nil, grpcErr).Once()

	_, err := evaluator.callWorker(ctx, "+", 1.0, 2.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка воркера при операции '+'")
	s, ok := status.FromError(errors.Unwrap(err))
	require.True(t, ok)
	assert.Equal(t, codes.Unavailable, s.Code())
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_callWorker_Timeout(t *testing.T) {
    evaluator, mockWorkerClient := setupEvaluatorTest(t)
    // Этот контекст будет передан в callWorker
    // callWorker создаст свой дочерний opCtx с таймаутом 5с
    // Если этот внешний ctx истечет раньше, то opCtx тоже отменится
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond) // Очень короткий таймаут
    defer cancel()

    // Ожидаем, что CalculateOperation будет вызван, но вернет ошибку из-за отмены контекста
    // (если он успеет вызваться до того, как мы залогируем ошибку отмены)
    // Или он может вообще не вызваться, если проверка ctx.Done() в callWorker сработает раньше.
    // В данном случае, таймаут в callWorker (5s) больше чем 1ms, так что отмена произойдет
    // из-за внешнего контекста.
    // Если CalculateOperation успеет вызваться, он получит уже отмененный контекст.
    // GRPC клиент обычно преобразует context.DeadlineExceeded в gRPC ошибку codes.DeadlineExceeded
    mockWorkerClient.On("CalculateOperation",
        mock.Anything, // Контекст, который будет уже отменен или близок к отмене
        mock.AnythingOfType("*worker_grpc.CalculateOperationRequest"),
    ).Return(nil, status.Error(codes.DeadlineExceeded, "контекст вызова операции истек")).Maybe() // Maybe, т.к. вызов может не случиться или вернуть ошибку

    time.Sleep(5 * time.Millisecond) // Даем контексту время точно истечь

    _, err := evaluator.callWorker(ctx, "+", 1.0, 2.0)
    require.Error(t, err)

    // Проверяем, что ошибка связана с таймаутом/отменой
    // Ошибка может быть либо ErrEvaluationTimeout (наш), либо обернутая gRPC ошибка codes.DeadlineExceeded
    if !errors.Is(err, ErrEvaluationTimeout) {
        s, ok := status.FromError(errors.Unwrap(err)) // errors.Unwrap(err) чтобы добраться до gRPC ошибки
        if ok {
            assert.Equal(t, codes.DeadlineExceeded, s.Code(), "Ожидался gRPC DeadlineExceeded")
        } else {
            // Если это не gRPC ошибка, проверяем на context.DeadlineExceeded
            assert.ErrorIs(t, err, context.DeadlineExceeded, "Ожидалась ошибка context.DeadlineExceeded или обернутый DeadlineExceeded")
        }
    }
    // mockWorkerClient.AssertExpectations(t) // Может не выполниться, если контекст истек до вызова
}


// --- Тесты для Evaluate ---
func TestExpressionEvaluator_Evaluate_IntegerNode(t *testing.T) {
	evaluator, _ := setupEvaluatorTest(t)
	node := &ast.IntegerNode{Value: 123}
	result, err := evaluator.Evaluate(context.Background(), node)
	require.NoError(t, err)
	assert.Equal(t, 123.0, result)
}

func TestExpressionEvaluator_Evaluate_FloatNode(t *testing.T) {
	evaluator, _ := setupEvaluatorTest(t)
	node := &ast.FloatNode{Value: 12.34}
	result, err := evaluator.Evaluate(context.Background(), node)
	require.NoError(t, err)
	assert.Equal(t, 12.34, result)
}

func TestExpressionEvaluator_Evaluate_UnaryNode_Minus(t *testing.T) {
	evaluator, mockWorkerClient := setupEvaluatorTest(t)
	ctx := context.Background()
	node := &ast.UnaryNode{Operator: "-", Node: &ast.IntegerNode{Value: 5}}
	expectedWorkerResult := -5.0

	mockWorkerClient.On("CalculateOperation",
		mock.Anything, // Контекст
		mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
			return req.OperationSymbol == "neg" && req.OperandA == 5.0
		}),
	).Return(&pb_worker.CalculateOperationResponse{Result: expectedWorkerResult}, nil).Once()

	result, err := evaluator.Evaluate(ctx, node)
	require.NoError(t, err)
	assert.Equal(t, expectedWorkerResult, result)
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_Evaluate_BinaryNode_Simple(t *testing.T) {
	evaluator, mockWorkerClient := setupEvaluatorTest(t)
	ctx := context.Background()
	node := &ast.BinaryNode{Operator: "+", Left: &ast.IntegerNode{Value: 2}, Right: &ast.IntegerNode{Value: 3}}
	expectedWorkerResult := 5.0

	mockWorkerClient.On("CalculateOperation",
		mock.Anything, // Контекст
		mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
			return req.OperationSymbol == "+" && req.OperandA == 2.0 && req.OperandB == 3.0
		}),
	).Return(&pb_worker.CalculateOperationResponse{Result: expectedWorkerResult}, nil).Once()

	result, err := evaluator.Evaluate(ctx, node)
	require.NoError(t, err)
	assert.Equal(t, expectedWorkerResult, result)
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_Evaluate_BinaryNode_Nested(t *testing.T) {
    evaluator, mockWorkerClient := setupEvaluatorTest(t)
    ctx := context.Background()
    node := &ast.BinaryNode{
        Operator: "*",
        Left: &ast.BinaryNode{Operator: "+", Left: &ast.IntegerNode{Value: 2}, Right: &ast.IntegerNode{Value: 3}},
        Right:    &ast.IntegerNode{Value: 4},
    }

    // Ожидаем первый вызов воркера для 2+3
    mockWorkerClient.On("CalculateOperation",
        mock.Anything,
        mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
            return req.OperationSymbol == "+" && req.OperandA == 2.0 && req.OperandB == 3.0
        }),
    ).Return(&pb_worker.CalculateOperationResponse{Result: 5.0}, nil).Once()

    // Ожидаем второй вызов воркера для (результат_сложения)*4, т.е. 5*4
    mockWorkerClient.On("CalculateOperation",
        mock.Anything,
        mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
            return req.OperationSymbol == "*" && req.OperandA == 5.0 && req.OperandB == 4.0
        }),
    ).Return(&pb_worker.CalculateOperationResponse{Result: 20.0}, nil).Once()

    result, err := evaluator.Evaluate(ctx, node)
    require.NoError(t, err)
    assert.Equal(t, 20.0, result)
    mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_Evaluate_UnsupportedNode(t *testing.T) {
    evaluator, _ := setupEvaluatorTest(t)
    node := &ast.StringNode{Value: "hello"}
    _, err := evaluator.Evaluate(context.Background(), node)
    require.Error(t, err)
    assert.ErrorIs(t, err, ErrUnsupportedNodeType)
    assert.Contains(t, err.Error(), "строковое значение")
}

func TestExpressionEvaluator_Evaluate_ErrorInLeftChild(t *testing.T) {
    evaluator, mockWorkerClient := setupEvaluatorTest(t)
    ctx := context.Background()
    node := &ast.BinaryNode{
        Operator: "+",
        Left:     &ast.StringNode{Value: "error"},
        Right:    &ast.IntegerNode{Value: 3},
    }

    result, err := evaluator.Evaluate(ctx, node)
    require.Error(t, err)
    assert.ErrorIs(t, err, ErrUnsupportedNodeType)
    assert.Contains(t, err.Error(), "левый операнд для '+'") // Ошибка должна быть обернута
    assert.Equal(t, 0.0, result)
    mockWorkerClient.AssertNotCalled(t, "CalculateOperation")
}


func TestExpressionEvaluator_Evaluate_ErrorInRightChild_WorkerCall(t *testing.T) {
    evaluator, mockWorkerClient := setupEvaluatorTest(t)
    ctx := context.Background()
    node := &ast.BinaryNode{
        Operator: "+",
        Left:     &ast.IntegerNode{Value: 2},
        Right: &ast.UnaryNode{Operator: "-", Node: &ast.IntegerNode{Value: 5}},
    }
    grpcErr := status.Error(codes.Internal, "внутренняя ошибка воркера")

    // Правый операнд (-5) вызовет воркер, который вернет ошибку
    mockWorkerClient.On("CalculateOperation",
        mock.Anything,
        mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
            return req.OperationSymbol == "neg" && req.OperandA == 5.0
        }),
    ).Return(nil, grpcErr).Once()


    result, err := evaluator.Evaluate(ctx, node)
    require.Error(t, err)
    assert.Contains(t, err.Error(), "правый операнд для '+'")
    assert.Contains(t, err.Error(), "ошибка воркера при операции 'neg'")
    // Проверяем, что обернутая ошибка содержит gRPC статус
    cause := errors.Unwrap(errors.Unwrap(err)) // Два уровня обертки
    s, ok := status.FromError(cause)
    require.True(t, ok, "Ожидалась gRPC ошибка, обернутая в ошибку evaluator")
    assert.Equal(t, codes.Internal, s.Code())
    assert.Equal(t, 0.0, result)
    mockWorkerClient.AssertExpectations(t)
}

// В файле internal/orchestrator/service/evaluator_test.go

func TestExpressionEvaluator_Evaluate_ContextCancelled(t *testing.T) {
    evaluator, mockWorkerClient := setupEvaluatorTest(t)
    // Создаем контекст, который истечет очень быстро
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond) // Короткий таймаут

    // Используем простое выражение, которое сделает только один вызов воркера
    // AST для 2 + 3
    node := &ast.BinaryNode{
        Operator: "+",
        Left:     &ast.IntegerNode{Value: 2},
        Right:    &ast.IntegerNode{Value: 3},
    }

    // Мокируем вызов для "+"
    // Используем .Maybe() так как из-за быстрой отмены контекста, этот вызов может и не произойти
    // или произойдет, но функция callWorker вернет ошибку контекста до получения ответа от мока.
    mockWorkerClient.On("CalculateOperation",
        mock.Anything, // Контекст
        mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
            return req.OperationSymbol == "+" && req.OperandA == 2.0 && req.OperandB == 3.0
        }),
    ).Run(func(args mock.Arguments) {
        // Имитируем некоторую работу, чтобы контекст мог отмениться
        // пока воркер "работает"
        callCtx := args.Get(0).(context.Context)
        select {
        case <-time.After(50 * time.Millisecond): // Длительная "работа" воркера
        case <-callCtx.Done(): // Если контекст отменился во время "работы"
            return
        }
    }).Return(&pb_worker.CalculateOperationResponse{Result: 5.0}, nil).Maybe()


    // Гарантированно отменяем контекст после небольшой задержки,
    // чтобы дать горутинам в Evaluate шанс запуститься, но отмениться до завершения.
    // Или можно отменить ДО вызова Evaluate, чтобы проверить самый ранний выход.
    time.Sleep(5 * time.Millisecond) // Даем немного времени, но меньше, чем "работа" воркера
    cancel() // Отменяем контекст

    _, err := evaluator.Evaluate(ctx, node)
    require.Error(t, err, "Ожидалась ошибка из-за отмены контекста")

    // Проверяем, что это ошибка, связанная с отменой контекста.
    // Ошибка может быть обернута несколько раз.
    // evaluator.Evaluate -> ошибка из Evaluate дочернего узла -> ошибка из callWorker -> context.DeadlineExceeded/Canceled
    ok := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
    if !ok {
        // Если это не стандартная ошибка контекста, возможно, это наша ErrEvaluationTimeout
        ok = errors.Is(err, ErrEvaluationTimeout)
    }
    assert.True(t, ok, "Ожидалась ошибка context.Canceled, context.DeadlineExceeded или ErrEvaluationTimeout, получено: %v", err)

    // mockWorkerClient.AssertExpectations(t) // Этот ассерт может быть ненадежным здесь,
                                          // так как вызов .Maybe() и отмена контекста
                                          // делают выполнение вызова воркера необязательным.
                                          // Главное, что Evaluate вернул ошибку отмены.
}