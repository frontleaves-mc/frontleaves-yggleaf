package entity

import (
	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// GameProfile 游戏档案实体，存储 Minecraft 游戏账号信息，一个用户可以拥有多个游戏档案。
type GameProfile struct {
	xModels.BaseEntity                        // 嵌入基础实体字段
	UserID             xSnowflake.SnowflakeID `gorm:"not null;index:idx_user_id;comment:关联用户ID" json:"user_id"`            // 关联用户ID
	UUID               string                 `gorm:"unique;not null;type:varchar(36);comment:Minecraft UUID" json:"uuid"` // Minecraft UUID
	Name               string                 `gorm:"not null;type:varchar(32);comment:游戏内用户名" json:"name"`                // 游戏内用户名
	SkinURL            *string                `gorm:"type:varchar(512);comment:皮肤URL" json:"skin_url"`                     // 皮肤URL
	CapeURL            *string                `gorm:"type:varchar(512);comment:披风URL" json:"cape_url"`                     // 披风URL

	// ----------
	//  外键约束
	// ----------
	User User `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"` // 关联用户
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *GameProfile) GetGene() xSnowflake.Gene {
	return bConst.GeneForGameProfile
}
