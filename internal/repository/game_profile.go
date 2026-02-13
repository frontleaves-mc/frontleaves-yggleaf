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

// GameProfileRepo 游戏档案仓储，负责游戏档案数据访问。
type GameProfileRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewGameProfileRepo 初始化并返回 GameProfileRepo 实例。
func NewGameProfileRepo(db *gorm.DB) *GameProfileRepo {
	return &GameProfileRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "GameProfileRepo"),
	}
}

// Create 创建游戏档案记录。
func (r *GameProfileRepo) Create(ctx context.Context, tx *gorm.DB, profile *entity.GameProfile) (*entity.GameProfile, *xError.Error) {
	r.log.Info(ctx, "Create - 创建游戏档案")

	if err := r.pickDB(ctx, tx).Create(profile).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建游戏档案失败", true, err)
	}
	return profile, nil
}

// ExistsByUUID 检查指定 UUID 是否已存在。
func (r *GameProfileRepo) ExistsByUUID(ctx context.Context, tx *gorm.DB, uuid string) (bool, *xError.Error) {
	r.log.Info(ctx, "ExistsByUUID - 检查 UUID 是否存在")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("uuid = ?", uuid).Count(&count).Error; err != nil {
		return false, xError.NewError(ctx, xError.DatabaseError, "查询游戏档案 UUID 失败", true, err)
	}
	return count > 0, nil
}

// ExistsByNameExceptID 检查用户名是否存在（排除指定档案 ID）。
func (r *GameProfileRepo) ExistsByNameExceptID(ctx context.Context, tx *gorm.DB, name string, profileID xSnowflake.SnowflakeID) (bool, *xError.Error) {
	r.log.Info(ctx, "ExistsByNameExceptID - 检查用户名是否存在")

	var count int64
	query := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("name = ?", name)
	if !profileID.IsZero() {
		query = query.Where("id <> ?", profileID)
	}
	if err := query.Count(&count).Error; err != nil {
		return false, xError.NewError(ctx, xError.DatabaseError, "查询游戏档案名称失败", true, err)
	}
	return count > 0, nil
}

// GetByIDAndUserID 根据档案 ID 与用户 ID 查询游戏档案。
func (r *GameProfileRepo) GetByIDAndUserID(ctx context.Context, tx *gorm.DB, profileID xSnowflake.SnowflakeID, userID xSnowflake.SnowflakeID, forUpdate bool) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByIDAndUserID - 根据档案 ID 与用户 ID 获取游戏档案")

	query := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("id = ? AND user_id = ?", profileID, userID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var profile entity.GameProfile
	err := query.First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询游戏档案失败", true, err)
}

// GetByID 根据档案 ID 查询游戏档案。
func (r *GameProfileRepo) GetByID(ctx context.Context, tx *gorm.DB, profileID xSnowflake.SnowflakeID) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据档案 ID 获取游戏档案")

	var profile entity.GameProfile
	err := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("id = ?", profileID).First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询游戏档案失败", true, err)
}

// UpdateName 更新指定档案的用户名。
func (r *GameProfileRepo) UpdateName(ctx context.Context, tx *gorm.DB, profileID xSnowflake.SnowflakeID, name string) (*entity.GameProfile, *xError.Error) {
	r.log.Info(ctx, "UpdateName - 更新游戏档案用户名")

	if err := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("id = ?", profileID).Update("name", name).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新游戏档案名称失败", true, err)
	}

	updatedProfile, found, xErr := r.GetByID(ctx, tx, profileID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "游戏档案不存在", true)
	}
	return updatedProfile, nil
}

func (r *GameProfileRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
