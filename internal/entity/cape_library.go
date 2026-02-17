package entity

import (
	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// CapeLibrary 披风库实体，存储系统内置或用户上传的披风资源。
type CapeLibrary struct {
	xModels.BaseEntity                         // 嵌入基础实体字段
	UserID             *xSnowflake.SnowflakeID `gorm:"type:bigint;index:idx_cape_library_user_id;comment:关联用户ID(为空代表系统内置披风)" json:"user_id,omitempty"`     // 关联用户ID(为空代表系统内置披风)
	Name               string                  `gorm:"not null;type:varchar(64);comment:披风名称" json:"name"`                                                 // 披风名称
	TextureURL         string                  `gorm:"not null;type:varchar(512);comment:披风纹理URL" json:"texture_url"`                                      // 披风纹理URL
	TextureHash        string                  `gorm:"not null;type:char(64);uniqueIndex:uk_cape_library_texture_hash;comment:披风纹理哈希" json:"texture_hash"` // 披风纹理哈希
	IsPublic           bool                    `gorm:"not null;type:boolean;default:false;index:idx_cape_library_is_public;comment:是否公开" json:"is_public"` // 是否公开

	// ----------
	//  外键约束
	// ----------
	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:SET NULL;comment:关联用户" json:"user,omitempty"` // 关联用户
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *CapeLibrary) GetGene() xSnowflake.Gene {
	return bConst.GeneForCapeLibrary
}
