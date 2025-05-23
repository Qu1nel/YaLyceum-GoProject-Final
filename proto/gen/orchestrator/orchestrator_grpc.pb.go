// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.5.1
// - protoc             v6.30.2
// source: proto/orchestrator.proto

package orchestrator_grpc

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	OrchestratorService_SubmitExpression_FullMethodName = "/orchestrator.OrchestratorService/SubmitExpression"
	OrchestratorService_GetTaskDetails_FullMethodName   = "/orchestrator.OrchestratorService/GetTaskDetails"
	OrchestratorService_ListUserTasks_FullMethodName    = "/orchestrator.OrchestratorService/ListUserTasks"
)

// OrchestratorServiceClient is the client API for OrchestratorService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
//
// Сервис Оркестратора
type OrchestratorServiceClient interface {
	// Отправка выражения на вычисление (вызывается Агентом)
	SubmitExpression(ctx context.Context, in *ExpressionRequest, opts ...grpc.CallOption) (*ExpressionResponse, error)
	// Получение статуса и результата задачи (вызывается Агентом) (TBD)
	GetTaskDetails(ctx context.Context, in *TaskDetailsRequest, opts ...grpc.CallOption) (*TaskDetailsResponse, error)
	// Получение списка задач пользователя (вызывается Агентом) (TBD)
	ListUserTasks(ctx context.Context, in *UserTasksRequest, opts ...grpc.CallOption) (*UserTasksResponse, error)
}

type orchestratorServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewOrchestratorServiceClient(cc grpc.ClientConnInterface) OrchestratorServiceClient {
	return &orchestratorServiceClient{cc}
}

func (c *orchestratorServiceClient) SubmitExpression(ctx context.Context, in *ExpressionRequest, opts ...grpc.CallOption) (*ExpressionResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ExpressionResponse)
	err := c.cc.Invoke(ctx, OrchestratorService_SubmitExpression_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orchestratorServiceClient) GetTaskDetails(ctx context.Context, in *TaskDetailsRequest, opts ...grpc.CallOption) (*TaskDetailsResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(TaskDetailsResponse)
	err := c.cc.Invoke(ctx, OrchestratorService_GetTaskDetails_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *orchestratorServiceClient) ListUserTasks(ctx context.Context, in *UserTasksRequest, opts ...grpc.CallOption) (*UserTasksResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(UserTasksResponse)
	err := c.cc.Invoke(ctx, OrchestratorService_ListUserTasks_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// OrchestratorServiceServer is the server API for OrchestratorService service.
// All implementations must embed UnimplementedOrchestratorServiceServer
// for forward compatibility.
//
// Сервис Оркестратора
type OrchestratorServiceServer interface {
	// Отправка выражения на вычисление (вызывается Агентом)
	SubmitExpression(context.Context, *ExpressionRequest) (*ExpressionResponse, error)
	// Получение статуса и результата задачи (вызывается Агентом) (TBD)
	GetTaskDetails(context.Context, *TaskDetailsRequest) (*TaskDetailsResponse, error)
	// Получение списка задач пользователя (вызывается Агентом) (TBD)
	ListUserTasks(context.Context, *UserTasksRequest) (*UserTasksResponse, error)
	mustEmbedUnimplementedOrchestratorServiceServer()
}

// UnimplementedOrchestratorServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedOrchestratorServiceServer struct{}

func (UnimplementedOrchestratorServiceServer) SubmitExpression(context.Context, *ExpressionRequest) (*ExpressionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SubmitExpression not implemented")
}
func (UnimplementedOrchestratorServiceServer) GetTaskDetails(context.Context, *TaskDetailsRequest) (*TaskDetailsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetTaskDetails not implemented")
}
func (UnimplementedOrchestratorServiceServer) ListUserTasks(context.Context, *UserTasksRequest) (*UserTasksResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListUserTasks not implemented")
}
func (UnimplementedOrchestratorServiceServer) mustEmbedUnimplementedOrchestratorServiceServer() {}
func (UnimplementedOrchestratorServiceServer) testEmbeddedByValue()                             {}

// UnsafeOrchestratorServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to OrchestratorServiceServer will
// result in compilation errors.
type UnsafeOrchestratorServiceServer interface {
	mustEmbedUnimplementedOrchestratorServiceServer()
}

func RegisterOrchestratorServiceServer(s grpc.ServiceRegistrar, srv OrchestratorServiceServer) {
	// If the following call pancis, it indicates UnimplementedOrchestratorServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&OrchestratorService_ServiceDesc, srv)
}

func _OrchestratorService_SubmitExpression_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ExpressionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestratorServiceServer).SubmitExpression(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: OrchestratorService_SubmitExpression_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestratorServiceServer).SubmitExpression(ctx, req.(*ExpressionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _OrchestratorService_GetTaskDetails_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(TaskDetailsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestratorServiceServer).GetTaskDetails(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: OrchestratorService_GetTaskDetails_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestratorServiceServer).GetTaskDetails(ctx, req.(*TaskDetailsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _OrchestratorService_ListUserTasks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(UserTasksRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(OrchestratorServiceServer).ListUserTasks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: OrchestratorService_ListUserTasks_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(OrchestratorServiceServer).ListUserTasks(ctx, req.(*UserTasksRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// OrchestratorService_ServiceDesc is the grpc.ServiceDesc for OrchestratorService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var OrchestratorService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "orchestrator.OrchestratorService",
	HandlerType: (*OrchestratorServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SubmitExpression",
			Handler:    _OrchestratorService_SubmitExpression_Handler,
		},
		{
			MethodName: "GetTaskDetails",
			Handler:    _OrchestratorService_GetTaskDetails_Handler,
		},
		{
			MethodName: "ListUserTasks",
			Handler:    _OrchestratorService_ListUserTasks_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/orchestrator.proto",
}
