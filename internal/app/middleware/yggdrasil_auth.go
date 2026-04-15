package middleware

import (
	"context"
	"net/http"
	"strings"

	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	apiYgg "github.com/frontleaves-mc/frontleaves-yggleaf/api/yggdrasil"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic/yggdrasil"
	"github.com/gin-gonic/gin"
)

// YggdrasilBearerAuth Yggdrasil Bearer Token 认证中间件。
//
// 该中间件从请求的 Authorization 头中提取 accessToken，通过查询 GameToken 实体
// 验证令牌有效性，并将有效的游戏令牌实体注入 Gin 上下文中。
//
// 与现有的 OAuth2/SSO 认证中间件完全独立，两套认证体系通过不同的路由组隔离：
//   - 现有路由组：OAuth2 Bearer Token → SSO Userinfo → User 实体
//   - Yggdrasil 路由组：accessToken → GameToken 实体 → User/GameProfile
//
// 注意：YggdrasilLogic 实例在闭包外部创建一次（启动时），所有请求共享同一实例。
// YggdrasilLogic 本身是无状态的（仅持有 db/rdb 引用和 RSA 密钥引用），线程安全。
//
// 参数:
//   - ctx: 主上下文，用于依赖注入和日志记录
func YggdrasilBearerAuth(ctx context.Context) gin.HandlerFunc {
	log := xLog.WithName(xLog.NamedMIDE, "YggdrasilBearerAuth")
	yggLogic := yggdrasil.NewYggdrasilLogic(ctx)

	return func(c *gin.Context) {
		log.Info(c, "验证 Yggdrasil Bearer Token")

		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			apiYgg.AbortWithPredefinedError(c, http.StatusUnauthorized, apiYgg.ErrUnauthorized)
			return
		}

		// 提取 Bearer Token
		accessToken := strings.TrimPrefix(authHeader, "Bearer ")
		if accessToken == authHeader {
			// 没有 Bearer 前缀，视为无效
			apiYgg.AbortWithPredefinedError(c, http.StatusUnauthorized, apiYgg.ErrUnauthorized)
			return
		}

		// 查询游戏令牌实体
		gameToken, found, xErr := yggLogic.ValidateGameToken(c.Request.Context(), accessToken)
		if xErr != nil {
			apiYgg.AbortYggError(c, http.StatusInternalServerError, "InternalServerError", "游戏令牌验证失败")
			return
		}
		if !found {
			apiYgg.AbortWithPredefinedError(c, http.StatusForbidden, apiYgg.ErrForbidden)
			return
		}

		// 注入游戏令牌实体到上下文
		newCtx := context.WithValue(c.Request.Context(), bConst.CtxYggdrasilGameToken, gameToken)
		c.Request = c.Request.WithContext(newCtx)
		c.Next()
	}
}
