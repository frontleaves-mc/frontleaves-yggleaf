package logic

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/models"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	repotxn "github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/txn"
	bCtx "github.com/frontleaves-mc/frontleaves-yggleaf/pkg/context"
	bBucket "github.com/phalanx-labs/beacon-bucket-sdk"
	bBucketApi "github.com/phalanx-labs/beacon-bucket-sdk/api"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
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

// ==================== Texture URL 解析 ====================

// resolveTextureURL 通过 beacon-bucket SDK 的 Get 方法将 Texture ID 解析为下载链接。
//
// 参数说明:
//   - ctx: 请求上下文
//   - textureID: 数据库中存储的 int64 纹理文件 ID
//
// 返回值:
//   - string: 文件下载链接
//   - *xError.Error: 当 Bucket 服务不可用或文件不存在时返回错误
func (l *LibraryLogic) resolveTextureURL(ctx context.Context, textureID int64) (string, *xError.Error) {
	fileID := strconv.FormatInt(textureID, 10)
	resp, err := l.helper.bucket.Normal.Get(ctx, &bBucketApi.GetRequest{
		FileId: fileID,
	})
	if err != nil {
		return "", xError.NewError(ctx, xError.ServerInternalError, "获取纹理文件信息失败", true, err)
	}
	if resp.GetObj() == nil {
		return "", xError.NewError(ctx, xError.ServerInternalError, "纹理文件元数据缺失", true)
	}
	link := resp.GetObj().GetLink()
	if link == "" {
		return "", xError.NewError(ctx, xError.ServerInternalError, "纹理文件下载链接为空", true)
	}
	return link, nil
}

// cacheVerifyFile 将缓存态文件确认为永久态。
//
// 必须在数据库事务成功后调用。失败仅记录日志不返回错误，
// 因为 CacheVerify 是幂等接口，已确认的文件重复调用不会产生副作用。
func (l *LibraryLogic) cacheVerifyFile(ctx context.Context, fileId string) {
	_, err := l.helper.bucket.Normal.CacheVerify(ctx, &bBucketApi.CacheVerifyRequest{
		FileId: fileId,
	})
	if err != nil {
		l.log.Warn(ctx, fmt.Sprintf("CacheVerify 调用失败，文件可能仍为缓存态: %v", err))
	}
}

// deleteBucketFile 从对象存储中删除指定文件。
//
// 用于业务数据删除后同步清理对应文件。删除失败仅记录日志，
// 不影响业务流程（可通过后续补偿任务清理残留）。
func (l *LibraryLogic) deleteBucketFile(ctx context.Context, textureID int64) {
	fileId := strconv.FormatInt(textureID, 10)
	_, err := l.helper.bucket.Normal.Delete(ctx, &bBucketApi.DeleteRequest{
		FileId: fileId,
	})
	if err != nil {
		l.log.Warn(ctx, fmt.Sprintf("Bucket 文件删除失败(fileId=%s)，存在残留风险: %v", fileId, err))
	}
}

// buildSkinDTO 将 SkinLibrary 实体转换为 SkinDTO。
//
// 在转换过程中调用 bucket.Get 解析 Texture ID 为下载链接。
// 若 Bucket 服务不可用，直接返回错误（不降级）。
func (l *LibraryLogic) buildSkinDTO(ctx context.Context, skin *entity.SkinLibrary) (*models.SkinDTO, *xError.Error) {
	url, xErr := l.resolveTextureURL(ctx, skin.Texture)
	if xErr != nil {
		return nil, xErr
	}
	return &models.SkinDTO{
		ID:          skin.ID,
		UserID:      skin.UserID,
		Name:        skin.Name,
		TextureURL:  url,
		TextureHash: skin.TextureHash,
		Model:       skin.Model,
		IsPublic:    skin.IsPublic,
		UpdatedAt:   skin.UpdatedAt,
	}, nil
}

// buildCapeDTO 将 CapeLibrary 实体转换为 CapeDTO。
func (l *LibraryLogic) buildCapeDTO(ctx context.Context, cape *entity.CapeLibrary) (*models.CapeDTO, *xError.Error) {
	url, xErr := l.resolveTextureURL(ctx, cape.Texture)
	if xErr != nil {
		return nil, xErr
	}
	return &models.CapeDTO{
		ID:          cape.ID,
		UserID:      cape.UserID,
		Name:        cape.Name,
		TextureURL:  url,
		TextureHash: cape.TextureHash,
		IsPublic:    cape.IsPublic,
		UpdatedAt:   cape.UpdatedAt,
	}, nil
}

