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
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func setupEvaluatorTest(t *testing.T) (Evaluator, *mocks.WorkerServiceClientMock) {
	logger := zap.NewNop()
	mockWorkerClient := mocks.NewWorkerServiceClientMock(t)
	evaluator := NewExpressionEvaluator(logger, mockWorkerClient)
	return evaluator, mockWorkerClient
}

func TestExpressionEvaluator_callWorker_Success(t *testing.T) {

	logger := zap.NewNop()
	mockWorkerClient := mocks.NewWorkerServiceClientMock(t)
	evaluatorImpl := NewExpressionEvaluator(logger, mockWorkerClient).(*ExpressionEvaluator)

	ctx := context.Background()
	opSymbol := "+"
	opA, opB := 10.0, 5.0
	expectedResult := 15.0

	mockWorkerClient.On("CalculateOperation",
		mock.Anything,
		mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
			return req.OperationSymbol == opSymbol && req.OperandA == opA && req.OperandB == opB
		}),
	).Return(&pb_worker.CalculateOperationResponse{Result: expectedResult}, nil).Once()

	result, err := evaluatorImpl.callWorker(ctx, opSymbol, opA, opB)
	require.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_callWorker_WorkerError(t *testing.T) {
	logger := zap.NewNop()
	mockWorkerClient := mocks.NewWorkerServiceClientMock(t)
	evaluatorImpl := NewExpressionEvaluator(logger, mockWorkerClient).(*ExpressionEvaluator)

	ctx := context.Background()
	workerErrMsg := "деление на ноль от воркера"

	mockWorkerClient.On("CalculateOperation", mock.Anything, mock.AnythingOfType("*worker_grpc.CalculateOperationRequest")).
		Return(&pb_worker.CalculateOperationResponse{ErrorMessage: workerErrMsg}, nil).Once()

	_, err := evaluatorImpl.callWorker(ctx, "/", 10.0, 0.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "деление на ноль от воркера")
	assert.Contains(t, err.Error(), workerErrMsg)
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_callWorker_gRPCError(t *testing.T) {
	logger := zap.NewNop()
	mockWorkerClient := mocks.NewWorkerServiceClientMock(t)
	evaluatorImpl := NewExpressionEvaluator(logger, mockWorkerClient).(*ExpressionEvaluator)

	ctx := context.Background()
	grpcErr := status.Error(codes.Unavailable, "воркер недоступен")

	mockWorkerClient.On("CalculateOperation", mock.Anything, mock.AnythingOfType("*worker_grpc.CalculateOperationRequest")).
		Return(nil, grpcErr).Once()

	_, err := evaluatorImpl.callWorker(ctx, "+", 1.0, 2.0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "воркер недоступен")
	_, ok := status.FromError(errors.Unwrap(err))
	require.True(t, ok)
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_callWorker_Timeout(t *testing.T) {
	logger := zap.NewNop()
	mockWorkerClient := mocks.NewWorkerServiceClientMock(t)
	evaluatorImpl := NewExpressionEvaluator(logger, mockWorkerClient).(*ExpressionEvaluator)

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	mockWorkerClient.On("CalculateOperation",
		mock.Anything,
		mock.AnythingOfType("*worker_grpc.CalculateOperationRequest"),
	).Return(nil, status.Error(codes.DeadlineExceeded, "контекст вызова операции истек")).Maybe()

	time.Sleep(5 * time.Millisecond)

	_, err := evaluatorImpl.callWorker(ctx, "+", 1.0, 2.0)
	require.Error(t, err)

	if !errors.Is(err, ErrEvaluationTimeout) {
		s, ok := status.FromError(errors.Unwrap(err))
		if ok {
			assert.Equal(t, codes.DeadlineExceeded, s.Code(), "Ожидался gRPC DeadlineExceeded")
		} else {
			assert.ErrorIs(t, err, context.DeadlineExceeded, "Ожидалась ошибка context.DeadlineExceeded или обернутый DeadlineExceeded")
		}
	}
}

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
		mock.Anything,
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
		mock.Anything,
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
		Left:     &ast.BinaryNode{Operator: "+", Left: &ast.IntegerNode{Value: 2}, Right: &ast.IntegerNode{Value: 3}},
		Right:    &ast.IntegerNode{Value: 4},
	}

	mockWorkerClient.On("CalculateOperation",
		mock.Anything,
		mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
			return req.OperationSymbol == "+" && req.OperandA == 2.0 && req.OperandB == 3.0
		}),
	).Return(&pb_worker.CalculateOperationResponse{Result: 5.0}, nil).Once()

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
	assert.Contains(t, err.Error(), "неподдерживаемый тип узла AST")
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
	assert.Contains(t, err.Error(), "левый операнд для '+'")
	assert.Equal(t, 0.0, result)
	mockWorkerClient.AssertNotCalled(t, "CalculateOperation")
}

func TestExpressionEvaluator_Evaluate_ErrorInRightChild_WorkerCall(t *testing.T) {
	evaluator, mockWorkerClient := setupEvaluatorTest(t)
	ctx := context.Background()
	node := &ast.BinaryNode{
		Operator: "+",
		Left:     &ast.IntegerNode{Value: 2},
		Right:    &ast.UnaryNode{Operator: "-", Node: &ast.IntegerNode{Value: 5}},
	}
	grpcErr := status.Error(codes.Internal, "внутренняя ошибка воркера")

	mockWorkerClient.On("CalculateOperation",
		mock.Anything,
		mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
			return req.OperationSymbol == "neg" && req.OperandA == 5.0
		}),
	).Return(nil, grpcErr).Once()

	_, err := evaluator.Evaluate(ctx, node)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "правый операнд для '+'")
	assert.Contains(t, err.Error(), "внутренняя ошибка воркера")
	cause := errors.Unwrap(errors.Unwrap(err))
	_, ok := status.FromError(cause)
	require.True(t, ok, "Ожидалась gRPC ошибка, обернутая в ошибку evaluator")
	mockWorkerClient.AssertExpectations(t)
}

func TestExpressionEvaluator_Evaluate_ContextCancelled(t *testing.T) {
	evaluator, mockWorkerClient := setupEvaluatorTest(t)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	node := &ast.BinaryNode{
		Operator: "+",
		Left:     &ast.IntegerNode{Value: 2},
		Right:    &ast.IntegerNode{Value: 3},
	}
	mockWorkerClient.On("CalculateOperation",
		mock.Anything,
		mock.MatchedBy(func(req *pb_worker.CalculateOperationRequest) bool {
			return req.OperationSymbol == "+" && req.OperandA == 2.0 && req.OperandB == 3.0
		}),
	).Run(func(args mock.Arguments) {
		callCtx := args.Get(0).(context.Context)
		select {
		case <-time.After(50 * time.Millisecond):
		case <-callCtx.Done():
			return
		}
	}).Return(&pb_worker.CalculateOperationResponse{Result: 5.0}, nil).Maybe()

	time.Sleep(5 * time.Millisecond)
	cancel()

	_, err := evaluator.Evaluate(ctx, node)
	require.Error(t, err, "Ожидалась ошибка из-за отмены контекста")
	ok := errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
	if !ok {
		ok = errors.Is(err, ErrEvaluationTimeout)
	}
	assert.True(t, ok, "Ожидалась ошибка context.Canceled, context.DeadlineExceeded или ErrEvaluationTimeout, получено: %v", err)
}
