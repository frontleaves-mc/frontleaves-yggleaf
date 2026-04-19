package handler

import (
	"strconv"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	xResult "github.com/bamboo-services/bamboo-base-go/major/result"
	apiIssue "github.com/frontleaves-mc/frontleaves-yggleaf/api/issue"
	apiAdmin "github.com/frontleaves-mc/frontleaves-yggleaf/api/admin"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/gin-gonic/gin"
	bSdkUtil "github.com/phalanx-labs/beacon-sso-sdk/utility"
)

// IssueHandler 问题接口处理器。
type IssueHandler handler

const (
	issueDefaultPage     = 1
	issueDefaultPageSize = 20 // 默认每页条数，用户可通过 page_size 覆盖
	issueMaxPageSize     = 50 // 每页上限，与 IssueListQuery.PageSize binding:"max=50" 对齐
)

// ==================== 用户端接口 ====================

// CreateIssue 提交问题反馈
//
// @Summary     [玩家] 提交问题
// @Description 已登录用户提交新的问题反馈，需指定问题类型、标题和内容，可选设置优先级（默认 medium）
// @Tags        问题接口
// @Accept      json
// @Produce     json
// @Param       request body apiIssue.CreateIssueRequest true "提交问题请求"
// @Success     200   {object}  xBase.BaseResponse{data=apiIssue.IssueListItem}      "提交成功"
// @Failure     400   {object}  xBase.BaseResponse                                "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                                "未授权"
// @Failure     404   {object}  xBase.BaseResponse                                "问题类型不存在或已禁用"
// @Router      /issue [POST]
func (h *IssueHandler) CreateIssue(ctx *gin.Context) {
	h.log.Info(ctx, "CreateIssue - 提交问题")
	req := xUtil.Bind(ctx, &apiIssue.CreateIssueRequest{}).Data()
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
	priority := bConst.IssuePriority(req.Priority)
	if priority == "" {
		priority = bConst.PriorityMedium
	}
	dto, xErr := h.service.issueLogic.CreateIssue(ctx.Request.Context(), userID, req.IssueTypeID, req.Title, req.Content, priority)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "提交问题成功", issueDTOToResponse(dto))
}

// GetIssueList 获取我的问题列表
//
// @Summary     [玩家] 我的问题列表
// @Description 获取当前已登录用户的问题反馈列表，支持按状态、优先级、问题类型筛选，分页返回
// @Tags        问题接口
// @Accept      json
// @Produce     json
// @Param       page query int false "页码（默认 1）"
// @Param       page_size query int false "每页数量（默认 20，最大 50）"
// @Param       status query string false "状态筛选：registered/pending/processing/resolved/unplanned/closed"
// @Param       priority query string false "优先级筛选：low/medium/high/urgent"
// @Param       issue_type_id query int false "问题类型 ID"
// @Success     200   {object}  xBase.BaseResponse{data=apiIssue.IssueListResponse} "获取成功"
// @Failure     401   {object}  xBase.BaseResponse                             "未授权"
// @Router      /issue/list [GET]
func (h *IssueHandler) GetIssueList(ctx *gin.Context) {
	h.log.Info(ctx, "GetIssueList - 获取我的问题列表")
	page, pageSize := h.parsePagination(ctx)
	userinfo := ctx.Request.Context().Value(bConst.CtxUserinfoKey).(*entity.User)

	var status *bConst.IssueStatus
	if s := ctx.Query("status"); s != "" {
		st := bConst.IssueStatus(s)
		status = &st
	}
	var priority *bConst.IssuePriority
	if p := ctx.Query("priority"); p != "" {
		pr := bConst.IssuePriority(p)
		priority = &pr
	}
	var issueTypeID *xSnowflake.SnowflakeID
	if tidStr := ctx.Query("issue_type_id"); tidStr != "" {
		if tid, parseErr := strconv.ParseInt(tidStr, 10, 64); parseErr == nil {
			id := xSnowflake.SnowflakeID(tid)
			issueTypeID = &id
		}
	}

	items, total, xErr := h.service.issueLogic.GetIssueList(ctx.Request.Context(), userinfo.ID, page, pageSize, status, priority, issueTypeID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	resp := apiIssue.IssueListResponse{Total: total, Items: make([]apiIssue.IssueListItem, len(items))}
	for i, dto := range items {
		resp.Items[i] = issueDTOToListItem(&dto)
	}
	xResult.SuccessHasData(ctx, "获取成功", resp)
}

// GetIssueDetail 获取问题详情
//
// @Summary     [玩家] 问题详情
// @Description 根据问题 ID 获取详情，包含回复列表、附件列表和类型信息；非管理员仅能查看自己的问题
// @Tags        问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "问题 ID"
// @Success     200   {object}  xBase.BaseResponse{data=apiIssue.IssueDetailResponse} "获取成功"
// @Failure     400   {object}  xBase.BaseResponse                                  "无效的问题 ID"
// @Failure     401   {object}  xBase.BaseResponse                                  "未授权"
// @Failure     403   {object}  xBase.BaseResponse                                  "无权查看该问题"
// @Failure     404   {object}  xBase.BaseResponse                                  "问题不存在"
// @Router      /issue/{id} [GET]
func (h *IssueHandler) GetIssueDetail(ctx *gin.Context) {
	h.log.Info(ctx, "GetIssueDetail - 获取问题详情")
	issueID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的问题 ID", true, err))
		return
	}
	userinfo := ctx.Request.Context().Value(bConst.CtxUserinfoKey).(*entity.User)
	isAdmin := isAdminRole(userinfo)
	dto, xErr := h.service.issueLogic.GetIssueDetail(ctx.Request.Context(), issueID, userinfo.ID, isAdmin)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "获取成功", issueDetailDTOToResponse(dto))
}

