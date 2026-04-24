package main

import (
	"context"

	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xMain "github.com/bamboo-services/bamboo-base-go/major/main"
	xReg "github.com/bamboo-services/bamboo-base-go/major/register"
	xGrpcIUnary "github.com/bamboo-services/bamboo-base-go/plugins/grpc/interceptor/unary"
	xGrpcRunner "github.com/bamboo-services/bamboo-base-go/plugins/grpc/runner"
	grpcApp "github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/grpc"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/route"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/startup"
	"google.golang.org/grpc"
)

func main() {
	reg := xReg.Register(startup.Init())
	log := xLog.WithName(xLog.NamedMAIN)

	grpcTask := xGrpcRunner.New(
		xGrpcRunner.WithRegisterService(func(ctx context.Context, server grpc.ServiceRegistrar) {
			grpcApp.RegisterGRPCServices(ctx, server)
		}),
		xGrpcRunner.WithUnaryInterceptors(
			xGrpcIUnary.InitContext(reg.Init.Ctx),
			xGrpcIUnary.Recover(),
			xGrpcIUnary.Middleware(),
			xGrpcIUnary.ResponseBuilder(),
		),
	)

	xMain.Runner(reg, log, route.NewRoute, grpcTask)
}
