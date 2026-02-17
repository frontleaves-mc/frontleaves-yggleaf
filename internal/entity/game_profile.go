package entity

import (
	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/google/uuid"
)

// GameProfile 游戏档案实体，存储 Minecraft 游戏账号信息，一个用户可以拥有多个游戏档案。
type GameProfile struct {
	xModels.BaseEntity                         // 嵌入基础实体字段
	UserID             xSnowflake.SnowflakeID  `gorm:"not null;index:idx_user_id;comment:关联用户ID" json:"user_id"`                                            // 关联用户ID
	UUID               uuid.UUID               `gorm:"unique;not null;type:varchar(36);comment:Minecraft UUID" json:"uuid"`                                 // Minecraft UUID
	Name               string                  `gorm:"not null;type:varchar(32);comment:游戏内用户名" json:"name"`                                                // 游戏内用户名
	SkinLibraryID      *xSnowflake.SnowflakeID `gorm:"type:bigint;index:idx_game_profile_skin_library_id;comment:关联皮肤库ID" json:"skin_library_id,omitempty"` // 关联皮肤库ID
	CapeLibraryID      *xSnowflake.SnowflakeID `gorm:"type:bigint;index:idx_game_profile_cape_library_id;comment:关联披风库ID" json:"cape_library_id,omitempty"` // 关联披风库ID

	// ----------
	//  外键约束
	// ----------
	User        User         `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"`                                                  // 关联用户
	SkinLibrary *SkinLibrary `gorm:"foreignKey:SkinLibraryID;references:ID;constraint:OnDelete:SET NULL;comment:关联皮肤库" json:"skin_library,omitempty"` // 关联皮肤库
	CapeLibrary *CapeLibrary `gorm:"foreignKey:CapeLibraryID;references:ID;constraint:OnDelete:SET NULL;comment:关联披风库" json:"cape_library,omitempty"` // 关联披风库
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *GameProfile) GetGene() xSnowflake.Gene {
	return bConst.GeneForGameProfile
}
