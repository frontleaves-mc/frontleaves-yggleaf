package handler

import (
	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	apiAdmin "github.com/frontleaves-mc/frontleaves-yggleaf/api/admin"
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

// ListAdminUsers 管理员获取用户分页列表
//
// @Summary 	[超管] 用户列表
// @Description 管理员分页查询用户列表，支持角色、关键词、时间范围筛选
// @Tags        管理员-用户接口
// @Accept      json
// @Produce     json
// @Param       page query int false "页码" default(1)
// @Param       page_size query int false "每页条数(最大100)" default(20)
// @Param       role query string false "角色筛选(SUPER_ADMIN/ADMIN/PLAYER)"
// @Param       keyword query string false "关键词搜索(用户名/邮箱)"
// @Param       start_time query string false "注册时间起始(RFC3339)"
// @Param       end_time query string false "注册时间截止(RFC3339)"
// @Success     200   {object}  xBase.BaseResponse{data=admin.AdminUserListResponse}	"查询成功"
// @Failure     400   {object}  xBase.BaseResponse          			"请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse         				"未授权"
// @Failure     403   {object}  xBase.BaseResponse          			"需要超级管理员权限"
// @Security    BearerAuth
// @Router       /admin/users [GET]
func (h *UserHandler) ListAdminUsers(ctx *gin.Context) {
	h.log.Info(ctx, "ListAdminUsers - 管理员获取用户分页列表")

	req := &apiAdmin.AdminUserListRequest{}
	if err := ctx.ShouldBindQuery(req); err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "请求参数错误", true, err))
		return
	}

	response, xErr := h.service.userLogic.ListAdminUsers(ctx.Request.Context(), req)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "获取用户列表成功", response)
}

// GetAdminUserDetail 管理员获取用户详情
//
// @Summary 	[超管] 用户详情
// @Description 获取用户完整详情，包含账户信息、游戏档案配额、资源库配额及皮肤/披风资源列表
// @Tags        管理员-用户接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Success     200   {object}  xBase.BaseResponse{data=admin.AdminUserDetailResponse}	"查询成功"
// @Failure     400   {object}  xBase.BaseResponse          			"请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse         				"未授权"
// @Failure     403   {object}  xBase.BaseResponse          			"需要超级管理员权限"
// @Failure     404   {object}  xBase.BaseResponse          			"用户不存在"
// @Security    BearerAuth
// @Router       /admin/users/{user_id} [GET]
func (h *UserHandler) GetAdminUserDetail(ctx *gin.Context) {
	h.log.Info(ctx, "GetAdminUserDetail - 管理员获取用户详情")

	targetUserID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的用户 ID", true, err))
		return
	}

	raw, xErr := h.service.userLogic.GetAdminUserDetailRaw(ctx.Request.Context(), targetUserID.String())
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	resp := apiAdmin.AdminUserDetailResponse{
		User: apiAdmin.AdminUserBasic{
			ID:        raw.User.ID.String(),
			Username:  raw.User.Username,
			Email:     raw.User.Email,
			Phone:     raw.User.Phone,
			RoleName:  raw.User.RoleName,
			HasBan:    raw.User.HasBan,
			JailedAt:  raw.User.JailedAt,
			CreatedAt: raw.User.CreatedAt,
			UpdatedAt: raw.User.UpdatedAt,
		},
	}

	if raw.GameProfile != nil {
		resp.GameProfile = &apiAdmin.GameProfileQuotaInfo{
			Total: raw.GameProfile.Total,
			Used:  raw.GameProfile.Used,
		}
	}

	if raw.LibraryQuota != nil {
		resp.LibraryQuota = &apiAdmin.LibraryQuotaInfo{
			SkinsPrivateTotal: raw.LibraryQuota.SkinsPrivateTotal,
			SkinsPublicTotal:  raw.LibraryQuota.SkinsPublicTotal,
			SkinsPrivateUsed:  raw.LibraryQuota.SkinsPrivateUsed,
			SkinsPublicUsed:   raw.LibraryQuota.SkinsPublicUsed,
			CapesPrivateTotal: raw.LibraryQuota.CapesPrivateTotal,
			CapesPublicTotal:  raw.LibraryQuota.CapesPublicTotal,
			CapesPrivateUsed:  raw.LibraryQuota.CapesPrivateUsed,
			CapesPublicUsed:   raw.LibraryQuota.CapesPublicUsed,
		}
	}

	var textureIDs []int64
	for _, s := range raw.SkinLibraries {
		textureIDs = append(textureIDs, s.Texture)
	}
	for _, c := range raw.CapeLibraries {
		textureIDs = append(textureIDs, c.Texture)
	}

	urlMap := make(map[int64]string)
	if len(textureIDs) > 0 {
		urlMap, xErr = h.service.libraryLogic.ResolveTextureURLsBatchForAdmin(ctx.Request.Context(), textureIDs)
		if xErr != nil {
			h.log.Warn(ctx, "批量解析纹理 URL 失败: "+string(xErr.ErrorMessage))
		}
	}

	resp.SkinList = make([]apiAdmin.AdminSkinItem, len(raw.SkinLibraries))
	for i, s := range raw.SkinLibraries {
		resp.SkinList[i] = apiAdmin.AdminSkinItem{
			ID:         s.ID.String(),
			Name:       s.Name,
			Model:      adminModelTypeToString(uint8(s.Model)),
			IsPublic:   s.IsPublic,
			TextureURL: urlMap[s.Texture],
			CreatedAt:  s.CreatedAt,
		}
	}

	resp.CapeList = make([]apiAdmin.AdminCapeItem, len(raw.CapeLibraries))
	for i, c := range raw.CapeLibraries {
		resp.CapeList[i] = apiAdmin.AdminCapeItem{
			ID:         c.ID.String(),
			Name:       c.Name,
			IsPublic:   c.IsPublic,
			TextureURL: urlMap[c.Texture],
			CreatedAt:  c.CreatedAt,
		}
	}

	xResult.SuccessHasData(ctx, "获取用户详情成功", resp)
}

// adminModelTypeToString 将 entity.ModelType 转为可读字符串。
func adminModelTypeToString(mt uint8) string {
	switch mt {
	case 1:
		return "STEVE"
	case 2:
		return "ALEX"
	default:
		return "UNKNOWN"
	}
}
