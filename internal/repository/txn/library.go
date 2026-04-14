package txn

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
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
	db           *gorm.DB                       // GORM 数据库实例（用于开启事务）
	log          *xLog.LogNamedLogger            // 日志实例
	skinRepo     *repository.SkinLibraryRepo     // 皮肤库仓储
	capeRepo     *repository.CapeLibraryRepo     // 披风库仓储
	quotaRepo    *repository.LibraryQuotaRepo    // 资源库配额仓储
	userSkinRepo *repository.UserSkinLibraryRepo // 用户皮肤关联仓储
	userCapeRepo *repository.UserCapeLibraryRepo // 用户披风关联仓储
}

// NewLibraryTxnRepo 初始化并返回 LibraryTxnRepo 实例。
func NewLibraryTxnRepo(
	db *gorm.DB,
	skinRepo *repository.SkinLibraryRepo,
	capeRepo *repository.CapeLibraryRepo,
	quotaRepo *repository.LibraryQuotaRepo,
	userSkinRepo *repository.UserSkinLibraryRepo,
	userCapeRepo *repository.UserCapeLibraryRepo,
) *LibraryTxnRepo {
	return &LibraryTxnRepo{
		db:           db,
		log:          xLog.WithName(xLog.NamedREPO, "LibraryTxnRepo"),
		skinRepo:     skinRepo,
		capeRepo:     capeRepo,
		quotaRepo:    quotaRepo,
		userSkinRepo: userSkinRepo,
		userCapeRepo: userCapeRepo,
	}
}

// CreateSkinWithQuota 在事务内完成皮肤创建、用户关联及对应配额扣减。
//
// 该方法执行以下原子操作序列：
//  1. 行锁查询用户资源库配额
//  2. 根据 IsPublic 校验对应配额是否充足（公开配额或私有配额）
//  3. 检查纹理哈希是否已存在（去重）
//  4. 创建皮肤记录
//  5. 创建用户皮肤关联记录（AssignmentType=Normal）
//  6. 更新对应已用配额数量 (+1)
//
// 任一步骤失败将触发整体回滚。调用方需确保 Bucket 上传已完成并将文件 ID 填入 skin.Texture。
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
				// 幂等：检查是否已有关联记录
				_, assocFound, xErr := t.userSkinRepo.GetByUserAndSkin(ctx, tx, *skin.UserID, existingSkin.ID)
				if xErr != nil {
					bizErr = xErr
					return xErr
				}
				if !assocFound {
					// 创建关联记录但不扣配额（皮肤记录已存在，配额在首次创建时已扣减）
					_, bizErr = t.userSkinRepo.Create(ctx, tx, &entity.UserSkinLibrary{
						UserID:         *skin.UserID,
						SkinLibraryID:  existingSkin.ID,
						AssignmentType: entityType.AssignmentTypeNormal,
					})
					if bizErr != nil {
						return bizErr
					}
				}
				return nil
			}
			bizErr = xError.NewError(ctx, xError.DataConflict, "该皮肤纹理已存在", true)
			return bizErr
		}

		// 4. 创建皮肤记录
		createdSkin, bizErr = t.skinRepo.Create(ctx, tx, skin)
		if bizErr != nil {
			return bizErr
		}

		// 5. 创建用户皮肤关联记录（Normal 类型）
		_, bizErr = t.userSkinRepo.Create(ctx, tx, &entity.UserSkinLibrary{
			UserID:        *skin.UserID,
			SkinLibraryID: createdSkin.ID,
			AssignmentType: entityType.AssignmentTypeNormal,
		})
		if bizErr != nil {
			return bizErr
		}

		// 6. 根据 IsPublic 更新对应配额
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
		// 校验用户关联类型，只有 Normal 类型允许调整配额
		association, found, xErr := t.userSkinRepo.GetByUserAndSkin(ctx, tx, userID, skinID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户未关联该皮肤", true)
			return bizErr
		}
		if !association.AssignmentType.CountsTowardQuota() {
			bizErr = xError.NewError(ctx, xError.PermissionDenied, "赠送的资源不能修改公开状态", true)
			return bizErr
		}

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

