package logic

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"

	xError "github.com/bamboo-services/bamboo-base-go/error"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/utility/ctxutil"
	apiLibrary "github.com/frontleaves-mc/frontleaves-yggleaf/api/library"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	bCtx "github.com/frontleaves-mc/frontleaves-yggleaf/pkg/context"
	"github.com/gin-gonic/gin"
	bBucketApi "github.com/phalanx-labs/beacon-bucket-sdk/api"
	"gorm.io/gorm"
)

const (
	skinNameMinLength = 1
	skinNameMaxLength = 64
	capeNameMinLength = 1
	capeNameMaxLength = 64
)

type libraryRepo struct {
	skinRepo  *repository.SkinLibraryRepo
	capeRepo  *repository.CapeLibraryRepo
	quotaRepo *repository.LibraryQuotaRepo
}

// LibraryLogic 资源库业务逻辑层
type LibraryLogic struct {
	logic
	repo libraryRepo
}

// NewLibraryLogic 创建 LibraryLogic 实例
func NewLibraryLogic(ctx context.Context) *LibraryLogic {
	db := xCtxUtil.MustGetDB(ctx)
	rdb := xCtxUtil.MustGetRDB(ctx)
	return &LibraryLogic{
		logic: logic{
			db:  db,
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "LibraryLogic"),
		},
		repo: libraryRepo{
			skinRepo:  repository.NewSkinLibraryRepo(db),
			capeRepo:  repository.NewCapeLibraryRepo(db),
			quotaRepo: repository.NewLibraryQuotaRepo(db),
		},
	}
}

// ==================== Skin Logic ====================

// CreateSkin 创建皮肤
func (l *LibraryLogic) CreateSkin(ctx *gin.Context, userID xSnowflake.SnowflakeID, req *apiLibrary.CreateSkinRequest) (*entity.SkinLibrary, *xError.Error) {
	l.log.Info(ctx, "CreateSkin - 创建皮肤")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	// 验证皮肤名称
	name, xErr := l.validateSkinName(ctx, req.Name)
	if xErr != nil {
		return nil, xErr
	}

	// 验证模型类型
	model := entity.ModelType(req.Model)
	if model != entity.ModelTypeClassic && model != entity.ModelTypeSlim {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效皮肤模型类型", true)
	}

	// 解码 base64 纹理
	textureData, xErr := l.decodeBase64Texture(ctx, req.Texture)
	if xErr != nil {
		return nil, xErr
	}

	// 计算纹理哈希
	textureHash := l.calculateTextureHash(textureData)

	var createdSkin *entity.SkinLibrary
	var bizErr *xError.Error

	err := l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 检查配额
		quota, found, xErr := l.repo.quotaRepo.GetByUserID(ctx.Request.Context(), tx, userID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}
		if quota.SkinsPrivateUsed >= quota.SkinsPrivateTotal {
			bizErr = xError.NewError(ctx, xError.ResourceExhausted, "私有皮肤配额不足", true)
			return bizErr
		}

		// 检查纹理哈希是否已存在
		existingSkin, found, xErr := l.repo.skinRepo.GetByTextureHash(ctx.Request.Context(), tx, textureHash)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if found {
			// 如果已存在且是当前用户的，直接返回
			if existingSkin.UserID != nil && *existingSkin.UserID == userID {
				createdSkin = existingSkin
				return nil
			}
			// 如果已存在但不是当前用户的，返回冲突错误
			bizErr = xError.NewError(ctx, xError.DataConflict, "该皮肤纹理已存在", true)
			return bizErr
		}

		// 上传到存储桶
		bucketClient := bCtx.MustGetBucket(ctx)
		uploadResp, err := bucketClient.Normal.Upload(ctx, &bBucketApi.UploadRequest{
			BucketId:      "yggleaf",
			PathId:        "skins",
			ContentBase64: req.Texture,
		})
		if err != nil {
			bizErr = xError.NewError(ctx, xError.ServerInternalError, "上传皮肤纹理失败", true, err)
			return bizErr
		}

		// 创建皮肤记录
		skinId, _ := strconv.ParseInt(uploadResp.FileId, 10, 64)
		skin := &entity.SkinLibrary{
			UserID:      &userID,
			Name:        name,
			Texture:     skinId,
			TextureHash: textureHash,
			Model:       model,
			IsPublic:    false,
		}

		createdSkin, bizErr = l.repo.skinRepo.Create(ctx.Request.Context(), tx, skin)
		if bizErr != nil {
			return bizErr
		}

		// 更新配额
		bizErr = l.repo.quotaRepo.UpdateSkinsPrivateUsed(ctx.Request.Context(), tx, quota.ID, quota.SkinsPrivateUsed+1)
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

// UpdateSkin 更新皮肤（名称/公开状态）
func (l *LibraryLogic) UpdateSkin(ctx *gin.Context, userID xSnowflake.SnowflakeID, skinID xSnowflake.SnowflakeID, req *apiLibrary.UpdateSkinRequest) (*entity.SkinLibrary, *xError.Error) {
	l.log.Info(ctx, "UpdateSkin - 更新皮肤")

	// 获取皮肤记录
	skin, found, xErr := l.repo.skinRepo.GetByIDAndUserID(ctx.Request.Context(), nil, skinID, userID, false)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "皮肤不存在", true)
	}

	// 系统内置皮肤不能修改
	if skin.UserID == nil {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "系统内置皮肤不能修改", true)
	}

	// 确定新名称和公开状态
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

	// 如果没有变化，直接返回
	if skin.Name == newName && skin.IsPublic == newIsPublic {
		return skin, nil
	}

	var updatedSkin *entity.SkinLibrary
	var bizErr *xError.Error

	err := l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 如果公开状态发生变化，检查配额
		if skin.IsPublic != newIsPublic {
			quota, found, xErr := l.repo.quotaRepo.GetByUserID(ctx.Request.Context(), tx, userID, true)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if !found {
				bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
				return bizErr
			}

			if newIsPublic {
				// 转为公开，检查公开配额
				if quota.SkinsPublicUsed >= quota.SkinsPublicTotal {
					bizErr = xError.NewError(ctx, xError.ResourceExhausted, "公开皮肤配额不足", true)
					return bizErr
				}
				// 更新配额
				bizErr = l.repo.quotaRepo.UpdateSkinsPublicUsed(ctx.Request.Context(), tx, quota.ID, quota.SkinsPublicUsed+1)
				if bizErr != nil {
					return bizErr
				}
				bizErr = l.repo.quotaRepo.UpdateSkinsPrivateUsed(ctx.Request.Context(), tx, quota.ID, quota.SkinsPrivateUsed-1)
				if bizErr != nil {
					return bizErr
				}
			} else {
				// 转为私有，检查私有配额
				if quota.SkinsPrivateUsed >= quota.SkinsPrivateTotal {
					bizErr = xError.NewError(ctx, xError.ResourceExhausted, "私有皮肤配额不足", true)
					return bizErr
				}
				// 更新配额
				bizErr = l.repo.quotaRepo.UpdateSkinsPrivateUsed(ctx.Request.Context(), tx, quota.ID, quota.SkinsPrivateUsed+1)
				if bizErr != nil {
					return bizErr
				}
				bizErr = l.repo.quotaRepo.UpdateSkinsPublicUsed(ctx.Request.Context(), tx, quota.ID, quota.SkinsPublicUsed-1)
				if bizErr != nil {
					return bizErr
				}
			}
		}

		updatedSkin, bizErr = l.repo.skinRepo.UpdateNameAndIsPublic(ctx.Request.Context(), tx, skinID, newName, newIsPublic)
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