// buildSkinDTOs 批量将 SkinLibrary 实体列表转换为 SkinDTO 列表。
//
// 逐个调用 resolveTextureURL 解析纹理链接。若任一解析失败，整个批次中止并返回错误。
func (l *LibraryLogic) buildSkinDTOs(ctx context.Context, skins []entity.SkinLibrary) ([]models.SkinDTO, *xError.Error) {
	responses := make([]models.SkinDTO, len(skins))
	for i, skin := range skins {
		resp, xErr := l.buildSkinDTO(ctx, &skin)
		if xErr != nil {
			return nil, xErr
		}
		responses[i] = *resp
	}
	return responses, nil
}

// buildCapeDTOs 批量将 CapeLibrary 实体列表转换为 CapeDTO 列表。
func (l *LibraryLogic) buildCapeDTOs(ctx context.Context, capes []entity.CapeLibrary) ([]models.CapeDTO, *xError.Error) {
	responses := make([]models.CapeDTO, len(capes))
	for i, cape := range capes {
		resp, xErr := l.buildCapeDTO(ctx, &cape)
		if xErr != nil {
			return nil, xErr
		}
		responses[i] = *resp
	}
	return responses, nil
}

// buildSkinSimpleDTOs 将 UserSkinLibrary 关联列表转换为 SkinSimpleDTO 列表（仅 ID + Name）。
func (l *LibraryLogic) buildSkinSimpleDTOs(associations []entity.UserSkinLibrary) []models.SkinSimpleDTO {
	items := make([]models.SkinSimpleDTO, 0, len(associations))
	for _, assoc := range associations {
		if assoc.SkinLibrary != nil {
			items = append(items, models.SkinSimpleDTO{
				ID:   assoc.SkinLibrary.ID,
				Name: assoc.SkinLibrary.Name,
			})
		}
	}
	return items
}

// buildCapeSimpleDTOs 将 UserCapeLibrary 关联列表转换为 CapeSimpleDTO 列表（仅 ID + Name）。
func (l *LibraryLogic) buildCapeSimpleDTOs(associations []entity.UserCapeLibrary) []models.CapeSimpleDTO {
	items := make([]models.CapeSimpleDTO, 0, len(associations))
	for _, assoc := range associations {
		if assoc.CapeLibrary != nil {
			items = append(items, models.CapeSimpleDTO{
				ID:   assoc.CapeLibrary.ID,
				Name: assoc.CapeLibrary.Name,
			})
		}
	}
	return items
}

// buildUserSkinAssociationDTOs 将 UserSkinLibrary 关联列表转换为 SkinDTO 列表。
//
// 从关联实体中提取 SkinLibrary（GORM Preload）和 AssignmentType，
// 并调用 bucket.Get 解析纹理链接。
func (l *LibraryLogic) buildUserSkinAssociationDTOs(ctx context.Context, associations []entity.UserSkinLibrary) ([]models.SkinDTO, *xError.Error) {
	responses := make([]models.SkinDTO, len(associations))
	for i, assoc := range associations {
		resp := models.SkinDTO{
			AssignmentType: assoc.AssignmentType,
		}
		if assoc.SkinLibrary != nil {
			url, xErr := l.resolveTextureURL(ctx, assoc.SkinLibrary.Texture)
			if xErr != nil {
				return nil, xErr
			}
			resp.ID = assoc.SkinLibrary.ID
			resp.UserID = assoc.SkinLibrary.UserID
			resp.Name = assoc.SkinLibrary.Name
			resp.TextureURL = url
			resp.TextureHash = assoc.SkinLibrary.TextureHash
			resp.Model = assoc.SkinLibrary.Model
			resp.IsPublic = assoc.SkinLibrary.IsPublic
			resp.UpdatedAt = assoc.SkinLibrary.UpdatedAt
		}
		responses[i] = resp
	}
	return responses, nil
}

