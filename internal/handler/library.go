package handler

import (
	"strconv"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	apiLibrary "github.com/frontleaves-mc/frontleaves-yggleaf/api/library"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"github.com/gin-gonic/gin"
	bSdkUtil "github.com/phalanx-labs/beacon-sso-sdk/utility"
)

const (
	defaultPage     = 1
	defaultPageSize = 20
	maxPageSize     = 100
)

// ==================== Skin Handlers ====================

// CreateSkin 创建皮肤
//
// @Summary     [玩家] 创建皮肤
// @Description 上传皮肤纹理文件并创建皮肤记录，支持 classic/slim 两种模型
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       request body apiLibrary.CreateSkinRequest true "创建皮肤请求"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.SkinResponse} "创建成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Router      /library/skins [POST]
func (h *LibraryHandler) CreateSkin(ctx *gin.Context) {
	h.log.Info(ctx, "CreateSkin - 创建皮肤")

	req := xUtil.Bind(ctx, &apiLibrary.CreateSkinRequest{}).Data()
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

	skin, xErr := h.service.libraryLogic.CreateSkin(ctx.Request.Context(), userID, req.Name, req.Model, req.Texture, req.IsPublic)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "创建皮肤成功", skinDTOToResponse(skin))
}

// ListSkins 获取皮肤列表
//
// @Summary     [玩家] 获取皮肤列表
// @Description 分页获取皮肤列表，支持 mine（我的皮肤）和 market（市场皮肤）两种模式
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       mode query string false "列表模式：mine 或 market，默认 mine"
// @Param       page query int false "页码，默认 1"
// @Param       page_size query int false "每页数量，默认 20，最大 100"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.SkinListResponse} "获取成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Router      /library/skins [GET]
func (h *LibraryHandler) ListSkins(ctx *gin.Context) {
	h.log.Info(ctx, "ListSkins - 获取皮肤列表")

	mode := ctx.DefaultQuery("mode", "mine")
	if mode != "market" && mode != "mine" {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的模式参数：必须是 market 或 mine", true))
		return
	}

	page, pageSize := h.parsePagination(ctx)

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

	if mode == "market" {
		skins, total, xErr := h.service.libraryLogic.ListSkins(ctx.Request.Context(), page, pageSize)
		if xErr != nil {
			_ = ctx.Error(xErr)
			return
		}
		response := apiLibrary.SkinListResponse{
			Total: total,
			Items: skinDTOsToResponses(skins),
		}
		xResult.SuccessHasData(ctx, "获取皮肤列表成功", response)
	} else {
		associations, total, xErr := h.service.libraryLogic.ListMySkins(ctx.Request.Context(), userID, page, pageSize)
		if xErr != nil {
			_ = ctx.Error(xErr)
			return
		}
		response := apiLibrary.SkinListResponse{
			Total: total,
			Items: skinDTOsToResponses(associations),
		}
		xResult.SuccessHasData(ctx, "获取皮肤列表成功", response)
	}
}

// UpdateSkin 更新皮肤
//
// @Summary     [玩家] 更新皮肤
// @Description 根据皮肤 ID 更新皮肤的名称和公开状态
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       skin_id path string true "皮肤 ID"
// @Param       request body apiLibrary.UpdateSkinRequest true "更新皮肤请求"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.SkinResponse} "更新成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     404 {object} xBase.BaseResponse "资源不存在"
// @Router      /library/skins/{skin_id} [PATCH]
func (h *LibraryHandler) UpdateSkin(ctx *gin.Context) {
	h.log.Info(ctx, "UpdateSkin - 更新皮肤")

	skinID, err := xSnowflake.ParseSnowflakeID(ctx.Param("skin_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析皮肤 ID 失败", true, err))
		return
	}

	req := xUtil.Bind(ctx, &apiLibrary.UpdateSkinRequest{}).Data()
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

	updatedSkin, xErr := h.service.libraryLogic.UpdateSkin(ctx.Request.Context(), userID, skinID, req.Name, req.IsPublic)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "更新皮肤成功", skinDTOToResponse(updatedSkin))
}

