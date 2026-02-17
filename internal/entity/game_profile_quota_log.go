package entity

import (
	"fmt"

	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"gorm.io/gorm"
)

type GameProfileQuotaLog struct {
	xModels.BaseEntity
	UserID         xSnowflake.SnowflakeID  `gorm:"not null;index:idx_game_profile_quota_log_user_id;comment:关联用户ID" json:"user_id"`
	OpType         entityType.ObType       `gorm:"not null;type:varchar(32);index:idx_game_profile_quota_log_op_type;comment:操作类型" json:"op_type"`
	Delta          int32                   `gorm:"not null;comment:额度变化值(正增负减)" json:"delta"`
	BeforeUsed     int32                   `gorm:"not null;comment:变更前已使用额度" json:"before_used"`
	AfterUsed      int32                   `gorm:"not null;comment:变更后已使用额度" json:"after_used"`
	BeforeTotal    int32                   `gorm:"not null;comment:变更前总额度" json:"before_total"`
	AfterTotal     int32                   `gorm:"not null;comment:变更后总额度" json:"after_total"`
	IdempotencyKey string                  `gorm:"not null;type:varchar(255);uniqueIndex:uk_game_profile_quota_log_idempotency_key;comment:幂等键" json:"idempotency_key"`
	RefProfileID   *xSnowflake.SnowflakeID `gorm:"type:bigint;index:idx_game_profile_quota_log_ref_profile_id;comment:关联游戏档案ID" json:"ref_profile_id,omitempty"`
	Remark         *string                 `gorm:"type:varchar(255);comment:备注" json:"remark,omitempty"`

	// ----------
	//  外键约束
	// ----------
	User       User         `gorm:"constraint:OnDelete:CASCADE;comment:关联用户" json:"user,omitempty"`
	RefProfile *GameProfile `gorm:"foreignKey:RefProfileID;references:ID;constraint:OnDelete:SET NULL;comment:关联游戏档案" json:"ref_profile,omitempty"`
}

func (_ *GameProfileQuotaLog) GetGene() xSnowflake.Gene {
	return bConst.GeneForGameProfileQuotaLog
}

func (g *GameProfileQuotaLog) BeforeSave(_ *gorm.DB) error {
	if g.OpType.IsValid() {
		return nil
	}
	return fmt.Errorf("invalid ObType: %s", g.OpType)
}
