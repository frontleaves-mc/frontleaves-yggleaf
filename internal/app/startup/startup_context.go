package startup

import (
	xCtx "github.com/bamboo-services/bamboo-base-go/context"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	"github.com/gin-gonic/gin"
)

type handler struct {
	Data *Reg
}

// businessContextInit 初始化系统上下文。
//
// 该方法为系统设置必要的上下文中间件，用于扩展 Gin 的上下文功能，
// 以便在请求生命周期内共享状态或传递必要的信息。
//
// 注意: 确保在 Gin 引擎初始化后调用此方法，以正确注册中间件。
func (r *Reg) businessContextInit() {
	xLog.WithName(xLog.NamedINIT).Info(r.Context, "上下文系统注入")

	// 创建处理器实例
	handler := &handler{
		Data: r,
	}

	// 注册系统上下文处理函数
	r.Serve.Use(handler.systemContextHandlerFunc)
}

// systemContextHandlerFunc 为 Gin 的中间件函数，用于在请求上下文中注入数据库和 Redis 客户端实例。
//
// 该方法通过设置 `xCtx.DatabaseKey` 和 `xCtx.RedisClientKey` 来提供共享的数据库
// 和 Redis 客户端，在整个请求生命周期内可供后续逻辑访问。
//
// 注意:
//   - 需要确保 `handler` 的 `Data` 属性被正确初始化，包含有效的数据库和 Redis 实例。
//   - 在调用 `c.Next()` 之前，所有注入的上下文内容都将在下一个中间件或最终的 handler 中可用。
func (h *handler) systemContextHandlerFunc(c *gin.Context) {
	// 数据库放行准则
	c.Set(xCtx.DatabaseKey.String(), h.Data.DB)     // 数据库实例
	c.Set(xCtx.RedisClientKey.String(), h.Data.RDB) // Redis 客户端实例

	// 放行内容
	c.Next()
}
