package txn

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	"gorm.io/gorm"
)

// LibraryTxnRepo 资源库事务协调仓储。
//
// 封装涉及皮肤/披风的增删改及配额变更的多表事务场景，
// 在单个数据库事务内完成资源记录操作与配额扣减/释放，
// 保证操作的原子性。所有事务的开启、提交和回滚均在此层完成，
// 上层 Logic 无需感知事务细节。
//
// 注意：对象存储（Bucket）上传不在事务范围内，由 Logic 层在上传成功后
// 将文件 ID 填入实体再调用本仓储的方法，避免长事务占用数据库连接。
type LibraryTxnRepo struct {
	db        *gorm.DB                     // GORM 数据库实例（用于开启事务）
	log       *xLog.LogNamedLogger          // 日志实例
	skinRepo  *repository.SkinLibraryRepo   // 皮肤库仓储
	capeRepo  *repository.CapeLibraryRepo   // 披风库仓储
	quotaRepo *repository.LibraryQuotaRepo  // 资源库配额仓储
}

// NewLibraryTxnRepo 初始化并返回 LibraryTxnRepo 实例。
//
// 参数说明:
//   - db: GORM 数据库实例，用于开启和管理事务。
//   - skinRepo: 皮肤库仓储实例。
//   - capeRepo: 披风库仓储实例。
//   - quotaRepo: 资源库配额仓储实例。
//
// 返回值:
//   - *LibraryTxnRepo: 初始化完成的事务协调仓储实例指针。
func NewLibraryTxnRepo(
	db *gorm.DB,
	skinRepo *repository.SkinLibraryRepo,
	capeRepo *repository.CapeLibraryRepo,
	quotaRepo *repository.LibraryQuotaRepo,
) *LibraryTxnRepo {
	return &LibraryTxnRepo{
		db:        db,
		log:       xLog.WithName(xLog.NamedREPO, "LibraryTxnRepo"),
		skinRepo:  skinRepo,
		capeRepo:  capeRepo,
		quotaRepo: quotaRepo,
	}
}

// CreateSkinWithQuota 在事务内完成皮肤创建及对应配额扣减。
//
// 该方法执行以下原子操作序列：
//  1. 行锁查询用户资源库配额
//  2. 根据 IsPublic 校验对应配额是否充足（公开配额或私有配额）
//  3. 检查纹理哈希是否已存在（去重）
//  4. 创建皮肤记录
//  5. 更新对应已用配额数量 (+1)
//
// 任一步骤失败将触发整体回滚。调用方需确保 Bucket 上传已完成并将文件 ID 填入 skin.Texture。
//
// 参数:
//   - ctx: 标准库上下文对象。
//   - skin: 待创建的皮肤实体指针，需已填充 UserID、Name、Texture、TextureHash、Model、IsPublic 字段。
//
// 返回值:
//   - *entity.SkinLibrary: 创建成功的皮肤实体指针；若纹理哈希已存在且属于当前用户则直接返回已有记录。
//   - *xError.Error: 业务校验失败或数据库操作错误。
func (t *LibraryTxnRepo) CreateSkinWithQuota(
	ctx context.Context,
	skin *entity.SkinLibrary,
) (*entity.SkinLibrary, *xError.Error) {
	t.log.Info(ctx, "CreateSkinWithQuota - 事务内创建皮肤")

	var createdSkin *entity.SkinLibrary
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 行锁查询配额
		quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, *skin.UserID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}
		// 2. 根据 IsPublic 校验对应配额
		if skin.IsPublic {
			if quota.SkinsPublicUsed >= quota.SkinsPublicTotal {
				bizErr = xError.NewError(ctx, xError.ResourceExhausted, "公开皮肤配额不足", true)
				return bizErr
			}
		} else {
			if quota.SkinsPrivateUsed >= quota.SkinsPrivateTotal {
				bizErr = xError.NewError(ctx, xError.ResourceExhausted, "私有皮肤配额不足", true)
				return bizErr
			}
		}

		// 3. 纹理哈希去重检查
		existingSkin, found, xErr := t.skinRepo.GetByTextureHash(ctx, tx, skin.TextureHash)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if found {
			if existingSkin.UserID != nil && *existingSkin.UserID == *skin.UserID {
				createdSkin = existingSkin
				return nil
			}
			bizErr = xError.NewError(ctx, xError.DataConflict, "该皮肤纹理已存在", true)
			return bizErr
		}

		// 3. 创建皮肤记录
		createdSkin, bizErr = t.skinRepo.Create(ctx, tx, skin)
		if bizErr != nil {
			return bizErr
		}

		// 4. 根据 IsPublic 更新对应配额
		if skin.IsPublic {
			bizErr = t.quotaRepo.UpdateSkinsPublicUsed(ctx, tx, quota.ID, quota.SkinsPublicUsed+1)
		} else {
			bizErr = t.quotaRepo.UpdateSkinsPrivateUsed(ctx, tx, quota.ID, quota.SkinsPrivateUsed+1)
		}
		if bizErr != nil {
			return bizErr
		}

		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建皮肤失败", true, err)
	}
	return createdSkin, nil
}