// DeleteSkinWithQuota 在事务内完成用户皮肤关联删除、配额释放及按需清理皮肤记录。
//
// 新事务序列：
//  1. 查找 UserSkinLibrary 关联记录
//  2. 若 AssignmentType 为 Normal 则释放对应配额（事务内读取权威 IsPublic）
//  3. 删除 UserSkinLibrary 关联记录
//  4. 检查 SkinLibrary 引用计数，零引用且为当前用户创建的资源则删除
func (t *LibraryTxnRepo) DeleteSkinWithQuota(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	skinID xSnowflake.SnowflakeID,
) *xError.Error {
	t.log.Info(ctx, "DeleteSkinWithQuota - 事务内删除皮肤")

	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 查找用户皮肤关联记录
		association, found, xErr := t.userSkinRepo.GetByUserAndSkin(ctx, tx, userID, skinID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户未关联该皮肤", true)
			return bizErr
		}

		// 2. Normal 类型释放配额（事务内读取权威 IsPublic）
		if association.AssignmentType.CountsTowardQuota() {
			// 在事务内获取权威的皮肤记录，避免 TOCTOU 竞态
			skinRec, skinFound, xErr := t.skinRepo.GetByID(ctx, tx, skinID)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if !skinFound {
				bizErr = xError.NewError(ctx, xError.ResourceNotFound, "皮肤库记录不存在", true)
				return bizErr
			}

			quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, userID, true)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if !found {
				bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
				return bizErr
			}
			if skinRec.IsPublic {
				bizErr = t.quotaRepo.UpdateSkinsPublicUsed(ctx, tx, quota.ID, quota.SkinsPublicUsed-1)
			} else {
				bizErr = t.quotaRepo.UpdateSkinsPrivateUsed(ctx, tx, quota.ID, quota.SkinsPrivateUsed-1)
			}
			if bizErr != nil {
				return bizErr
			}

			// 3. 删除用户皮肤关联记录
			bizErr = t.userSkinRepo.DeleteByUserAndSkin(ctx, tx, userID, skinID)
			if bizErr != nil {
				return bizErr
			}

			// 4. 检查引用计数，零引用且为当前用户创建的资源才删除 SkinLibrary
			refCount, xErr := t.userSkinRepo.CountReferences(ctx, tx, skinID)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if refCount == 0 && skinRec.UserID != nil && *skinRec.UserID == userID {
				bizErr = t.skinRepo.DeleteByID(ctx, tx, skinID)
				if bizErr != nil {
					return bizErr
				}
			}
		} else {
			// 非 Normal 类型（Gift/Admin）：仅删除关联记录，不操作配额

			// 3. 删除用户皮肤关联记录
			bizErr = t.userSkinRepo.DeleteByUserAndSkin(ctx, tx, userID, skinID)
			if bizErr != nil {
				return bizErr
			}

			// 4. 检查引用计数，零引用且为用户创建的资源才删除 SkinLibrary
			refCount, xErr := t.userSkinRepo.CountReferences(ctx, tx, skinID)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if refCount == 0 {
				skinRec, skinFound, xErr := t.skinRepo.GetByID(ctx, tx, skinID)
				if xErr != nil {
					bizErr = xErr
					return xErr
				}
				if skinFound && skinRec.UserID != nil && *skinRec.UserID == userID {
					bizErr = t.skinRepo.DeleteByID(ctx, tx, skinID)
					if bizErr != nil {
						return bizErr
					}
				}
			}
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

// CreateCapeWithQuota 在事务内完成披风创建、用户关联及对应配额扣减。
//
// 该方法执行以下原子操作序列：
//  1. 行锁查询用户资源库配额
//  2. 根据 IsPublic 校验对应配额是否充足（公开配额或私有配额）
//  3. 检查纹理哈希是否已存在（去重）
//  4. 创建披风记录
//  5. 创建用户披风关联记录（AssignmentType=Normal）
//  6. 更新对应已用配额数量 (+1)
//
// 任一步骤失败将触发整体回滚。调用方需确保 Bucket 上传已完成并将文件 ID 填入 cape.Texture。
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
				// 幂等：检查是否已有关联记录
				_, assocFound, xErr := t.userCapeRepo.GetByUserAndCape(ctx, tx, *cape.UserID, existingCape.ID)
				if xErr != nil {
					bizErr = xErr
					return xErr
				}
				if !assocFound {
					// 创建关联记录但不扣配额（披风记录已存在，配额在首次创建时已扣减）
					_, bizErr = t.userCapeRepo.Create(ctx, tx, &entity.UserCapeLibrary{
						UserID:         *cape.UserID,
						CapeLibraryID:  existingCape.ID,
						AssignmentType: entityType.AssignmentTypeNormal,
					})
					if bizErr != nil {
						return bizErr
					}
				}
				return nil
			}
			bizErr = xError.NewError(ctx, xError.DataConflict, "该披风纹理已存在", true)
			return bizErr
		}

		// 4. 创建披风记录
		createdCape, bizErr = t.capeRepo.Create(ctx, tx, cape)
		if bizErr != nil {
			return bizErr
		}

		// 5. 创建用户披风关联记录（Normal 类型）
		_, bizErr = t.userCapeRepo.Create(ctx, tx, &entity.UserCapeLibrary{
			UserID:        *cape.UserID,
			CapeLibraryID: createdCape.ID,
			AssignmentType: entityType.AssignmentTypeNormal,
		})
		if bizErr != nil {
			return bizErr
		}

		// 6. 根据 IsPublic 更新对应配额
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
		// 校验用户关联类型，只有 Normal 类型允许调整配额
		association, found, xErr := t.userCapeRepo.GetByUserAndCape(ctx, tx, userID, capeID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户未关联该披风", true)
			return bizErr
		}
		if !association.AssignmentType.CountsTowardQuota() {
			bizErr = xError.NewError(ctx, xError.PermissionDenied, "赠送的资源不能修改公开状态", true)
			return bizErr
		}

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

