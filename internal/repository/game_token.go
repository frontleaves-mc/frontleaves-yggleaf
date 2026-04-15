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
)

// GameTokenRepo Yggdrasil 游戏令牌仓储，负责令牌数据的持久化与查询。
type GameTokenRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewGameTokenRepo 初始化并返回 GameTokenRepo 实例。
func NewGameTokenRepo(db *gorm.DB) *GameTokenRepo {
	return &GameTokenRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "GameTokenRepo"),
	}
}

// Create 创建游戏令牌记录。
func (r *GameTokenRepo) Create(ctx context.Context, tx *gorm.DB, token *entity.GameToken) (*entity.GameToken, *xError.Error) {
	r.log.Info(ctx, "Create - 创建游戏令牌")

	if err := r.pickDB(ctx, tx).Create(token).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建游戏令牌失败", true, err)
	}
	return token, nil
}

// GetByAccessToken 根据访问令牌查询游戏令牌记录。
func (r *GameTokenRepo) GetByAccessToken(ctx context.Context, tx *gorm.DB, accessToken string) (*entity.GameToken, bool, *xError.Error) {
	r.log.Info(ctx, "GetByAccessToken - 根据访问令牌获取记录")

	var token entity.GameToken
	err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).Where("access_token = ?", accessToken).First(&token).Error
	if err == nil {
		return &token, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询游戏令牌失败", true, err)
}

// GetByAccessTokenAndClientToken 根据访问令牌与客户端令牌查询游戏令牌记录。
func (r *GameTokenRepo) GetByAccessTokenAndClientToken(ctx context.Context, tx *gorm.DB, accessToken string, clientToken string) (*entity.GameToken, bool, *xError.Error) {
	r.log.Info(ctx, "GetByAccessTokenAndClientToken - 根据访问令牌与客户端令牌获取记录")

	var token entity.GameToken
	err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
		Where("access_token = ? AND client_token = ?", accessToken, clientToken).
		First(&token).Error
	if err == nil {
		return &token, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询游戏令牌与客户端令牌失败", true, err)
}

// GetValidTokensByUserID 根据用户 ID 查询所有有效且未过期的游戏令牌。
func (r *GameTokenRepo) GetValidTokensByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) ([]entity.GameToken, *xError.Error) {
	r.log.Info(ctx, "GetValidTokensByUserID - 根据用户 ID 获取有效游戏令牌列表")

	var tokens []entity.GameToken
	if err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, entity.GameTokenStatusValid, time.Now()).
		Order("created_at ASC").
		Find(&tokens).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "查询用户有效游戏令牌列表失败", true, err)
	}
	return tokens, nil
}

// UpdateStatus 更新指定游戏令牌的状态。
func (r *GameTokenRepo) UpdateStatus(ctx context.Context, tx *gorm.DB, tokenID xSnowflake.SnowflakeID, status entity.GameTokenStatus) (*entity.GameToken, *xError.Error) {
	r.log.Info(ctx, "UpdateStatus - 更新游戏令牌状态")

	if err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).Where("id = ?", tokenID).Update("status", status).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新游戏令牌状态失败", true, err)
	}

	updatedToken, found, xErr := r.GetByID(ctx, tx, tokenID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "游戏令牌不存在", true)
	}
	return updatedToken, nil
}

// RevokeValidOrTempInvalid 条件性吊销令牌（仅当状态为 Valid 或 TempInvalid 时生效）。
//
// 使用 WHERE status IN (?, ?) 条件更新，通过 RowsAffected 判断是否成功吊销了
// 有效/暂时失效状态的令牌。用于解决 TOCTOU 竞态条件：并发刷新请求中，
// 只有第一个请求能成功吊销原令牌，后续请求因 RowsAffected==0 而失败。
//
// 参数:
//   - ctx: 上下文对象
//   - tx: GORM 事务实例（非 nil 表示在事务内执行）
//   - tokenID: 待吊销的令牌 ID
//
// 返回值:
//   - int64: 受影响的行数（1=成功吊销, 0=令牌已不在有效状态）
//   - *xError.Error: 数据库操作异常
func (r *GameTokenRepo) RevokeValidOrTempInvalid(ctx context.Context, tx *gorm.DB, tokenID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "RevokeValidOrTempInvalid - 条件性吊销有效/暂时失效令牌")

	result := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
		Where("id = ? AND status IN (?, ?) AND expires_at > ?", tokenID, entity.GameTokenStatusValid, entity.GameTokenStatusTempInvalid, time.Now()).
		Update("status", entity.GameTokenStatusInvalid)
	if result.Error != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "条件性吊销令牌失败", true, result.Error)
	}
	return result.RowsAffected, nil
}