// DeleteSkin 删除皮肤
//
// @Summary     [玩家] 删除皮肤
// @Description 根据皮肤 ID 删除皮肤记录及其关联的纹理文件
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       skin_id path string true "皮肤 ID"
// @Success     200 {object} xBase.BaseResponse "删除成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     404 {object} xBase.BaseResponse "资源不存在"
// @Router      /library/skins/{skin_id} [DELETE]
func (h *LibraryHandler) DeleteSkin(ctx *gin.Context) {
	h.log.Info(ctx, "DeleteSkin - 删除皮肤")

	skinID, err := xSnowflake.ParseSnowflakeID(ctx.Param("skin_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析皮肤 ID 失败", true, err))
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

	xErr = h.service.libraryLogic.DeleteSkin(ctx.Request.Context(), userID, skinID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.Success(ctx, "删除皮肤成功")
}

// ==================== Cape Handlers ====================

// CreateCape 创建披风
//
// @Summary     [玩家] 创建披风
// @Description 上传披风纹理文件并创建披风记录
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       request body apiLibrary.CreateCapeRequest true "创建披风请求"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.CapeResponse} "创建成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Router      /library/capes [POST]
func (h *LibraryHandler) CreateCape(ctx *gin.Context) {
	h.log.Info(ctx, "CreateCape - 创建披风")

	req := xUtil.Bind(ctx, &apiLibrary.CreateCapeRequest{}).Data()
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

	cape, xErr := h.service.libraryLogic.CreateCape(ctx.Request.Context(), userID, req.Name, req.Texture, req.IsPublic)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "创建披风成功", capeDTOToResponse(cape))
}

// ListCapes 获取披风列表
//
// @Summary     [玩家] 获取披风列表
// @Description 分页获取披风列表，支持 mine（我的披风）和 market（市场披风）两种模式
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       mode query string false "列表模式：mine 或 market，默认 mine"
// @Param       page query int false "页码，默认 1"
// @Param       page_size query int false "每页数量，默认 20，最大 100"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.CapeListResponse} "获取成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Router      /library/capes [GET]
func (h *LibraryHandler) ListCapes(ctx *gin.Context) {
	h.log.Info(ctx, "ListCapes - 获取披风列表")

	mode := ctx.DefaultQuery("mode", "mine")
	if mode != "market" && mode != "mine" {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的模式参数：必须是 market 或 mine", true))
		return
	}

	page, pageSize := h.parsePagination(ctx)

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

	if mode == "market" {
		capes, total, xErr := h.service.libraryLogic.ListCapes(ctx.Request.Context(), page, pageSize)
		if xErr != nil {
			_ = ctx.Error(xErr)
			return
		}
		response := apiLibrary.CapeListResponse{
			Total: total,
			Items: capeDTOsToResponses(capes),
		}
		xResult.SuccessHasData(ctx, "获取披风列表成功", response)
	} else {
		associations, total, xErr := h.service.libraryLogic.ListMyCapes(ctx.Request.Context(), userID, page, pageSize)
		if xErr != nil {
			_ = ctx.Error(xErr)
			return
		}
		response := apiLibrary.CapeListResponse{
			Total: total,
			Items: capeDTOsToResponses(associations),
		}
		xResult.SuccessHasData(ctx, "获取披风列表成功", response)
	}
}