// DeleteCapeWithQuota 在事务内完成用户披风关联删除、配额释放及按需清理披风记录。
//
// 新事务序列：
//  1. 查找 UserCapeLibrary 关联记录
//  2. 若 AssignmentType 为 Normal 则释放对应配额（事务内读取权威 IsPublic）
//  3. 删除 UserCapeLibrary 关联记录
//  4. 检查 CapeLibrary 引用计数，零引用且为当前用户创建的资源则删除
func (t *LibraryTxnRepo) DeleteCapeWithQuota(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
	capeID xSnowflake.SnowflakeID,
) *xError.Error {
	t.log.Info(ctx, "DeleteCapeWithQuota - 事务内删除披风")

	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 查找用户披风关联记录
		association, found, xErr := t.userCapeRepo.GetByUserAndCape(ctx, tx, userID, capeID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户未关联该披风", true)
			return bizErr
		}

		// 2. Normal 类型释放配额（事务内读取权威 IsPublic）
		if association.AssignmentType.CountsTowardQuota() {
			// 在事务内获取权威的披风记录，避免 TOCTOU 竞态
			capeRec, capeFound, xErr := t.capeRepo.GetByID(ctx, tx, capeID)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if !capeFound {
				bizErr = xError.NewError(ctx, xError.ResourceNotFound, "披风库记录不存在", true)
				return bizErr
			}

			quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, userID, true)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if !found {
				bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
				return bizErr
			}
			if capeRec.IsPublic {
				bizErr = t.quotaRepo.UpdateCapesPublicUsed(ctx, tx, quota.ID, quota.CapesPublicUsed-1)
			} else {
				bizErr = t.quotaRepo.UpdateCapesPrivateUsed(ctx, tx, quota.ID, quota.CapesPrivateUsed-1)
			}
			if bizErr != nil {
				return bizErr
			}

			// 3. 删除用户披风关联记录
			bizErr = t.userCapeRepo.DeleteByUserAndCape(ctx, tx, userID, capeID)
			if bizErr != nil {
				return bizErr
			}

			// 4. 检查引用计数，零引用且为当前用户创建的资源才删除 CapeLibrary
			refCount, xErr := t.userCapeRepo.CountReferences(ctx, tx, capeID)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if refCount == 0 && capeRec.UserID != nil && *capeRec.UserID == userID {
				bizErr = t.capeRepo.DeleteByID(ctx, tx, capeID)
				if bizErr != nil {
					return bizErr
				}
			}
		} else {
			// 非 Normal 类型（Gift/Admin）：仅删除关联记录，不操作配额

			// 3. 删除用户披风关联记录
			bizErr = t.userCapeRepo.DeleteByUserAndCape(ctx, tx, userID, capeID)
			if bizErr != nil {
				return bizErr
			}

			// 4. 检查引用计数，零引用且为用户创建的资源才删除 CapeLibrary
			refCount, xErr := t.userCapeRepo.CountReferences(ctx, tx, capeID)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if refCount == 0 {
				capeRec, capeFound, xErr := t.capeRepo.GetByID(ctx, tx, capeID)
				if xErr != nil {
					bizErr = xErr
					return xErr
				}
				if capeFound && capeRec.UserID != nil && *capeRec.UserID == userID {
					bizErr = t.capeRepo.DeleteByID(ctx, tx, capeID)
					if bizErr != nil {
						return bizErr
					}
				}
			}
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

// GiftSkinToUser 在事务内完成管理员向用户赠送皮肤。
//
// 事务序列：校验 SkinLibrary 存在 → 校验不重复关联 → 创建 UserSkinLibrary(Gift)。
// 不检查配额，不修改 Used。
func (t *LibraryTxnRepo) GiftSkinToUser(
	ctx context.Context,
	targetUserID xSnowflake.SnowflakeID,
	skinLibraryID xSnowflake.SnowflakeID,
	assignmentType entityType.AssignmentType,
) (*entity.UserSkinLibrary, *xError.Error) {
	t.log.Info(ctx, "GiftSkinToUser - 事务内赠送皮肤")

	var createdAssoc *entity.UserSkinLibrary
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 校验 SkinLibrary 存在
		_, found, xErr := t.skinRepo.GetByID(ctx, tx, skinLibraryID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "皮肤库记录不存在", true)
			return bizErr
		}

		// 2. 校验不重复关联
		exists, xErr := t.userSkinRepo.ExistsByUserAndSkin(ctx, tx, targetUserID, skinLibraryID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if exists {
			bizErr = xError.NewError(ctx, xError.DataConflict, "用户已关联该皮肤", true)
			return bizErr
		}

		// 3. 创建 UserSkinLibrary
		createdAssoc, bizErr = t.userSkinRepo.Create(ctx, tx, &entity.UserSkinLibrary{
			UserID:         targetUserID,
			SkinLibraryID:  skinLibraryID,
			AssignmentType: assignmentType,
		})
		if bizErr != nil {
			return bizErr
		}


		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "赠送皮肤失败", true, err)
	}
	return createdAssoc, nil
}