// UpdateSkinWithQuota 在事务内完成皮肤更新及配额调整（公开/私有互转）。
//
// 当公开状态发生变化时，该方法会自动调整对应的公开/私有配额计数：
//   - 私有→公开：公开配额 +1，私有配额 -1
//   - 公开→私有：私有配额 +1，公开配额 -1
//
// 任一步骤失败将触发整体回滚。
//
// 参数:
//   - ctx: 标准库上下文对象。
//   - userID: 操作者的雪花 ID。
//   - skinID: 目标皮肤的雪花 ID。
//   - newName: 新名称（未变更时传入原值）。
//   - newIsPublic: 新的公开状态。
//   - oldIsPublic: 当前的公开状态（用于判断是否需要调整配额）。
//
// 返回值:
//   - *entity.SkinLibrary: 更新后的皮肤实体指针。
//   - *xError.Error: 业务校验失败或数据库操作错误。
func (t *LibraryTxnRepo) UpdateSkinWithQuota(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	skinID xSnowflake.SnowflakeID,
	newName string,
	newIsPublic bool,
	oldIsPublic bool,
) (*entity.SkinLibrary, *xError.Error) {
	t.log.Info(ctx, "UpdateSkinWithQuota - 事务内更新皮肤")

	var updatedSkin *entity.SkinLibrary
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 公开状态变更时调整配额
		if oldIsPublic != newIsPublic {
			quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, userID, true)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if !found {
				bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
				return bizErr
			}

			if newIsPublic {
				// 私有 → 公开
				if quota.SkinsPublicUsed >= quota.SkinsPublicTotal {
					bizErr = xError.NewError(ctx, xError.ResourceExhausted, "公开皮肤配额不足", true)
					return bizErr
				}
				bizErr = t.quotaRepo.UpdateSkinsPublicUsed(ctx, tx, quota.ID, quota.SkinsPublicUsed+1)
				if bizErr != nil {
					return bizErr
				}
				bizErr = t.quotaRepo.UpdateSkinsPrivateUsed(ctx, tx, quota.ID, quota.SkinsPrivateUsed-1)
				if bizErr != nil {
					return bizErr
				}
			} else {
				// 公开 → 私有
				if quota.SkinsPrivateUsed >= quota.SkinsPrivateTotal {
					bizErr = xError.NewError(ctx, xError.ResourceExhausted, "私有皮肤配额不足", true)
					return bizErr
				}
				bizErr = t.quotaRepo.UpdateSkinsPrivateUsed(ctx, tx, quota.ID, quota.SkinsPrivateUsed+1)
				if bizErr != nil {
					return bizErr
				}
				bizErr = t.quotaRepo.UpdateSkinsPublicUsed(ctx, tx, quota.ID, quota.SkinsPublicUsed-1)
				if bizErr != nil {
					return bizErr
				}
			}
		}

		// 更新皮肤记录
		updatedSkin, bizErr = t.skinRepo.UpdateNameAndIsPublic(ctx, tx, skinID, newName, newIsPublic)
		if bizErr != nil {
			return bizErr
		}

		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新皮肤失败", true, err)
	}
	return updatedSkin, nil
}

