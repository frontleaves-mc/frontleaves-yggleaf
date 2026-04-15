package yggdrasil

// MetadataResponse API 元数据响应
type MetadataResponse struct {
	Meta               MetadataMeta  `json:"meta"`               // 元数据信息
	SkinDomains        []string      `json:"skinDomains"`        // 皮肤域名白名单
	SignaturePublickey string        `json:"signaturePublickey"` // RSA 签名公钥（PEM 格式）
}

// MetadataMeta API 元数据中的 meta 字段
type MetadataMeta struct {
	ServerName            string        `json:"serverName"`                      // 服务端名称
	ImplementationName    string        `json:"implementationName"`              // 实现名称
	ImplementationVersion string        `json:"implementationVersion"`           // 实现版本
	Links                 MetadataLinks `json:"links,omitempty"`                 // 相关链接（可选）
	FeatureNonEmailLogin  bool          `json:"feature.non_email_login"`         // 是否支持非邮箱登录
}

// MetadataLinks 元数据中的链接信息
type MetadataLinks struct {
	Homepage string `json:"homepage"` // 主页地址
	Register string `json:"register"` // 注册地址
}

// AuthenticateResponse 登录认证响应
type AuthenticateResponse struct {
	AccessToken       string            `json:"accessToken"`                  // 服务端生成的访问令牌
	ClientToken       string            `json:"clientToken"`                  // 客户端令牌（与请求一致）
	AvailableProfiles []ProfileResponse `json:"availableProfiles"`            // 用户可用的角色列表
	SelectedProfile   *ProfileResponse  `json:"selectedProfile,omitempty"`    // 自动选中的角色（单角色时）
	User              *UserResponse     `json:"user,omitempty"`               // 用户信息（requestUser 时返回）
}

// RefreshResponse 刷新令牌响应
type RefreshResponse struct {
	AccessToken     string          `json:"accessToken"`                // 新的访问令牌
	ClientToken     string          `json:"clientToken"`                // 客户端令牌（与原令牌一致）
	SelectedProfile *ProfileResponse `json:"selectedProfile,omitempty"` // 选中的角色信息
	User            *UserResponse   `json:"user,omitempty"`             // 用户信息（requestUser 时返回）
}

// ProfileResponse 角色信息响应
type ProfileResponse struct {
	ID         string             `json:"id"`                            // 角色无符号 UUID
	Name       string             `json:"name"`                          // 角色名称
	Properties []PropertyResponse `json:"properties,omitempty"`          // 角色属性列表（含材质签名等）
}

// PropertyResponse 属性响应
type PropertyResponse struct {
	Name      string `json:"name"`                   // 属性名称（如 "textures"）
	Value     string `json:"value"`                  // 属性值（Base64 编码）
	Signature string `json:"signature,omitempty"`    // 属性签名（Base64 编码，可选）
}

// UserResponse 用户信息响应
type UserResponse struct {
	ID         string                `json:"id"`                        // 用户无符号 UUID
	Properties []UserPropertyResponse `json:"properties,omitempty"`     // 用户属性列表
}

// UserPropertyResponse 用户属性响应
type UserPropertyResponse struct {
	Name  string `json:"name"`  // 属性名称（如 "preferredLanguage"）
	Value string `json:"value"` // 属性值
}

// TexturesPayload 材质信息载荷（Base64 编码前的 JSON 结构）
type TexturesPayload struct {
	Timestamp   int64        `json:"timestamp"`              // 时间戳（毫秒）
	ProfileID   string       `json:"profileId"`              // 角色无符号 UUID
	ProfileName string       `json:"profileName"`            // 角色名称
	Textures    TexturesInfo `json:"textures"`               // 材质信息
}

// TexturesInfo 材质信息
type TexturesInfo struct {
	SKIN *SkinTexture `json:"SKIN,omitempty"` // 皮肤材质（可选）
	CAPE *CapeTexture `json:"CAPE,omitempty"` // 披风材质（可选）
}

// SkinTexture 皮肤材质
type SkinTexture struct {
	URL      string        `json:"url"`                // 材质文件 URL
	Metadata *SkinMetadata `json:"metadata,omitempty"` // 皮肤元数据（可选）
}

// SkinMetadata 皮肤元数据
type SkinMetadata struct {
	Model string `json:"model"` // 皮肤模型："slim" 或 "classic"（默认空为 classic）
}

// CapeTexture 披风材质
type CapeTexture struct {
	URL string `json:"url"` // 材质文件 URL
}

// BatchProfileItem 批量查询角色响应项
type BatchProfileItem struct {
	ID   string `json:"id"`   // 角色无符号 UUID
	Name string `json:"name"` // 角色名称
}
