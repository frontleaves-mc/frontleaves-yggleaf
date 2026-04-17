package entity

import (
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// IssueType 问题类型实体，用于分类用户提交的问题反馈。
type IssueType struct {
	xModels.BaseEntity
	Name        string `gorm:"not null;type:varchar(32);uniqueIndex:uk_issue_type_name;comment:类型名称" json:"name"`
	Description string `gorm:"type:varchar(255);comment:类型描述" json:"description,omitempty"`
	SortOrder   int    `gorm:"not null;type:int;default:0;comment:排序序号" json:"sort_order"`
	IsEnabled   bool   `gorm:"not null;type:boolean;default:true;comment:是否启用" json:"is_enabled"`
}

func (_ *IssueType) GetGene() xSnowflake.Gene {
	return bConst.GeneForIssueType
}
