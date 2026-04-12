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
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	repotxn "github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/txn"
	bCtx "github.com/frontleaves-mc/frontleaves-yggleaf/pkg/context"
	"github.com/gin-gonic/gin"
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
// 以及事务协调仓储（TxnRepo），供 LibraryLogic 统一调用。
type libraryRepo struct {
	skinRepo  *repository.SkinLibraryRepo  // 皮肤库仓储
	capeRepo  *repository.CapeLibraryRepo  // 披风库仓储
	quotaRepo *repository.LibraryQuotaRepo // 资源库配额仓储
	txn       *repotxn.LibraryTxnRepo   // 资源库事务协调仓储
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

	return &LibraryLogic{
		logic: logic{
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "LibraryLogic"),
		},
		repo: libraryRepo{
			skinRepo:  skinRepo,
			capeRepo:  capeRepo,
			quotaRepo: quotaRepo,
			txn:      repotxn.NewLibraryTxnRepo(db, skinRepo, capeRepo, quotaRepo),
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
//  7. 委托 Repository 层在事务内完成：配额检查 → 哈希去重 → 记录创建 → 配额扣减
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID。
//   - req: 创建皮肤的请求结构体，包含名称、模型类型和 Base64 编码的纹理数据。
//
// 返回值:
//   - *entity.SkinLibrary: 创建成功的皮肤实体；若纹理哈希已存在且属于当前用户则直接返回已有记录。
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *LibraryLogic) CreateSkin(ctx *gin.Context, userID xSnowflake.SnowflakeID, req *apiLibrary.CreateSkinRequest) (*entity.SkinLibrary, *xError.Error) {
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
		BucketId:      "yggleaf",
		PathId:        "skins",
		ContentBase64: req.Texture,
	})
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "上传皮肤纹理失败", true, err)
	}

	skinId, _ := strconv.ParseInt(uploadResp.FileId, 10, 64)
	skin := &entity.SkinLibrary{
		UserID:      &userID,
		Name:        name,
		Texture:     skinId,
		TextureHash: textureHash,
		Model:       model,
		IsPublic:    false,
	}

	// 委托 Repository 层在事务内完成创建与配额操作
	return l.repo.txn.CreateSkinWithQuota(ctx.Request.Context(), skin)
}

// UpdateSkin 更新皮肤（名称/公开状态）。
//
// 该方法执行以下业务流程：
//  1. 获取皮肤记录并校验归属权（系统内置皮肤不可修改）
//  2. 解析并校验新参数（名称/公开状态）
//  3. 短路优化：若所有字段均未变更则直接返回
//  4. 委托 Repository 层在事务内完成：公开状态变更时的配额调整 → 皮肤记录更新
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID。
//   - skinID: 目标皮肤的雪花 ID。
//   - req: 更新皮肤的请求结构体，名称和公开状态均为可选字段。
//
// 返回值:
//   - *entity.SkinLibrary: 更新后的皮肤实体。
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *LibraryLogic) UpdateSkin(ctx *gin.Context, userID xSnowflake.SnowflakeID, skinID xSnowflake.SnowflakeID, req *apiLibrary.UpdateSkinRequest) (*entity.SkinLibrary, *xError.Error) {
	l.log.Info(ctx, "UpdateSkin - 更新皮肤")

	skin, found, xErr := l.repo.skinRepo.GetByIDAndUserID(ctx.Request.Context(), nil, skinID, userID, false)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "皮肤不存在", true)
	}

	if skin.UserID == nil {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "系统内置皮肤不能修改", true)
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
	return l.repo.txn.UpdateSkinWithQuota(ctx.Request.Context(), userID, skinID, newName, newIsPublic, skin.IsPublic)
}

// DeleteSkin 删除皮肤。
//
// 该方法执行以下业务流程：
//  1. 获取皮肤记录并校验归属权（系统内置皮肤不可删除）
//  2. 委托 Repository 层在事务内完成：对应配额释放 → 皮肤记录删除
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID。
//   - skinID: 目标皮肤的雪花 ID。
//
// 返回值:
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *LibraryLogic) DeleteSkin(ctx *gin.Context, userID xSnowflake.SnowflakeID, skinID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "DeleteSkin - 删除皮肤")

	skin, found, xErr := l.repo.skinRepo.GetByIDAndUserID(ctx.Request.Context(), nil, skinID, userID, false)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ResourceNotFound, "皮肤不存在", true)
	}

	if skin.UserID == nil {
		return xError.NewError(ctx, xError.PermissionDenied, "系统内置皮肤不能删除", true)
	}

	// 委托 Repository 层在事务内完成配额释放与记录删除
	return l.repo.txn.DeleteSkinWithQuota(ctx.Request.Context(), userID, skinID, skin.IsPublic)
}