// ReplyIssue 回复问题
//
// @Summary     [玩家/管理] 回复问题
// @Description 向指定问题追加回复，管理员和问题创建者均可回复，回复内容长度限制 1~5000 字符
// @Tags        问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "问题 ID"
// @Param       request body apiIssue.ReplyIssueRequest true "回复问题请求"
// @Success     200   {object}  xBase.BaseResponse{data=apiIssue.IssueReplyItem} "回复成功"
// @Failure     400   {object}  xBase.BaseResponse                            "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                            "未授权"
// @Failure     403   {object}  xBase.BaseResponse                            "无权操作该问题"
// @Failure     404   {object}  xBase.BaseResponse                            "问题不存在"
// @Router      /issue/{id}/reply [POST]
func (h *IssueHandler) ReplyIssue(ctx *gin.Context) {
	h.log.Info(ctx, "ReplyIssue - 追加回复")
	issueID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的问题 ID", true, err))
		return
	}
	req := xUtil.Bind(ctx, &apiIssue.ReplyIssueRequest{}).Data()
	if req == nil {
		return
	}
	userinfo := ctx.Request.Context().Value(bConst.CtxUserinfoKey).(*entity.User)
	isAdmin := isAdminRole(userinfo)
	dto, xErr := h.service.issueLogic.ReplyIssue(ctx.Request.Context(), issueID, userinfo.ID, req.Content, isAdmin)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "回复成功", issueReplyDTOToResponse(dto))
}

// UploadAttachment 上传附件
//
// @Summary     [玩家/管理] 上传附件
// @Description 向指定问题上传附件文件（Base64 编码），支持图片、PDF、文本、压缩包格式，单文件不超过 10MB，单问题最多 9 个附件
// @Tags        问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "问题 ID"
// @Param       request body apiIssue.UploadAttachmentRequest true "上传附件请求"
// @Success     200   {object}  xBase.BaseResponse{data=apiIssue.IssueAttachmentItem} "上传成功"
// @Failure     400   {object}  xBase.BaseResponse                                   "请求参数错误或不支持的文件类型"
// @Failure     401   {object}  xBase.BaseResponse                                   "未授权"
// @Failure     403   {object}  xBase.BaseResponse                                   "无权上传附件"
// @Failure     404   {object}  xBase.BaseResponse                                   "问题不存在"
// @Failure     429   {object}  xBase.BaseResponse                                   "附件数量已达上限"
// @Router      /issue/{id}/attachment [POST]
func (h *IssueHandler) UploadAttachment(ctx *gin.Context) {
	h.log.Info(ctx, "UploadAttachment - 上传附件")
	issueID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的问题 ID", true, err))
		return
	}
	req := xUtil.Bind(ctx, &apiIssue.UploadAttachmentRequest{}).Data()
	if req == nil {
		return
	}
	userinfo := ctx.Request.Context().Value(bConst.CtxUserinfoKey).(*entity.User)
	isAdmin := isAdminRole(userinfo)
	dto, xErr := h.service.issueLogic.UploadAttachment(ctx.Request.Context(), issueID, userinfo.ID, isAdmin, req.FileName, req.Content, req.MimeType)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "上传成功", issueAttachmentDTOToResponse(dto))
}

