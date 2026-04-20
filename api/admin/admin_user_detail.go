package admin

import "time"

// AdminUserDetailResponse 管理员用户详情响应。
type AdminUserDetailResponse struct {
	User         AdminUserBasic        `json:"user"`
	GameProfile  *GameProfileQuotaInfo  `json:"game_profile"`
	LibraryQuota *LibraryQuotaInfo     `json:"library_quota"`
	SkinList     []AdminSkinItem       `json:"skin_list"`
	CapeList     []AdminCapeItem       `json:"cape_list"`
}

// AdminUserBasic 用户基本信息（脱敏，不含 game_password）。
type AdminUserBasic struct {
	ID        string     `json:"id"`
	Username  string     `json:"username"`
	Email     *string    `json:"email"`
	Phone     *string    `json:"phone"`
	RoleName  *string    `json:"role_name"`
	HasBan    bool       `json:"has_ban"`
	JailedAt  *time.Time `json:"jailed_at"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// GameProfileQuotaInfo 游戏档案配额信息。
type GameProfileQuotaInfo struct {
	Total int32 `json:"total"`
	Used  int32 `json:"used"`
}

// LibraryQuotaInfo 资源库配额信息（皮肤 + 披风）。
type LibraryQuotaInfo struct {
	SkinsPrivateTotal int32 `json:"skins_private_total"`
	SkinsPublicTotal  int32 `json:"skins_public_total"`
	SkinsPrivateUsed  int32 `json:"skins_private_used"`
	SkinsPublicUsed   int32 `json:"skins_public_used"`
	CapesPrivateTotal int32 `json:"capes_private_total"`
	CapesPublicTotal  int32 `json:"capes_public_total"`
	CapesPrivateUsed  int32 `json:"capes_private_used"`
	CapesPublicUsed   int32 `json:"capes_public_used"`
}

// AdminSkinItem 皮肤条目（含纹理下载链接）。
type AdminSkinItem struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Model      string    `json:"model"`      // "STEVE" 或 "ALEX"
	IsPublic   bool      `json:"is_public"`
	TextureURL string    `json:"texture_url"`
	CreatedAt  time.Time `json:"created_at"`
}

// AdminCapeItem 披风条目（含纹理下载链接）.
type AdminCapeItem struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	IsPublic   bool      `json:"is_public"`
	TextureURL string    `json:"texture_url"`
	CreatedAt  time.Time `json:"created_at"`
}