// buildUserCapeAssociationDTOs 将 UserCapeLibrary 关联列表转换为 CapeDTO 列表。
func (l *LibraryLogic) buildUserCapeAssociationDTOs(ctx context.Context, associations []entity.UserCapeLibrary) ([]models.CapeDTO, *xError.Error) {
	responses := make([]models.CapeDTO, len(associations))
	for i, assoc := range associations {
		resp := models.CapeDTO{
			AssignmentType: assoc.AssignmentType,
		}
		if assoc.CapeLibrary != nil {
			url, xErr := l.resolveTextureURL(ctx, assoc.CapeLibrary.Texture)
			if xErr != nil {
				return nil, xErr
			}
			resp.ID = assoc.CapeLibrary.ID
			resp.UserID = assoc.CapeLibrary.UserID
			resp.Name = assoc.CapeLibrary.Name
			resp.TextureURL = url
			resp.TextureHash = assoc.CapeLibrary.TextureHash
			resp.IsPublic = assoc.CapeLibrary.IsPublic
			resp.UpdatedAt = assoc.CapeLibrary.UpdatedAt
		}
		responses[i] = resp
	}
	return responses, nil
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
func (l *LibraryLogic) CreateSkin(ctx context.Context, userID xSnowflake.SnowflakeID, name string, modelType uint8, texture string, isPublic *bool) (*models.SkinDTO, *xError.Error) {
	l.log.Info(ctx, "CreateSkin - 创建皮肤")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	validatedName, xErr := l.validateSkinName(ctx, name)
	if xErr != nil {
		return nil, xErr
	}

	model := entity.ModelType(modelType)
	if model != entity.ModelTypeClassic && model != entity.ModelTypeSlim {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效皮肤模型类型", true)
	}

	textureData, xErr := l.decodeBase64Texture(ctx, texture)
	if xErr != nil {
		return nil, xErr
	}
	textureHash := l.calculateTextureHash(textureData)

	// 上传到对象存储（事务外执行，避免长事务占用连接）
	uploadResp, err := l.helper.bucket.Normal.Upload(ctx, &bBucketApi.UploadRequest{
		BucketId:      xEnv.GetEnvString(bConst.EnvBucketSkinBucketId, ""),
		PathId:        xEnv.GetEnvString(bConst.EnvBucketSkinPathId, ""),
		ContentBase64: texture,
	})
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "上传皮肤纹理失败", true, err)
	}

	skinId, err := strconv.ParseInt(uploadResp.FileId, 10, 64)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "解析纹理文件 ID 失败", true, err)
	}
	isPublicVal := false
	if isPublic != nil {
		isPublicVal = *isPublic
	}
	skin := &entity.SkinLibrary{
		UserID:      &userID,
		Name:        validatedName,
		Texture:     skinId,
		TextureHash: textureHash,
		Model:       model,
		IsPublic:    isPublicVal,
	}

	// 委托 Repository 层在事务内完成创建、关联与配额操作
	createdSkin, xErr := l.repo.txn.CreateSkinWithQuota(ctx, skin)
	if xErr != nil {
		return nil, xErr
	}

	// 事务成功后确认文件转为永久态（必须在 DB 写入成功后调用）
	l.cacheVerifyFile(ctx, uploadResp.FileId)

	// 将 entity 转换为 DTO（含纹理链接解析）
	skinDTO, xErr := l.buildSkinDTO(ctx, createdSkin)
	if xErr != nil {
		return nil, xErr
	}

	// M3 优化：复用 UploadResponse 中已有的下载链接，避免冗余 Get 调用
	if uploadResp.GetObj() != nil && uploadResp.GetObj().GetLink() != "" {
		skinDTO.TextureURL = uploadResp.GetObj().GetLink()
	}
	return skinDTO, nil
}

