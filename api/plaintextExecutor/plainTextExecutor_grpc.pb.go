// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.4.0
// - protoc             v5.27.2
// source: plainTextExecutor.proto

package plainTextExecutor

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	wrapperspb "google.golang.org/protobuf/types/known/wrapperspb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.62.0 or later.
const _ = grpc.SupportPackageIsVersion8

const (
	PlainTextExecutor_ExecuteBatch_FullMethodName = "/plainTextExecutor/executeBatch"
	PlainTextExecutor_InitDb_FullMethodName       = "/plainTextExecutor/initDb"
)

// PlainTextExecutorClient is the client API for PlainTextExecutor service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type PlainTextExecutorClient interface {
	ExecuteBatch(ctx context.Context, in *RequestBatch, opts ...grpc.CallOption) (*RespondBatch, error)
	InitDb(ctx context.Context, in *RequestBatch, opts ...grpc.CallOption) (*wrapperspb.BoolValue, error)
}

type plainTextExecutorClient struct {
	cc grpc.ClientConnInterface
}

func NewPlainTextExecutorClient(cc grpc.ClientConnInterface) PlainTextExecutorClient {
	return &plainTextExecutorClient{cc}
}

func (c *plainTextExecutorClient) ExecuteBatch(ctx context.Context, in *RequestBatch, opts ...grpc.CallOption) (*RespondBatch, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(RespondBatch)
	err := c.cc.Invoke(ctx, PlainTextExecutor_ExecuteBatch_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *plainTextExecutorClient) InitDb(ctx context.Context, in *RequestBatch, opts ...grpc.CallOption) (*wrapperspb.BoolValue, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(wrapperspb.BoolValue)
	err := c.cc.Invoke(ctx, PlainTextExecutor_InitDb_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PlainTextExecutorServer is the server API for PlainTextExecutor service.
// All implementations must embed UnimplementedPlainTextExecutorServer
// for forward compatibility
type PlainTextExecutorServer interface {
	ExecuteBatch(context.Context, *RequestBatch) (*RespondBatch, error)
	InitDb(context.Context, *RequestBatch) (*wrapperspb.BoolValue, error)
	mustEmbedUnimplementedPlainTextExecutorServer()
}

// UnimplementedPlainTextExecutorServer must be embedded to have forward compatible implementations.
type UnimplementedPlainTextExecutorServer struct {
}

func (UnimplementedPlainTextExecutorServer) ExecuteBatch(context.Context, *RequestBatch) (*RespondBatch, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ExecuteBatch not implemented")
}
func (UnimplementedPlainTextExecutorServer) InitDb(context.Context, *RequestBatch) (*wrapperspb.BoolValue, error) {
	return nil, status.Errorf(codes.Unimplemented, "method InitDb not implemented")
}
func (UnimplementedPlainTextExecutorServer) mustEmbedUnimplementedPlainTextExecutorServer() {}

// UnsafePlainTextExecutorServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to PlainTextExecutorServer will
// result in compilation errors.
type UnsafePlainTextExecutorServer interface {
	mustEmbedUnimplementedPlainTextExecutorServer()
}

func RegisterPlainTextExecutorServer(s grpc.ServiceRegistrar, srv PlainTextExecutorServer) {
	s.RegisterService(&PlainTextExecutor_ServiceDesc, srv)
}

func _PlainTextExecutor_ExecuteBatch_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RequestBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlainTextExecutorServer).ExecuteBatch(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PlainTextExecutor_ExecuteBatch_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlainTextExecutorServer).ExecuteBatch(ctx, req.(*RequestBatch))
	}
	return interceptor(ctx, in, info, handler)
}

func _PlainTextExecutor_InitDb_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(RequestBatch)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PlainTextExecutorServer).InitDb(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: PlainTextExecutor_InitDb_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PlainTextExecutorServer).InitDb(ctx, req.(*RequestBatch))
	}
	return interceptor(ctx, in, info, handler)
}

// PlainTextExecutor_ServiceDesc is the grpc.ServiceDesc for PlainTextExecutor service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var PlainTextExecutor_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "plainTextExecutor",
	HandlerType: (*PlainTextExecutorServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "executeBatch",
			Handler:    _PlainTextExecutor_ExecuteBatch_Handler,
		},
		{
			MethodName: "initDb",
			Handler:    _PlainTextExecutor_InitDb_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "plainTextExecutor.proto",
}