// DeleteAttachment 删除附件
//
// @Summary     [玩家/管理] 删除附件
// @Description 删除指定附件，同时清理对象存储中的文件；仅上传者本人或管理员可删除
// @Tags        问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "附件 ID"
// @Success     200   {object}  xBase.BaseResponse "删除成功"
// @Failure     400   {object}  xBase.BaseResponse "无效的附件 ID"
// @Failure     401   {object}  xBase.BaseResponse "未授权"
// @Failure     403   {object}  xBase.BaseResponse "无权删除该附件"
// @Failure     404   {object}  xBase.BaseResponse "附件不存在"
// @Router      /issue/attachment/{id} [DELETE]
func (h *IssueHandler) DeleteAttachment(ctx *gin.Context) {
	h.log.Info(ctx, "DeleteAttachment - 删除附件")
	attachmentID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的附件 ID", true, err))
		return
	}
	userinfo := ctx.Request.Context().Value(bConst.CtxUserinfoKey).(*entity.User)
	isAdmin := isAdminRole(userinfo)
	if xErr := h.service.issueLogic.DeleteAttachment(ctx.Request.Context(), attachmentID, userinfo.ID, isAdmin); xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "删除成功", nil)
}

// ==================== 管理员端接口 ====================

// GetIssueListAdmin 管理员全量问题列表
//
// @Summary     [管理] 全量问题列表
// @Description 管理员查看所有用户的问题列表，支持按状态、优先级、问题类型、关键词筛选，分页返回
// @Tags        管理员问题接口
// @Accept      json
// @Produce     json
// @Param       page query int false "页码（默认 1）"
// @Param       page_size query int false "每页数量（默认 20，最大 50）"
// @Param       status query string false "状态筛选：registered/pending/processing/resolved/unplanned/closed"
// @Param       priority query string false "优先级筛选：low/medium/high/urgent"
// @Param       issue_type_id query int false "问题类型 ID"
// @Param       keyword query string false "关键词搜索（匹配标题）"
// @Success     200   {object}  xBase.BaseResponse{data=apiIssue.IssueListResponse} "获取成功"
// @Failure     401   {object}  xBase.BaseResponse                               "未授权"
// @Failure     403   {object}  xBase.BaseResponse                               "需要管理员权限"
// @Router      /admin/issue/list [GET]
func (h *IssueHandler) GetIssueListAdmin(ctx *gin.Context) {
	h.log.Info(ctx, "GetIssueListAdmin - 管理员全量列表")
	page, pageSize := h.parsePagination(ctx)
	var status *bConst.IssueStatus
	if s := ctx.Query("status"); s != "" {
		st := bConst.IssueStatus(s)
		status = &st
	}
	var priority *bConst.IssuePriority
	if p := ctx.Query("priority"); p != "" {
		pr := bConst.IssuePriority(p)
		priority = &pr
	}
	var issueTypeID *xSnowflake.SnowflakeID
	if tidStr := ctx.Query("issue_type_id"); tidStr != "" {
		if tid, parseErr := strconv.ParseInt(tidStr, 10, 64); parseErr == nil {
			id := xSnowflake.SnowflakeID(tid)
			issueTypeID = &id
		}
	}
	items, total, xErr := h.service.issueLogic.GetIssueListAdmin(ctx.Request.Context(), page, pageSize, status, priority, issueTypeID, ctx.Query("keyword"))
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	resp := apiIssue.IssueListResponse{Total: total, Items: make([]apiIssue.IssueListItem, len(items))}
	for i, dto := range items {
		resp.Items[i] = issueDTOToListItem(&dto)
	}
	xResult.SuccessHasData(ctx, "获取成功", resp)
}

// UpdateIssueStatus 修改问题状态
//
// @Summary     [管理] 修改状态
// @Description 管理员修改问题的处理状态，系统会校验状态流转合法性并自动记录关闭时间
// @Tags        管理员问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "问题 ID"
// @Param       request body apiAdmin.UpdateIssueStatusRequest true "修改状态请求"
// @Success     200   {object}  xBase.BaseResponse "状态更新成功"
// @Failure     400   {object}  xBase.BaseResponse "请求参数错误或非法的状态流转"
// @Failure     401   {object}  xBase.BaseResponse "未授权"
// @Failure     403   {object}  xBase.BaseResponse "需要管理员权限"
// @Failure     404   {object}  xBase.BaseResponse "问题不存在"
// @Router      /admin/issue/{id}/status [PUT]
func (h *IssueHandler) UpdateIssueStatus(ctx *gin.Context) {
	h.log.Info(ctx, "UpdateIssueStatus - 修改问题状态")
	issueID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的问题 ID", true, err))
		return
	}
	req := xUtil.Bind(ctx, &apiAdmin.UpdateIssueStatusRequest{}).Data()
	if req == nil {
		return
	}
	if xErr := h.service.issueLogic.UpdateStatus(ctx.Request.Context(), issueID, bConst.IssueStatus(req.Status)); xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "状态更新成功", nil)
}

