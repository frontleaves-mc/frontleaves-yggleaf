package entity

import (
	"fmt"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"gorm.io/gorm"
)

// UserCapeLibrary 用户披风关联实体，记录用户对披风资源的拥有关系。
//
// 设计同构于 UserSkinLibrary，用于披风资源的用户关联管理。
type UserCapeLibrary struct {
	xModels.BaseEntity                           // 嵌入基础实体字段
	UserID             xSnowflake.SnowflakeID    `gorm:"not null;uniqueIndex:uk_user_cape_library_user_cape;comment:关联用户ID" json:"user_id"`             // 关联用户ID
	CapeLibraryID      xSnowflake.SnowflakeID    `gorm:"not null;uniqueIndex:uk_user_cape_library_user_cape;comment:关联披风库ID" json:"cape_library_id"`    // 关联披风库ID
	AssignmentType     entityType.AssignmentType `gorm:"not null;type:smallint;default:1;comment:关联类型(1=normal,2=gift,3=admin)" json:"assignment_type"` // 关联类型

	// ----------
	//  外键约束
	// ----------
	User        *User        `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"`                                                 // 关联用户
	CapeLibrary *CapeLibrary `gorm:"foreignKey:CapeLibraryID;references:ID;constraint:OnDelete:CASCADE;comment:关联披风库" json:"cape_library,omitempty"` // 关联披风库
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *UserCapeLibrary) GetGene() xSnowflake.Gene {
	return bConst.GeneForUserCapeLibrary
}

func (u *UserCapeLibrary) BeforeCreate(_ *gorm.DB) error {
	if u.AssignmentType.IsValid() {
		return nil
	}

	return fmt.Errorf("无效的关联类型: %d", u.AssignmentType)
}

func (u *UserCapeLibrary) BeforeUpdate(_ *gorm.DB) error {
	if u.AssignmentType.IsValid() {
		return nil
	}
	return fmt.Errorf("无效的关联类型: %d", u.AssignmentType)
}