// UpdateSkin 更新皮肤（名称/公开状态）。
//
// 该方法执行以下业务流程：
//  1. 通过 UserSkinLibrary 校验用户是否关联该皮肤
//  2. 获取皮肤记录并校验（系统内置皮肤不可修改）
//  3. 解析并校验新参数（名称/公开状态）
//  4. 短路优化：若所有字段均未变更则直接返回
//  5. 委托 Repository 层在事务内完成：公开状态变更时的配额调整 → 皮肤记录更新
func (l *LibraryLogic) UpdateSkin(ctx context.Context, userID xSnowflake.SnowflakeID, skinID xSnowflake.SnowflakeID, newName *string, newIsPublic *bool) (*models.SkinDTO, *xError.Error) {
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

	newNameVal := skin.Name
	newIsPublicVal := skin.IsPublic

	if newName != nil {
		validatedName, xErr := l.validateSkinName(ctx, *newName)
		if xErr != nil {
			return nil, xErr
		}
		newNameVal = validatedName
	}

	if newIsPublic != nil {
		newIsPublicVal = *newIsPublic
	}

	if skin.Name == newNameVal && skin.IsPublic == newIsPublicVal {
		return l.buildSkinDTO(ctx, skin)
	}

	// 委托 Repository 层在事务内完成配额调整与记录更新
	updatedSkin, xErr := l.repo.txn.UpdateSkinWithQuota(ctx, userID, skinID, newNameVal, newIsPublicVal, skin.IsPublic)
	if xErr != nil {
		return nil, xErr
	}
	return l.buildSkinDTO(ctx, updatedSkin)
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
	xErr = l.repo.txn.DeleteSkinWithQuota(ctx, userID, skinID)
	if xErr != nil {
		return xErr
	}

	// DB 删除成功后同步清理 Bucket 中的文件
	l.deleteBucketFile(ctx, skin.Texture)
	return nil
}

// ListSkins 获取市场公开皮肤列表。
//
// 分页获取所有公开的皮肤。该方法为纯读操作，无需事务包裹。
func (l *LibraryLogic) ListSkins(ctx context.Context, page int, pageSize int) ([]models.SkinDTO, int64, *xError.Error) {
	l.log.Info(ctx, "ListSkins - 获取公开皮肤列表")

	skins, total, xErr := l.repo.skinRepo.ListPublic(ctx, nil, page, pageSize)
	if xErr != nil {
		return nil, 0, xErr
	}

	responses, xErr := l.buildSkinDTOs(ctx, skins)
	if xErr != nil {
		return nil, 0, xErr
	}

	return responses, total, nil
}

// ListMySkins 获取当前用户的皮肤关联列表。
//
// 通过 UserSkinLibrary 查询，包含 Preloaded 的 SkinLibrary 信息。
// 该方法为纯读操作，无需事务包裹。
func (l *LibraryLogic) ListMySkins(ctx context.Context, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]models.SkinDTO, int64, *xError.Error) {
	l.log.Info(ctx, "ListMySkins - 获取我的皮肤列表")

	associations, total, xErr := l.repo.userSkinRepo.ListByUserID(ctx, nil, userID, page, pageSize)
	if xErr != nil {
		return nil, 0, xErr
	}

	responses, xErr := l.buildUserSkinAssociationDTOs(ctx, associations)
	if xErr != nil {
		return nil, 0, xErr
	}

	return responses, total, nil
}

// ListMySkinsSimple 获取当前用户的皮肤精简列表（仅 ID + Name，不分页）。
//
// 用于前端皮肤选择器等轻量场景，不解析纹理链接。
func (l *LibraryLogic) ListMySkinsSimple(ctx context.Context, userID xSnowflake.SnowflakeID) ([]models.SkinSimpleDTO, *xError.Error) {
	l.log.Info(ctx, "ListMySkinsSimple - 获取我的皮肤精简列表")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	associations, xErr := l.repo.userSkinRepo.ListAllByUserID(ctx, nil, userID)
	if xErr != nil {
		return nil, xErr
	}

	return l.buildSkinSimpleDTOs(associations), nil
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
func (l *LibraryLogic) CreateCape(ctx context.Context, userID xSnowflake.SnowflakeID, name string, texture string, isPublic *bool) (*models.CapeDTO, *xError.Error) {
	l.log.Info(ctx, "CreateCape - 创建披风")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	validatedName, xErr := l.validateCapeName(ctx, name)
	if xErr != nil {
		return nil, xErr
	}

	textureData, xErr := l.decodeBase64Texture(ctx, texture)
	if xErr != nil {
		return nil, xErr
	}
	textureHash := l.calculateTextureHash(textureData)

	// 上传到对象存储（事务外执行，避免长事务占用连接）
	uploadResp, err := l.helper.bucket.Normal.Upload(ctx, &bBucketApi.UploadRequest{
		BucketId:      xEnv.GetEnvString(bConst.EnvBucketCapeBucketId, ""),
		PathId:        xEnv.GetEnvString(bConst.EnvBucketCapePathId, ""),
		ContentBase64: texture,
	})
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "上传披风纹理失败", true, err)
	}

	capeId, err := strconv.ParseInt(uploadResp.FileId, 10, 64)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "解析纹理文件 ID 失败", true, err)
	}
	isPublicVal := false
	if isPublic != nil {
		isPublicVal = *isPublic
	}
	cape := &entity.CapeLibrary{
		UserID:      &userID,
		Name:        validatedName,
		Texture:     capeId,
		TextureHash: textureHash,
		IsPublic:    isPublicVal,
	}

	// 委托 Repository 层在事务内完成创建、关联与配额操作
	createdCape, xErr := l.repo.txn.CreateCapeWithQuota(ctx, cape)
	if xErr != nil {
		return nil, xErr
	}

	// 事务成功后确认文件转为永久态（必须在 DB 写入成功后调用）
	l.cacheVerifyFile(ctx, uploadResp.FileId)

	// 将 entity 转换为 DTO（含纹理链接解析）
	capeDTO, xErr := l.buildCapeDTO(ctx, createdCape)
	if xErr != nil {
		return nil, xErr
	}

	// M3 优化：复用 UploadResponse 中已有的下载链接，避免冗余 Get 调用
	if uploadResp.GetObj() != nil && uploadResp.GetObj().GetLink() != "" {
		capeDTO.TextureURL = uploadResp.GetObj().GetLink()
	}
	return capeDTO, nil
}

