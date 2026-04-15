package user

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	apiLibrary "github.com/frontleaves-mc/frontleaves-yggleaf/api/library"
)

// AddGameProfileRequest 创建游戏档案请求
type AddGameProfileRequest struct {
	Name string `json:"name" binding:"required"`
}

// ChangeUsernameRequest 修改用户名请求
type ChangeUsernameRequest struct {
	NewName string `json:"new_name" binding:"required"`
}

// GameProfileResponse 游戏档案响应 DTO。
//
// 不再嵌入 entity.GameProfile，改为显式字段定义。
// 嵌套的 Skin/Cape 使用 library 包的 DTO（已包含 texture_url），
// 不再使用 entity 的原始 Texture int64 字段。
type GameProfileResponse struct {
	ID            xSnowflake.SnowflakeID   `json:"id"`                        // 档案 ID
	UserID        xSnowflake.SnowflakeID   `json:"user_id"`                   // 关联用户 ID
	UUID          string                   `json:"uuid"`                      // UUIDv7 标识
	Name          string                   `json:"name"`                      // 档案用户名
	SkinLibraryID *xSnowflake.SnowflakeID  `json:"skin_library_id,omitempty"` // 装备的皮肤库 ID
	CapeLibraryID *xSnowflake.SnowflakeID  `json:"cape_library_id,omitempty"` // 装备的披风库 ID
	UpdatedAt     time.Time                `json:"updated_at"`                // 更新时间
	Skin          *apiLibrary.SkinResponse `json:"skin,omitempty"`            // 装备的皮肤信息（含 texture_url）
	Cape          *apiLibrary.CapeResponse `json:"cape,omitempty"`            // 装备的披风信息（含 texture_url）
}

// GameProfileListResponse 游戏档案列表响应
type GameProfileListResponse struct {
	Items []GameProfileResponse `json:"items"` // 游戏档案列表
}