// ListMySkinsSimple 获取当前用户的皮肤精简列表（仅 ID + Name，不分页）
//
// @Summary     [玩家] 获取皮肤精简列表
// @Description 获取当前用户拥有的所有皮肤的精简列表，仅返回 ID 和名称
// @Tags        资源库接口
// @Produce     json
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.SkinSimpleListResponse} "获取成功"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Router      /library/skins/list [GET]
func (h *LibraryHandler) ListMySkinsSimple(ctx *gin.Context) {
	h.log.Info(ctx, "ListMySkinsSimple - 获取我的皮肤精简列表")

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

	items, xErr := h.service.libraryLogic.ListMySkinsSimple(ctx.Request.Context(), userID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	response := apiLibrary.SkinSimpleListResponse{
		Items: skinSimpleDTOsToResponses(items),
	}
	xResult.SuccessHasData(ctx, "获取皮肤精简列表成功", response)
}

// ListMyCapesSimple 获取当前用户的披风精简列表（仅 ID + Name，不分页）
//
// @Summary     [玩家] 获取披风精简列表
// @Description 获取当前用户拥有的所有披风的精简列表，仅返回 ID 和名称
// @Tags        资源库接口
// @Produce     json
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.CapeSimpleListResponse} "获取成功"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Router      /library/capes/list [GET]
func (h *LibraryHandler) ListMyCapesSimple(ctx *gin.Context) {
	h.log.Info(ctx, "ListMyCapesSimple - 获取我的披风精简列表")

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

	items, xErr := h.service.libraryLogic.ListMyCapesSimple(ctx.Request.Context(), userID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	response := apiLibrary.CapeSimpleListResponse{
		Items: capeSimpleDTOsToResponses(items),
	}
	xResult.SuccessHasData(ctx, "获取披风精简列表成功", response)
}

// UpdateCape 更新披风
//
// @Summary     [玩家] 更新披风
// @Description 根据披风 ID 更新披风的名称和公开状态
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       cape_id path string true "披风 ID"
// @Param       request body apiLibrary.UpdateCapeRequest true "更新披风请求"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.CapeResponse} "更新成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     404 {object} xBase.BaseResponse "资源不存在"
// @Router      /library/capes/{cape_id} [PATCH]
func (h *LibraryHandler) UpdateCape(ctx *gin.Context) {
	h.log.Info(ctx, "UpdateCape - 更新披风")

	capeID, err := xSnowflake.ParseSnowflakeID(ctx.Param("cape_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析披风 ID 失败", true, err))
		return
	}

	req := xUtil.Bind(ctx, &apiLibrary.UpdateCapeRequest{}).Data()
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

	updatedCape, xErr := h.service.libraryLogic.UpdateCape(ctx.Request.Context(), userID, capeID, req.Name, req.IsPublic)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "更新披风成功", capeDTOToResponse(updatedCape))
}

// DeleteCape 删除披风
//
// @Summary     [玩家] 删除披风
// @Description 根据披风 ID 删除披风记录及其关联的纹理文件
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       cape_id path string true "披风 ID"
// @Success     200 {object} xBase.BaseResponse "删除成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     404 {object} xBase.BaseResponse "资源不存在"
// @Router      /library/capes/{cape_id} [DELETE]
func (h *LibraryHandler) DeleteCape(ctx *gin.Context) {
	h.log.Info(ctx, "DeleteCape - 删除披风")

	capeID, err := xSnowflake.ParseSnowflakeID(ctx.Param("cape_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析披风 ID 失败", true, err))
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

	xErr = h.service.libraryLogic.DeleteCape(ctx.Request.Context(), userID, capeID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.Success(ctx, "删除披风成功")
}

// ==================== Quota Handler ====================

// GetQuota 获取当前用户的资源库配额
//
// @Summary     [玩家] 获取资源库配额
// @Description 获取当前用户的资源库配额信息，包括总额度与已使用额度
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Success     200 {object} xBase.BaseResponse{data=entity.LibraryQuota} "获取成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Router      /library/quota [GET]
func (h *LibraryHandler) GetQuota(ctx *gin.Context) {
	h.log.Info(ctx, "GetQuota - 获取资源库配额")

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

	quota, xErr := h.service.libraryLogic.GetQuota(ctx.Request.Context(), userID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "获取资源库配额成功", quota)
}

// ==================== Admin Handlers ====================

// GiftSkin 管理员赠送皮肤
//
// @Summary     [管理] 赠送皮肤
// @Description 管理员向指定用户赠送皮肤，支持 gift 和 admin 两种分配类型
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Param       request body apiLibrary.GiftSkinRequest true "赠送皮肤请求"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.SkinResponse} "赠送成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     403 {object} xBase.BaseResponse "无权限"
// @Failure     404 {object} xBase.BaseResponse "资源不存在"
// @Router      /library/admin/users/{user_id}/skins/gift [POST]
func (h *LibraryHandler) GiftSkin(ctx *gin.Context) {
	h.log.Info(ctx, "GiftSkin - 管理员赠送皮肤")

	targetUserID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析目标用户 ID 失败", true, err))
		return
	}

	req := xUtil.Bind(ctx, &apiLibrary.GiftSkinRequest{}).Data()
	if req == nil {
		return
	}

	skinLibraryID, err := xSnowflake.ParseSnowflakeID(req.SkinLibraryID)
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析皮肤库 ID 失败", true, err))
		return
	}

	userinfo, xErr := h.service.oauthLogic.Userinfo(ctx, bSdkUtil.GetAuthorization(ctx))
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	operatorID, err := xSnowflake.ParseSnowflakeID(userinfo.Sub)
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析操作者 ID 失败", true, err))
		return
	}

	assignmentType := entityType.AssignmentType(req.AssignmentType)
	result, xErr := h.service.libraryLogic.GiftSkin(ctx.Request.Context(), operatorID, targetUserID, skinLibraryID, assignmentType)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "赠送皮肤成功", skinDTOToResponse(result))
}

