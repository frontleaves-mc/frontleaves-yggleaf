package middleware

import (
	"context"

	xHttp "github.com/bamboo-services/bamboo-base-go/http"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xResult "github.com/bamboo-services/bamboo-base-go/result"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic"
	"github.com/gin-gonic/gin"
	bSdkLogic "github.com/phalanx/beacon-sso-sdk/logic"
)

// User 中间件用于注入用户身份信息到 Gin 的上下文中。
//
// 该中间件采用缓存优先策略：首先通过 AccessToken 的 MD5 摘要从 Redis 缓存中获取用户实体，
// 命中则直接注入上下文；未命中时回退到远端 SSO 获取用户信息并回写缓存，减少对 SSO 服务的重复调用。
//
// 参数说明:
//   - ctx: 主上下文，用于依赖注入和日志记录。
//
// 使用注意:
//   - 需要确保第三方 OAuth 平台和用户业务逻辑层的正确初始化。
//   - 任何阶段的错误都会直接终止链路调用并返回错误响应。
func User(ctx context.Context) gin.HandlerFunc {
	log := xLog.WithName(xLog.NamedMIDE, "User")

	userLogic := logic.NewUserLogic(ctx)
	accessUserLogic := logic.NewAccessUserLogic(ctx)
	oauthLogic := bSdkLogic.NewBusiness(ctx)

	return func(c *gin.Context) {
		log.Info(ctx, "获取注入用户身份信息")

		accessToken := c.GetString(xHttp.HeaderAuthorization.String())

		// 缓存优先：尝试从 AccessToken 缓存获取用户
		cachedUser, xErr := accessUserLogic.GetUserByAT(c, accessToken)
		if xErr != nil {
			xResult.AbortError(c, xErr.ErrorCode, xErr.ErrorMessage, false)
			return
		}

		var takeUser *entity.User
		if cachedUser != nil {
			takeUser = cachedUser
		} else {
			// 缓存未命中，走远端 SSO 获取流程
			getUser, xErr := oauthLogic.Userinfo(c, accessToken)
			if xErr != nil {
				xResult.AbortError(c, xErr.ErrorCode, xErr.ErrorMessage, false)
				return
			}

			takeUser, xErr = userLogic.TakeUser(c, getUser)
			if xErr != nil {
				xResult.AbortError(c, xErr.ErrorCode, xErr.ErrorMessage, false)
				return
			}

			// 回写缓存（失败不影响主流程）
			_ = accessUserLogic.SetUserByAT(c, accessToken, takeUser)
		}

		newCtx := context.WithValue(c.Request.Context(), bConst.CtxUserinfoKey, takeUser)
		c.Request.WithContext(newCtx)
		c.Next()
	}
}
