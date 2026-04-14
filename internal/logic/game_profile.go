package logic

import (
	"context"
	"regexp"
	"strings"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	repotxn "github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/txn"
	"github.com/google/uuid"
)

const (
	gameProfileNameMinLength = 3
	gameProfileNameMaxLength = 16
)

var gameProfileNameRegex = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// gameProfileRepo 游戏档案数据访问适配器。
//
// 聚合游戏档案相关的各仓储实例，包括档案本体、配额和配额日志，
// 以及事务协调仓储（TxnRepo），供 GameProfileLogic 统一调用。
type gameProfileRepo struct {
	profile  *repository.GameProfileRepo         // 游戏档案仓储
	quota    *repository.GameProfileQuotaRepo     // 游戏档案配额仓储
	quotaLog *repository.GameProfileQuotaLogRepo  // 游戏档案配额日志仓储
	txn      *repotxn.GameProfileTxnRepo       // 游戏档案事务协调仓储
}

// GameProfileLogic 游戏档案业务逻辑处理者。
//
// 封装了游戏档案相关的核心业务逻辑，包括档案创建、用户名修改等操作。
// 它通过嵌入匿名的 `logic` 结构体，继承了 Redis 客户端 (`rdb`) 和日志记录器 (`log`)，
// 通过 `gameProfileRepo` 适配器调用 Repository 层完成数据持久化。
//
// 设计约束：本层不直接操作数据库事务，所有涉及多表写入的事务性操作
// 均委托给 Repository 层的 GameProfileTxnRepo 完成。
type GameProfileLogic struct {
	logic
	repo gameProfileRepo
}

// NewGameProfileLogic 创建游戏档案业务逻辑实例。
//
// 该函数用于初始化并返回一个 `GameProfileLogic` 结构体指针。它会尝试从传入的上下文
// (context.Context) 中获取必需的依赖项（数据库连接、Redis 连接），并初始化所有关联的
// Repository 实例及事务协调仓储。
//
// 参数说明:
//   - ctx: 上下文对象，用于传递请求范围的数据、取消信号和截止时间，同时用于提取基础资源。
//
// 返回值:
//   - *GameProfileLogic: 初始化完成的游戏档案业务逻辑实例指针。
//
// 注意: 该函数依赖于 `xCtxUtil.MustGetDB` 和 `xCtxUtil.MustGetRDB`。如果上下文中缺少
// 必要的数据库或 Redis 连接，这些辅助函数会触发 panic。请确保上下文已通过中间件正确注入了这些资源。
func NewGameProfileLogic(ctx context.Context) *GameProfileLogic {
	db := xCtxUtil.MustGetDB(ctx)
	rdb := xCtxUtil.MustGetRDB(ctx)

	profileRepo := repository.NewGameProfileRepo(db)
	quotaRepo := repository.NewGameProfileQuotaRepo(db)
	quotaLogRepo := repository.NewGameProfileQuotaLogRepo(db)

	return &GameProfileLogic{
		logic: logic{
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "GameProfileLogic"),
		},
		repo: gameProfileRepo{
			profile:  profileRepo,
			quota:    quotaRepo,
			quotaLog: quotaLogRepo,
			txn:      repotxn.NewGameProfileTxnRepo(db, profileRepo, quotaRepo, quotaLogRepo),
		},
	}
}

// AddGameProfile 为指定用户新增游戏档案。
//
// 该方法执行以下业务流程：
//  1. 校验用户 ID 有效性与名称合法性（长度、格式）
//  2. 生成 V7 UUID 作为档案唯一标识
//  3. 构建游戏档案实体
//  4. 委托 Repository 层在事务内完成：配额检查 → 唯一性校验 → 档案创建 → 配额扣减 → 日志记录
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID。
//   - name: 游戏档案用户名，需满足 3-16 位字母数字下划线组合。
//
// 返回值:
//   - *entity.GameProfile: 创建成功的游戏档案实体。
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *GameProfileLogic) AddGameProfile(ctx context.Context, userID xSnowflake.SnowflakeID, name string) (*entity.GameProfile, *xError.Error) {
	l.log.Info(ctx, "AddGameProfile - 新增游戏档案")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	normalizedName, xErr := validateGameProfileName(ctx, name)
	if xErr != nil {
		return nil, xErr
	}

	profileUUID, err := uuid.NewV7()
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "生成游戏档案 UUID 失败", true, err)
	}

	profile := &entity.GameProfile{
		UserID: userID,
		UUID:   profileUUID,
		Name:   normalizedName,
	}

	// 委托 Repository 层在事务内完成创建与配额操作
	return l.repo.txn.AddProfileWithQuota(ctx, profile)
}

