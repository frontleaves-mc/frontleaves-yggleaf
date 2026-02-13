package handler

import (
	"context"

	xLog "github.com/bamboo-services/bamboo-base-go/log"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic"
	bSdkLogic "github.com/phalanx/beacon-sso-sdk/logic"
)

// service 核心业务逻辑处理层
//
// 该类型封装了应用程序的核心业务规则和数据处理流程，充当 Handler 与数据访问层之间的桥梁。
type service struct {
	userLogic        *logic.UserLogic
	gameProfileLogic *logic.GameProfileLogic
	oauthLogic       *bSdkLogic.BusinessLogic
}

// handler HTTP 请求处理器的基类结构体
//
// 该结构体作为所有具体业务 Handler（如 UserHandler）的嵌入基础，提供了统一的日志记录和业务逻辑调用能力。
// 它遵循依赖倒置原则，将具体的业务处理委托给 `service` 层，自身仅负责请求鉴权、参数校验和响应封装。
//
// 字段说明:
//   - log: 结构化日志组件，携带 "CONT" 命名标识，用于记录请求上下文和错误追踪。
//   - service: 核心业务逻辑处理层实例，包含具体的业务规则实现。
type handler struct {
	name    string               // name 处理器名称标识，用于在日志和上下文中唯一标识该业务模块。
	log     *xLog.LogNamedLogger // 日志实例
	service *service             // 服务实例
}

type IHandler interface {
	~struct {
		name    string               // name 处理器名称标识，用于在日志和上下文中唯一标识该业务模块。
		log     *xLog.LogNamedLogger // 日志实例
		service *service             // 服务实例
	}
}

// NewHandler 泛型处理器构造函数
//
// 通过泛型类型 T 实例化并返回一个新的处理器指针 *T。
// 该函数利用泛型约束 T IHandler，确保 T 必须实现了 IHandler 接口。
//
// 该函数主要执行以下初始化逻辑：
// 1. 依赖注入: 从传入的上下文 (ctx) 中提取并注入数据库连接 (*gorm.DB) 和 Redis 客户端 (*redis.Client)。
// 2. 日志组件: 初始化并绑定一个名为 "CONT" 的日志记录器，用于链路追踪和问题排查。
// 3. 服务层聚合: 内部实例化核心业务逻辑层 service，并将其挂载到 Handler 中，形成标准的 Handler -> Service 架构。
//
// 类型参数:
//   - T IHandler: 目标处理器类型，必须基于 handler 结构体或其别名，并实现了 IHandler 接口。
//
// 参数:
//   - ctx context.Context: 应用上下文，必须包含由 xCtxUtil 管理的数据库和 Redis 连接实例。
//
// 返回值:
//   - *T: 初始化完成的处理器实例指针。若上下文缺少必要依赖（如 db 或 rdb），该函数内部调用的
//     xCtxUtil.MustGetDB/rdb 将会触发 panic。请确保上下文已通过中间件正确注入了这些资源。
func NewHandler[T IHandler](ctx context.Context, handlerName string) *T {
	return &T{
		name: handlerName,
		log:  xLog.WithName(xLog.NamedCONT, handlerName),
		service: &service{
			userLogic:        logic.NewUserLogic(ctx),
			gameProfileLogic: logic.NewGameProfileLogic(ctx),
			oauthLogic:       bSdkLogic.NewBusiness(ctx),
		},
	}
}

// ###########
//   Handler
// ###########

// UserHandler 用户接口
type UserHandler handler

type GameProfileHandler handler
