package handler

import (
	xError "github.com/bamboo-services/bamboo-base-go/error"
	xResult "github.com/bamboo-services/bamboo-base-go/result"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/utility"
	apiUser "github.com/frontleaves-mc/frontleaves-yggleaf/api/user"
	"github.com/gin-gonic/gin"
	bSdkUtil "github.com/phalanx/beacon-sso-sdk/utility"
)

// AddGameProfile 创建当前用户的游戏档案
//
// @Summary     [玩家] 创建游戏档案
// @Description 根据当前登录用户创建游戏档案，UUID 由系统按 UUIDv7 自动生成
// @Tags        游戏档案接口
// @Accept      json
// @Produce     json
// @Param       request body apiUser.AddGameProfileRequest true "创建游戏档案请求"
// @Success     200   {object}  xBase.BaseResponse{data=entity.GameProfile} "创建成功"
// @Failure     400   {object}  xBase.BaseResponse                               "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
// @Failure     404   {object}  xBase.BaseResponse                               "资源不存在"
// @Failure     409   {object}  xBase.BaseResponse                               "资源冲突"
// @Failure     503   {object}  xBase.BaseResponse                               "资源耗尽"
// @Router      /api/v1/game-profile [POST]
func (h *GameProfileHandler) AddGameProfile(ctx *gin.Context) {
	h.log.Info(ctx, "AddGameProfile - 创建游戏档案")

	req := xUtil.BindData(ctx, &apiUser.AddGameProfileRequest{})
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

	profile, xErr := h.service.gameProfileLogic.AddGameProfile(ctx, userID, req.Name)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "创建游戏档案成功", profile)
}

// ChangeUsername 修改当前用户指定游戏档案用户名
//
// @Summary     [玩家] 修改用户名
// @Description 根据档案 ID 修改游戏档案用户名
// @Tags        游戏档案接口
// @Accept      json
// @Produce     json
// @Param       profile_id path string true "游戏档案 ID"
// @Param       request body apiUser.ChangeUsernameRequest true "修改用户名请求"
// @Success     200   {object}  xBase.BaseResponse{data=entity.GameProfile} "修改成功"
// @Failure     400   {object}  xBase.BaseResponse                               "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
// @Failure     404   {object}  xBase.BaseResponse                               "资源不存在"
// @Failure     409   {object}  xBase.BaseResponse                               "资源冲突"
// @Router      /api/v1/game-profile/{profile_id}/username [PATCH]
func (h *GameProfileHandler) ChangeUsername(ctx *gin.Context) {
	h.log.Info(ctx, "ChangeUsername - 修改游戏档案用户名")

	profileID, err := xSnowflake.ParseSnowflakeID(ctx.Param("profile_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析游戏档案 ID 失败", true, err))
		return
	}

	req := xUtil.BindData(ctx, &apiUser.ChangeUsernameRequest{})
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

	updatedProfile, xErr := h.service.gameProfileLogic.ChangeUsername(ctx, userID, profileID, req.NewName)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "修改游戏档案用户名成功", updatedProfile)
}