// UpdateIssuePriority 修改优先级
//
// @Summary     [管理] 修改优先级
// @Description 管理员修改问题的优先级级别（low/medium/high/urgent）
// @Tags        管理员问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "问题 ID"
// @Param       request body apiAdmin.UpdateIssuePriorityRequest true "修改优先级请求"
// @Success     200   {object}  xBase.BaseResponse "优先级更新成功"
// @Failure     400   {object}  xBase.BaseResponse "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse "未授权"
// @Failure     403   {object}  xBase.BaseResponse "需要管理员权限"
// @Failure     404   {object}  xBase.BaseResponse "问题不存在"
// @Router      /admin/issue/{id}/priority [PUT]
func (h *IssueHandler) UpdateIssuePriority(ctx *gin.Context) {
	h.log.Info(ctx, "UpdateIssuePriority - 修改优先级")
	issueID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的问题 ID", true, err))
		return
	}
	req := xUtil.Bind(ctx, &apiAdmin.UpdateIssuePriorityRequest{}).Data()
	if req == nil {
		return
	}
	if xErr := h.service.issueLogic.UpdatePriority(ctx.Request.Context(), issueID, bConst.IssuePriority(req.Priority)); xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "优先级更新成功", nil)
}

// UpdateIssueNote 更新内部备注
//
// @Summary     [管理] 更新备注
// @Description 管理员更新问题的内部备注信息，备注内容长度不超过 2000 字符，仅管理员可见
// @Tags        管理员问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "问题 ID"
// @Param       request body apiAdmin.UpdateIssueNoteRequest true "更新备注请求"
// @Success     200   {object}  xBase.BaseResponse "备注更新成功"
// @Failure     400   {object}  xBase.BaseResponse "请求参数错误或备注超长"
// @Failure     401   {object}  xBase.BaseResponse "未授权"
// @Failure     403   {object}  xBase.BaseResponse "需要管理员权限"
// @Failure     404   {object}  xBase.BaseResponse "问题不存在"
// @Router      /admin/issue/{id}/note [PUT]
func (h *IssueHandler) UpdateIssueNote(ctx *gin.Context) {
	h.log.Info(ctx, "UpdateIssueNote - 更新内部备注")
	issueID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的问题 ID", true, err))
		return
	}
	req := xUtil.Bind(ctx, &apiAdmin.UpdateIssueNoteRequest{}).Data()
	if req == nil {
		return
	}
	if xErr := h.service.issueLogic.UpdateNote(ctx.Request.Context(), issueID, req.AdminNote); xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "备注更新成功", nil)
}

// ==================== 类型管理接口 ====================

// ListIssueTypes 获取启用的类型列表
//
// @Summary     [公开] 类型列表
// @Description 获取所有已启用的问题类型列表，无需登录即可访问
// @Tags        问题类型接口
// @Accept      json
// @Produce     json
// @Success     200   {object}  xBase.BaseResponse{data=[]apiIssue.IssueTypeListItem} "获取成功"
// @Router      /issue-type/list [GET]
func (h *IssueHandler) ListIssueTypes(ctx *gin.Context) {
	h.log.Info(ctx, "ListIssueTypes - 获取类型列表")
	dtos, xErr := h.service.issueLogic.ListIssueTypes(ctx.Request.Context())
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	items := make([]apiIssue.IssueTypeListItem, len(dtos))
	for i, dto := range dtos {
		items[i] = apiIssue.IssueTypeListItem{ID: dto.ID, Name: dto.Name, Description: dto.Description, SortOrder: dto.SortOrder}
	}
	xResult.SuccessHasData(ctx, "获取成功", items)
}

