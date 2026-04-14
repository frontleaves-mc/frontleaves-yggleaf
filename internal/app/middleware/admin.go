package middleware

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/gin-gonic/gin"
)

// Admin 中间件用于校验当前用户是否具有管理员权限。
//
// 该中间件从上下文中提取已注入的 User 实体，检查其 RoleName 是否为
// SUPER_ADMIN 或 ADMIN。若不是，则直接中止请求并返回 403 错误。
//
// 使用注意:
//   - 必须在 User 中间件之后使用，以确保上下文中已注入用户信息。
func Admin(ctx context.Context) gin.HandlerFunc {
	log := xLog.WithName(xLog.NamedMIDE, "Admin")

	return func(c *gin.Context) {
		log.Info(c, "校验管理员权限")

		takeUser, ok := c.Request.Context().Value(bConst.CtxUserinfoKey).(*entity.User)
		if !ok || takeUser == nil {
			xErr := xError.NewError(c, xError.Unauthorized, "未获取到用户信息", true)
			xResult.AbortError(c, xErr.ErrorCode, xErr.ErrorMessage, false)
			return
		}

		if takeUser.RoleName == nil {
			xErr := xError.NewError(c, xError.PermissionDenied, "权限不足", true)
			xResult.AbortError(c, xErr.ErrorCode, xErr.ErrorMessage, false)
			return
		}

		role := *takeUser.RoleName
		if role != "SUPER_ADMIN" && role != "ADMIN" {
			xErr := xError.NewError(c, xError.PermissionDenied, "权限不足", true)
			xResult.AbortError(c, xErr.ErrorCode, xErr.ErrorMessage, false)
			return
		}

		c.Next()
	}
}
