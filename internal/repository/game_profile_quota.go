package repository

import (
	"context"
	"errors"

	xError "github.com/bamboo-services/bamboo-base-go/error"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GameProfileQuotaRepo 游戏档案配额仓储，负责配额数据访问。
type GameProfileQuotaRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewGameProfileQuotaRepo 初始化并返回 GameProfileQuotaRepo 实例。
func NewGameProfileQuotaRepo(db *gorm.DB) *GameProfileQuotaRepo {
	return &GameProfileQuotaRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "GameProfileQuotaRepo"),
	}
}

// GetByUserID 根据用户 ID 查询游戏档案配额。
func (r *GameProfileQuotaRepo) GetByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, forUpdate bool) (*entity.GameProfileQuota, bool, *xError.Error) {
	r.log.Info(ctx, "GetByUserID - 根据用户 ID 获取游戏档案配额")

	query := r.pickDB(ctx, tx).Model(&entity.GameProfileQuota{}).Where("user_id = ?", userID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var quota entity.GameProfileQuota
	err := query.First(&quota).Error
	if err == nil {
		return &quota, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		newQuota := &entity.GameProfileQuota{
			UserID: userID,
			Total:  1,
			Used:   0,
		}
		err = r.pickDB(ctx, tx).Model(&entity.GameProfileQuota{}).Create(newQuota).Error
		if err != nil {
			return nil, false, xError.NewError(ctx, xError.DatabaseError, "创建游戏档案配额失败", true, err)
		}
		return newQuota, true, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询游戏档案配额失败", true, err)
}

// UpdateUsed 更新配额已使用数量。
func (r *GameProfileQuotaRepo) UpdateUsed(ctx context.Context, tx *gorm.DB, quotaID xSnowflake.SnowflakeID, used int32) *xError.Error {
	r.log.Info(ctx, "UpdateUsed - 更新游戏档案配额已使用值")

	if err := r.pickDB(ctx, tx).Model(&entity.GameProfileQuota{}).Where("id = ?", quotaID).Update("used", used).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "更新游戏档案配额失败", true, err)
	}
	return nil
}

func (r *GameProfileQuotaRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
