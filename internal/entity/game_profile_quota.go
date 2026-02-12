package entity

import (
	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
)

// GameProfileQuota 用户游戏档案配额实体，记录用户当前总额度与已使用额度。
type GameProfileQuota struct {
	xModels.BaseEntity                        // 嵌入基础实体字段
	UserID             xSnowflake.SnowflakeID `gorm:"not null;uniqueIndex:uk_game_profile_quota_user_id;comment:关联用户ID" json:"user_id"` // 关联用户ID
	Total              int32                  `gorm:"not null;default:0;comment:总额度" json:"total"`                                      // 总额度
	Used               int32                  `gorm:"not null;default:0;comment:已使用额度" json:"used"`                                     // 已使用额度

	// ----------
	//  外键约束
	// ----------
	User User `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"` // 关联用户
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *GameProfileQuota) GetGene() xSnowflake.Gene {
	return bConst.GeneForGameProfileQuota
}

// GameProfileQuotaLog 用户游戏档案配额日志实体，记录每次额度变化快照。
type GameProfileQuotaLog struct {
	xModels.BaseEntity                         // 嵌入基础实体字段
	UserID             xSnowflake.SnowflakeID  `gorm:"not null;index:idx_game_profile_quota_log_user_id;comment:关联用户ID" json:"user_id"`                                    // 关联用户ID
	OpType             string                  `gorm:"not null;type:varchar(32);index:idx_game_profile_quota_log_op_type;comment:操作类型" json:"op_type"`                     // 操作类型
	Delta              int32                   `gorm:"not null;comment:额度变化值(正增负减)" json:"delta"`                                                                          // 额度变化值(正增负减)
	BeforeUsed         int32                   `gorm:"not null;comment:变更前已使用额度" json:"before_used"`                                                                       // 变更前已使用额度
	AfterUsed          int32                   `gorm:"not null;comment:变更后已使用额度" json:"after_used"`                                                                        // 变更后已使用额度
	BeforeTotal        int32                   `gorm:"not null;comment:变更前总额度" json:"before_total"`                                                                        // 变更前总额度
	AfterTotal         int32                   `gorm:"not null;comment:变更后总额度" json:"after_total"`                                                                         // 变更后总额度
	IdempotencyKey     string                  `gorm:"not null;type:varchar(64);uniqueIndex:uk_game_profile_quota_log_idempotency_key;comment:幂等键" json:"idempotency_key"` // 幂等键
	RefProfileID       *xSnowflake.SnowflakeID `gorm:"type:bigint;index:idx_game_profile_quota_log_ref_profile_id;comment:关联游戏档案ID" json:"ref_profile_id,omitempty"`       // 关联游戏档案ID
	Remark             *string                 `gorm:"type:varchar(255);comment:备注" json:"remark,omitempty"`                                                               // 备注

	// ----------
	//  外键约束
	// ----------
	User       User         `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"`                                                 // 关联用户
	RefProfile *GameProfile `gorm:"foreignKey:RefProfileID;references:ID;constraint:OnDelete:SET NULL;comment:关联游戏档案" json:"ref_profile,omitempty"` // 关联游戏档案
}

// GetGene 返回 xSnowflake.Gene，用于标识该实体在 ID 生成时使用的基因类型。
func (_ *GameProfileQuotaLog) GetGene() xSnowflake.Gene {
	return bConst.GeneForGameProfileQuotaLog
}