// DeleteSkinWithQuota 在事务内完成皮肤删除及对应配额释放。
//
// 根据皮肤的当前公开状态释放对应配额：
//   - 公开皮肤：公开配额 -1
//   - 私有皮肤：私有配额 -1
//
// 任一步骤失败将触发整体回滚。
//
// 参数:
//   - ctx: 标准库上下文对象。
//   - userID: 操作者的雪花 ID。
//   - skinID: 目标皮肤的雪花 ID。
//   - isPublic: 当前皮肤的公开状态。
//
// 返回值:
//   - *xError.Error: 数据库操作过程中发生的错误。
func (t *LibraryTxnRepo) DeleteSkinWithQuota(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	skinID xSnowflake.SnowflakeID,
	isPublic bool,
) *xError.Error {
	t.log.Info(ctx, "DeleteSkinWithQuota - 事务内删除皮肤")

	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 行锁查询并释放配额
		quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, userID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}

		if isPublic {
			bizErr = t.quotaRepo.UpdateSkinsPublicUsed(ctx, tx, quota.ID, quota.SkinsPublicUsed-1)
		} else {
			bizErr = t.quotaRepo.UpdateSkinsPrivateUsed(ctx, tx, quota.ID, quota.SkinsPrivateUsed-1)
		}
		if bizErr != nil {
			return bizErr
		}

		// 2. 删除皮肤记录
		bizErr = t.skinRepo.DeleteByID(ctx, tx, skinID)
		if bizErr != nil {
			return bizErr
		}

		return nil
	})
	if bizErr != nil {
		return bizErr
	}
	if err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "删除皮肤失败", true, err)
	}
	return nil
}

// CreateCapeWithQuota 在事务内完成披风创建及对应配额扣减。
//
// 该方法执行以下原子操作序列：
//  1. 行锁查询用户资源库配额
//  2. 根据 IsPublic 校验对应配额是否充足（公开配额或私有配额）
//  3. 检查纹理哈希是否已存在（去重）
//  4. 创建披风记录
//  5. 更新对应已用配额数量 (+1)
//
// 任一步骤失败将触发整体回滚。调用方需确保 Bucket 上传已完成并将文件 ID 填入 cape.Texture。
//
// 参数:
//   - ctx: 标准库上下文对象。
//   - cape: 待创建的披风实体指针，需已填充 UserID、Name、Texture、TextureHash、IsPublic 字段。
//
// 返回值:
//   - *entity.CapeLibrary: 创建成功的披风实体指针；若纹理哈希已存在且属于当前用户则直接返回已有记录。
//   - *xError.Error: 业务校验失败或数据库操作错误。
func (t *LibraryTxnRepo) CreateCapeWithQuota(
	ctx context.Context,
	cape *entity.CapeLibrary,
) (*entity.CapeLibrary, *xError.Error) {
	t.log.Info(ctx, "CreateCapeWithQuota - 事务内创建披风")

	var createdCape *entity.CapeLibrary
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 行锁查询配额
		quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, *cape.UserID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}
		// 2. 根据 IsPublic 校验对应配额
		if cape.IsPublic {
			if quota.CapesPublicUsed >= quota.CapesPublicTotal {
				bizErr = xError.NewError(ctx, xError.ResourceExhausted, "公开披风配额不足", true)
				return bizErr
			}
		} else {
			if quota.CapesPrivateUsed >= quota.CapesPrivateTotal {
				bizErr = xError.NewError(ctx, xError.ResourceExhausted, "私有披风配额不足", true)
				return bizErr
			}
		}

		// 3. 纹理哈希去重检查
		existingCape, found, xErr := t.capeRepo.GetByTextureHash(ctx, tx, cape.TextureHash)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if found {
			if existingCape.UserID != nil && *existingCape.UserID == *cape.UserID {
				createdCape = existingCape
				return nil
			}
			bizErr = xError.NewError(ctx, xError.DataConflict, "该披风纹理已存在", true)
			return bizErr
		}

		// 3. 创建披风记录
		createdCape, bizErr = t.capeRepo.Create(ctx, tx, cape)
		if bizErr != nil {
			return bizErr
		}

		// 4. 根据 IsPublic 更新对应配额
		if cape.IsPublic {
			bizErr = t.quotaRepo.UpdateCapesPublicUsed(ctx, tx, quota.ID, quota.CapesPublicUsed+1)
		} else {
			bizErr = t.quotaRepo.UpdateCapesPrivateUsed(ctx, tx, quota.ID, quota.CapesPrivateUsed+1)
		}
		if bizErr != nil {
			return bizErr
		}

		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建披风失败", true, err)
	}
	return createdCape, nil
}

