package entity

import (
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// GameTokenStatus 游戏令牌状态类型，用于标识 Yggdrasil 游戏令牌的当前生命周期阶段。
type GameTokenStatus uint8

const (
	GameTokenStatusValid       GameTokenStatus = 1 // 有效 — 令牌可正常使用
	GameTokenStatusTempInvalid GameTokenStatus = 2 // 暂时失效 — 预留给未来 invalidate-then-refresh 语义（当前代码路径不写入此状态，但 RevokeValidOrTempInvalid 的 WHERE 条件已包含此值以兼容未来扩展）
	GameTokenStatusInvalid     GameTokenStatus = 3 // 无效 — 已被吊销，不可恢复
)

// GameToken Yggdrasil 游戏令牌实体，存储外置登录认证所需的令牌信息。
//
// 该实体用于管理 Yggdrasil 协议中的 accessToken / clientToken 生命周期，
// 每个令牌绑定一个用户，可选绑定一个游戏档案（角色）。
// 令牌状态遵循严格的状态机转换：
//
//	有效 (1) ──→ 暂时失效 (2) ──→ 无效 (3)
//	  │                                ↑
//	  └────────────────────────────────┘
//
// 字段说明:
//   - AccessToken: 服务端生成的访问令牌，具有全局唯一性
//   - ClientToken: 客户端提供的令牌标识，用于关联刷新操作
//   - UserID: 令牌所属用户的 Snowflake ID
//   - BoundProfileID: 令牌绑定的游戏档案 ID（可选，多角色时通过 refresh 绑定）
//   - Status: 令牌当前状态（有效/暂时失效/无效）
//   - IssuedAt: 令牌颁发时间
//   - ExpiresAt: 令牌过期时间，过期后需刷新或重新认证
type GameToken struct {
	xModels.BaseEntity                                                                     // 嵌入基础实体字段
	AccessToken    string                   `gorm:"not null;type:varchar(128);uniqueIndex:uk_token_access_token;comment:访问令牌" json:"access_token"`                        // 访问令牌
	ClientToken    string                   `gorm:"not null;type:varchar(128);index:idx_token_client_token;comment:客户端令牌标识" json:"client_token"`                              // 客户端令牌标识
	UserID         xSnowflake.SnowflakeID   `gorm:"not null;index:idx_token_user_id;comment:关联用户ID" json:"user_id"`                                                     // 关联用户ID
	BoundProfileID *xSnowflake.SnowflakeID  `gorm:"type:bigint;index:idx_token_bound_profile_id;comment:绑定游戏档案ID" json:"bound_profile_id,omitempty"`                     // 绑定游戏档案ID
	Status         GameTokenStatus          `gorm:"not null;type:smallint;default:1;index:idx_token_status;comment:令牌状态(1=有效,2=暂时失效,3=无效)" json:"status"`                 // 令牌状态
	IssuedAt       time.Time                `gorm:"not null;type:timestamptz;comment:颁发时间" json:"issued_at"`                                                            // 颁发时间
	ExpiresAt      time.Time                `gorm:"not null;type:timestamptz;comment:过期时间" json:"expires_at"`                                                           // 过期时间

	// ----------
	//  外键约束
	// ----------
	User         *User         `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"`                                                              // 关联用户
	BoundProfile *GameProfile  `gorm:"foreignKey:BoundProfileID;references:ID;constraint:OnDelete:SET NULL;comment:绑定游戏档案" json:"bound_profile,omitempty"` // 绑定游戏档案
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *GameToken) GetGene() xSnowflake.Gene {
	return bConst.GeneForGameToken
}
