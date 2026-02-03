package entity

import (
	"regexp"
	"time"

	"gorm.io/gorm"
)

type RoleName string

const (
	RoleSuperAdmin RoleName = "SUPER_ADMIN" // 超级管理员角色名称
	RoleAdmin      RoleName = "ADMIN"       // 管理员角色名称
	RolePlayer     RoleName = "PLAYER"      // 普通玩家角色名称
)

func (rN RoleName) String() string {
	return string(rN)
}

var (
	// roleRegex 角色名称正则，只允许大写字母和下划线，2-32位
	roleRegex = regexp.MustCompile(`^[A-Z_]{2,32}$`)
)

// Role 角色实体，使用 Name 作为主键，不继承 BaseEntity。
type Role struct {
	Name        RoleName  `gorm:"primaryKey;type:varchar(32);comment:角色名称" json:"name"`
	DisplayName string    `gorm:"not null;type:varchar(64);comment:角色显示名称" json:"display_name"`
	Description string    `gorm:"not null;type:varchar(255);comment:角色描述" json:"description"`
	CreatedAt   time.Time `gorm:"not null;type:timestamptz;autoCreateTime:milli;comment:创建时间" json:"-"`
}

// BeforeCreate GORM 钩子，创建前验证角色名称格式。
func (r *Role) BeforeCreate(_ *gorm.DB) error {
	if !roleRegex.MatchString(r.Name.String()) {
		return &RoleNameError{Name: r.Name.String()}
	}
	return nil
}

// RoleNameError 角色名称格式错误。
type RoleNameError struct {
	Name string
}

func (e *RoleNameError) Error() string {
	return "无效角色名称：必须匹配 ^[A-Z_]{2,32}"
}
