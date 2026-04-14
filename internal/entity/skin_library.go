package entity

import (
	"errors"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"gorm.io/gorm"
)

// ModelType 皮肤模型类型
type ModelType uint8

const (
	ModelTypeClassic ModelType = 1 // 经典皮肤模型 (Steve)
	ModelTypeSlim    ModelType = 2 //纤细皮肤模型 (Alex)
)

// SkinLibrary 皮肤库实体，存储系统内置或用户上传的皮肤资源。
type SkinLibrary struct {
	xModels.BaseEntity                         // 嵌入基础实体字段
	UserID             *xSnowflake.SnowflakeID `gorm:"type:bigint;index:idx_skin_library_user_id;comment:创建者/上传者用户ID(为空代表系统内置皮肤)" json:"user_id,omitempty"`     // 创建者/上传者用户ID(为空代表系统内置皮肤)
	Name               string                  `gorm:"not null;type:varchar(64);comment:皮肤名称" json:"name"`                                                 // 皮肤名称
	Texture            int64                   `gorm:"not null;type:bigint;comment:皮肤纹理文件ID(雪花算法)" json:"texture"`                                         // 皮肤纹理文件ID(雪花算法)
	TextureHash        string                  `gorm:"not null;type:char(64);uniqueIndex:uk_skin_library_texture_hash;comment:皮肤纹理哈希" json:"texture_hash"` // 皮肤纹理哈希
	Model              ModelType               `gorm:"not null;type:smallint;default:1;comment:皮肤模型(1=classic,2=slim)" json:"model"`                       // 皮肤模型(1=classic,2=slim)
	IsPublic           bool                    `gorm:"not null;type:boolean;default:false;index:idx_skin_library_is_public;comment:是否公开" json:"is_public"` // 是否公开

	// ----------
	//  外键约束
	// ----------
	User *User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:SET NULL;comment:关联用户" json:"user,omitempty"` // 关联用户
}

func (s *SkinLibrary) BeforeCreate(tx *gorm.DB) error {
	switch s.Model {
	case ModelTypeClassic:
	case ModelTypeSlim:
		break
	default:
		return errors.New("皮肤库的模型类型无效")
	}

	return nil
}

func (s *SkinLibrary) BeforeUpdate(tx *gorm.DB) error {
	switch s.Model {
	case ModelTypeClassic:
	case ModelTypeSlim:
		break
	default:
		return errors.New("皮肤库的模型类型无效")
	}

	return nil
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (s *SkinLibrary) GetGene() xSnowflake.Gene {
	return bConst.GeneForSkinLibrary
}
