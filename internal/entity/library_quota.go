package entity

import (
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// LibraryQuota 皮肤库与披风库共享配额实体，记录用户在不同资源类型下的额度使用情况。
type LibraryQuota struct {
	xModels.BaseEntity                        // 嵌入基础实体字段
	UserID             xSnowflake.SnowflakeID `gorm:"not null;uniqueIndex:uk_library_quota_user_id;comment:关联用户ID" json:"user_id"` // 关联用户ID

	// 皮肤配额
	SkinsPrivateTotal int32 `gorm:"not null;default:0;comment:私有皮肤总额度" json:"skins_private_total"`  // 私有皮肤总额度
	SkinsPublicTotal  int32 `gorm:"not null;default:0;comment:公开皮肤总额度" json:"skins_public_total"`   // 公开皮肤总额度
	SkinsPrivateUsed  int32 `gorm:"not null;default:0;comment:私有皮肤已使用额度" json:"skins_private_used"` // 私有皮肤已使用额度
	SkinsPublicUsed   int32 `gorm:"not null;default:0;comment:公开皮肤已使用额度" json:"skins_public_used"`  // 公开皮肤已使用额度

	// 披风配额
	CapesPrivateTotal int32 `gorm:"not null;default:0;comment:私有披风总额度" json:"capes_private_total"`  // 私有披风总额度
	CapesPublicTotal  int32 `gorm:"not null;default:0;comment:公开披风总额度" json:"capes_public_total"`   // 公开披风总额度
	CapesPrivateUsed  int32 `gorm:"not null;default:0;comment:私有披风已使用额度" json:"capes_private_used"` // 私有披风已使用额度
	CapesPublicUsed   int32 `gorm:"not null;default:0;comment:公开披风已使用额度" json:"capes_public_used"`  // 公开披风已使用额度

	// ----------
	//  外键约束
	// ----------
	User User `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"` // 关联用户
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *LibraryQuota) GetGene() xSnowflake.Gene {
	return bConst.GeneForLibraryQuota
}
