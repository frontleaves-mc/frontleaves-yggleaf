package models

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
)

// GameProfileDTO 游戏档案数据传输对象。
//
// 由 Logic 层构建，包含已解析的纹理链接。
// Handler 层负责将此 DTO 转换为 api/user.GameProfileResponse。
type GameProfileDTO struct {
	ID            xSnowflake.SnowflakeID  // 档案 ID
	UserID        xSnowflake.SnowflakeID  // 关联用户 ID
	UUID          string                  // UUIDv7 标识
	Name          string                  // 档案用户名
	SkinLibraryID *xSnowflake.SnowflakeID // 装备的皮肤库 ID
	CapeLibraryID *xSnowflake.SnowflakeID // 装备的披风库 ID
	UpdatedAt     time.Time               // 更新时间
	Skin          *SkinDTO                // 装备的皮肤信息（含 texture_url）
	Cape          *CapeDTO                // 装备的披风信息（含 texture_url）
}