// ListSkins 获取皮肤列表。
//
// 支持两种查询模式：
//   - market（市场模式）：分页获取所有公开的皮肤
//   - mine（我的模式）：分页获取当前用户的全部皮肤
//
// 该方法为纯读操作，无需事务包裹。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID（mine 模式下使用）。
//   - mode: 查询模式，"market" 或其他值（默认为 mine）。
//   - page: 页码（从 1 开始）。
//   - pageSize: 每页数量。
//
// 返回值:
//   - []entity.SkinLibrary: 当前页的皮肤列表。
//   - int64: 符合条件的总记录数（用于分页计算）。
//   - *xError.Error: 数据查询过程中发生的错误。
func (l *LibraryLogic) ListSkins(ctx *gin.Context, userID xSnowflake.SnowflakeID, mode string, page int, pageSize int) ([]entity.SkinLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListSkins - 获取皮肤列表")

	if mode == "market" {
		return l.repo.skinRepo.ListPublic(ctx.Request.Context(), nil, page, pageSize)
	}

	return l.repo.skinRepo.ListByUserID(ctx.Request.Context(), nil, userID, page, pageSize)
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
//  6. 委托 Repository 层在事务内完成：配额检查 → 哈希去重 → 记录创建 → 配额扣减
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID。
//   - req: 创建披风的请求结构体，包含名称和 Base64 编码的纹理数据。
//
// 返回值:
//   - *entity.CapeLibrary: 创建成功的披风实体；若纹理哈希已存在且属于当前用户则直接返回已有记录。
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *LibraryLogic) CreateCape(ctx *gin.Context, userID xSnowflake.SnowflakeID, req *apiLibrary.CreateCapeRequest) (*entity.CapeLibrary, *xError.Error) {
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

	capeId, _ := strconv.ParseInt(uploadResp.FileId, 10, 64)
	cape := &entity.CapeLibrary{
		UserID:      &userID,
		Name:        name,
		Texture:     capeId,
		TextureHash: textureHash,
		IsPublic:    false,
	}

	// 委托 Repository 层在事务内完成创建与配额操作
	return l.repo.txn.CreateCapeWithQuota(ctx.Request.Context(), cape)
}

// UpdateCape 更新披风（名称/公开状态）。
//
// 该方法执行以下业务流程：
//  1. 获取披风记录并校验归属权（系统内置披风不可修改）
//  2. 解析并校验新参数（名称/公开状态）
//  3. 短路优化：若所有字段均未变更则直接返回
//  4. 委托 Repository 层在事务内完成：公开状态变更时的配额调整 → 披风记录更新
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID。
//   - capeID: 目标披风的雪花 ID。
//   - req: 更新披风的请求结构体，名称和公开状态均为可选字段。
//
// 返回值:
//   - *entity.CapeLibrary: 更新后的披风实体。
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *LibraryLogic) UpdateCape(ctx *gin.Context, userID xSnowflake.SnowflakeID, capeID xSnowflake.SnowflakeID, req *apiLibrary.UpdateCapeRequest) (*entity.CapeLibrary, *xError.Error) {
	l.log.Info(ctx, "UpdateCape - 更新披风")

	cape, found, xErr := l.repo.capeRepo.GetByIDAndUserID(ctx.Request.Context(), nil, capeID, userID, false)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "披风不存在", true)
	}

	if cape.UserID == nil {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "系统内置披风不能修改", true)
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
	return l.repo.txn.UpdateCapeWithQuota(ctx.Request.Context(), userID, capeID, newName, newIsPublic, cape.IsPublic)
}

