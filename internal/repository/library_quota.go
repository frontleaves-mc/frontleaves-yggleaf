package repository

import (
	"context"
	"errors"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultSkinsPublicTotal  = 1
	defaultSkinsPrivateTotal = 2
	defaultCapesPublicTotal  = 1
	defaultCapesPrivateTotal = 2
)

// LibraryQuotaRepo 资源库配额仓储，负责资源库配额数据访问。
type LibraryQuotaRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewLibraryQuotaRepo 初始化并返回 LibraryQuotaRepo 实例。
func NewLibraryQuotaRepo(db *gorm.DB) *LibraryQuotaRepo {
	return &LibraryQuotaRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "LibraryQuotaRepo"),
	}
}

// GetByUserID 根据用户 ID 查询资源库配额。
func (r *LibraryQuotaRepo) GetByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, forUpdate bool) (*entity.LibraryQuota, bool, *xError.Error) {
	r.log.Info(ctx, "GetByUserID - 根据用户 ID 获取资源库配额")

	query := r.pickDB(ctx, tx).Model(&entity.LibraryQuota{}).Where("user_id = ?", userID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var quota entity.LibraryQuota
	err := query.First(&quota).Error
	if err == nil {
		return &quota, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		newQuota := &entity.LibraryQuota{
			UserID:            userID,
			SkinsPublicTotal:  defaultSkinsPublicTotal,
			SkinsPrivateTotal: defaultSkinsPrivateTotal,
			SkinsPublicUsed:   0,
			SkinsPrivateUsed:  0,
			CapesPublicTotal:  defaultCapesPublicTotal,
			CapesPrivateTotal: defaultCapesPrivateTotal,
			CapesPublicUsed:   0,
			CapesPrivateUsed:  0,
		}
		err = r.pickDB(ctx, tx).Model(&entity.LibraryQuota{}).Create(newQuota).Error
		if err != nil {
			return nil, false, xError.NewError(ctx, xError.DatabaseError, "创建资源库配额失败", true, err)
		}
		return newQuota, true, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询资源库配额失败", true, err)
}

// UpdateSkinsPublicUsed 更新公开皮肤已使用数量。
func (r *LibraryQuotaRepo) UpdateSkinsPublicUsed(ctx context.Context, tx *gorm.DB, quotaID xSnowflake.SnowflakeID, used int32) *xError.Error {
	r.log.Info(ctx, "UpdateSkinsPublicUsed - 更新公开皮肤已使用值")

	if used < 0 {
		return xError.NewError(ctx, xError.ServerInternalError, "配额已用值不能为负数", true)
	}
	if err := r.pickDB(ctx, tx).Model(&entity.LibraryQuota{}).Where("id = ?", quotaID).Update("skins_public_used", used).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "更新公开皮肤配额失败", true, err)
	}
	return nil
}

// UpdateSkinsPrivateUsed 更新私有皮肤已使用数量。
func (r *LibraryQuotaRepo) UpdateSkinsPrivateUsed(ctx context.Context, tx *gorm.DB, quotaID xSnowflake.SnowflakeID, used int32) *xError.Error {
	r.log.Info(ctx, "UpdateSkinsPrivateUsed - 更新私有皮肤已使用值")

	if used < 0 {
		return xError.NewError(ctx, xError.ServerInternalError, "配额已用值不能为负数", true)
	}
	if err := r.pickDB(ctx, tx).Model(&entity.LibraryQuota{}).Where("id = ?", quotaID).Update("skins_private_used", used).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "更新私有皮肤配额失败", true, err)
	}
	return nil
}

// UpdateCapesPublicUsed 更新公开披风已使用数量。
func (r *LibraryQuotaRepo) UpdateCapesPublicUsed(ctx context.Context, tx *gorm.DB, quotaID xSnowflake.SnowflakeID, used int32) *xError.Error {
	r.log.Info(ctx, "UpdateCapesPublicUsed - 更新公开披风已使用值")

	if used < 0 {
		return xError.NewError(ctx, xError.ServerInternalError, "配额已用值不能为负数", true)
	}
	if err := r.pickDB(ctx, tx).Model(&entity.LibraryQuota{}).Where("id = ?", quotaID).Update("capes_public_used", used).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "更新公开披风配额失败", true, err)
	}
	return nil
}

// UpdateCapesPrivateUsed 更新私有披风已使用数量。
func (r *LibraryQuotaRepo) UpdateCapesPrivateUsed(ctx context.Context, tx *gorm.DB, quotaID xSnowflake.SnowflakeID, used int32) *xError.Error {
	r.log.Info(ctx, "UpdateCapesPrivateUsed - 更新私有披风已使用值")

	if used < 0 {
		return xError.NewError(ctx, xError.ServerInternalError, "配额已用值不能为负数", true)
	}
	if err := r.pickDB(ctx, tx).Model(&entity.LibraryQuota{}).Where("id = ?", quotaID).Update("capes_private_used", used).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "更新私有披风配额失败", true, err)
	}
	return nil
}

// UpdateAllUsed 一次性更新配额的全部 4 个 Used 字段，用于配额重算场景。
func (r *LibraryQuotaRepo) UpdateAllUsed(ctx context.Context, tx *gorm.DB, quotaID xSnowflake.SnowflakeID, skinsPubUsed, skinsPriUsed, capesPubUsed, capesPriUsed int32) *xError.Error {
	r.log.Info(ctx, "UpdateAllUsed - 一次性更新配额全部 Used 字段")

	updates := map[string]interface{}{
		"skins_public_used":  skinsPubUsed,
		"skins_private_used": skinsPriUsed,
		"capes_public_used":  capesPubUsed,
		"capes_private_used": capesPriUsed,
	}
	if err := r.pickDB(ctx, tx).Model(&entity.LibraryQuota{}).Where("id = ?", quotaID).Updates(updates).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "更新配额 Used 字段失败", true, err)
	}
	return nil
}

func (r *LibraryQuotaRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
