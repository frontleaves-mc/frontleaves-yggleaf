package handler

import (
	xResult "github.com/bamboo-services/bamboo-base-go/result"
	"github.com/gin-gonic/gin"
	bSdkUtil "github.com/phalanx/beacon-sso-sdk/utility"
)

// UserCurrent 获取用户的信息
//
// @Summary [用户] 用户信息
// @Description  根据 AT 获取用户信息
// @Tags         用户接口
// @Accept       json
// @Produce      json
// @Success      200   {object}  xBase.BaseResponse{data=entity.User}	"登录成功"
// @Failure      400   {object}  xBase.BaseResponse          			"请求体格式不正确"
// @Failure      401   {object}  xBase.BaseResponse         			"用户名或密码错误"
// @Failure      403   {object}  xBase.BaseResponse          			"用户已禁用或账户已锁定"
// @Failure      404   {object}  xBase.BaseResponse          			"用户不存在"
// @Router       /api/v1/user/info [GET]
func (h *UserHandler) UserCurrent(ctx *gin.Context) {
	h.log.Info(ctx, "UserCurrent - 获取用户信息")

	userinfo, xErr := h.service.oauthLogic.Userinfo(ctx, bSdkUtil.GetAuthorization(ctx))
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	getUser, xErr := h.service.userLogic.TakeUser(ctx, userinfo)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "测试", getUser)
}
