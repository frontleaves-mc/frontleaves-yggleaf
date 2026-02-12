package entity

import (
	"time"

	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
)

// User 用户实体，包含账号验证、安全状态及关联游戏档案信息。
type User struct {
	xModels.BaseEntity            // 嵌入基础实体字段
	Username           string     `gorm:"not null;type:varchar(255);comment:用户用户名" json:"username"`               // 用户用户名
	Email              *string    `gorm:"type:varchar(255);comment:用户邮箱;index:idx_email" json:"email"`            // 用户邮箱
	Phone              *string    `gorm:"type:varchar(32);comment:用户手机号;index:idx_phone" json:"phone"`            // 用户手机号
	RoleName           *string    `gorm:"type:varchar(32);comment:关联角色名称" json:"role_name"`                       // 关联角色名称
	GamePassword       string     `gorm:"not null;type:varchar(255);comment:游戏账户密码" json:"-"`                     // 游戏账户密码
	HasBan             bool       `gorm:"not null;type:boolean;default:false;comment:用户是否被封禁禁止登录" json:"has_ban"` // 用户是否被封禁禁止登录
	JailedAt           *time.Time `gorm:"type:timestamptz;comment:用户被监禁的时间" json:"jailed_at"`                     // 用户被监禁的时间

	// ----------
	//  外键约束
	// ----------
	Role         *Role         `gorm:"foreignKey:RoleName;references:Name;constraint:OnDelete:RESTRICT;comment:关联角色" json:"role,omitempty"` // 关联角色
	GameProfiles []GameProfile `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE;comment:游戏档案关联" json:"game_profiles,omitempty"`         // 游戏档案关联
}

// GetGene 返回 xSnowflake.GeneUser，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *User) GetGene() xSnowflake.Gene {
	return xSnowflake.GeneUser
}
