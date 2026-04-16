package handler

import (
	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	apiUser "github.com/frontleaves-mc/frontleaves-yggleaf/api/user"
	"github.com/gin-gonic/gin"
	bSdkUtil "github.com/phalanx-labs/beacon-sso-sdk/utility"
)

// UserCurrent 获取用户的信息
//
// @Summary 	[玩家] 用户信息
// @Description 根据 AT 获取用户信息，获取到本程序的用户信息（含账户完善状态）
// @Tags        用户接口
// @Accept      json
// @Produce     json
// @Success     200   {object}  xBase.BaseResponse{data=user.UserCurrentResponse}	"获取成功"
// @Failure     400   {object}  xBase.BaseResponse          			"请求体格式不正确"
// @Failure     401   {object}  xBase.BaseResponse         				"用户名或密码错误"
// @Failure     403   {object}  xBase.BaseResponse          			"用户已禁用或账户已锁定"
// @Failure     404   {object}  xBase.BaseResponse          			"用户不存在"
// @Router       /user/info [GET]
func (h *UserHandler) UserCurrent(ctx *gin.Context) {
	h.log.Info(ctx, "UserCurrent - 获取用户信息")

	userinfo, xErr := h.service.oauthLogic.Userinfo(ctx, bSdkUtil.GetAuthorization(ctx))
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	response, xErr := h.service.userLogic.GetUserCurrent(ctx.Request.Context(), userinfo)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "获取用户信息成功", response)
}

// UpdateGamePassword 更新当前用户的游戏密码
//
// @Summary 	[玩家] 更新游戏密码
// @Description 已通过 OAuth2 认证的用户可直接设置/重置游戏密码，无需旧密码
// @Tags        用户接口
// @Accept      json
// @Produce     json
// @Param       request body apiUser.UpdateGamePasswordRequest true "更新游戏密码请求"
// @Success     200   {object}  xBase.BaseResponse{data=user.UserCurrentResponse}	"更新成功"
// @Failure     400   {object}  xBase.BaseResponse          			"请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse         				"未授权"
// @Failure     404   {object}  xBase.BaseResponse          			"用户不存在"
// @Router       /user/game-password [PUT]
func (h *UserHandler) UpdateGamePassword(ctx *gin.Context) {
	h.log.Info(ctx, "UpdateGamePassword - 更新游戏密码")

	req := xUtil.Bind(ctx, &apiUser.UpdateGamePasswordRequest{}).Data()
	if req == nil {
		return
	}

	userinfo, xErr := h.service.oauthLogic.Userinfo(ctx, bSdkUtil.GetAuthorization(ctx))
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	userID, err := xSnowflake.ParseSnowflakeID(userinfo.Sub)
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析用户 ID 失败", true, err))
		return
	}

	response, xErr := h.service.userLogic.UpdateGamePassword(ctx.Request.Context(), userID, req)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "游戏密码更新成功", response)
}
