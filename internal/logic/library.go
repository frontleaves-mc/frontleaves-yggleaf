package logic

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	apiLibrary "github.com/frontleaves-mc/frontleaves-yggleaf/api/library"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	repotxn "github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/txn"
	bCtx "github.com/frontleaves-mc/frontleaves-yggleaf/pkg/context"
	bBucket "github.com/phalanx-labs/beacon-bucket-sdk"
	bBucketApi "github.com/phalanx-labs/beacon-bucket-sdk/api"
)

const (
	skinNameMinLength = 1
	skinNameMaxLength = 64
	capeNameMinLength = 1
	capeNameMaxLength = 64
)

// libraryRepo 资源库数据访问适配器。
//
// 聚合资源库（皮肤/披风）相关的各仓储实例，包括皮肤仓储、披风仓储、配额仓储，
// 用户关联仓储，以及事务协调仓储（TxnRepo），供 LibraryLogic 统一调用。
type libraryRepo struct {
	skinRepo     *repository.SkinLibraryRepo     // 皮肤库仓储
	capeRepo     *repository.CapeLibraryRepo     // 披风库仓储
	quotaRepo    *repository.LibraryQuotaRepo    // 资源库配额仓储
	userSkinRepo *repository.UserSkinLibraryRepo // 用户皮肤关联仓储
	userCapeRepo *repository.UserCapeLibraryRepo // 用户披风关联仓储
	txn          *repotxn.LibraryTxnRepo         // 资源库事务协调仓储
}

// libraryHelper 资源库外部服务辅助器。
//
// 封装对象存储（Bucket）客户端等外部依赖，用于处理纹理文件上传等
// 不属于数据库事务范围的外部服务调用。
type libraryHelper struct {
	bucket *bBucket.BucketClient // 对象存储客户端
}

// LibraryLogic 资源库业务逻辑处理者。
//
// 封装了资源库（皮肤、披风）相关的核心业务逻辑，包括创建、更新、删除和列表查询等操作。
// 它通过嵌入匿名的 `logic` 结构体，继承了 Redis 客户端 (`rdb`) 和日志记录器 (`log`)，
// 通过 `libraryRepo` 适配器调用 Repository 层完成数据持久化，
// 通过 `libraryHelper` 完成对象存储等外部服务调用。
//
// 设计约束：本层不直接操作数据库事务，所有涉及多表写入的事务性操作
// 均委托给 Repository 层的 LibraryTxnRepo 完成。对象存储上传在事务外执行，
// 上传成功后将文件 ID 填入实体再进入事务流程。
type LibraryLogic struct {
	logic
	repo   libraryRepo
	helper libraryHelper
}

// NewLibraryLogic 创建资源库业务逻辑实例。
//
// 该函数用于初始化并返回一个 `LibraryLogic` 结构体指针。它会尝试从传入的上下文
// (context.Context) 中获取必需的依赖项（数据库连接、Redis 连接、对象存储客户端），
// 并初始化所有关联的 Repository 实例及事务协调仓储。
//
// 参数说明:
//   - ctx: 上下文对象，用于传递请求范围的数据、取消信号和截止时间，同时用于提取基础资源。
//
// 返回值:
//   - *LibraryLogic: 初始化完成的资源库业务逻辑实例指针。
//
// 注意: 该函数依赖于 `xCtxUtil.MustGetDB`、`xCtxUtil.MustGetRDB` 和 `bCtx.MustGetBucket`。
// 如果上下文中缺少必要的依赖，这些辅助函数会触发 panic。请确保上下文已通过中间件正确注入了这些资源。
func NewLibraryLogic(ctx context.Context) *LibraryLogic {
	db := xCtxUtil.MustGetDB(ctx)
	rdb := xCtxUtil.MustGetRDB(ctx)

	skinRepo := repository.NewSkinLibraryRepo(db)
	capeRepo := repository.NewCapeLibraryRepo(db)
	quotaRepo := repository.NewLibraryQuotaRepo(db)
	userSkinRepo := repository.NewUserSkinLibraryRepo(db)
	userCapeRepo := repository.NewUserCapeLibraryRepo(db)

	return &LibraryLogic{
		logic: logic{
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "LibraryLogic"),
		},
		repo: libraryRepo{
			skinRepo:     skinRepo,
			capeRepo:     capeRepo,
			quotaRepo:    quotaRepo,
			userSkinRepo: userSkinRepo,
			userCapeRepo: userCapeRepo,
			txn: repotxn.NewLibraryTxnRepo(
				db, skinRepo, capeRepo, quotaRepo,
				userSkinRepo, userCapeRepo,
			),
		},
		helper: libraryHelper{
			bucket: bCtx.MustGetBucket(ctx),
		},
	}
}

