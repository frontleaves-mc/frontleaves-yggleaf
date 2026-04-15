package yggdrasil

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
)

// VerifyProfileOwnership 验证角色是否属于指定用户。
//
// 检查给定 UUID 的游戏档案是否存在，且其 UserID 与传入的 userID 匹配。
// 用于材质上传/删除接口中的权限验证。
//
// 参数:
//   - ctx: 上下文对象
//   - userID: 用户的 Snowflake ID
//   - unsignedUUID: 角色的无符号 UUID
//
// 返回值:
//   - *entity.GameProfile: 验证通过的角色实体
//   - bool: 是否验证通过
//   - *xError.Error: 验证过程中的错误
func (l *YggdrasilLogic) VerifyProfileOwnership(ctx context.Context, userID int64, unsignedUUID string) (*entity.GameProfile, bool, *xError.Error) {
	l.log.Info(ctx, "VerifyProfileOwnership - 验证角色归属")

	profile, found, xErr := l.repo.profileRepo.GetByUUIDUnsigned(ctx, nil, unsignedUUID)
	if xErr != nil {
		return nil, false, xErr
	}
	if !found {
		return nil, false, nil
	}

	// 验证角色属于该用户
	if profile.UserID.Int64() != userID {
		return nil, false, nil
	}

	return profile, true, nil
}

// UpdateProfileSkin 更新角色的皮肤库关联。
//
// 将指定角色的 SkinLibraryID 更新为新的皮肤库 ID。
//
// 参数:
//   - ctx: 上下文对象
//   - profileID: 角色 Snowflake ID
//   - skinLibraryID: 皮肤库 Snowflake ID（nil 表示清除关联）
//
// 返回值:
//   - *xError.Error: 更新过程中的错误
func (l *YggdrasilLogic) UpdateProfileSkin(ctx context.Context, profileID int64, skinLibraryID *int64) *xError.Error {
	l.log.Info(ctx, "UpdateProfileSkin - 更新角色皮肤关联")
	return xError.NewError(ctx, xError.ServerInternalError, "材质管理功能待实现：需要 BucketClient 集成", true)
}

// UpdateProfileCape 更新角色的披风库关联。
//
// 将指定角色的 CapeLibraryID 更新为新的披风库 ID。
//
// 参数:
//   - ctx: 上下文对象
//   - profileID: 角色 Snowflake ID
//   - capeLibraryID: 披风库 Snowflake ID（nil 表示清除关联）
//
// 返回值:
//   - *xError.Error: 更新过程中的错误
func (l *YggdrasilLogic) UpdateProfileCape(ctx context.Context, profileID int64, capeLibraryID *int64) *xError.Error {
	l.log.Info(ctx, "UpdateProfileCape - 更新角色披风关联")
	return xError.NewError(ctx, xError.ServerInternalError, "材质管理功能待实现：需要 BucketClient 集成", true)
}

// ClearProfileSkin 清除角色的皮肤关联。
//
// 将指定角色的 SkinLibraryID 置为 NULL。
//
// 参数:
//   - ctx: 上下文对象
//   - profileID: 角色 Snowflake ID
//
// 返回值:
//   - *xError.Error: 操作过程中的错误
func (l *YggdrasilLogic) ClearProfileSkin(ctx context.Context, profileID int64) *xError.Error {
	l.log.Info(ctx, "ClearProfileSkin - 清除角色皮肤关联")
	return xError.NewError(ctx, xError.ServerInternalError, "材质管理功能待实现：需要 BucketClient 集成", true)
}

// ClearProfileCape 清除角色的披风关联。
//
// 将指定角色的 CapeLibraryID 置为 NULL。
//
// 参数:
//   - ctx: 上下文对象
//   - profileID: 角色 Snowflake ID
//
// 返回值:
//   - *xError.Error: 操作过程中的错误
func (l *YggdrasilLogic) ClearProfileCape(ctx context.Context, profileID int64) *xError.Error {
	l.log.Info(ctx, "ClearProfileCape - 清除角色披风关联")
	return xError.NewError(ctx, xError.ServerInternalError, "材质管理功能待实现：需要 BucketClient 集成", true)
}