// RevokeSkin 管理员撤销皮肤
//
// @Summary     [管理] 撤销皮肤
// @Description 管理员撤销指定用户的皮肤关联
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Param       skin_library_id path string true "皮肤库 ID"
// @Success     200 {object} xBase.BaseResponse "撤销成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     403 {object} xBase.BaseResponse "无权限"
// @Failure     404 {object} xBase.BaseResponse "资源不存在"
// @Router      /library/admin/users/{user_id}/skins/{skin_library_id} [DELETE]
func (h *LibraryHandler) RevokeSkin(ctx *gin.Context) {
	h.log.Info(ctx, "RevokeSkin - 管理员撤销皮肤")

	targetUserID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析目标用户 ID 失败", true, err))
		return
	}

	skinLibraryID, err := xSnowflake.ParseSnowflakeID(ctx.Param("skin_library_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析皮肤库 ID 失败", true, err))
		return
	}

	xErr := h.service.libraryLogic.RevokeSkin(ctx.Request.Context(), targetUserID, skinLibraryID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.Success(ctx, "撤销皮肤成功")
}

// GiftCape 管理员赠送披风
//
// @Summary     [管理] 赠送披风
// @Description 管理员向指定用户赠送披风，支持 gift 和 admin 两种分配类型
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Param       request body apiLibrary.GiftCapeRequest true "赠送披风请求"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.CapeResponse} "赠送成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     403 {object} xBase.BaseResponse "无权限"
// @Failure     404 {object} xBase.BaseResponse "资源不存在"
// @Router      /library/admin/users/{user_id}/capes/gift [POST]
func (h *LibraryHandler) GiftCape(ctx *gin.Context) {
	h.log.Info(ctx, "GiftCape - 管理员赠送披风")

	targetUserID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析目标用户 ID 失败", true, err))
		return
	}

	req := xUtil.Bind(ctx, &apiLibrary.GiftCapeRequest{}).Data()
	if req == nil {
		return
	}

	capeLibraryID, err := xSnowflake.ParseSnowflakeID(req.CapeLibraryID)
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析披风库 ID 失败", true, err))
		return
	}

	userinfo, xErr := h.service.oauthLogic.Userinfo(ctx, bSdkUtil.GetAuthorization(ctx))
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	operatorID, err := xSnowflake.ParseSnowflakeID(userinfo.Sub)
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析操作者 ID 失败", true, err))
		return
	}

	assignmentType := entityType.AssignmentType(req.AssignmentType)
	result, xErr := h.service.libraryLogic.GiftCape(ctx.Request.Context(), operatorID, targetUserID, capeLibraryID, assignmentType)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "赠送披风成功", capeDTOToResponse(result))
}

