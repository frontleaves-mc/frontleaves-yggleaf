package repository

import (
	"context"
	"errors"
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GameOnlineProfileRepo 正版档案缓存仓储，负责 game_online_profile 表的数据访问。
type GameOnlineProfileRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewGameOnlineProfileRepo 初始化并返回 GameOnlineProfileRepo 实例。
func NewGameOnlineProfileRepo(db *gorm.DB) *GameOnlineProfileRepo {
	return &GameOnlineProfileRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "GameOnlineProfileRepo"),
	}
}

// GetValidByGameProfileID 根据游戏档案 ID 获取未过期的在线档案缓存。
//
// 查询条件：game_profile_id 匹配且 expires_at > NOW()。
// 无论 IsOnline 值如何均返回（非正版用户的 is_online=false 缓存也有效）。
func (r *GameOnlineProfileRepo) GetValidByGameProfileID(ctx context.Context, tx *gorm.DB, profileID xSnowflake.SnowflakeID) (*entity.GameOnlineProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetValidByGameProfileID - 获取未过期的在线档案缓存")

	var onlineProfile entity.GameOnlineProfile
	err := r.pickDB(ctx, tx).
		Where("game_profile_id = ? AND expires_at > ?", profileID, time.Now()).
		First(&onlineProfile).Error
	if err == nil {
		return &onlineProfile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询在线档案缓存失败", true, err)
}

// Upsert 创建或更新在线档案缓存。
//
// 使用 PostgreSQL ON CONFLICT 语义：当 game_profile_id 冲突时更新所有可变字段。
// 冲突目标为唯一索引 uk_online_profile_game_profile_id（game_profile_id 列）。
func (r *GameOnlineProfileRepo) Upsert(ctx context.Context, tx *gorm.DB, onlineProfile *entity.GameOnlineProfile) (*entity.GameOnlineProfile, *xError.Error) {
	r.log.Info(ctx, "Upsert - 创建或更新在线档案缓存")

	err := r.pickDB(ctx, tx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "game_profile_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"online_uuid", "skin_url", "skin_model", "cape_url", "is_online", "expires_at", "updated_at"}),
	}).Create(onlineProfile).Error
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "写入在线档案缓存失败", true, err)
	}
	return onlineProfile, nil
}

// GetValidByGameProfileIDs 根据多个游戏档案 ID 批量获取未过期的在线档案缓存。
//
// 查询条件：game_profile_id IN (ids) 且 expires_at > NOW()。
// 返回以 game_profile_id 为键的映射，方便调用方按档案 ID 快速查找。
func (r *GameOnlineProfileRepo) GetValidByGameProfileIDs(ctx context.Context, tx *gorm.DB, profileIDs []xSnowflake.SnowflakeID) (map[xSnowflake.SnowflakeID]*entity.GameOnlineProfile, *xError.Error) {
	r.log.Info(ctx, "GetValidByGameProfileIDs - 批量获取未过期的在线档案缓存")

	if len(profileIDs) == 0 {
		return make(map[xSnowflake.SnowflakeID]*entity.GameOnlineProfile), nil
	}

	var onlineProfiles []entity.GameOnlineProfile
	err := r.pickDB(ctx, tx).
		Where("game_profile_id IN ? AND expires_at > ?", profileIDs, time.Now()).
		Find(&onlineProfiles).Error
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "批量查询在线档案缓存失败", true, err)
	}

	result := make(map[xSnowflake.SnowflakeID]*entity.GameOnlineProfile, len(onlineProfiles))
	for i := range onlineProfiles {
		result[onlineProfiles[i].GameProfileID] = &onlineProfiles[i]
	}
	return result, nil
}

func (r *GameOnlineProfileRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