// DeleteSkin 删除皮肤
func (l *LibraryLogic) DeleteSkin(ctx *gin.Context, userID xSnowflake.SnowflakeID, skinID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "DeleteSkin - 删除皮肤")

	// 获取皮肤记录
	skin, found, xErr := l.repo.skinRepo.GetByIDAndUserID(ctx.Request.Context(), nil, skinID, userID, false)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ResourceNotFound, "皮肤不存在", true)
	}

	// 系统内置皮肤不能删除
	if skin.UserID == nil {
		return xError.NewError(ctx, xError.PermissionDenied, "系统内置皮肤不能删除", true)
	}

	var bizErr *xError.Error

	err := l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新配额
		quota, found, xErr := l.repo.quotaRepo.GetByUserID(ctx.Request.Context(), tx, userID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}

		if skin.IsPublic {
			bizErr = l.repo.quotaRepo.UpdateSkinsPublicUsed(ctx.Request.Context(), tx, quota.ID, quota.SkinsPublicUsed-1)
		} else {
			bizErr = l.repo.quotaRepo.UpdateSkinsPrivateUsed(ctx.Request.Context(), tx, quota.ID, quota.SkinsPrivateUsed-1)
		}
		if bizErr != nil {
			return bizErr
		}

		// 删除皮肤记录
		bizErr = l.repo.skinRepo.DeleteByID(ctx.Request.Context(), tx, skinID)
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

// ListSkins 获取皮肤列表
func (l *LibraryLogic) ListSkins(ctx *gin.Context, userID xSnowflake.SnowflakeID, mode string, page int, pageSize int) ([]entity.SkinLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListSkins - 获取皮肤列表")

	if mode == "market" {
		// 市场模式：获取所有公开的皮肤
		return l.repo.skinRepo.ListPublic(ctx.Request.Context(), nil, page, pageSize)
	}

	// 我的模式：获取当前用户的皮肤
	return l.repo.skinRepo.ListByUserID(ctx.Request.Context(), nil, userID, page, pageSize)
}

