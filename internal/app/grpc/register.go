package grpc

import (
	"context"

	grpchandler "github.com/frontleaves-mc/frontleaves-yggleaf/internal/grpc"
	"google.golang.org/grpc"
)

// RegisterGRPCServices 注册所有 gRPC 服务
//
// 每个服务在 Handler 构造函数中绑定各自的服务级中间件。
func RegisterGRPCServices(ctx context.Context, server grpc.ServiceRegistrar) {
	grpchandler.NewAuthHandler(ctx, server)
}