// InvalidateByAccessToken 根据 accessToken 条件性吊销有效且未过期的游戏令牌。
//
// 使用单条原子 UPDATE 操作，避免 SELECT + UPDATE 之间的 TOCTOU 竞态窗口。
// 仅当令牌状态为 Valid 且未过期时才会被吊销。
//
// 参数:
//   - ctx: 上下文对象
//   - tx: GORM 事务实例（非 nil 表示在事务内执行）
//   - accessToken: 待吊销的访问令牌
//
// 返回值:
//   - int64: 受影响的行数（1=成功吊销, 0=令牌不存在或已无效/已过期）
//   - *xError.Error: 数据库操作异常
func (r *GameTokenRepo) InvalidateByAccessToken(ctx context.Context, tx *gorm.DB, accessToken string) (int64, *xError.Error) {
	r.log.Info(ctx, "InvalidateByAccessToken - 条件性吊销指定令牌")

	result := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
		Where("access_token = ? AND status = ? AND expires_at > ?", accessToken, entity.GameTokenStatusValid, time.Now()).
		Update("status", entity.GameTokenStatusInvalid)
	if result.Error != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "条件性吊销令牌失败", true, result.Error)
	}
	return result.RowsAffected, nil
}

// CountValidByUserID 统计指定用户的有效且未过期的游戏令牌数量。
//
// 用于配额管理（CreateWithQuotaCheck）和 Signout 二次扫描兜底等场景。
//
// 参数:
//   - ctx: 上下文对象
//   - tx: GORM 事务实例（非 nil 表示在事务内执行）
//   - userID: 用户 Snowflake ID
//
// 返回值:
//   - int64: 有效且未过期令牌数量
//   - *xError.Error: 数据库操作异常
func (r *GameTokenRepo) CountValidByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountValidByUserID - 统计用户有效游戏令牌数量")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, entity.GameTokenStatusValid, time.Now()).
		Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户有效游戏令牌失败", true, err)
	}
	return count, nil
}

// UpdateBoundProfile 更新指定游戏令牌的绑定游戏档案 ID。
func (r *GameTokenRepo) UpdateBoundProfile(ctx context.Context, tx *gorm.DB, tokenID xSnowflake.SnowflakeID, profileID *xSnowflake.SnowflakeID) (*entity.GameToken, *xError.Error) {
	r.log.Info(ctx, "UpdateBoundProfile - 更新游戏令牌绑定游戏档案")

	if err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).Where("id = ?", tokenID).Update("bound_profile_id", profileID).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新游戏令牌绑定游戏档案失败", true, err)
	}

	updatedToken, found, xErr := r.GetByID(ctx, tx, tokenID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "游戏令牌不存在", true)
	}
	return updatedToken, nil
}

// InvalidateAllByUserID 将指定用户的所有有效且未过期的游戏令牌设为无效状态。
func (r *GameTokenRepo) InvalidateAllByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) *xError.Error {
	r.log.Info(ctx, "InvalidateAllByUserID - 将用户所有有效游戏令牌设为无效")

	result := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, entity.GameTokenStatusValid, time.Now()).
		Update("status", entity.GameTokenStatusInvalid)
	if result.Error != nil {
		return xError.NewError(ctx, xError.DatabaseError, "批量失效用户游戏令牌失败", true, result.Error)
	}
	return nil
}

// RevokeOldestByUserID 将指定用户最早的有效游戏令牌设为无效状态。
//
// 使用单条 UPDATE ... ORDER BY ... LIMIT 1 原子操作，
// 避免 SELECT + UPDATE 两步操作之间的 TOCTOU 竞态窗口。
func (r *GameTokenRepo) RevokeOldestByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) *xError.Error {
	r.log.Info(ctx, "RevokeOldestByUserID - 撤销用户最早的有效游戏令牌")

	result := r.pickDB(ctx, tx).Model(&entity.GameToken{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, entity.GameTokenStatusValid, time.Now()).
		Order("created_at ASC").
		Limit(1).
		Update("status", entity.GameTokenStatusInvalid)
	if result.Error != nil {
		return xError.NewError(ctx, xError.DatabaseError, "撤销最早游戏令牌失败", true, result.Error)
	}
	// RowsAffected == 0 表示该用户无有效令牌，属于正常情况
	return nil
}

// GetByID 根据游戏令牌 ID 查询游戏令牌记录。
func (r *GameTokenRepo) GetByID(ctx context.Context, tx *gorm.DB, tokenID xSnowflake.SnowflakeID) (*entity.GameToken, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据游戏令牌 ID 获取记录")

	var token entity.GameToken
	err := r.pickDB(ctx, tx).Model(&entity.GameToken{}).Where("id = ?", tokenID).First(&token).Error
	if err == nil {
		return &token, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询游戏令牌失败", true, err)
}

func (r *GameTokenRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
