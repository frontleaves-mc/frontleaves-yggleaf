package yggdrasil

// AuthenticateRequest 登录认证请求
type AuthenticateRequest struct {
	Username    string `json:"username" binding:"required,max=320"` // 邮箱或手机号 (RFC 5321 max)
	Password    string `json:"password" binding:"required,max=128"` // 游戏账户密码
	ClientToken string `json:"clientToken"`                         // 客户端令牌标识（可选）
	RequestUser bool   `json:"requestUser"`                         // 是否请求用户信息
	Agent       *Agent `json:"agent"`                               // 客户端代理信息
}

// Agent 客户端代理信息
type Agent struct {
	Name    string `json:"name"`    // 代理名称（如 "Minecraft"）
	Version int    `json:"version"` // 代理版本
}

// RefreshRequest 刷新令牌请求
type RefreshRequest struct {
	AccessToken     string            `json:"accessToken" binding:"required,max=368"` // 当前令牌
	ClientToken     string            `json:"clientToken" binding:"omitempty,max=368"` // 客户端令牌（可选）
	RequestUser     bool              `json:"requestUser"`                           // 是否请求用户信息
	SelectedProfile *ProfileSelection `json:"selectedProfile"`                       // 要选择的角色
}

// ProfileSelection 角色选择信息
type ProfileSelection struct {
	ID   string `json:"id"`   // 角色 UUID（无符号）
	Name string `json:"name"` // 角色名称
}

// ValidateRequest 验证令牌请求
type ValidateRequest struct {
	AccessToken string `json:"accessToken" binding:"required,max=368"` // 访问令牌
	ClientToken string `json:"clientToken" binding:"omitempty,max=368"` // 客户端令牌（可选）
}

// InvalidateRequest 吊销令牌请求
type InvalidateRequest struct {
	AccessToken string `json:"accessToken" binding:"required,max=368"` // 访问令牌
	ClientToken string `json:"clientToken" binding:"omitempty,max=368"` // 客户端令牌（可选）
}

// SignoutRequest 登出请求
type SignoutRequest struct {
	Username string `json:"username" binding:"required,max=320"` // 邮箱或手机号
	Password string `json:"password" binding:"required,max=128"` // 密码
}

// JoinServerRequest 客户端加入服务器请求
type JoinServerRequest struct {
	AccessToken     string `json:"accessToken" binding:"required,max=368"`     // 访问令牌 (UUID 格式 + 余量)
	SelectedProfile string `json:"selectedProfile" binding:"required,max=32"`  // 角色无符号 UUID (固定 32 字符)
	ServerID        string `json:"serverId" binding:"required,max=256"`        // 服务端随机标识
}