// CreateIssueType 创建问题类型
//
// @Summary     [管理] 创建类型
// @Description 管理员创建新的问题分类类型，需指定名称和排序值
// @Tags        管理员问题接口
// @Accept      json
// @Produce     json
// @Param       request body apiAdmin.CreateIssueTypeRequest true "创建问题类型请求"
// @Success     200   {object}  xBase.BaseResponse{data=apiIssue.IssueTypeListItem} "创建成功"
// @Failure     400   {object}  xBase.BaseResponse                              "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                              "未授权"
// @Failure     403   {object}  xBase.BaseResponse                              "需要管理员权限"
// @Router      /admin/issue-type [POST]
func (h *IssueHandler) CreateIssueType(ctx *gin.Context) {
	h.log.Info(ctx, "CreateIssueType - 创建问题类型")
	req := xUtil.Bind(ctx, &apiAdmin.CreateIssueTypeRequest{}).Data()
	if req == nil {
		return
	}
	dto, xErr := h.service.issueLogic.CreateIssueType(ctx.Request.Context(), req.Name, req.Description, req.SortOrder)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "创建成功", issueTypeDTOToResponse(dto))
}

// UpdateIssueType 编辑问题类型
//
// @Summary     [管理] 编辑类型
// @Description 管理员编辑已有问题类型的名称、描述、排序或启用状态
// @Tags        管理员问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "类型 ID"
// @Param       request body apiAdmin.UpdateIssueTypeRequest true "编辑问题类型请求"
// @Success     200   {object}  xBase.BaseResponse{data=apiIssue.IssueTypeListItem} "更新成功"
// @Failure     400   {object}  xBase.BaseResponse                              "请求参数错误"
// @Failure     401   {object}  xBase.BaseResponse                              "未授权"
// @Failure     403   {object}  xBase.BaseResponse                              "需要管理员权限"
// @Failure     404   {object}  xBase.BaseResponse                              "问题类型不存在"
// @Router      /admin/issue-type/{id} [PUT]
func (h *IssueHandler) UpdateIssueType(ctx *gin.Context) {
	h.log.Info(ctx, "UpdateIssueType - 编辑问题类型")
	id, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的类型 ID", true, err))
		return
	}
	req := xUtil.Bind(ctx, &apiAdmin.UpdateIssueTypeRequest{}).Data()
	if req == nil {
		return
	}
	dto, xErr := h.service.issueLogic.UpdateIssueType(ctx.Request.Context(), id, req.Name, req.Description, req.SortOrder, req.IsEnabled)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "更新成功", issueTypeDTOToResponse(dto))
}

// DeleteIssueType 删除问题类型
//
// @Summary     [管理] 删除类型
// @Description 管理员删除指定的问题类型，已关联的问题不受影响
// @Tags        管理员问题接口
// @Accept      json
// @Produce     json
// @Param       id path int true "类型 ID"
// @Success     200   {object}  xBase.BaseResponse "删除成功"
// @Failure     400   {object}  xBase.BaseResponse "无效的类型 ID"
// @Failure     401   {object}  xBase.BaseResponse "未授权"
// @Failure     403   {object}  xBase.BaseResponse "需要管理员权限"
// @Router      /admin/issue-type/{id} [DELETE]
func (h *IssueHandler) DeleteIssueType(ctx *gin.Context) {
	h.log.Info(ctx, "DeleteIssueType - 删除问题类型")
	id, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的类型 ID", true, err))
		return
	}
	if xErr := h.service.issueLogic.DeleteIssueType(ctx.Request.Context(), id); xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "删除成功", nil)
}

// ==================== Helper Methods ====================

// isAdminRole 判断用户是否具有管理员角色（SUPER_ADMIN 或 ADMIN）。
func isAdminRole(userinfo *entity.User) bool {
	return userinfo.RoleName != nil && (*userinfo.RoleName == "SUPER_ADMIN" || *userinfo.RoleName == "ADMIN")
}

// parsePagination 从请求中解析分页参数，返回 (page, pageSize)。
// 缺失或非法值回退到默认值（page=1, pageSize=20），pageSize 上限截断为 50。
func (h *IssueHandler) parsePagination(ctx *gin.Context) (int, int) {
	pageStr := ctx.DefaultQuery("page", strconv.Itoa(issueDefaultPage))
	pageSizeStr := ctx.DefaultQuery("page_size", strconv.Itoa(issueDefaultPageSize))

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = issueDefaultPage
	}

	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 {
		pageSize = issueDefaultPageSize
	}
	if pageSize > issueMaxPageSize {
		pageSize = issueMaxPageSize
	}
	return page, pageSize
}
