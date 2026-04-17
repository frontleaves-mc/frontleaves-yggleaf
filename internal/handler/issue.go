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
	issueDefaultPageSize = 20
	issueMaxPageSize     = 100
)

// ==================== 用户端接口 ====================

// CreateIssue 提交问题。
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

// GetIssueList 我的问题列表。
func (h *IssueHandler) GetIssueList(ctx *gin.Context) {
	h.log.Info(ctx, "GetIssueList - 获取我的问题列表")
	page, pageSize := h.parsePagination(ctx)
	userinfo := ctx.MustGet(bConst.CtxUserinfoKey).(*entity.User)
	items, total, xErr := h.service.issueLogic.GetIssueList(ctx.Request.Context(), userinfo.ID, page, pageSize)
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

// GetIssueDetail 问题详情。
func (h *IssueHandler) GetIssueDetail(ctx *gin.Context) {
	h.log.Info(ctx, "GetIssueDetail - 获取问题详情")
	issueID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的问题 ID", true, err))
		return
	}
	dto, xErr := h.service.issueLogic.GetIssueDetail(ctx.Request.Context(), issueID)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "获取成功", issueDetailDTOToResponse(dto))
}

// ReplyIssue 追加回复。
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
	userinfo := ctx.MustGet(bConst.CtxUserinfoKey).(*entity.User)
	isAdmin := userinfo.RoleName != nil && (*userinfo.RoleName == "SUPER_ADMIN" || *userinfo.RoleName == "ADMIN")
	dto, xErr := h.service.issueLogic.ReplyIssue(ctx.Request.Context(), issueID, userinfo.ID, req.Content, isAdmin)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "回复成功", issueReplyDTOToResponse(dto))
}

// UploadAttachment 上传附件。
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
	userinfo := ctx.MustGet(bConst.CtxUserinfoKey).(*entity.User)
	dto, xErr := h.service.issueLogic.UploadAttachment(ctx.Request.Context(), issueID, userinfo.ID, req.FileName, req.Content, req.MimeType)
	if xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "上传成功", issueAttachmentDTOToResponse(dto))
}

// DeleteAttachment 删除附件。
func (h *IssueHandler) DeleteAttachment(ctx *gin.Context) {
	h.log.Info(ctx, "DeleteAttachment - 删除附件")
	attachmentID, err := xSnowflake.ParseSnowflakeID(ctx.Param("id"))
	if err != nil {
		_ = ctx.Error(xError.NewError(ctx, xError.ParameterError, "无效的附件 ID", true, err))
		return
	}
	if xErr := h.service.issueLogic.DeleteAttachment(ctx.Request.Context(), attachmentID); xErr != nil {
		_ = ctx.Error(xErr)
		return
	}
	xResult.SuccessHasData(ctx, "删除成功", nil)
}

// ==================== 管理员端接口 ====================

// GetIssueListAdmin 全部问题列表（管理员）。
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

// UpdateIssueStatus 修改问题状态。
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

// UpdateIssuePriority 修改优先级。
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

// UpdateIssueNote 更新内部备注。
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

// ListIssueTypes 获取启用的类型列表（公开）。
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

// CreateIssueType 创建问题类型（管理员）。
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

// UpdateIssueType 编辑问题类型（管理员）。
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

// DeleteIssueType 删除问题类型（管理员）。
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
