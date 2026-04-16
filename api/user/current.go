package user

import "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"

// UserExtend 用户信息扩展字段。
//
// 包含账户完善度等计算型数据，不存储于数据库，
// 由 Logic 层在响应构建时动态计算。
type UserExtend struct {
	// AccountReady 账户完善状态。
	//
	//   - "ready":          所有必要信息已填写完毕
	//   - "game_password":  游戏密码未设置（返回缺失字段名）
	AccountReady string `json:"account_ready"` // 账户完善状态
}

// UserCurrentResponse 用户当前信息响应 DTO。
//
// 将 entity.User 包装为嵌套结构，并附带 extend 扩展信息。
// 遵循项目 DTO 模式：Handler 不再直接返回 entity。
type UserCurrentResponse struct {
	User   entity.User `json:"user"`   // 用户实体信息
	Extend UserExtend  `json:"extend"` // 扩展信息（含账户完善状态）
}