// RevokeSkinFromUser 在事务内完成管理员撤销用户皮肤关联。
//
// 事务序列：查找 UserSkinLibrary → 校验 Type ≠ Normal → 删除 UserSkinLibrary。
// 不修改配额。Normal 类型不允许撤销（用户自行删除走 DeleteSkinWithQuota）。
func (t *LibraryTxnRepo) RevokeSkinFromUser(
	ctx context.Context,
	targetUserID xSnowflake.SnowflakeID,
	skinLibraryID xSnowflake.SnowflakeID,
) *xError.Error {
	t.log.Info(ctx, "RevokeSkinFromUser - 事务内撤销皮肤")

	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 查找关联记录
		association, found, xErr := t.userSkinRepo.GetByUserAndSkin(ctx, tx, targetUserID, skinLibraryID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户未关联该皮肤", true)
			return bizErr
		}

		// 2. 校验 Type ≠ Normal
		if association.AssignmentType == entityType.AssignmentTypeNormal {
			bizErr = xError.NewError(ctx, xError.PermissionDenied, "无法撤销用户自主上传的资源", true)
			return bizErr
		}

		// 3. 删除关联记录
		bizErr = t.userSkinRepo.DeleteByUserAndSkin(ctx, tx, targetUserID, skinLibraryID)
		if bizErr != nil {
			return bizErr
		}

		// 4. 检查 SkinLibrary 引用计数，零引用且为用户创建的资源才删除
		refCount, xErr := t.userSkinRepo.CountReferences(ctx, tx, skinLibraryID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if refCount == 0 {
			skinRec, skinFound, xErr := t.skinRepo.GetByID(ctx, tx, skinLibraryID)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if skinFound && skinRec.UserID != nil {
				bizErr = t.skinRepo.DeleteByID(ctx, tx, skinLibraryID)
				if bizErr != nil {
					return bizErr
				}
			}
		}

		return nil
	})
	if bizErr != nil {
		return bizErr
	}
	if err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "撤销皮肤失败", true, err)
	}
	return nil
}

