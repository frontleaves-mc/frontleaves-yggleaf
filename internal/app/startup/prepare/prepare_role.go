package prepare

import (
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
)

// prepareRole 初始化并保存系统默认的角色数据
//
// 该方法用于在系统初始化阶段预置三种核心角色：超级管理员、管理员和商户。
// 它通过 GORM 的 Save 操作实现 "Insert On Duplicate Update" 逻辑，确保即使
// 记录已存在也能更新其显示名称和描述，保证角色定义的一致性。
//
// 包含的角色:
//   - SUPER_ADMIN (超级管理员): 拥有所有权限。
//   - ADMIN (管理员): 拥有大部分管理权限。
//   - PLAYER (玩家): 拥有基本使用权限。
func (p *Prepare) prepareRole() {
	p.db.Model(&entity.Role{}).Where("name = ?", entity.RoleSuperAdmin).Save(&entity.Role{
		Name:        entity.RoleSuperAdmin,
		DisplayName: "超级管理员",
		Description: "最高级别的系统管理员，拥有所有权限",
	})
	p.db.Model(&entity.Role{}).Where("name = ?", entity.RoleAdmin).Save(&entity.Role{
		Name:        entity.RoleAdmin,
		DisplayName: "管理员",
		Description: "系统管理员，拥有大部分管理权限",
	})
	p.db.Model(&entity.Role{}).Where("name = ?", entity.RolePlayer).Save(&entity.Role{
		Name:        entity.RolePlayer,
		DisplayName: "玩家",
		Description: "玩家，拥有基本使用权限",
	})
}