// ==================== Skin Logic ====================

// CreateSkin 创建皮肤。
//
// 该方法执行以下业务流程：
//  1. 校验用户 ID 有效性
//  2. 校验并规范化皮肤名称
//  3. 校验模型类型合法性（Classic / Slim）
//  4. 解码 Base64 纹理数据
//  5. 计算 SHA256 纹理哈希（用于去重）
//  6. 上传纹理到对象存储（事务外执行）
//  7. 委托 Repository 层在事务内完成：配额检查 → 哈希去重 → 记录创建 → 关联创建 → 配额扣减
func (l *LibraryLogic) CreateSkin(ctx context.Context, userID xSnowflake.SnowflakeID, req *apiLibrary.CreateSkinRequest) (*entity.SkinLibrary, *xError.Error) {
	l.log.Info(ctx, "CreateSkin - 创建皮肤")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	name, xErr := l.validateSkinName(ctx, req.Name)
	if xErr != nil {
		return nil, xErr
	}

	model := entity.ModelType(req.Model)
	if model != entity.ModelTypeClassic && model != entity.ModelTypeSlim {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效皮肤模型类型", true)
	}

	textureData, xErr := l.decodeBase64Texture(ctx, req.Texture)
	if xErr != nil {
		return nil, xErr
	}
	textureHash := l.calculateTextureHash(textureData)

	// 上传到对象存储（事务外执行，避免长事务占用连接）
	uploadResp, err := l.helper.bucket.Normal.Upload(ctx, &bBucketApi.UploadRequest{
		BucketId:      "360607182437229568",
		PathId:        "360607485278626816",
		ContentBase64: req.Texture,
	})
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "上传皮肤纹理失败", true, err)
	}

	skinId, err := strconv.ParseInt(uploadResp.FileId, 10, 64)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "解析纹理文件 ID 失败", true, err)
	}
	isPublic := false
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}
	skin := &entity.SkinLibrary{
		UserID:      &userID,
		Name:        name,
		Texture:     skinId,
		TextureHash: textureHash,
		Model:       model,
		IsPublic:    isPublic,
	}

	// 委托 Repository 层在事务内完成创建、关联与配额操作
	return l.repo.txn.CreateSkinWithQuota(ctx, skin)
}

// UpdateSkin 更新皮肤（名称/公开状态）。
//
// 该方法执行以下业务流程：
//  1. 通过 UserSkinLibrary 校验用户是否关联该皮肤
//  2. 获取皮肤记录并校验（系统内置皮肤不可修改）
//  3. 解析并校验新参数（名称/公开状态）
//  4. 短路优化：若所有字段均未变更则直接返回
//  5. 委托 Repository 层在事务内完成：公开状态变更时的配额调整 → 皮肤记录更新
func (l *LibraryLogic) UpdateSkin(ctx context.Context, userID xSnowflake.SnowflakeID, skinID xSnowflake.SnowflakeID, req *apiLibrary.UpdateSkinRequest) (*entity.SkinLibrary, *xError.Error) {
	l.log.Info(ctx, "UpdateSkin - 更新皮肤")

	// 通过用户关联校验归属
	_, found, xErr := l.repo.userSkinRepo.GetByUserAndSkin(ctx, nil, userID, skinID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "皮肤不存在或不属于当前用户", true)
	}

	// 获取皮肤记录
	skin, found, xErr := l.repo.skinRepo.GetByID(ctx, nil, skinID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "皮肤不存在", true)
	}

	if skin.UserID == nil {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "系统内置皮肤不能修改", true)
	}
	if *skin.UserID != userID {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "只有资源创建者可以修改资源属性", true)
	}

	newName := skin.Name
	newIsPublic := skin.IsPublic

	if req.Name != nil {
		validatedName, xErr := l.validateSkinName(ctx, *req.Name)
		if xErr != nil {
			return nil, xErr
		}
		newName = validatedName
	}

	if req.IsPublic != nil {
		newIsPublic = *req.IsPublic
	}

	if skin.Name == newName && skin.IsPublic == newIsPublic {
		return skin, nil
	}

	// 委托 Repository 层在事务内完成配额调整与记录更新
	return l.repo.txn.UpdateSkinWithQuota(ctx, userID, skinID, newName, newIsPublic, skin.IsPublic)
}

