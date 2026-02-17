package entity

import (
	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// SkinLibrary 皮肤库实体，存储系统内置或用户上传的皮肤资源。
type SkinLibrary struct {
	xModels.BaseEntity                         // 嵌入基础实体字段
	UserID             *xSnowflake.SnowflakeID `gorm:"type:bigint;index:idx_skin_library_user_id;comment:关联用户ID(为空代表系统内置皮肤)" json:"user_id,omitempty"`     // 关联用户ID(为空代表系统内置皮肤)
	Name               string                  `gorm:"not null;type:varchar(64);comment:皮肤名称" json:"name"`                                                 // 皮肤名称
	TextureURL         string                  `gorm:"not null;type:varchar(512);comment:皮肤纹理URL" json:"texture_url"`                                      // 皮肤纹理URL
	TextureHash        string                  `gorm:"not null;type:char(64);uniqueIndex:uk_skin_library_texture_hash;comment:皮肤纹理哈希" json:"texture_hash"` // 皮肤纹理哈希
	Model              *string                 `gorm:"type:varchar(16);comment:皮肤模型(classic/slim)" json:"model,omitempty"`                                 // 皮肤模型(classic/slim)
	IsPublic           bool                    `gorm:"not null;type:boolean;default:false;index:idx_skin_library_is_public;comment:是否公开" json:"is_public"` // 是否公开

	// ----------
	//  外键约束
	// ----------
	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:SET NULL;comment:关联用户" json:"user,omitempty"` // 关联用户
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *SkinLibrary) GetGene() xSnowflake.Gene {
	return bConst.GeneForSkinLibrary
}
