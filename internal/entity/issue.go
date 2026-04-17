package entity

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// Issue 问题主表实体，记录用户提交的问题反馈。
type Issue struct {
	xModels.BaseEntity
	UserID       xSnowflake.SnowflakeID `gorm:"not null;type:bigint;index:idx_issue_user_id;comment:提交者用户ID" json:"user_id"`
	IssueTypeID  xSnowflake.SnowflakeID `gorm:"not null;type:bigint;index:idx_issue_type_id;comment:问题类型ID" json:"issue_type_id"`
	Title        string                `gorm:"not null;type:varchar(128);comment:问题标题" json:"title"`
	Content      string                `gorm:"not null;type:text;comment:问题描述" json:"content"`
	Status       bConst.IssueStatus   `gorm:"not null;type:varchar(20);default:registered;index:idx_issue_status;comment:当前状态" json:"status"`
	Priority     bConst.IssuePriority `gorm:"not null;type:varchar(10);default:medium;comment:优先级" json:"priority"`
	AdminNote    string                `gorm:"type:text;comment:内部备注(仅管理员可见)" json:"admin_note,omitempty"`
	ClosedAt     *time.Time            `gorm:"type:timestamptz;comment:关闭时间" json:"closed_at,omitempty"`

	// 外键约束
	User      *User      `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"`
	IssueType *IssueType `gorm:"foreignKey:IssueTypeID;references:ID;constraint:OnDelete:RESTRICT;comment:关联问题类型" json:"issue_type,omitempty"`
}

func (_ *Issue) GetGene() xSnowflake.Gene {
	return bConst.GeneForIssue
}