// RevokeCape 管理员撤销披风
//
// @Summary     [管理] 撤销披风
// @Description 管理员撤销指定用户的披风关联
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Param       cape_library_id path string true "披风库 ID"
// @Success     200 {object} xBase.BaseResponse "撤销成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     403 {object} xBase.BaseResponse "无权限"
// @Failure     404 {object} xBase.BaseResponse "资源不存在"
// @Router      /library/admin/users/{user_id}/capes/{cape_library_id} [DELETE]
func (h *LibraryHandler) RevokeCape(ctx *gin.Context) {
	h.log.Info(ctx, "RevokeCape - 管理员撤销披风")

	targetUserID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析目标用户 ID 失败", true, err))
		return
	}

	capeLibraryID, err := xSnowflake.ParseSnowflakeID(ctx.Param("cape_library_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析披风库 ID 失败", true, err))
		return
	}

	xErr := h.service.libraryLogic.RevokeCape(ctx.Request.Context(), targetUserID, capeLibraryID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.Success(ctx, "撤销披风成功")
}

// SyncQuota 管理员同步用户配额
//
// @Summary     [管理] 同步配额
// @Description 管理员重新计算并同步指定用户的资源库配额
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Success     200 {object} xBase.BaseResponse "同步成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     403 {object} xBase.BaseResponse "无权限"
// @Router      /library/admin/users/{user_id}/quota/sync [POST]
func (h *LibraryHandler) SyncQuota(ctx *gin.Context) {
	h.log.Info(ctx, "SyncQuota - 管理员同步配额")

	userID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析用户 ID 失败", true, err))
		return
	}

	xErr := h.service.libraryLogic.RecalculateQuota(ctx.Request.Context(), userID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.Success(ctx, "同步配额成功")
}


// ListUserSkins 查询指定用户的皮肤列表（管理员）
//
// @Summary     [管理] 查询用户皮肤列表
// @Description 管理员分页查询指定用户关联的所有皮肤
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Param       page query int false "页码，默认 1"
// @Param       page_size query int false "每页数量，默认 20，最大 100"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.SkinListResponse} "获取成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     403 {object} xBase.BaseResponse "无权限"
// @Router      /library/admin/users/{user_id}/skins [GET]
func (h *LibraryHandler) ListUserSkins(ctx *gin.Context) {
	h.log.Info(ctx, "ListUserSkins - 查询用户皮肤列表")

	userID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析用户 ID 失败", true, err))
		return
	}

	page, pageSize := h.parsePagination(ctx)

	associations, total, xErr := h.service.libraryLogic.ListUserSkins(ctx.Request.Context(), userID, page, pageSize)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	response := apiLibrary.SkinListResponse{
		Total: total,
		Items: skinDTOsToResponses(associations),
	}
	xResult.SuccessHasData(ctx, "获取用户皮肤列表成功", response)
}

// ListUserCapes 查询指定用户的披风列表（管理员）
//
// @Summary     [管理] 查询用户披风列表
// @Description 管理员分页查询指定用户关联的所有披风
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       user_id path string true "目标用户 ID"
// @Param       page query int false "页码，默认 1"
// @Param       page_size query int false "每页数量，默认 20，最大 100"
// @Success     200 {object} xBase.BaseResponse{data=apiLibrary.CapeListResponse} "获取成功"
// @Failure     400 {object} xBase.BaseResponse "请求参数错误"
// @Failure     401 {object} xBase.BaseResponse "未授权"
// @Failure     403 {object} xBase.BaseResponse "无权限"
// @Router      /library/admin/users/{user_id}/capes [GET]
func (h *LibraryHandler) ListUserCapes(ctx *gin.Context) {
	h.log.Info(ctx, "ListUserCapes - 查询用户披风列表")

	userID, err := xSnowflake.ParseSnowflakeID(ctx.Param("user_id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "解析用户 ID 失败", true, err))
		return
	}

	page, pageSize := h.parsePagination(ctx)

	associations, total, xErr := h.service.libraryLogic.ListUserCapes(ctx.Request.Context(), userID, page, pageSize)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	response := apiLibrary.CapeListResponse{
		Total: total,
		Items: capeDTOsToResponses(associations),
	}
	xResult.SuccessHasData(ctx, "获取用户披风列表成功", response)
}

// ==================== Helper Methods ====================

func (h *LibraryHandler) parsePagination(ctx *gin.Context) (int, int) {
	pageStr := ctx.DefaultQuery("page", strconv.Itoa(defaultPage))
	pageSizeStr := ctx.DefaultQuery("page_size", strconv.Itoa(defaultPageSize))

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = defaultPage
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}

	return page, pageSize
}