// DeleteCape 删除披风。
//
// 该方法执行以下业务流程：
//  1. 获取披风记录并校验归属权（系统内置披风不可删除）
//  2. 委托 Repository 层在事务内完成：对应配额释放 → 披风记录删除
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID。
//   - capeID: 目标披风的雪花 ID。
//
// 返回值:
//   - *xError.Error: 业务校验失败或数据操作过程中发生的错误。
func (l *LibraryLogic) DeleteCape(ctx *gin.Context, userID xSnowflake.SnowflakeID, capeID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "DeleteCape - 删除披风")

	cape, found, xErr := l.repo.capeRepo.GetByIDAndUserID(ctx.Request.Context(), nil, capeID, userID, false)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ResourceNotFound, "披风不存在", true)
	}

	if cape.UserID == nil {
		return xError.NewError(ctx, xError.PermissionDenied, "系统内置披风不能删除", true)
	}

	// 委托 Repository 层在事务内完成配额释放与记录删除
	return l.repo.txn.DeleteCapeWithQuota(ctx.Request.Context(), userID, capeID, cape.IsPublic)
}

// ListCapes 获取披风列表。
//
// 支持两种查询模式：
//   - market（市场模式）：分页获取所有公开的披风
//   - mine（我的模式）：分页获取当前用户的全部披风
//
// 该方法为纯读操作，无需事务包裹。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - userID: 操作者的雪花 ID（mine 模式下使用）。
//   - mode: 查询模式，"market" 或其他值（默认为 mine）。
//   - page: 页码（从 1 开始）。
//   - pageSize: 每页数量。
//
// 返回值:
//   - []entity.CapeLibrary: 当前页的披风列表。
//   - int64: 符合条件的总记录数（用于分页计算）。
//   - *xError.Error: 数据查询过程中发生的错误。
func (l *LibraryLogic) ListCapes(ctx *gin.Context, userID xSnowflake.SnowflakeID, mode string, page int, pageSize int) ([]entity.CapeLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListCapes - 获取披风列表")

	if mode == "market" {
		return l.repo.capeRepo.ListPublic(ctx.Request.Context(), nil, page, pageSize)
	}

	return l.repo.capeRepo.ListByUserID(ctx.Request.Context(), nil, userID, page, pageSize)
}

// ==================== Helper Methods ====================

// validateSkinName 校验并规范化皮肤名称。
//
// 仅校验长度是否在合法范围内（1-64 字符），自动去除首尾空白。
//
// 参数:
//   - ctx: Gin 上下文对象，用于构造错误响应。
//   - name: 待校验的原始名称字符串。
//
// 返回值:
//   - string: 规范化后的名称（已去除首尾空白）。
//   - *xError.Error: 校验失败时返回具体错误信息。
func (l *LibraryLogic) validateSkinName(ctx *gin.Context, name string) (string, *xError.Error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < skinNameMinLength || len(trimmedName) > skinNameMaxLength {
		return "", xError.NewError(ctx, xError.ParameterError, "无效皮肤名称长度：必须在 1-64 个字符之间", true)
	}
	return trimmedName, nil
}

// validateCapeName 校验并规范化披风名称。
//
// 仅校验长度是否在合法范围内（1-64 字符），自动去除首尾空白。
//
// 参数:
//   - ctx: Gin 上下文对象，用于构造错误响应。
//   - name: 待校验的原始名称字符串。
//
// 返回值:
//   - string: 规范化后的名称（已去除首尾空白）。
//   - *xError.Error: 校验失败时返回具体错误信息。
func (l *LibraryLogic) validateCapeName(ctx *gin.Context, name string) (string, *xError.Error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < capeNameMinLength || len(trimmedName) > capeNameMaxLength {
		return "", xError.NewError(ctx, xError.ParameterError, "无效披风名称长度：必须在 1-64 个字符之间", true)
	}
	return trimmedName, nil
}

// decodeBase64Texture 解码 Base64 编码的纹理数据。
//
// 支持标准 Base64 和 MIME 类型 Base64（如 data:image/png;base64,...）两种格式。
//
// 参数:
//   - ctx: Gin 上下文对象，用于构造错误响应。
//   - texture: Base64 编码的纹理字符串。
//
// 返回值:
//   - []byte: 解码后的原始二进制数据。
//   - *xError.Error: 解码失败时返回具体错误信息。
func (l *LibraryLogic) decodeBase64Texture(ctx *gin.Context, texture string) ([]byte, *xError.Error) {
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
//
// 用于皮肤/披风纹理的去重判断，相同纹理内容的文件会产生相同的哈希值。
//
// 参数:
//   - data: 纹理的原始二进制数据。
//
// 返回值:
//   - string: 十六进制编码的 SHA256 哈希字符串。
func (l *LibraryLogic) calculateTextureHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
