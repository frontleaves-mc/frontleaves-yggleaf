package entity

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// GameOnlineProfile 正版档案缓存实体，存储从 Mojang API 获取的在线皮肤/披风信息。
//
// 当本平台用户未设置皮肤或披风时，系统会从 Mojang 正版平台获取该用户名下的
// 皮肤/披风信息作为回退。该实体将 Mojang API 返回的数据进行 DB 级缓存，
// 避免频繁调用受速率限制的 Mojang API。
//
// 缓存策略:
//   - 有效期 30 分钟（ExpiresAt 字段），过期后下次请求重新获取
//   - 非正版用户也会缓存（IsOnline = false），避免反复查询 Mojang API
//   - 与 GameProfile 一对一关系，GameProfile 删除时级联删除
type GameOnlineProfile struct {
	xModels.BaseEntity // 嵌入基础实体字段
	GameProfileID      xSnowflake.SnowflakeID `gorm:"not null;uniqueIndex:uk_online_profile_game_profile_id;comment:关联游戏档案ID" json:"game_profile_id"`                // 关联游戏档案ID
	OnlineUUID         *string                `gorm:"type:varchar(32);comment:正版无连字符UUID" json:"online_uuid,omitempty"`                                                          // Mojang 正版 UUID（无连字符，32位）
	SkinURL            *string                `gorm:"type:varchar(512);comment:正版皮肤URL" json:"skin_url,omitempty"`                                                                // Mojang 皮肤下载链接
	SkinModel          *ModelType             `gorm:"type:smallint;comment:皮肤模型(1=classic,2=slim)" json:"skin_model,omitempty"`                                                   // 皮肤模型类型
	CapeURL            *string                `gorm:"type:varchar(512);comment:正版披风URL" json:"cape_url,omitempty"`                                                                // Mojang 披风下载链接
	IsOnline           bool                   `gorm:"not null;type:boolean;default:false;comment:是否为正版用户" json:"is_online"`                                                       // 是否为正版用户（非正版也缓存，避免重复查询）
	ExpiresAt          time.Time              `gorm:"not null;type:timestamptz;index:idx_online_profile_expires_at;comment:缓存过期时间" json:"expires_at"`                               // 缓存过期时间

	// ----------
	//  外键约束
	// ----------
	GameProfile *GameProfile `gorm:"constraint:OnDelete:CASCADE;comment:关联游戏档案" json:"game_profile,omitempty"` // 关联游戏档案
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *GameOnlineProfile) GetGene() xSnowflake.Gene {
	return bConst.GeneForGameOnlineProfile
}
