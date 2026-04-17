package issue

import xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"

// CreateIssueRequest 提交问题请求。
type CreateIssueRequest struct {
	IssueTypeID xSnowflake.SnowflakeID `json:"issue_type_id" binding:"required"`
	Title       string                 `json:"title" binding:"required,max=128"`
	Content     string                 `json:"content" binding:"required,max=10000"`
	Priority    string                 `json:"priority" binding:"omitempty,oneof=low medium high urgent"`
}

// ReplyIssueRequest 回复问题请求。
type ReplyIssueRequest struct {
	Content string `json:"content" binding:"required,max=5000"`
}

// UploadAttachmentRequest 上传附件请求。
type UploadAttachmentRequest struct {
	FileName string `json:"file_name" binding:"required,max=255"`
	Content  string `json:"content" binding:"required"`
	MimeType string `json:"mime_type" binding:"omitempty,max=64"`
}

// IssueListQuery 问题列表查询参数。
type IssueListQuery struct {
	Page     int    `form:"page" binding:"omitempty,min=1"`
	PageSize int    `form:"page_size" binding:"omitempty,min=1,max=50"`
	Status   string `form:"status" binding:"omitempty,oneof=registered pending processing resolved unplanned closed"`
	Priority string `form:"priority" binding:"omitempty,oneof=low medium high urgent"`
	Keyword  string `form:"keyword" binding:"omitempty,max=64"`
}