// DeleteSkin 删除皮肤关联。
//
// 该方法执行以下业务流程：
//  1. 通过 UserSkinLibrary 校验用户是否关联该皮肤
//  2. 获取皮肤记录用于判断公开状态
//  3. 委托 Repository 层在事务内完成：配额释放 → 关联删除 → 按需清理皮肤记录
func (l *LibraryLogic) DeleteSkin(ctx context.Context, userID xSnowflake.SnowflakeID, skinID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "DeleteSkin - 删除皮肤")

	// 通过用户关联校验归属
	_, found, xErr := l.repo.userSkinRepo.GetByUserAndSkin(ctx, nil, userID, skinID)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ResourceNotFound, "皮肤不存在或不属于当前用户", true)
	}

	// 获取皮肤记录（用于判断公开状态）
	skin, found, xErr := l.repo.skinRepo.GetByID(ctx, nil, skinID)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ResourceNotFound, "皮肤不存在", true)
	}

	if skin.UserID == nil {
		return xError.NewError(ctx, xError.PermissionDenied, "系统内置皮肤不能删除", true)
	}

	// 委托 Repository 层在事务内完成配额释放、关联删除与记录清理
	return l.repo.txn.DeleteSkinWithQuota(ctx, userID, skinID)
}

// ListSkins 获取市场公开皮肤列表。
//
// 分页获取所有公开的皮肤。该方法为纯读操作，无需事务包裹。
func (l *LibraryLogic) ListSkins(ctx context.Context, page int, pageSize int) ([]entity.SkinLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListSkins - 获取公开皮肤列表")

	return l.repo.skinRepo.ListPublic(ctx, nil, page, pageSize)
}

// ListMySkins 获取当前用户的皮肤关联列表。
//
// 通过 UserSkinLibrary 查询，包含 Preloaded 的 SkinLibrary 信息。
// 该方法为纯读操作，无需事务包裹。
func (l *LibraryLogic) ListMySkins(ctx context.Context, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]entity.UserSkinLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListMySkins - 获取我的皮肤列表")

	return l.repo.userSkinRepo.ListByUserID(ctx, nil, userID, page, pageSize)
}

// ==================== Cape Logic ====================

// CreateCape 创建披风。
//
// 该方法执行以下业务流程：
//  1. 校验用户 ID 有效性
//  2. 校验并规范化披风名称
//  3. 解码 Base64 纹理数据
//  4. 计算 SHA256 纹理哈希（用于去重）
//  5. 上传纹理到对象存储（事务外执行）
//  6. 委托 Repository 层在事务内完成：配额检查 → 哈希去重 → 记录创建 → 关联创建 → 配额扣减
func (l *LibraryLogic) CreateCape(ctx context.Context, userID xSnowflake.SnowflakeID, req *apiLibrary.CreateCapeRequest) (*entity.CapeLibrary, *xError.Error) {
	l.log.Info(ctx, "CreateCape - 创建披风")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	name, xErr := l.validateCapeName(ctx, req.Name)
	if xErr != nil {
		return nil, xErr
	}

	textureData, xErr := l.decodeBase64Texture(ctx, req.Texture)
	if xErr != nil {
		return nil, xErr
	}
	textureHash := l.calculateTextureHash(textureData)

	// 上传到对象存储（事务外执行，避免长事务占用连接）
	uploadResp, err := l.helper.bucket.Normal.Upload(ctx, &bBucketApi.UploadRequest{
		BucketId:      "yggleaf",
		PathId:        "capes",
		ContentBase64: req.Texture,
	})
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "上传披风纹理失败", true, err)
	}

	capeId, err := strconv.ParseInt(uploadResp.FileId, 10, 64)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "解析纹理文件 ID 失败", true, err)
	}
	isPublic := false
	if req.IsPublic != nil {
		isPublic = *req.IsPublic
	}
	cape := &entity.CapeLibrary{
		UserID:      &userID,
		Name:        name,
		Texture:     capeId,
		TextureHash: textureHash,
		IsPublic:    isPublic,
	}

	// 委托 Repository 层在事务内完成创建、关联与配额操作
	return l.repo.txn.CreateCapeWithQuota(ctx, cape)
}

