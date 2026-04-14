package entity

import (
	"fmt"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"gorm.io/gorm"
)

// UserSkinLibrary 用户皮肤关联实体，记录用户对皮肤资源的拥有关系。
//
// 该实体是 SkinLibrary（资源定义）与 User（所有者）之间的关联表，
// 通过 AssignmentType 区分资源来源并决定配额计算行为：
//   - normal：用户自行上传，计入配额
//   - gift：管理员赠送，不计入配额
//   - admin：系统预置，不计入配额
type UserSkinLibrary struct {
	xModels.BaseEntity                                                   // 嵌入基础实体字段
	UserID         xSnowflake.SnowflakeID    `gorm:"not null;uniqueIndex:uk_user_skin_library_user_skin;comment:关联用户ID" json:"user_id"`                                // 关联用户ID
	SkinLibraryID  xSnowflake.SnowflakeID    `gorm:"not null;uniqueIndex:uk_user_skin_library_user_skin;comment:关联皮肤库ID" json:"skin_library_id"`                       // 关联皮肤库ID
	AssignmentType entityType.AssignmentType `gorm:"not null;type:smallint;default:1;comment:关联类型(1=normal,2=gift,3=admin)" json:"assignment_type"` // 关联类型

	// ----------
	//  外键约束
	// ----------
	User        *User        `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"`                                               // 关联用户
	SkinLibrary *SkinLibrary `gorm:"foreignKey:SkinLibraryID;references:ID;constraint:OnDelete:CASCADE;comment:关联皮肤库" json:"skin_library,omitempty"` // 关联皮肤库
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *UserSkinLibrary) GetGene() xSnowflake.Gene {
	return bConst.GeneForUserSkinLibrary
}

func (u *UserSkinLibrary) BeforeCreate(_ *gorm.DB) error {
	if u.AssignmentType.IsValid() {
		return nil
	}
	return fmt.Errorf("无效的关联类型: %d", u.AssignmentType)
}

func (u *UserSkinLibrary) BeforeUpdate(_ *gorm.DB) error {
	if u.AssignmentType.IsValid() {
		return nil
	}
	return fmt.Errorf("无效的关联类型: %d", u.AssignmentType)
}
