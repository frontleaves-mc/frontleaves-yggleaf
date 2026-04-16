package repository

import (
	"context"
	"errors"
	"fmt"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
)

// GameProfileYggRepo Yggdrasil 游戏档案仓储，负责 Yggdrasil 协议相关的游戏档案查询。
type GameProfileYggRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewGameProfileYggRepo 初始化并返回 GameProfileYggRepo 实例。
func NewGameProfileYggRepo(db *gorm.DB) *GameProfileYggRepo {
	return &GameProfileYggRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "GameProfileYggRepo"),
	}
}

// GetByUUIDUnsigned 根据无连字符 UUID 查询游戏档案。
//
// 将无连字符 UUID 转换为标准格式后查询对应游戏档案。
func (r *GameProfileYggRepo) GetByUUIDUnsigned(ctx context.Context, tx *gorm.DB, unsignedUUID string) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByUUIDUnsigned - 根据无连字符 UUID 获取游戏档案")

	standardUUID, err := unsignedUUIDToStandard(unsignedUUID)
	if err != nil {
		return nil, false, xError.NewError(ctx, xError.ParameterError, "无连字符 UUID 格式无效", true, err)
	}

	var profile entity.GameProfile
	err = r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("uuid = ?", standardUUID).First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "根据 UUID 查询游戏档案失败", true, err)
}

// GetByUUIDUnsignedWithTextures 根据无连字符 UUID 查询游戏档案（含关联皮肤和披风）。
//
// 将无连字符 UUID 转换为标准格式后查询对应游戏档案，并预加载皮肤库和披风库。
func (r *GameProfileYggRepo) GetByUUIDUnsignedWithTextures(ctx context.Context, tx *gorm.DB, unsignedUUID string) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByUUIDUnsignedWithTextures - 根据无连字符 UUID 获取游戏档案详情")

	standardUUID, err := unsignedUUIDToStandard(unsignedUUID)
	if err != nil {
		return nil, false, xError.NewError(ctx, xError.ParameterError, "无连字符 UUID 格式无效", true, err)
	}

	var profile entity.GameProfile
	err = r.pickDB(ctx, tx).
		Model(&entity.GameProfile{}).
		Preload("SkinLibrary").
		Preload("CapeLibrary").
		Where("uuid = ?", standardUUID).
		First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "根据 UUID 查询游戏档案详情失败", true, err)
}

// GetByName 根据用户名查询游戏档案。
func (r *GameProfileYggRepo) GetByName(ctx context.Context, tx *gorm.DB, name string) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByName - 根据用户名获取游戏档案")

	var profile entity.GameProfile
	err := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("name = ?", name).First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "根据用户名查询游戏档案失败", true, err)
}

// BatchGetByNames 根据用户名列表批量查询游戏档案。
func (r *GameProfileYggRepo) BatchGetByNames(ctx context.Context, tx *gorm.DB, names []string) ([]entity.GameProfile, *xError.Error) {
	r.log.Info(ctx, "BatchGetByNames - 根据用户名列表批量获取游戏档案")

	if len(names) == 0 {
		return []entity.GameProfile{}, nil
	}

	var profiles []entity.GameProfile
	if err := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("name IN ?", names).Find(&profiles).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "批量查询游戏档案失败", true, err)
	}
	return profiles, nil
}

// ListByUserID 根据用户 ID 查询其所有游戏档案。
func (r *GameProfileYggRepo) ListByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) ([]entity.GameProfile, *xError.Error) {
	r.log.Info(ctx, "ListByUserID - 根据用户 ID 获取游戏档案列表")

	var profiles []entity.GameProfile
	if err := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("user_id = ?", userID).Order("created_at ASC").Find(&profiles).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "查询游戏档案列表失败", true, err)
	}
	return profiles, nil
}

// ListByUserIDWithTextures 根据用户 ID 查询其所有游戏档案（含关联皮肤和披风）。
//
// 与 ListByUserID 的区别在于额外预加载 SkinLibrary 和 CapeLibrary 关联，
// 用于需要构建含 textures 属性的 ProfileResponse 场景（如 Authenticate/Refresh）。
func (r *GameProfileYggRepo) ListByUserIDWithTextures(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) ([]entity.GameProfile, *xError.Error) {
	r.log.Info(ctx, "ListByUserIDWithTextures - 根据用户 ID 获取游戏档案列表（含纹理）")

	var profiles []entity.GameProfile
	if err := r.pickDB(ctx, tx).
		Model(&entity.GameProfile{}).
		Preload("SkinLibrary").
		Preload("CapeLibrary").
		Where("user_id = ?", userID).
		Order("created_at ASC").
		Find(&profiles).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "查询游戏档案列表失败", true, err)
	}
	return profiles, nil
}