// UpdateCape 更新披风（名称/公开状态）。
//
// 该方法执行以下业务流程：
//  1. 通过 UserCapeLibrary 校验用户是否关联该披风
//  2. 获取披风记录并校验（系统内置披风不可修改）
//  3. 解析并校验新参数（名称/公开状态）
//  4. 短路优化：若所有字段均未变更则直接返回
//  5. 委托 Repository 层在事务内完成：公开状态变更时的配额调整 → 披风记录更新
func (l *LibraryLogic) UpdateCape(ctx context.Context, userID xSnowflake.SnowflakeID, capeID xSnowflake.SnowflakeID, req *apiLibrary.UpdateCapeRequest) (*entity.CapeLibrary, *xError.Error) {
	l.log.Info(ctx, "UpdateCape - 更新披风")

	// 通过用户关联校验归属
	_, found, xErr := l.repo.userCapeRepo.GetByUserAndCape(ctx, nil, userID, capeID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "披风不存在或不属于当前用户", true)
	}

	// 获取披风记录
	cape, found, xErr := l.repo.capeRepo.GetByID(ctx, nil, capeID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "披风不存在", true)
	}

	if cape.UserID == nil {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "系统内置披风不能修改", true)
	}
	if *cape.UserID != userID {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "只有资源创建者可以修改资源属性", true)
	}

	newName := cape.Name
	newIsPublic := cape.IsPublic

	if req.Name != nil {
		validatedName, xErr := l.validateCapeName(ctx, *req.Name)
		if xErr != nil {
			return nil, xErr
		}
		newName = validatedName
	}

	if req.IsPublic != nil {
		newIsPublic = *req.IsPublic
	}

	if cape.Name == newName && cape.IsPublic == newIsPublic {
		return cape, nil
	}

	// 委托 Repository 层在事务内完成配额调整与记录更新
	return l.repo.txn.UpdateCapeWithQuota(ctx, userID, capeID, newName, newIsPublic, cape.IsPublic)
}

// DeleteCape 删除披风关联。
//
// 该方法执行以下业务流程：
//  1. 通过 UserCapeLibrary 校验用户是否关联该披风
//  2. 获取披风记录用于判断公开状态
//  3. 委托 Repository 层在事务内完成：配额释放 → 关联删除 → 按需清理披风记录
func (l *LibraryLogic) DeleteCape(ctx context.Context, userID xSnowflake.SnowflakeID, capeID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "DeleteCape - 删除披风")

	// 通过用户关联校验归属
	_, found, xErr := l.repo.userCapeRepo.GetByUserAndCape(ctx, nil, userID, capeID)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ResourceNotFound, "披风不存在或不属于当前用户", true)
	}

	// 获取披风记录（用于判断公开状态）
	cape, found, xErr := l.repo.capeRepo.GetByID(ctx, nil, capeID)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ResourceNotFound, "披风不存在", true)
	}

	if cape.UserID == nil {
		return xError.NewError(ctx, xError.PermissionDenied, "系统内置披风不能删除", true)
	}

	// 委托 Repository 层在事务内完成配额释放、关联删除与记录清理
	return l.repo.txn.DeleteCapeWithQuota(ctx, userID, capeID)
}

// ListCapes 获取市场公开披风列表。
//
// 分页获取所有公开的披风。该方法为纯读操作，无需事务包裹。
func (l *LibraryLogic) ListCapes(ctx context.Context, page int, pageSize int) ([]entity.CapeLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListCapes - 获取公开披风列表")

	return l.repo.capeRepo.ListPublic(ctx, nil, page, pageSize)
}

// ListMyCapes 获取当前用户的披风关联列表。
//
// 通过 UserCapeLibrary 查询，包含 Preloaded 的 CapeLibrary 信息。
// 该方法为纯读操作，无需事务包裹。
func (l *LibraryLogic) ListMyCapes(ctx context.Context, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]entity.UserCapeLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListMyCapes - 获取我的披风列表")

	return l.repo.userCapeRepo.ListByUserID(ctx, nil, userID, page, pageSize)
}

// ==================== Quota Logic ====================

// GetQuota 获取指定用户的资源库配额信息。
//
// 若该用户尚无配额记录，Repository 层会自动创建默认配额。
func (l *LibraryLogic) GetQuota(ctx context.Context, userID xSnowflake.SnowflakeID) (*entity.LibraryQuota, *xError.Error) {
	l.log.Info(ctx, "GetQuota - 获取资源库配额")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	quota, _, xErr := l.repo.quotaRepo.GetByUserID(ctx, nil, userID, false)
	if xErr != nil {
		return nil, xErr
	}
	return quota, nil
}

// ==================== Admin Logic ====================

// GiftSkin 管理员向用户赠送皮肤。
func (l *LibraryLogic) GiftSkin(ctx context.Context, operatorID xSnowflake.SnowflakeID, targetUserID xSnowflake.SnowflakeID, skinLibraryID xSnowflake.SnowflakeID, assignmentType entityType.AssignmentType) (*entity.UserSkinLibrary, *xError.Error) {
	l.log.Info(ctx, "GiftSkin - 管理员赠送皮肤")

	if !assignmentType.IsValid() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效的关联类型", true)
	}
	if assignmentType == entityType.AssignmentTypeNormal {
		return nil, xError.NewError(ctx, xError.ParameterError, "管理员赠送不能使用 Normal 类型", true)
	}
	if operatorID == targetUserID {
		return nil, xError.NewError(ctx, xError.ParameterError, "不能向自己赠送资源", true)
	}

	return l.repo.txn.GiftSkinToUser(ctx, targetUserID, skinLibraryID, assignmentType)
}

