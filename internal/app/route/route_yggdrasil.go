package route

import (
	yggmiddleware "github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/middleware"
	ygghandler "github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler/yggdrasil"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler/yggdrasil/server"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler/yggdrasil/client"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/handler/yggdrasil/share"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/gin-gonic/gin"
)

// yggdrasilRouter 注册 Yggdrasil 外置登录协议的所有路由。
//
// Yggdrasil 路由挂载在 `/api/v1/yggdrasil` 前缀下，与现有的 `/api/v1` 管理路由
// （用户、档案、资源库）互不冲突。前端页面通过 ALI 响应头指向此前缀。
//
// 路由结构：
//   - /api/v1/yggdrasil/                             → API 元数据（share）
//   - /api/v1/yggdrasil/authserver/*                 → 认证服务（client）
//   - /api/v1/yggdrasil/sessionserver/session/minecraft/* → 会话服务（server + client）
//   - /api/v1/yggdrasil/api/*                        → 角色查询 + 材质管理（server + share）
func (r *route) yggdrasilRouter() {
	base := ygghandler.NewYggdrasilBase(r.context, "YggdrasilHandler")
	serverHandler := server.NewServerHandler(base)
	clientHandler := client.NewClientHandler(base)
	shareHandler := share.NewShareHandler(base)

	// 添加 ALI 响应头中间件
	yggGroup := r.engine.Group(bConst.YggdrasilAPIPrefix)
	yggGroup.Use(func(c *gin.Context) {
		c.Header(bConst.YggdrasilALIHeader, bConst.YggdrasilALIPath)
		c.Next()
	})

	// #1: API 元数据（无需认证）
	yggGroup.GET("/", shareHandler.APIMetadata)

	// 认证服务（无需认证——这些接口本身就是认证端点）
	authGroup := yggGroup.Group("/authserver")
	{
		// #2: 登录认证（速率限制：按 username 限流，防止暴力破解）
		authGroup.POST("/authenticate",
			yggmiddleware.YggdrasilAuthRateLimit("authenticate", bConst.YggdrasilAuthRateLimit),
			clientHandler.Authenticate)
		authGroup.POST("/refresh", clientHandler.Refresh)             // #3
		authGroup.POST("/validate", clientHandler.Validate)           // #4
		authGroup.POST("/invalidate", clientHandler.Invalidate)       // #5
		// #6: 登出（速率限制：按 username 限流，防止强制登出 DoS）
		authGroup.POST("/signout",
			yggmiddleware.YggdrasilAuthRateLimit("signout", bConst.YggdrasilSignoutRateLimit),
			clientHandler.Signout)
	}

	// 会话服务
	sessionGroup := yggGroup.Group("/sessionserver/session/minecraft")
	{
		sessionGroup.POST("/join", clientHandler.JoinServer)        // #7: accessToken 在请求体中（handler 内验证）
		sessionGroup.GET("/hasJoined", serverHandler.HasJoined)     // #8: 无需认证
	}

	// #9: 查询角色属性（无需认证 — 必须在 Bearer Auth 中间件挂载之前注册）
	yggGroup.GET("/sessionserver/session/minecraft/profile/:uuid", serverHandler.ProfileQuery)

	// #10: 批量查询角色（无需认证 — 同上）
	yggGroup.POST("/api/profiles/minecraft", serverHandler.ProfilesBatchLookup)

	// 需 Bearer Token 认证的路由组（#11, #12 通过 Authorization 头认证）
	// 注意：Gin 的 group.Use() 原地修改中间件链，此后注册到 yggGroup 的路由都会继承该中间件
	authRequired := yggGroup.Use(yggmiddleware.YggdrasilBearerAuth(r.context))
	{
		// #11: 上传材质（Bearer 认证，令牌从 context 获取）
		authRequired.PUT("/api/user/profile/:uuid/:textureType", shareHandler.UploadTexture)
		// #12: 清除材质（Bearer 认证，令牌从 context 获取）
		authRequired.DELETE("/api/user/profile/:uuid/:textureType", shareHandler.DeleteTexture)
	}
}