// UpdateCape 更新披风（名称/公开状态）。
//
// 该方法执行以下业务流程：
//  1. 通过 UserCapeLibrary 校验用户是否关联该披风
//  2. 获取披风记录并校验（系统内置披风不可修改）
//  3. 解析并校验新参数（名称/公开状态）
//  4. 短路优化：若所有字段均未变更则直接返回
//  5. 委托 Repository 层在事务内完成：公开状态变更时的配额调整 → 披风记录更新
func (l *LibraryLogic) UpdateCape(ctx context.Context, userID xSnowflake.SnowflakeID, capeID xSnowflake.SnowflakeID, newName *string, newIsPublic *bool) (*models.CapeDTO, *xError.Error) {
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

	newNameVal := cape.Name
	newIsPublicVal := cape.IsPublic

	if newName != nil {
		validatedName, xErr := l.validateCapeName(ctx, *newName)
		if xErr != nil {
			return nil, xErr
		}
		newNameVal = validatedName
	}

	if newIsPublic != nil {
		newIsPublicVal = *newIsPublic
	}

	if cape.Name == newNameVal && cape.IsPublic == newIsPublicVal {
		return l.buildCapeDTO(ctx, cape)
	}

	// 委托 Repository 层在事务内完成配额调整与记录更新
	updatedCape, xErr := l.repo.txn.UpdateCapeWithQuota(ctx, userID, capeID, newNameVal, newIsPublicVal, cape.IsPublic)
	if xErr != nil {
		return nil, xErr
	}
	return l.buildCapeDTO(ctx, updatedCape)
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
	xErr = l.repo.txn.DeleteCapeWithQuota(ctx, userID, capeID)
	if xErr != nil {
		return xErr
	}

	// DB 删除成功后同步清理 Bucket 中的文件
	l.deleteBucketFile(ctx, cape.Texture)
	return nil
}

// ListCapes 获取市场公开披风列表。
//
// 分页获取所有公开的披风。该方法为纯读操作，无需事务包裹。
func (l *LibraryLogic) ListCapes(ctx context.Context, page int, pageSize int) ([]models.CapeDTO, int64, *xError.Error) {
	l.log.Info(ctx, "ListCapes - 获取公开披风列表")

	capes, total, xErr := l.repo.capeRepo.ListPublic(ctx, nil, page, pageSize)
	if xErr != nil {
		return nil, 0, xErr
	}

	responses, xErr := l.buildCapeDTOs(ctx, capes)
	if xErr != nil {
		return nil, 0, xErr
	}

	return responses, total, nil
}

// ListMyCapes 获取当前用户的披风关联列表。
//
// 通过 UserCapeLibrary 查询，包含 Preloaded 的 CapeLibrary 信息。
// 该方法为纯读操作，无需事务包裹。
func (l *LibraryLogic) ListMyCapes(ctx context.Context, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]models.CapeDTO, int64, *xError.Error) {
	l.log.Info(ctx, "ListMyCapes - 获取我的披风列表")

	associations, total, xErr := l.repo.userCapeRepo.ListByUserID(ctx, nil, userID, page, pageSize)
	if xErr != nil {
		return nil, 0, xErr
	}

	responses, xErr := l.buildUserCapeAssociationDTOs(ctx, associations)
	if xErr != nil {
		return nil, 0, xErr
	}

	return responses, total, nil
}

