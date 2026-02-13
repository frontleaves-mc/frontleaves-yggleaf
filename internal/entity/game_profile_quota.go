package entity

import (
	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// GameProfileQuota 用户游戏档案配额实体，记录用户当前总额度与已使用额度。
type GameProfileQuota struct {
	xModels.BaseEntity                        // 嵌入基础实体字段
	UserID             xSnowflake.SnowflakeID `gorm:"not null;uniqueIndex:uk_game_profile_quota_user_id;comment:关联用户ID" json:"user_id"` // 关联用户ID
	Total              int32                  `gorm:"not null;default:0;comment:总额度" json:"total"`                                      // 总额度
	Used               int32                  `gorm:"not null;default:0;comment:已使用额度" json:"used"`                                     // 已使用额度

	// ----------
	//  外键约束
	// ----------
	User User `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"` // 关联用户
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *GameProfileQuota) GetGene() xSnowflake.Gene {
	return bConst.GeneForGameProfileQuota
}
