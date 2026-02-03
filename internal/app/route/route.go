package route

import (
	"context"

	xEnv "github.com/bamboo-services/bamboo-base-go/env"
	xMiddle "github.com/bamboo-services/bamboo-base-go/middleware"
	xReg "github.com/bamboo-services/bamboo-base-go/register"
	xRoute "github.com/bamboo-services/bamboo-base-go/route"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/startup"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// route 代表 HTTP 路由配置的核心结构体。
//
// 该类型用于管理和绑定应用中的所有路由。其核心字段 `engine` 是一个 `*gin.Engine`，用于处理 HTTP 请求。
//
// 注意: `route` 类型的实例应通过工厂方法初始化，确保路由结构的完整性和中间件的有效性。
type route struct {
	engine  *gin.Engine     // 路由引擎
	db      *gorm.DB        // GORM 数据库实例
	rdb     *redis.Client   // Redis 客户端实例
	context context.Context // 上下文，用于控制取消和超时
}

// NewRoute 初始化路由配置。
//
// 该函数接收一个 `*gin.Engine` 实例，用于注册应用程序的所有路由。
// 它通过内部方法调用完成特定模块的路由绑定。
//
// 参数说明:
//   - g: Gin 引擎实例，是 HTTP 请求的核心处理器。
//
// 注意: 确保在调用此函数之前已完成 Gin 引擎的初始化和中间件的注册。
func NewRoute(xReg *xReg.Reg, reg *startup.Reg, context context.Context) {
	r := &route{
		engine:  xReg.Serve,
		db:      reg.DB,
		rdb:     reg.RDB,
		context: context,
	}

	// 全局异常处理
	r.engine.NoMethod(xRoute.NoMethod)
	r.engine.NoRoute(xRoute.NoRoute)

	// 全局响应处理
	r.engine.Use(xMiddle.ResponseMiddleware)
	r.engine.Use(xMiddle.ReleaseAllCors)
	r.engine.Use(xMiddle.AllowOption)

	// Swagger Register
	if xEnv.GetEnvBool(xEnv.Debug, false) {
		swaggerRegister(r.engine)
	}

	// 路由初始化注册
	{
		apiRouter := r.engine.Group("/api/v1")
		r.authRouter(apiRouter)
	}
}
