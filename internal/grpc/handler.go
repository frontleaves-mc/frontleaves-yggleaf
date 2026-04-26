package grpc

import (
	"context"

	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic"
	bSdkLogic "github.com/phalanx-labs/beacon-sso-sdk/logic"
)

// grpcService gRPC 服务的业务逻辑处理层
//
// 该类型封装了 gRPC Handler 共用的业务规则和数据处理流程，
// 充当 Handler 与数据访问层之间的桥梁。
type grpcService struct {
	userLogic        *logic.UserLogic
	accessUserLogic  *logic.AccessUserLogic
	oauthLogic       *bSdkLogic.BusinessLogic
	gameProfileLogic *logic.GameProfileLogic
}

// grpcHandler gRPC 请求处理器的基类结构体
//
// 该结构体作为所有具体 gRPC Handler（如 AuthHandler）的嵌入基础，提供了统一的日志记录和业务逻辑调用能力。
// 它遵循依赖倒置原则，将具体的业务处理委托给 grpcService 层，自身仅负责参数校验和响应封装。
type grpcHandler struct {
	name    string
	log     *xLog.LogNamedLogger
	service *grpcService
}

// IGRPCHandler gRPC Handler 泛型约束接口
type IGRPCHandler interface {
	~struct {
		name    string
		log     *xLog.LogNamedLogger
		service *grpcService
	}
}

// NewGRPCHandler 泛型 gRPC 处理器构造函数
//
// 通过泛型类型 T 实例化并返回一个新的 gRPC 处理器指针 *T。
// 该函数利用泛型约束 T IGRPCHandler，确保 T 必须实现了 IGRPCHandler 接口。
func NewGRPCHandler[T IGRPCHandler](ctx context.Context, handlerName string) *T {
	return &T{
		name: handlerName,
		log:  xLog.WithName(xLog.NamedGRPC, handlerName),
		service: &grpcService{
			userLogic:        logic.NewUserLogic(ctx),
			accessUserLogic:  logic.NewAccessUserLogic(ctx),
			oauthLogic:       bSdkLogic.NewBusiness(ctx),
			gameProfileLogic: logic.NewGameProfileLogic(ctx, logic.NewLibraryLogic(ctx)),
		},
	}
}