// ChangeUsername 修改指定游戏档案的用户名。
//
// 该方法执行以下业务流程：
//  1. 校验档案归属权（档案必须属于当前用户）
//  2. 校验并规范化新用户名
//  3. 短路优化：若名称未变更则直接返回
//  4. 检查新名称是否与其他档案冲突
//  5. 更新档案名称
//
// 该方法为单表更新操作，无需事务包裹。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID。
//   - profileID: 目标游戏档案的雪花 ID。
//   - newName: 新的游戏档案用户名。
//
// 返回值:
//   - *entity.GameProfile: 更新后的游戏档案实体。
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *GameProfileLogic) ChangeUsername(ctx context.Context, userID xSnowflake.SnowflakeID, profileID xSnowflake.SnowflakeID, newName string) (*entity.GameProfile, *xError.Error) {
	l.log.Info(ctx, "ChangeUsername - 修改游戏档案用户名")

	profile, found, xErr := l.repo.profile.GetByIDAndUserID(ctx, nil, profileID, userID, false)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "游戏档案不存在", true)
	}

	normalizedName, xErr := validateGameProfileName(ctx, newName)
	if xErr != nil {
		return nil, xErr
	}
	if profile.Name == normalizedName {
		return profile, nil
	}

	nameExisted, xErr := l.repo.profile.ExistsByNameExceptID(ctx, nil, normalizedName, profile.ID)
	if xErr != nil {
		return nil, xErr
	}
	if nameExisted {
		return nil, xError.NewError(ctx, xError.DataConflict, "用户名已存在", true)
	}

	updatedProfile, xErr := l.repo.profile.UpdateName(ctx, nil, profile.ID, normalizedName)
	if xErr != nil {
		return nil, xErr
	}
	return updatedProfile, nil
}

// GetGameProfileDetail 获取指定游戏档案的详情（含关联皮肤和披风）。
//
// 参数:
//   - ctx: 上下文对象。
//   - userID: 操作者的雪花 ID。
//   - profileID: 目标游戏档案的雪花 ID。
//
// 返回值:
//   - *entity.GameProfile: 游戏档案详情（含关联数据）。
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *GameProfileLogic) GetGameProfileDetail(ctx context.Context, userID xSnowflake.SnowflakeID, profileID xSnowflake.SnowflakeID) (*entity.GameProfile, *xError.Error) {
	l.log.Info(ctx, "GetGameProfileDetail - 获取游戏档案详情")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}
	if profileID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效档案 ID：不能为 0", true)
	}

	profile, found, xErr := l.repo.profile.GetDetailByID(ctx, nil, profileID, userID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "游戏档案不存在", true)
	}
	return profile, nil
}

// ListGameProfiles 获取指定用户的所有游戏档案列表。
//
// 该方法按创建时间正序返回该用户的全部游戏档案，不含分页。
//
// 参数:
//   - ctx: 上下文对象。
//   - userID: 操作者的雪花 ID。
//
// 返回值:
//   - []entity.GameProfile: 游戏档案列表。
//   - *xError.Error: 数据操作过程中发生的错误。
func (l *GameProfileLogic) ListGameProfiles(ctx context.Context, userID xSnowflake.SnowflakeID) ([]entity.GameProfile, *xError.Error) {
	l.log.Info(ctx, "ListGameProfiles - 获取游戏档案列表")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	profiles, xErr := l.repo.profile.ListByUserID(ctx, nil, userID)
	if xErr != nil {
		return nil, xErr
	}
	return profiles, nil
}

// GetQuota 获取指定用户的游戏档案配额信息。
//
// 若该用户尚无配额记录，Repository 层会自动创建默认配额（Total=1, Used=0）。
//
// 参数:
//   - ctx: 上下文对象。
//   - userID: 操作者的雪花 ID。
//
// 返回值:
//   - *entity.GameProfileQuota: 用户配额实体。
//   - *xError.Error: 数据操作过程中发生的错误。
func (l *GameProfileLogic) GetQuota(ctx context.Context, userID xSnowflake.SnowflakeID) (*entity.GameProfileQuota, *xError.Error) {
	l.log.Info(ctx, "GetQuota - 获取游戏档案配额")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	quota, _, xErr := l.repo.quota.GetByUserID(ctx, nil, userID, false)
	if xErr != nil {
		return nil, xErr
	}
	return quota, nil
}

// validateGameProfileName 校验并规范化游戏档案用户名。
//
// 执行以下校验规则：
//  - 长度必须在 3-16 个字符之间
//  - 仅允许字母（大小写）、数字和下划线
//  - 自动去除首尾空白字符
//
// 参数:
//   - ctx: Gin 上下文对象，用于构造错误响应。
//   - name: 待校验的原始用户名字符串。
//
// 返回值:
//   - string: 规范化后的用户名（已去除首尾空白）。
//   - *xError.Error: 校验失败时返回具体错误信息。
func validateGameProfileName(ctx context.Context, name string) (string, *xError.Error) {
	normalizedName := strings.TrimSpace(name)
	if len(normalizedName) < gameProfileNameMinLength || len(normalizedName) > gameProfileNameMaxLength {
		return "", xError.NewError(ctx, xError.ParameterError, "无效用户名长度：必须在 3-16 个字符之间", true)
	}
	if !gameProfileNameRegex.MatchString(normalizedName) {
		return "", xError.NewError(ctx, xError.ParameterError, "无效用户名格式：只允许字母、数字和下划线", true)
	}
	return normalizedName, nil
}
