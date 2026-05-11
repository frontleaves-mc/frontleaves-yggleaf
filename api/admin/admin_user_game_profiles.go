package admin

import apiUser "github.com/frontleaves-mc/frontleaves-yggleaf/api/user"

// AdminUserGameProfilesResponse 管理员用户游戏档案列表响应。
type AdminUserGameProfilesResponse struct {
	Items []apiUser.GameProfileResponse `json:"items"` // 游戏档案列表
}
