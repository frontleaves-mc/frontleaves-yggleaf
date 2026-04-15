// Package txn 提供事务协调层。
//
// GameTokenTxnRepo 封装涉及游戏令牌的多步写入事务场景，
// 保证令牌状态变更与创建操作的原子性。
package txn

import (
	"context"
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	"gorm.io/gorm"
)

// GameTokenTxnRepo 游戏令牌事务协调仓储。
//
// 封装涉及游戏令牌状态变更与新令牌创建的原子操作，
// 保证吊销旧令牌与颁发新令牌的一致性，以及配额检查与令牌创建的原子性。
//
// 所有事务的开启、提交和回滚均在此层完成，
// 上层 Logic 无需感知事务细节。
type GameTokenTxnRepo struct {
	db            *gorm.DB                    // GORM 数据库实例（用于开启事务）
	log           *xLog.LogNamedLogger        // 日志实例
	gameTokenRepo *repository.GameTokenRepo // 游戏令牌仓储
}

// NewGameTokenTxnRepo 初始化并返回 GameTokenTxnRepo 实例。
func NewGameTokenTxnRepo(
	db *gorm.DB,
	gameTokenRepo *repository.GameTokenRepo,
) *GameTokenTxnRepo {
	return &GameTokenTxnRepo{
		db:            db,
		log:           xLog.WithName(xLog.NamedREPO, "GameTokenTxnRepo"),
		gameTokenRepo: gameTokenRepo,
	}
}

// RevokeAndCreate 在事务内完成旧令牌吊销与新令牌创建（可选含角色绑定）。
//
// 该方法执行以下原子操作序列：
//  1. 将旧令牌状态设为 Invalid
//  2. 创建新令牌记录
//  3. （可选）若提供 boundProfileID，将新令牌绑定到指定角色
//
// 任一步骤失败将触发整体回滚（旧令牌恢复 Valid 状态）。
// 专供 RefreshToken（以旧换新）场景使用，不触发配额检查。
//
// 参数:
//   - ctx: 标准库上下文对象
//   - oldTokenID: 待吊销的旧令牌 ID
//   - newToken: 待创建的新令牌实体指针（需已填充所有字段）
//   - boundProfileID: 可选，待绑定到新令牌的游戏档案 ID（nil 则不执行绑定）
//
// 返回值:
//   - *entity.GameToken: 创建成功的新令牌实体
//   - *xError.Error: 操作过程中的错误
func (t *GameTokenTxnRepo) RevokeAndCreate(
	ctx context.Context,
	oldTokenID xSnowflake.SnowflakeID,
	newToken *entity.GameToken,
	boundProfileID *xSnowflake.SnowflakeID,
) (*entity.GameToken, *xError.Error) {
	t.log.Info(ctx, "RevokeAndCreate - 事务内吊销旧令牌并创建新令牌")

	var createdToken *entity.GameToken
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 条件性吊销旧令牌（仅 Valid 或 TempInvalid 状态可被吊销）
		// 并发控制：如果 RowsAffected == 0，说明令牌已被其他并发请求吊销
		rowsAffected, revokeErr := t.gameTokenRepo.RevokeValidOrTempInvalid(ctx, tx, oldTokenID)
		if revokeErr != nil {
			bizErr = revokeErr
			return revokeErr
		}
		if rowsAffected == 0 {
			bizErr = xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
			return bizErr
		}

		// 2. 创建新令牌
		createdToken, bizErr = t.gameTokenRepo.Create(ctx, tx, newToken)
		if bizErr != nil {
			return bizErr
		}

		// 3. （可选）绑定角色到新令牌
		if boundProfileID != nil {
			_, bindErr := t.gameTokenRepo.UpdateBoundProfile(ctx, tx, createdToken.ID, boundProfileID)
			if bindErr != nil {
				bizErr = bindErr
				return bindErr
			}
		}

		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "刷新令牌事务失败", true, err)
	}
	return createdToken, nil
}

// CreateWithQuotaCheck 在事务内完成配额检查、超额吊销及令牌创建。
//
// 该方法执行以下原子操作序列：
//  1. 查询用户有效令牌数量
//  2. 若达到上限，吊销最早的有效令牌
//  3. 创建新令牌记录
//
// 任一步骤失败将触发整体回滚。防止并发场景下令牌数量超出上限。
// 专供 CreateGameToken（新登录/首次创建）场景使用。
//
// 参数:
//   - ctx: 标准库上下文对象
//   - userID: 用户 Snowflake ID
//   - token: 待创建的令牌实体指针（需已填充 AccessToken、ClientToken、UserID、Status、IssuedAt、ExpiresAt）
//   - maxPerUser: 每用户最大有效令牌数
//
// 返回值:
//   - *entity.GameToken: 创建成功的令牌实体
//   - *xError.Error: 操作过程中的错误
func (t *GameTokenTxnRepo) CreateWithQuotaCheck(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	token *entity.GameToken,
	maxPerUser int,
) (*entity.GameToken, *xError.Error) {
	t.log.Info(ctx, "CreateWithQuotaCheck - 事务内配额检查并创建令牌")

	var createdToken *entity.GameToken
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 查询用户有效令牌数量
		var count int64
		countErr := tx.Model(&entity.GameToken{}).
			Where("user_id = ? AND status = ? AND expires_at > ?", userID, entity.GameTokenStatusValid, time.Now()).
			Count(&count).Error
		if countErr != nil {
			bizErr = xError.NewError(ctx, xError.DatabaseError, "查询有效令牌数量失败", true, countErr)
			return bizErr
		}

		// 2. 若达到上限，吊销最早的令牌
		if int(count) >= maxPerUser {
			revokeErr := t.gameTokenRepo.RevokeOldestByUserID(ctx, tx, userID)
			if revokeErr != nil {
				bizErr = revokeErr
				return revokeErr
			}
		}

		// 3. 创建新令牌
		createdToken, bizErr = t.gameTokenRepo.Create(ctx, tx, token)
		if bizErr != nil {
			return bizErr
		}

		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建游戏令牌事务失败", true, err)
	}
	return createdToken, nil
}
