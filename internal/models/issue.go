package models

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// IssueDTO 问题数据传输对象（Logic 层构建，Handler 层转换为 Response）。
type IssueDTO struct {
	ID               xSnowflake.SnowflakeID
	UserID           xSnowflake.SnowflakeID
	IssueTypeID      xSnowflake.SnowflakeID
	IssueTypeName    string
	Title            string
	Content          string
	Status           bConst.IssueStatus
	Priority         bConst.IssuePriority
	AdminNote        string
	ClosedAt         *time.Time
	ReplyCount       int
	AttachmentCount  int
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// IssueListResponse 问题列表响应（Logic 层用）。
type IssueListResponse struct {
	Total int64
	Items []IssueDTO
}

// IssueDetailDTO 问题详情 DTO（含嵌套的回复和附件）。
type IssueDetailDTO struct {
	Issue       IssueDTO
	IssueType   entity.IssueType
	Replies     []IssueReplyDTO
	Attachments []IssueAttachmentDTO
}

// IssueReplyDTO 回复 DTO。
type IssueReplyDTO struct {
	ID           xSnowflake.SnowflakeID
	IssueID      xSnowflake.SnowflakeID
	UserID       xSnowflake.SnowflakeID
	Username     string
	Content      string
	IsAdminReply bool
	CreatedAt    time.Time
}

// IssueAttachmentDTO 附件 DTO（FileId 已解析为下载链接）。
type IssueAttachmentDTO struct {
	ID       int64
	FileName string
	FileSize int64
	MimeType string
	FileURL  string
}

// IssueTypeDTO 类型 DTO。
type IssueTypeDTO struct {
	ID          xSnowflake.SnowflakeID
	Name        string
	Description string
	SortOrder   int
	IsEnabled   bool
}
