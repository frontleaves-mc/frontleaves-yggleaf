package handler

import (
	"strconv"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	apiLibrary "github.com/frontleaves-mc/frontleaves-yggleaf/api/library"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
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
// @Description 上传并创建一个新的皮肤资源，纹理文件以 base64 编码传输
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       request body apiLibrary.CreateSkinRequest true "创建皮肤请求"
// @Success     200   {object}  xBase.BaseResponse{data=entity.SkinLibrary} "创建成功"
// @Failure     400   {object}  xBase.BaseResponse                               "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
// @Failure     409   {object}  xBase.BaseResponse                               "资源冲突"
// @Failure     503   {object}  xBase.BaseResponse                               "配额耗尽"
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

	skin, xErr := h.service.libraryLogic.CreateSkin(ctx.Request.Context(), userID, req)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "创建皮肤成功", skin)
}

// ListSkins 获取皮肤列表
//
// @Summary     [玩家] 获取皮肤列表
// @Description 根据模式获取皮肤列表，mode=market 获取所有公开皮肤，mode=mine 获取当前用户的皮肤
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       mode       query string false "列表模式：market(市场) / mine(我的)" default(mine)
// @Param       page       query int    false "页码" default(1)
// @Param       page_size  query int    false "每页数量" default(20)
// @Success     200   {object}  xBase.BaseResponse{data=apiLibrary.SkinListResponse} "获取成功"
// @Failure     400   {object}  xBase.BaseResponse                                       "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                                       "未授权"
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

	skins, total, xErr := h.service.libraryLogic.ListSkins(ctx.Request.Context(), userID, mode, page, pageSize)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	response := apiLibrary.SkinListResponse{
		Total: total,
		Items: h.convertSkinEntitiesToResponses(skins),
	}

	xResult.SuccessHasData(ctx, "获取皮肤列表成功", response)
}

// UpdateSkin 更新皮肤
//
// @Summary     [玩家] 更新皮肤
// @Description 修改指定皮肤的名称或公开状态，只能修改自己创建的皮肤
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       skin_id path string true "皮肤 ID"
// @Param       request body apiLibrary.UpdateSkinRequest true "更新皮肤请求"
// @Success     200   {object}  xBase.BaseResponse{data=entity.SkinLibrary} "更新成功"
// @Failure     400   {object}  xBase.BaseResponse                               "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
// @Failure     403   {object}  xBase.BaseResponse                               "无权限"
// @Failure     404   {object}  xBase.BaseResponse                               "资源不存在"
// @Failure     503   {object}  xBase.BaseResponse                               "配额耗尽"
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

	updatedSkin, xErr := h.service.libraryLogic.UpdateSkin(ctx.Request.Context(), userID, skinID, req)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "更新皮肤成功", updatedSkin)
}

// DeleteSkin 删除皮肤
//
// @Summary     [玩家] 删除皮肤
// @Description 删除指定皮肤，只能删除自己创建的皮肤
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       skin_id path string true "皮肤 ID"
// @Success     200   {object}  xBase.BaseResponse "删除成功"
// @Failure     400   {object}  xBase.BaseResponse "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse "未授权"
// @Failure     403   {object}  xBase.BaseResponse "无权限"
// @Failure     404   {object}  xBase.BaseResponse "资源不存在"
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
// @Description 上传并创建一个新的披风资源，纹理文件以 base64 编码传输
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       request body apiLibrary.CreateCapeRequest true "创建披风请求"
// @Success     200   {object}  xBase.BaseResponse{data=entity.CapeLibrary} "创建成功"
// @Failure     400   {object}  xBase.BaseResponse                               "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
// @Failure     409   {object}  xBase.BaseResponse                               "资源冲突"
// @Failure     503   {object}  xBase.BaseResponse                               "配额耗尽"
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

	cape, xErr := h.service.libraryLogic.CreateCape(ctx.Request.Context(), userID, req)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "创建披风成功", cape)
}

// ListCapes 获取披风列表
//
// @Summary     [玩家] 获取披风列表
// @Description 根据模式获取披风列表，mode=market 获取所有公开披风，mode=mine 获取当前用户的披风
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       mode       query string false "列表模式：market(市场) / mine(我的)" default(mine)
// @Param       page       query int    false "页码" default(1)
// @Param       page_size  query int    false "每页数量" default(20)
// @Success     200   {object}  xBase.BaseResponse{data=apiLibrary.CapeListResponse} "获取成功"
// @Failure     400   {object}  xBase.BaseResponse                                       "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                                       "未授权"
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

	capes, total, xErr := h.service.libraryLogic.ListCapes(ctx.Request.Context(), userID, mode, page, pageSize)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	response := apiLibrary.CapeListResponse{
		Total: total,
		Items: h.convertCapeEntitiesToResponses(capes),
	}

	xResult.SuccessHasData(ctx, "获取披风列表成功", response)
}

// UpdateCape 更新披风
//
// @Summary     [玩家] 更新披风
// @Description 修改指定披风的名称或公开状态，只能修改自己创建的披风
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       cape_id path string true "披风 ID"
// @Param       request body apiLibrary.UpdateCapeRequest true "更新披风请求"
// @Success     200   {object}  xBase.BaseResponse{data=entity.CapeLibrary} "更新成功"
// @Failure     400   {object}  xBase.BaseResponse                               "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
// @Failure     403   {object}  xBase.BaseResponse                               "无权限"
// @Failure     404   {object}  xBase.BaseResponse                               "资源不存在"
// @Failure     503   {object}  xBase.BaseResponse                               "配额耗尽"
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

	updatedCape, xErr := h.service.libraryLogic.UpdateCape(ctx.Request.Context(), userID, capeID, req)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}

	xResult.SuccessHasData(ctx, "更新披风成功", updatedCape)
}

// DeleteCape 删除披风
//
// @Summary     [玩家] 删除披风
// @Description 删除指定披风，只能删除自己创建的披风
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Param       cape_id path string true "披风 ID"
// @Success     200   {object}  xBase.BaseResponse "删除成功"
// @Failure     400   {object}  xBase.BaseResponse "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse "未授权"
// @Failure     403   {object}  xBase.BaseResponse "无权限"
// @Failure     404   {object}  xBase.BaseResponse "资源不存在"
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
// @Description 获取当前用户的资源库配额信息，包括皮肤和披风的公开/私有额度与已使用额度
// @Tags        资源库接口
// @Accept      json
// @Produce     json
// @Success     200   {object}  xBase.BaseResponse{data=entity.LibraryQuota} "获取成功"
// @Failure     400   {object}  xBase.BaseResponse                               "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
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

func (h *LibraryHandler) convertSkinEntitiesToResponses(skins []entity.SkinLibrary) []apiLibrary.SkinResponse {
	responses := make([]apiLibrary.SkinResponse, len(skins))
	for i, skin := range skins {
		responses[i] = apiLibrary.SkinResponse{SkinLibrary: skin}
	}
	return responses
}

func (h *LibraryHandler) convertCapeEntitiesToResponses(capes []entity.CapeLibrary) []apiLibrary.CapeResponse {
	responses := make([]apiLibrary.CapeResponse, len(capes))
	for i, cape := range capes {
		responses[i] = apiLibrary.CapeResponse{CapeLibrary: cape}
	}
	return responses
}
