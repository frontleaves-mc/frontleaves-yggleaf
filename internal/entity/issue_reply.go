package entity

import (
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// IssueReply 问题回复实体，记录用户或管理员对问题的追加回复。
type IssueReply struct {
	xModels.BaseEntity
	IssueID      xSnowflake.SnowflakeID `gorm:"not null;type:bigint;index:idx_issue_reply_issue_id;comment:关联问题ID" json:"issue_id"`
	UserID       xSnowflake.SnowflakeID `gorm:"not null;type:bigint;index:idx_issue_reply_user_id;comment:回复者用户ID" json:"user_id"`
	Content      string                `gorm:"not null;type:text;comment:回复内容" json:"content"`
	IsAdminReply bool                  `gorm:"not null;type:boolean;default:false;comment:是否为管理员回复" json:"is_admin_reply"`

	// 外键约束
	Issue *Issue `gorm:"foreignKey:IssueID;references:ID;constraint:OnDelete:CASCADE;comment:关联问题" json:"issue,omitempty"`
	User  *User  `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:RESTRICT;comment:关联用户" json:"user,omitempty"`
}

func (_ *IssueReply) GetGene() xSnowflake.Gene {
	return bConst.GeneForIssueReply
}