// RevokeSkin 管理员撤销用户皮肤关联。
func (l *LibraryLogic) RevokeSkin(ctx context.Context, targetUserID xSnowflake.SnowflakeID, skinLibraryID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "RevokeSkin - 管理员撤销皮肤")

	return l.repo.txn.RevokeSkinFromUser(ctx, targetUserID, skinLibraryID)
}

// GiftCape 管理员向用户赠送披风。
func (l *LibraryLogic) GiftCape(ctx context.Context, operatorID xSnowflake.SnowflakeID, targetUserID xSnowflake.SnowflakeID, capeLibraryID xSnowflake.SnowflakeID, assignmentType entityType.AssignmentType) (*entity.UserCapeLibrary, *xError.Error) {
	l.log.Info(ctx, "GiftCape - 管理员赠送披风")

	if !assignmentType.IsValid() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效的关联类型", true)
	}
	if assignmentType == entityType.AssignmentTypeNormal {
		return nil, xError.NewError(ctx, xError.ParameterError, "管理员赠送不能使用 Normal 类型", true)
	}
	if operatorID == targetUserID {
		return nil, xError.NewError(ctx, xError.ParameterError, "不能向自己赠送资源", true)
	}

	return l.repo.txn.GiftCapeToUser(ctx, targetUserID, capeLibraryID, assignmentType)
}

// RevokeCape 管理员撤销用户披风关联。
func (l *LibraryLogic) RevokeCape(ctx context.Context, targetUserID xSnowflake.SnowflakeID, capeLibraryID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "RevokeCape - 管理员撤销披风")

	return l.repo.txn.RevokeCapeFromUser(ctx, targetUserID, capeLibraryID)
}

// RecalculateQuota 重新计算用户配额的 Used 字段。
func (l *LibraryLogic) RecalculateQuota(ctx context.Context, userID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "RecalculateQuota - 重算配额")

	return l.repo.txn.RecalculateQuota(ctx, userID)
}


// ListUserSkins 查询指定用户的皮肤关联列表（管理员视角）。
func (l *LibraryLogic) ListUserSkins(ctx context.Context, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]entity.UserSkinLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListUserSkins - 查询用户皮肤关联")

	return l.repo.userSkinRepo.ListByUserID(ctx, nil, userID, page, pageSize)
}

// ListUserCapes 查询指定用户的披风关联列表（管理员视角）。
func (l *LibraryLogic) ListUserCapes(ctx context.Context, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]entity.UserCapeLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListUserCapes - 查询用户披风关联")

	return l.repo.userCapeRepo.ListByUserID(ctx, nil, userID, page, pageSize)
}

// ==================== Helper Methods ====================

// validateSkinName 校验并规范化皮肤名称。
func (l *LibraryLogic) validateSkinName(ctx context.Context, name string) (string, *xError.Error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < skinNameMinLength || len(trimmedName) > skinNameMaxLength {
		return "", xError.NewError(ctx, xError.ParameterError, "无效皮肤名称长度：必须在 1-64 个字符之间", true)
	}
	return trimmedName, nil
}

// validateCapeName 校验并规范化披风名称。
func (l *LibraryLogic) validateCapeName(ctx context.Context, name string) (string, *xError.Error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < capeNameMinLength || len(trimmedName) > capeNameMaxLength {
		return "", xError.NewError(ctx, xError.ParameterError, "无效披风名称长度：必须在 1-64 个字符之间", true)
	}
	return trimmedName, nil
}

// decodeBase64Texture 解码 Base64 编码的纹理数据。
func (l *LibraryLogic) decodeBase64Texture(ctx context.Context, texture string) ([]byte, *xError.Error) {
	base64Data := texture
	if strings.HasPrefix(texture, "data:") {
		idx := strings.Index(texture, "base64,")
		if idx == -1 {
			return nil, xError.NewError(ctx, xError.ParameterError, "无效的 base64 MIME 格式：缺少 base64, 标记", true)
		}
		base64Data = texture[idx+len("base64,"):]
	}

	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效的 base64 纹理数据", true, err)
	}
	return data, nil
}

// calculateTextureHash 计算纹理数据的 SHA256 哈希值。
func (l *LibraryLogic) calculateTextureHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
