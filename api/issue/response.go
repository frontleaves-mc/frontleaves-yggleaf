package issue

import (
	time "time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
)

// IssueEntityWrapper 问题实体包装（控制 JSON 暴露字段）。
type IssueEntityWrapper struct {
	ID           xSnowflake.SnowflakeID `json:"id"`
	UserID       xSnowflake.SnowflakeID `json:"user_id"`
	IssueTypeID  xSnowflake.SnowflakeID `json:"issue_type_id"`
	Title        string                 `json:"title"`
	Content      string                 `json:"content"`
	Status       string                 `json:"status"`
	Priority     string                 `json:"priority"`
	AdminNote    string                 `json:"admin_note,omitempty"`
	ClosedAt     *time.Time             `json:"closed_at,omitempty"`
	CreatedAt    time.Time              `json:"created_at"`
	UpdatedAt    time.Time              `json:"updated_at"`
}

// IssueReplyEntityWrapper 回复实体包装。
type IssueReplyEntityWrapper struct {
	ID           xSnowflake.SnowflakeID `json:"id"`
	IssueID      xSnowflake.SnowflakeID `json:"issue_id"`
	UserID       xSnowflake.SnowflakeID `json:"user_id"`
	Content      string                 `json:"content"`
	IsAdminReply bool                   `json:"is_admin_reply"`
	CreatedAt    time.Time              `json:"created_at"`
}

// IssueTypeEntityWrapper 类型实体包装。
type IssueTypeEntityWrapper struct {
	ID          xSnowflake.SnowflakeID `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	SortOrder   int                    `json:"sort_order"`
	IsEnabled   bool                   `json:"is_enabled"`
}

// IssueListItem 问题列表项（含冗余字段避免联查）。
type IssueListItem struct {
	Issue         IssueEntityWrapper `json:"issue"`
	IssueTypeName string             `json:"issue_type_name"`
	ReplyCount    int                `json:"reply_count"`
}

// IssueListResponse 问题列表响应。
type IssueListResponse struct {
	Total int64            `json:"total"`
	Items []IssueListItem `json:"items"`
}

// IssueDetailResponse 问题详情响应（含回复+附件+类型信息）。
type IssueDetailResponse struct {
	Issue       IssueEntityWrapper     `json:"issue"`
	IssueType   IssueTypeEntityWrapper `json:"issue_type"`
	Replies     []IssueReplyItem      `json:"replies"`
	Attachments []IssueAttachmentItem `json:"attachments"`
}

// IssueReplyItem 回复项（含用户名）。
type IssueReplyItem struct {
	IssueReply IssueReplyEntityWrapper `json:"issue_reply"`
	Username   string                  `json:"username"`
}

// IssueAttachmentItem 附件响应项 — FileId 已通过 Bucket SDK 解析为下载链接。
type IssueAttachmentItem struct {
	ID       int64  `json:"id"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
	MimeType string `json:"mime_type"`
	FileURL  string `json:"file_url"`
}

// IssueTypeListItem 类型列表项。
type IssueTypeListItem struct {
	ID          xSnowflake.SnowflakeID `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	SortOrder   int                    `json:"sort_order"`
}
