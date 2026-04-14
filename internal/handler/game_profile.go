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
// @Router      /game-profile [POST]
func (h *GameProfileHandler) AddGameProfile(ctx *gin.Context) {
	h.log.Info(ctx, "AddGameProfile - 创建游戏档案")

	req := xUtil.Bind(ctx, &apiUser.AddGameProfileRequest{}).Data()
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

	profile, xErr := h.service.gameProfileLogic.AddGameProfile(ctx.Request.Context(), userID, req.Name)
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
// @Router      /game-profile/{profile_id}/username [PATCH]
func (h *GameProfileHandler) ChangeUsername(ctx *gin.Context) {
	h.log.Info(ctx, "ChangeUsername - 修改游戏档案用户名")

	profileID, err := xSnowflake.ParseSnowflakeID(ctx.Param("profile_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析游戏档案 ID 失败", true, err))
		return
	}

	req := xUtil.Bind(ctx, &apiUser.ChangeUsernameRequest{}).Data()
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

	updatedProfile, xErr := h.service.gameProfileLogic.ChangeUsername(ctx.Request.Context(), userID, profileID, req.NewName)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "修改游戏档案用户名成功", updatedProfile)
}

// ListGameProfiles 获取当前用户的游戏档案列表
//
// @Summary     [玩家] 获取游戏档案列表
// @Description 获取当前用户的所有游戏档案列表
// @Tags        游戏档案接口
// @Accept      json
// @Produce     json
// @Success     200   {object}  xBase.BaseResponse{data=apiUser.GameProfileListResponse} "获取成功"
// @Failure     400   {object}  xBase.BaseResponse                                       "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                                       "未授权"
// @Router      /game-profile [GET]
func (h *GameProfileHandler) ListGameProfiles(ctx *gin.Context) {
	h.log.Info(ctx, "ListGameProfiles - 获取游戏档案列表")

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

	profiles, xErr := h.service.gameProfileLogic.ListGameProfiles(ctx.Request.Context(), userID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	response := apiUser.GameProfileListResponse{
		Items: profiles,
	}

	xResult.SuccessHasData(ctx, "获取游戏档案列表成功", response)
}

// GetQuota 获取当前用户的游戏档案配额
//
// @Summary     [玩家] 获取游戏档案配额
// @Description 获取当前用户的游戏档案配额信息，包括总额度与已使用额度
// @Tags        游戏档案接口
// @Accept      json
// @Produce     json
// @Success     200   {object}  xBase.BaseResponse{data=entity.GameProfileQuota} "获取成功"
// @Failure     400   {object}  xBase.BaseResponse                               "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
// @Router      /game-profile/quota [GET]
func (h *GameProfileHandler) GetQuota(ctx *gin.Context) {
	h.log.Info(ctx, "GetQuota - 获取游戏档案配额")

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

	quota, xErr := h.service.gameProfileLogic.GetQuota(ctx.Request.Context(), userID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "获取游戏档案配额成功", quota)
}