// GiftCapeToUser 在事务内完成管理员向用户赠送披风。
//
// 同构于 GiftSkinToUser，Skin → Cape。
func (t *LibraryTxnRepo) GiftCapeToUser(
	ctx context.Context,
	targetUserID xSnowflake.SnowflakeID,
	capeLibraryID xSnowflake.SnowflakeID,
	assignmentType entityType.AssignmentType,
) (*entity.UserCapeLibrary, *xError.Error) {
	t.log.Info(ctx, "GiftCapeToUser - 事务内赠送披风")

	var createdAssoc *entity.UserCapeLibrary
	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 校验 CapeLibrary 存在
		_, found, xErr := t.capeRepo.GetByID(ctx, tx, capeLibraryID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "披风库记录不存在", true)
			return bizErr
		}

		// 2. 校验不重复关联
		exists, xErr := t.userCapeRepo.ExistsByUserAndCape(ctx, tx, targetUserID, capeLibraryID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if exists {
			bizErr = xError.NewError(ctx, xError.DataConflict, "用户已关联该披风", true)
			return bizErr
		}

		// 3. 创建 UserCapeLibrary
		createdAssoc, bizErr = t.userCapeRepo.Create(ctx, tx, &entity.UserCapeLibrary{
			UserID:         targetUserID,
			CapeLibraryID:  capeLibraryID,
			AssignmentType: assignmentType,
		})
		if bizErr != nil {
			return bizErr
		}


		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "赠送披风失败", true, err)
	}
	return createdAssoc, nil
}

// RevokeCapeFromUser 在事务内完成管理员撤销用户披风关联。
//
// 同构于 RevokeSkinFromUser，Skin → Cape。
func (t *LibraryTxnRepo) RevokeCapeFromUser(
	ctx context.Context,
	targetUserID xSnowflake.SnowflakeID,
	capeLibraryID xSnowflake.SnowflakeID,
) *xError.Error {
	t.log.Info(ctx, "RevokeCapeFromUser - 事务内撤销披风")

	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 查找关联记录
		association, found, xErr := t.userCapeRepo.GetByUserAndCape(ctx, tx, targetUserID, capeLibraryID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户未关联该披风", true)
			return bizErr
		}

		// 2. 校验 Type ≠ Normal
		if association.AssignmentType == entityType.AssignmentTypeNormal {
			bizErr = xError.NewError(ctx, xError.PermissionDenied, "无法撤销用户自主上传的资源", true)
			return bizErr
		}

		// 3. 删除关联记录
		bizErr = t.userCapeRepo.DeleteByUserAndCape(ctx, tx, targetUserID, capeLibraryID)
		if bizErr != nil {
			return bizErr
		}

		// 4. 检查 CapeLibrary 引用计数，零引用且为用户创建的资源才删除
		refCount, xErr := t.userCapeRepo.CountReferences(ctx, tx, capeLibraryID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if refCount == 0 {
			capeRec, capeFound, xErr := t.capeRepo.GetByID(ctx, tx, capeLibraryID)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if capeFound && capeRec.UserID != nil {
				bizErr = t.capeRepo.DeleteByID(ctx, tx, capeLibraryID)
				if bizErr != nil {
					return bizErr
				}
			}
		}

		return nil
	})
	if bizErr != nil {
		return bizErr
	}
	if err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "撤销披风失败", true, err)
	}
	return nil
}

// RecalculateQuota 在事务内重新计算用户配额的 Used 字段。
//
// 事务序列：锁配额 FOR UPDATE → 用 CountNormalPublicByUser/CountNormalPrivateByUser 重算 4 个 Used → 调用 UpdateAllUsed。
func (t *LibraryTxnRepo) RecalculateQuota(
	ctx context.Context,
	userID xSnowflake.SnowflakeID,
) *xError.Error {
	t.log.Info(ctx, "RecalculateQuota - 事务内重算配额")

	var bizErr *xError.Error

	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 行锁查询配额
		quota, found, xErr := t.quotaRepo.GetByUserID(ctx, tx, userID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}

		// 2. 重算皮肤配额
		skinsPub, xErr := t.userSkinRepo.CountNormalPublicByUser(ctx, tx, userID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		skinsPri, xErr := t.userSkinRepo.CountNormalPrivateByUser(ctx, tx, userID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}

		// 3. 重算披风配额
		capesPub, xErr := t.userCapeRepo.CountNormalPublicByUser(ctx, tx, userID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		capesPri, xErr := t.userCapeRepo.CountNormalPrivateByUser(ctx, tx, userID)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}

		// 4. 一次性更新 4 个 Used 字段
		bizErr = t.quotaRepo.UpdateAllUsed(ctx, tx, quota.ID, int32(skinsPub), int32(skinsPri), int32(capesPub), int32(capesPri))
		if bizErr != nil {
			return bizErr
		}

		return nil
	})
	if bizErr != nil {
		return bizErr
	}
	if err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "重算配额失败", true, err)
	}
	return nil
}