// GetByUserIDAndUUID 根据用户 ID 和 UUID 查询游戏档案。
func (r *GameProfileYggRepo) GetByUserIDAndUUID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, unsignedUUID string) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByUserIDAndUUID - 根据用户 ID 和无符号 UUID 获取游戏档案")

	standardUUID, err := unsignedUUIDToStandard(unsignedUUID)
	if err != nil {
		return nil, false, xError.NewError(ctx, xError.ParameterError, "无连字符 UUID 格式无效", true, err)
	}

	var profile entity.GameProfile
	err = r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("user_id = ? AND uuid = ?", userID, standardUUID).First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "根据用户 ID 和 UUID 查询游戏档案失败", true, err)
}

// GetByUserIDAndUUIDWithTextures 根据用户 ID 和 UUID 查询游戏档案（含关联皮肤和披风）。
//
// 与 GetByUserIDAndUUID 的区别在于额外预加载 SkinLibrary 和 CapeLibrary 关联，
// 用于 RefreshToken 角色选择场景中构建含 textures 属性的响应。
func (r *GameProfileYggRepo) GetByUserIDAndUUIDWithTextures(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, unsignedUUID string) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByUserIDAndUUIDWithTextures - 根据用户 ID 和无符号 UUID 获取游戏档案（含纹理）")

	standardUUID, err := unsignedUUIDToStandard(unsignedUUID)
	if err != nil {
		return nil, false, xError.NewError(ctx, xError.ParameterError, "无连字符 UUID 格式无效", true, err)
	}

	var profile entity.GameProfile
	err = r.pickDB(ctx, tx).
		Model(&entity.GameProfile{}).
		Preload("SkinLibrary").
		Preload("CapeLibrary").
		Where("user_id = ? AND uuid = ?", userID, standardUUID).
		First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "根据用户 ID 和 UUID 查询游戏档案失败", true, err)
}

// unsignedUUIDToStandard 将无连字符 UUID 转换为标准格式。
//
// 输入必须为 32 位合法十六进制字符（0-9, a-f, A-F），输出格式为 8-4-4-4-12。
// 该函数是所有无符号 UUID 输入的统一入口，在此处做字符合法性校验可一劳永逸地
// 覆盖所有调用方（UploadTexture / HasJoined / RefreshToken / ProfileQuery 等）。
func unsignedUUIDToStandard(s string) (string, error) {
	if len(s) != 32 {
		return "", fmt.Errorf("无效的无连字符 UUID 长度: %d", len(s))
	}
	// 十六进制字符合法性校验，防止非法字符穿透到数据库查询层
	for _, c := range s {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return "", fmt.Errorf("无效的无连字符 UUID 字符: %c", c)
		}
	}
	return s[:8] + "-" + s[8:12] + "-" + s[12:16] + "-" + s[16:20] + "-" + s[20:], nil
}

// GetByID 根据游戏档案 ID（SnowflakeID）查询游戏档案。
//
// 用于 RefreshToken 角色继承等需要按主键直接查询的场景。
func (r *GameProfileYggRepo) GetByID(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据游戏档案 ID 获取记录")

	var profile entity.GameProfile
	err := r.pickDB(ctx, tx).Model(&entity.GameProfile{}).Where("id = ?", id).First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "根据 ID 查询游戏档案失败", true, err)
}

// GetByIDWithTextures 根据游戏档案 ID（SnowflakeID）查询游戏档案（含关联皮肤和披风）。
//
// 与 GetByID 的区别在于额外预加载 SkinLibrary 和 CapeLibrary 关联，
// 用于 RefreshToken 角色继承场景中构建含 textures 属性的响应。
func (r *GameProfileYggRepo) GetByIDWithTextures(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID) (*entity.GameProfile, bool, *xError.Error) {
	r.log.Info(ctx, "GetByIDWithTextures - 根据游戏档案 ID 获取记录（含纹理）")

	var profile entity.GameProfile
	err := r.pickDB(ctx, tx).
		Model(&entity.GameProfile{}).
		Preload("SkinLibrary").
		Preload("CapeLibrary").
		Where("id = ?", id).
		First(&profile).Error
	if err == nil {
		return &profile, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "根据 ID 查询游戏档案失败", true, err)
}

func (r *GameProfileYggRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