// ==================== Cape Logic ====================

// CreateCape 创建披风
func (l *LibraryLogic) CreateCape(ctx *gin.Context, userID xSnowflake.SnowflakeID, req *apiLibrary.CreateCapeRequest) (*entity.CapeLibrary, *xError.Error) {
	l.log.Info(ctx, "CreateCape - 创建披风")

	if userID.IsZero() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效用户 ID：不能为 0", true)
	}

	// 验证披风名称
	name, xErr := l.validateCapeName(ctx, req.Name)
	if xErr != nil {
		return nil, xErr
	}

	// 解码 base64 纹理
	textureData, xErr := l.decodeBase64Texture(ctx, req.Texture)
	if xErr != nil {
		return nil, xErr
	}

	// 计算纹理哈希
	textureHash := l.calculateTextureHash(textureData)

	var createdCape *entity.CapeLibrary
	var bizErr *xError.Error

	err := l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 检查配额
		quota, found, xErr := l.repo.quotaRepo.GetByUserID(ctx.Request.Context(), tx, userID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}
		if quota.CapesPrivateUsed >= quota.CapesPrivateTotal {
			bizErr = xError.NewError(ctx, xError.ResourceExhausted, "私有披风配额不足", true)
			return bizErr
		}

		// 检查纹理哈希是否已存在
		existingCape, found, xErr := l.repo.capeRepo.GetByTextureHash(ctx.Request.Context(), tx, textureHash)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if found {
			// 如果已存在且是当前用户的，直接返回
			if existingCape.UserID != nil && *existingCape.UserID == userID {
				createdCape = existingCape
				return nil
			}
			// 如果已存在但不是当前用户的，返回冲突错误
			bizErr = xError.NewError(ctx, xError.DataConflict, "该披风纹理已存在", true)
			return bizErr
		}

		// 上传到存储桶
		bucketClient := bCtx.MustGetBucket(ctx)
		uploadResp, err := bucketClient.Normal.Upload(ctx, &bBucketApi.UploadRequest{
			BucketId:      "yggleaf",
			PathId:        "capes",
			ContentBase64: req.Texture,
		})
		if err != nil {
			bizErr = xError.NewError(ctx, xError.ServerInternalError, "上传披风纹理失败", true, err)
			return bizErr
		}

		// 创建披风记录
		capeId, _ := strconv.ParseInt(uploadResp.FileId, 10, 64)
		cape := &entity.CapeLibrary{
			UserID:      &userID,
			Name:        name,
			Texture:     capeId,
			TextureHash: textureHash,
			IsPublic:    false,
		}

		createdCape, bizErr = l.repo.capeRepo.Create(ctx.Request.Context(), tx, cape)
		if bizErr != nil {
			return bizErr
		}

		// 更新配额
		bizErr = l.repo.quotaRepo.UpdateCapesPrivateUsed(ctx.Request.Context(), tx, quota.ID, quota.CapesPrivateUsed+1)
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

// UpdateCape 更新披风（名称/公开状态）
func (l *LibraryLogic) UpdateCape(ctx *gin.Context, userID xSnowflake.SnowflakeID, capeID xSnowflake.SnowflakeID, req *apiLibrary.UpdateCapeRequest) (*entity.CapeLibrary, *xError.Error) {
	l.log.Info(ctx, "UpdateCape - 更新披风")

	// 获取披风记录
	cape, found, xErr := l.repo.capeRepo.GetByIDAndUserID(ctx.Request.Context(), nil, capeID, userID, false)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "披风不存在", true)
	}

	// 系统内置披风不能修改
	if cape.UserID == nil {
		return nil, xError.NewError(ctx, xError.PermissionDenied, "系统内置披风不能修改", true)
	}

	// 确定新名称和公开状态
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

	// 如果没有变化，直接返回
	if cape.Name == newName && cape.IsPublic == newIsPublic {
		return cape, nil
	}

	var updatedCape *entity.CapeLibrary
	var bizErr *xError.Error

	err := l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 如果公开状态发生变化，检查配额
		if cape.IsPublic != newIsPublic {
			quota, found, xErr := l.repo.quotaRepo.GetByUserID(ctx.Request.Context(), tx, userID, true)
			if xErr != nil {
				bizErr = xErr
				return xErr
			}
			if !found {
				bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
				return bizErr
			}

			if newIsPublic {
				// 转为公开，检查公开配额
				if quota.CapesPublicUsed >= quota.CapesPublicTotal {
					bizErr = xError.NewError(ctx, xError.ResourceExhausted, "公开披风配额不足", true)
					return bizErr
				}
				// 更新配额
				bizErr = l.repo.quotaRepo.UpdateCapesPublicUsed(ctx.Request.Context(), tx, quota.ID, quota.CapesPublicUsed+1)
				if bizErr != nil {
					return bizErr
				}
				bizErr = l.repo.quotaRepo.UpdateCapesPrivateUsed(ctx.Request.Context(), tx, quota.ID, quota.CapesPrivateUsed-1)
				if bizErr != nil {
					return bizErr
				}
			} else {
				// 转为私有，检查私有配额
				if quota.CapesPrivateUsed >= quota.CapesPrivateTotal {
					bizErr = xError.NewError(ctx, xError.ResourceExhausted, "私有披风配额不足", true)
					return bizErr
				}
				// 更新配额
				bizErr = l.repo.quotaRepo.UpdateCapesPrivateUsed(ctx.Request.Context(), tx, quota.ID, quota.CapesPrivateUsed+1)
				if bizErr != nil {
					return bizErr
				}
				bizErr = l.repo.quotaRepo.UpdateCapesPublicUsed(ctx.Request.Context(), tx, quota.ID, quota.CapesPublicUsed-1)
				if bizErr != nil {
					return bizErr
				}
			}
		}

		updatedCape, bizErr = l.repo.capeRepo.UpdateNameAndIsPublic(ctx.Request.Context(), tx, capeID, newName, newIsPublic)
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

