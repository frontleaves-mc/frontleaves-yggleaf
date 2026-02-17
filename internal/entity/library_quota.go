package entity

import (
	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// LibraryQuota 皮肤库与披风库共享配额实体，记录用户在不同资源类型下的额度使用情况。
type LibraryQuota struct {
	xModels.BaseEntity                        // 嵌入基础实体字段
	UserID             xSnowflake.SnowflakeID `gorm:"not null;uniqueIndex:uk_library_quota_user_resource_type;comment:关联用户ID" json:"user_id"`                                                                                       // 关联用户ID
	ResourceType       string                 `gorm:"not null;type:varchar(32);uniqueIndex:uk_library_quota_user_resource_type;index:idx_library_quota_resource_type;comment:资源类型(skin_library/cape_library)" json:"resource_type"` // 资源类型(skin_library/cape_library)
	Total              int32                  `gorm:"not null;default:0;comment:总额度" json:"total"`                                                                                                                                  // 总额度
	Used               int32                  `gorm:"not null;default:0;comment:已使用额度" json:"used"`                                                                                                                                 // 已使用额度

	// ----------
	//  外键约束
	// ----------
	User User `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"` // 关联用户
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *LibraryQuota) GetGene() xSnowflake.Gene {
	return bConst.GeneForLibraryQuota
}
