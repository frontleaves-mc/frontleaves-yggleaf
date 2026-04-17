package admin

// UpdateIssueStatusRequest 修改问题状态请求。
type UpdateIssueStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=registered pending processing resolved unplanned closed"`
}

// UpdateIssuePriorityRequest 修改优先级请求。
type UpdateIssuePriorityRequest struct {
	Priority string `json:"priority" binding:"required,oneof=low medium high urgent"`
}

// UpdateIssueNoteRequest 更新内部备注请求。
type UpdateIssueNoteRequest struct {
	AdminNote string `json:"admin_note" binding:"max=2000"`
}

// AdminIssueListQuery 管理员问题列表查询参数。
type AdminIssueListQuery struct {
	Page        int    `form:"page" binding:"omitempty,min=1"`
	PageSize    int    `form:"page_size" binding:"omitempty,min=1,max=50"`
	Status      string `form:"status" binding:"omitempty,oneof=registered pending processing resolved unplanned closed"`
	Priority    string `form:"priority" binding:"omitempty,oneof=low medium high urgent"`
	IssueTypeID int64  `form:"issue_type_id"`
	Keyword     string `form:"keyword" binding:"omitempty,max=64"`
}

// CreateIssueTypeRequest 创建问题类型请求。
type CreateIssueTypeRequest struct {
	Name        string `json:"name" binding:"required,max=32"`
	Description string `json:"description" binding:"omitempty,max=255"`
	SortOrder   int    `json:"sort_order"`
}

// UpdateIssueTypeRequest 编辑问题类型请求。
type UpdateIssueTypeRequest struct {
	Name        *string `json:"name,omitempty" binding:"omitempty,max=32"`
	Description *string `json:"description,omitempty" binding:"omitempty,max=255"`
	SortOrder   *int    `json:"sort_order,omitempty"`
	IsEnabled   *bool   `json:"is_enabled,omitempty"`
}