// DeleteCape 删除披风
func (l *LibraryLogic) DeleteCape(ctx *gin.Context, userID xSnowflake.SnowflakeID, capeID xSnowflake.SnowflakeID) *xError.Error {
	l.log.Info(ctx, "DeleteCape - 删除披风")

	// 获取披风记录
	cape, found, xErr := l.repo.capeRepo.GetByIDAndUserID(ctx.Request.Context(), nil, capeID, userID, false)
	if xErr != nil {
		return xErr
	}
	if !found {
		return xError.NewError(ctx, xError.ResourceNotFound, "披风不存在", true)
	}

	// 系统内置披风不能删除
	if cape.UserID == nil {
		return xError.NewError(ctx, xError.PermissionDenied, "系统内置披风不能删除", true)
	}

	var bizErr *xError.Error

	err := l.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 更新配额
		quota, found, xErr := l.repo.quotaRepo.GetByUserID(ctx.Request.Context(), tx, userID, true)
		if xErr != nil {
			bizErr = xErr
			return xErr
		}
		if !found {
			bizErr = xError.NewError(ctx, xError.ResourceNotFound, "用户资源库配额不存在", true)
			return bizErr
		}

		if cape.IsPublic {
			bizErr = l.repo.quotaRepo.UpdateCapesPublicUsed(ctx.Request.Context(), tx, quota.ID, quota.CapesPublicUsed-1)
		} else {
			bizErr = l.repo.quotaRepo.UpdateCapesPrivateUsed(ctx.Request.Context(), tx, quota.ID, quota.CapesPrivateUsed-1)
		}
		if bizErr != nil {
			return bizErr
		}

		// 删除披风记录
		bizErr = l.repo.capeRepo.DeleteByID(ctx.Request.Context(), tx, capeID)
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

// ListCapes 获取披风列表
func (l *LibraryLogic) ListCapes(ctx *gin.Context, userID xSnowflake.SnowflakeID, mode string, page int, pageSize int) ([]entity.CapeLibrary, int64, *xError.Error) {
	l.log.Info(ctx, "ListCapes - 获取披风列表")

	if mode == "market" {
		// 市场模式：获取所有公开的披风
		return l.repo.capeRepo.ListPublic(ctx.Request.Context(), nil, page, pageSize)
	}

	// 我的模式：获取当前用户的披风
	return l.repo.capeRepo.ListByUserID(ctx.Request.Context(), nil, userID, page, pageSize)
}

// ==================== Helper Methods ====================

func (l *LibraryLogic) validateSkinName(ctx *gin.Context, name string) (string, *xError.Error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < skinNameMinLength || len(trimmedName) > skinNameMaxLength {
		return "", xError.NewError(ctx, xError.ParameterError, "无效皮肤名称长度：必须在 1-64 个字符之间", true)
	}
	return trimmedName, nil
}

func (l *LibraryLogic) validateCapeName(ctx *gin.Context, name string) (string, *xError.Error) {
	trimmedName := strings.TrimSpace(name)
	if len(trimmedName) < capeNameMinLength || len(trimmedName) > capeNameMaxLength {
		return "", xError.NewError(ctx, xError.ParameterError, "无效披风名称长度：必须在 1-64 个字符之间", true)
	}
	return trimmedName, nil
}

func (l *LibraryLogic) decodeBase64Texture(ctx *gin.Context, texture string) ([]byte, *xError.Error) {
	data, err := base64.StdEncoding.DecodeString(texture)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效的 base64 纹理数据", true, err)
	}
	return data, nil
}

func (l *LibraryLogic) calculateTextureHash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}