// UpdateCapeWithQuota 在事务内完成披风更新及配额调整（公开/私有互转）。
//
// 当公开状态发生变化时，该方法会自动调整对应的公开/私有配额计数：
//   - 私有→公开：公开配额 +1，私有配额 -1
//   - 公开→私有：私有配额 +1，公开配额 -1
//
// 任一步骤失败将触发整体回滚。
//
// 参数:
//   - ctx: 标准库上下文对象。
//   - userID: 操作者的雪花 ID。
//   - capeID: 目标披风的雪花 ID。
//   - newName: 新名称（未变更时传入原值）。
//   - newIsPublic: 新的公开状态。
//   - oldIsPublic: 当前的公开状态（用于判断是否需要调整配额）。
//
// 返回值:
//   - *entity.CapeLibrary: 更新后的披风实体指针。
//   - *xError.Error: 业务校验失败或数据库操作错误。
func (t *LibraryTxnRepo) UpdateCapeWithQuota(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	capeID xSnowflake.SnowflakeID,
	newName string,
	newIsPublic bool,
	oldIsPublic bool,
) (*entity.CapeLibrary, *xError.Error) {
	t.log.Info(ctx, "UpdateCapeWithQuota - 事务内更新披风")

	var updatedCape *entity.CapeLibrary
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 公开状态变更时调整配额
		if oldIsPublic != newIsPublic {
			quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, userID, true)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if !found {
				bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
				return bizErr
			}

			if newIsPublic {
				// 私有 → 公开
				if quota.CapesPublicUsed >= quota.CapesPublicTotal {
					bizErr = xError.NewError(ctx, xError.ResourceExhausted, "公开披风配额不足", true)
					return bizErr
				}
				bizErr = t.quotaRepo.UpdateCapesPublicUsed(ctx, tx, quota.ID, quota.CapesPublicUsed+1)
				if bizErr != nil {
					return bizErr
				}
				bizErr = t.quotaRepo.UpdateCapesPrivateUsed(ctx, tx, quota.ID, quota.CapesPrivateUsed-1)
				if bizErr != nil {
					return bizErr
				}
			} else {
				// 公开 → 私有
				if quota.CapesPrivateUsed >= quota.CapesPrivateTotal {
					bizErr = xError.NewError(ctx, xError.ResourceExhausted, "私有披风配额不足", true)
					return bizErr
				}
				bizErr = t.quotaRepo.UpdateCapesPrivateUsed(ctx, tx, quota.ID, quota.CapesPrivateUsed+1)
				if bizErr != nil {
					return bizErr
				}
				bizErr = t.quotaRepo.UpdateCapesPublicUsed(ctx, tx, quota.ID, quota.CapesPublicUsed-1)
				if bizErr != nil {
					return bizErr
				}
			}
		}

		// 更新披风记录
		updatedCape, bizErr = t.capeRepo.UpdateNameAndIsPublic(ctx, tx, capeID, newName, newIsPublic)
		if bizErr != nil {
			return bizErr
		}

		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新披风失败", true, err)
	}
	return updatedCape, nil
}

// DeleteCapeWithQuota 在事务内完成披风删除及对应配额释放。
//
// 根据披风的当前公开状态释放对应配额：
//   - 公开披风：公开配额 -1
//   - 私有披风：私有配额 -1
//
// 任一步骤失败将触发整体回滚。
//
// 参数:
//   - ctx: 标准库上下文对象。
//   - userID: 操作者的雪花 ID。
//   - capeID: 目标披风的雪花 ID。
//   - isPublic: 当前披风的公开状态。
//
// 返回值:
//   - *xError.Error: 数据库操作过程中发生的错误。
func (t *LibraryTxnRepo) DeleteCapeWithQuota(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	capeID xSnowflake.SnowflakeID,
	isPublic bool,
) *xError.Error {
	t.log.Info(ctx, "DeleteCapeWithQuota - 事务内删除披风")

	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 行锁查询并释放配额
		quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, userID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}

		if isPublic {
			bizErr = t.quotaRepo.UpdateCapesPublicUsed(ctx, tx, quota.ID, quota.CapesPublicUsed-1)
		} else {
			bizErr = t.quotaRepo.UpdateCapesPrivateUsed(ctx, tx, quota.ID, quota.CapesPrivateUsed-1)
		}
		if bizErr != nil {
			return bizErr
		}

		// 2. 删除披风记录
		bizErr = t.capeRepo.DeleteByID(ctx, tx, capeID)
		if bizErr != nil {
			return bizErr
		}

		return nil
	})
	if bizErr != nil {
		return bizErr
	}
	if err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "删除披风失败", true, err)
	}
	return nil
}
