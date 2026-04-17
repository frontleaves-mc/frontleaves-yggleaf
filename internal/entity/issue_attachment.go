package entity

import (
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// IssueAttachment 问题附件实体，存储问题关联的文件（与 Library 模块 Texture 同模式：DB 仅存 int64 FileId）。
type IssueAttachment struct {
	xModels.BaseEntity
	IssueID  xSnowflake.SnowflakeID `gorm:"not null;type:bigint;index:idx_attachment_issue_id;comment:关联问题ID" json:"issue_id"`
	FileID   int64                  `gorm:"not null;type:bigint;comment:存储桶文件ID(雪花算法)" json:"file_id"`
	FileName string                 `gorm:"not null;type:varchar(255);comment:原始文件名" json:"file_name"`
	FileSize int64                  `gorm:"not null;type:bigint;comment:文件大小(字节)" json:"file_size"`
	MimeType string                 `gorm:"type:varchar(64);comment:MIME类型" json:"mime_type,omitempty"`

	// 外键约束
	Issue *Issue `gorm:"foreignKey:IssueID;references:ID;constraint:OnDelete:CASCADE;comment:关联问题" json:"issue,omitempty"`
}

func (_ *IssueAttachment) GetGene() xSnowflake.Gene {
	return bConst.GeneForIssueAttachment
}
