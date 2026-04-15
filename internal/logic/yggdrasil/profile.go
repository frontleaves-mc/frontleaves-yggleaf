package yggdrasil

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	apiYgg "github.com/frontleaves-mc/frontleaves-yggleaf/api/yggdrasil"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
)

// QueryProfile 根据无符号 UUID 查询角色属性信息。
//
// 根据 Yggdrasil 协议规范（§5.9），该方法查询指定 UUID 的角色信息，
// 并组装 textures 属性（含 Base64 编码的材质载荷）。
//
// 参数:
//   - ctx: 上下文对象
//   - unsignedUUID: 角色的无符号 UUID（32 位十六进制字符串）
//   - unsigned: true 不含签名 / false 含签名
//
// 返回值:
//   - *apiYgg.ProfileResponse: 角色信息响应
//   - bool: 是否找到角色
//   - *xError.Error: 查询过程中的错误
func (l *YggdrasilLogic) QueryProfile(ctx context.Context, unsignedUUID string, unsigned bool) (*apiYgg.ProfileResponse, bool, *xError.Error) {
	l.log.Info(ctx, "QueryProfile - 查询角色属性")

	// 查询角色信息（含关联皮肤和披风）
	profile, found, xErr := l.repo.profileRepo.GetByUUIDUnsignedWithTextures(ctx, nil, unsignedUUID)
	if xErr != nil {
		return nil, false, xErr
	}
	if !found {
		return nil, false, nil
	}

	resp := l.BuildProfileResponse(ctx, profile, unsigned)
	return resp, true, nil
}

// BatchLookupProfiles 根据名称列表批量查询角色。
//
// 根据 Yggdrasil 协议规范（§5.10），该方法按角色名称批量查询，
// 仅返回无符号 UUID 和名称，不包含角色属性（无 properties）。
//
// 参数:
//   - ctx: 上下文对象
//   - names: 角色名称列表
//
// 返回值:
//   - []apiYgg.BatchProfileItem: 匹配的角色列表
//   - *xError.Error: 查询过程中的错误
func (l *YggdrasilLogic) BatchLookupProfiles(ctx context.Context, names []string) ([]apiYgg.BatchProfileItem, *xError.Error) {
	l.log.Info(ctx, "BatchLookupProfiles - 批量查询角色")

	// 业务规则：单次最多查询 10 个角色（spec §5.10 防 CC 攻击）
	if len(names) > bConst.YggdrasilBatchLookupMaxNames {
		names = names[:bConst.YggdrasilBatchLookupMaxNames]
	}

	profiles, xErr := l.repo.profileRepo.BatchGetByNames(ctx, nil, names)
	if xErr != nil {
		return nil, xErr
	}

	// 转换为响应 DTO（仅 id 和 name，不含 properties）
	items := make([]apiYgg.BatchProfileItem, 0, len(profiles))
	for _, p := range profiles {
		items = append(items, apiYgg.BatchProfileItem{
			ID:   EncodeUnsignedUUID(p.UUID),
			Name: p.Name,
		})
	}

	return items, nil
}

// GetProfileByUUID 根据无符号 UUID 查询角色实体（不含纹理预加载）。
//
// 用于需要获取角色基础信息但不需要材质详情的场景。
//
// 参数:
//   - ctx: 上下文对象
//   - unsignedUUID: 角色的无符号 UUID
//
// 返回值:
//   - *entity.GameProfile: 角色实体
//   - bool: 是否找到
//   - *xError.Error: 查询过程中的错误
func (l *YggdrasilLogic) GetProfileByUUID(ctx context.Context, unsignedUUID string) (*entity.GameProfile, bool, *xError.Error) {
	l.log.Info(ctx, "GetProfileByUUID - 根据无符号 UUID 获取角色")

	return l.repo.profileRepo.GetByUUIDUnsigned(ctx, nil, unsignedUUID)
}
