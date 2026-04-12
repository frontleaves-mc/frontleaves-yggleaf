// Package txn 提供事务协调层。
//
// 该包负责封装涉及多表写入的复合业务场景，在单个数据库事务内完成
// 跨 Repository 的原子操作。所有事务的开启（Transaction）、提交和回滚
// 均在此层完成，上层 Logic 层只需调用一个复合方法即可，完全无需感知事务细节。
//
// 分层定位:
//   - 位于 Logic 层与 Repository 层之间
//   - 聚合多个 Repository 实例的原子方法
//   - 内部管理事务边界（begin/commit/rollback）
//   - 对外暴露语义化的复合操作接口
//
// 设计原则:
//   - 事务内不调用外部服务（如 Bucket 上传），避免长事务占用连接
//   - 事务失败时统一回滚，保证数据一致性
//   - 每个方法对应一个完整的业务用例（如"创建皮肤并扣减配额"）
package txn

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	"gorm.io/gorm"
)

// GameProfileTxnRepo 游戏档案事务协调仓储。
//
// 封装涉及多表写入的游戏档案业务场景（如创建游戏档案），
// 在单个数据库事务内完成配额检查、档案创建、配额扣减和日志记录，
// 保证操作的原子性。所有事务的开启、提交和回滚均在此层完成，
// 上层 Logic 无需感知事务细节。
type GameProfileTxnRepo struct {
	db       *gorm.DB                       // GORM 数据库实例（用于开启事务）
	log      *xLog.LogNamedLogger            // 日志实例
	profile  *repository.GameProfileRepo     // 游戏档案仓储
	quota    *repository.GameProfileQuotaRepo // 游戏档案配额仓储
	quotaLog *repository.GameProfileQuotaLogRepo // 游戏档案配额日志仓储
}

// NewGameProfileTxnRepo 初始化并返回 GameProfileTxnRepo 实例。
//
// 参数说明:
//   - db: GORM 数据库实例，用于开启和管理事务。
//   - profile: 游戏档案仓储实例。
//   - quota: 游戏档案配额仓储实例。
//   - quotaLog: 游戏档案配额日志仓储实例。
//
// 返回值:
//   - *GameProfileTxnRepo: 初始化完成的事务协调仓储实例指针。
func NewGameProfileTxnRepo(
	db *gorm.DB,
	profile *repository.GameProfileRepo,
	quota *repository.GameProfileQuotaRepo,
	quotaLog *repository.GameProfileQuotaLogRepo,
) *GameProfileTxnRepo {
	return &GameProfileTxnRepo{
		db:       db,
		log:      xLog.WithName(xLog.NamedREPO, "GameProfileTxnRepo"),
		profile:  profile,
		quota:    quota,
		quotaLog: quotaLog,
	}
}

// AddProfileWithQuota 在事务内完成游戏档案创建及配额扣减。
//
// 该方法执行以下原子操作序列：
//  1. 行锁查询用户配额记录（SELECT ... FOR UPDATE）
//  2. 校验配额余额是否充足
//  3. 校验 UUID 和名称唯一性
//  4. 创建游戏档案记录
//  5. 更新配额已用数量 (+1)
//  6. 写入配额变更日志
//
// 任一步骤失败将触发整体回滚。
//
// 参数:
//   - ctx: 标准库上下文对象，用于传递请求范围的数据、取消信号和截止时间。
//   - profile: 待创建的游戏档案实体指针，需已填充 UserID、UUID、Name 字段。
//
// 返回值:
//   - *entity.GameProfile: 创建成功的游戏档案实体指针。
//   - *xError.Error: 业务校验失败或数据库操作错误。
func (t *GameProfileTxnRepo) AddProfileWithQuota(
	ctx context.Context,
	profile *entity.GameProfile,
) (*entity.GameProfile, *xError.Error) {
	t.log.Info(ctx, "AddProfileWithQuota - 事务内创建游戏档案")

	var createdProfile *entity.GameProfile
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 行锁查询配额
		quota, found, xErr := t.quota.GetByUserID(ctx, tx, profile.UserID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户游戏档案配额不存在", true)
			return bizErr
		}
		if quota.Used >= quota.Total {
			bizErr = xError.NewError(ctx, xError.ResourceExhausted, "游戏档案配额不足", true)
			return bizErr
		}

		// 2. UUID 唯一性检查
		uuidExisted, xErr := t.profile.ExistsByUUID(ctx, tx, profile.UUID.String())
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if uuidExisted {
			bizErr = xError.NewError(ctx, xError.DataConflict, "UUID 已存在", true)
			return bizErr
		}

		// 3. 名称唯一性检查
		nameExisted, xErr := t.profile.ExistsByNameExceptID(ctx, tx, profile.Name, 0)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if nameExisted {
			bizErr = xError.NewError(ctx, xError.DataConflict, "用户名已存在", true)
			return bizErr
		}

		// 4. 创建档案
		createdProfile, xErr = t.profile.Create(ctx, tx, profile)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}

		// 5. 更新配额
		beforeUsed := quota.Used
		afterUsed := quota.Used + 1
		xErr = t.quota.UpdateUsed(ctx, tx, quota.ID, afterUsed)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}

		// 6. 写入日志
		_, xErr = t.quotaLog.Create(
			ctx, tx, profile.UserID,
			entityType.ObTypeAddGameProfile, 1,
			beforeUsed, quota.Total,
			xUtil.Ptr(createdProfile.ID),
			xUtil.Ptr("创建游戏档案"),
		)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}

		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "新增游戏档案失败", true, err)
	}
	return createdProfile, nil
}