// ListMyCapesSimple 获取当前用户的披风精简列表（仅 ID + Name，不分页）。
//
// 用于前端披风选择器等轻量场景，不解析纹理链接。
func (l *LibraryLogic) ListMyCapesSimple(ctx context.Context, userID xSnowflake.SnowflakeID) ([]models.CapeSimpleDTO, *xError.Error) {
	l.log.Info(ctx, "ListMyCapesSimple - 获取我的披风精简列表")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	associations, xErr := l.repo.userCapeRepo.ListAllByUserID(ctx, nil, userID)
	if xErr != nil {
		return nil, xErr
	}

	return l.buildCapeSimpleDTOs(associations), nil
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
func (l *LibraryLogic) GiftSkin(ctx context.Context, operatorID xSnowflake.SnowflakeID, targetUserID xSnowflake.SnowflakeID, skinLibraryID xSnowflake.SnowflakeID, assignmentType entityType.AssignmentType) (*models.SkinDTO, *xError.Error) {
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

	result, xErr := l.repo.txn.GiftSkinToUser(ctx, targetUserID, skinLibraryID, assignmentType)
	if xErr != nil {
		return nil, xErr
	}

	// 单独查询皮肤实体（事务方法未 Preload SkinLibrary）
	skinEntity, found, xErr := l.repo.skinRepo.GetByID(ctx, nil, skinLibraryID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "皮肤资源不存在", true)
	}

	skinResp, xErr := l.buildSkinDTO(ctx, skinEntity)
	if xErr != nil {
		return nil, xErr
	}
	skinResp.AssignmentType = result.AssignmentType
	return skinResp, nil
}

// RevokeSkin 管理员撤销用户皮肤关联。
func (l *LibraryLogic) RevokeSkin(ctx context.Context, targetUserID xSnowflake.SnowflakeID, skinLibraryID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "RevokeSkin - 管理员撤销皮肤")

	return l.repo.txn.RevokeSkinFromUser(ctx, targetUserID, skinLibraryID)
}

// GiftCape 管理员向用户赠送披风。
func (l *LibraryLogic) GiftCape(ctx context.Context, operatorID xSnowflake.SnowflakeID, targetUserID xSnowflake.SnowflakeID, capeLibraryID xSnowflake.SnowflakeID, assignmentType entityType.AssignmentType) (*models.CapeDTO, *xError.Error) {
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

	result, xErr := l.repo.txn.GiftCapeToUser(ctx, targetUserID, capeLibraryID, assignmentType)
	if xErr != nil {
		return nil, xErr
	}

	// 单独查询披风实体（事务方法未 Preload CapeLibrary）
	capeEntity, found, xErr := l.repo.capeRepo.GetByID(ctx, nil, capeLibraryID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "披风资源不存在", true)
	}

	capeResp, xErr := l.buildCapeDTO(ctx, capeEntity)
	if xErr != nil {
		return nil, xErr
	}
	capeResp.AssignmentType = result.AssignmentType
	return capeResp, nil
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
func (l *LibraryLogic) ListUserSkins(ctx context.Context, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]models.SkinDTO, int64, *xError.Error) {
	l.log.Info(ctx, "ListUserSkins - 查询用户皮肤关联")

	associations, total, xErr := l.repo.userSkinRepo.ListByUserID(ctx, nil, userID, page, pageSize)
	if xErr != nil {
		return nil, 0, xErr
	}

	responses, xErr := l.buildUserSkinAssociationDTOs(ctx, associations)
	if xErr != nil {
		return nil, 0, xErr
	}

	return responses, total, nil
}

// ListUserCapes 查询指定用户的披风关联列表（管理员视角）。
func (l *LibraryLogic) ListUserCapes(ctx context.Context, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]models.CapeDTO, int64, *xError.Error) {
	l.log.Info(ctx, "ListUserCapes - 查询用户披风关联")

	associations, total, xErr := l.repo.userCapeRepo.ListByUserID(ctx, nil, userID, page, pageSize)
	if xErr != nil {
		return nil, 0, xErr
	}

	responses, xErr := l.buildUserCapeAssociationDTOs(ctx, associations)
	if xErr != nil {
		return nil, 0, xErr
	}

	return responses, total, nil
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
