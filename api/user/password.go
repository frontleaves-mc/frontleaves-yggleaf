package user

// UpdateGamePasswordRequest 更新游戏密码请求。
//
// 已通过 OAuth2 AT 认证的用户可直接设置/重置游戏密码，
// 无需提供旧密码。前端需保证两次输入一致。
type UpdateGamePasswordRequest struct {
	NewPassword     string `json:"new_password" binding:"required,min=6,max=128"`     // 新游戏密码（6-128 字符）
	ConfirmPassword string `json:"confirm_password" binding:"required,min=6,max=128"` // 确认新密码（需与 new_password 一致）
}
