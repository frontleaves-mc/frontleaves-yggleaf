package repository

import (
	"context"
	"errors"

	xError "github.com/bamboo-services/bamboo-base-go/error"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SkinLibraryRepo 皮肤库仓储，负责皮肤库数据访问。
type SkinLibraryRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewSkinLibraryRepo 初始化并返回 SkinLibraryRepo 实例。
func NewSkinLibraryRepo(db *gorm.DB) *SkinLibraryRepo {
	return &SkinLibraryRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "SkinLibraryRepo"),
	}
}

// Create 创建皮肤库记录。
func (r *SkinLibraryRepo) Create(ctx context.Context, tx *gorm.DB, skin *entity.SkinLibrary) (*entity.SkinLibrary, *xError.Error) {
	r.log.Info(ctx, "Create - 创建皮肤库记录")

	if err := r.pickDB(ctx, tx).Create(skin).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建皮肤库记录失败", true, err)
	}
	return skin, nil
}

// GetByID 根据皮肤 ID 查询皮肤库记录。
func (r *SkinLibraryRepo) GetByID(ctx context.Context, tx *gorm.DB, skinID xSnowflake.SnowflakeID) (*entity.SkinLibrary, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据皮肤 ID 获取皮肤库记录")

	var skin entity.SkinLibrary
	err := r.pickDB(ctx, tx).Model(&entity.SkinLibrary{}).Where("id = ?", skinID).First(&skin).Error
	if err == nil {
		return &skin, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询皮肤库记录失败", true, err)
}

// GetByIDAndUserID 根据皮肤 ID 和用户 ID 查询皮肤库记录。
func (r *SkinLibraryRepo) GetByIDAndUserID(ctx context.Context, tx *gorm.DB, skinID xSnowflake.SnowflakeID, userID xSnowflake.SnowflakeID, forUpdate bool) (*entity.SkinLibrary, bool, *xError.Error) {
	r.log.Info(ctx, "GetByIDAndUserID - 根据皮肤 ID 与用户 ID 获取皮肤库记录")

	query := r.pickDB(ctx, tx).Model(&entity.SkinLibrary{}).Where("id = ? AND user_id = ?", skinID, userID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var skin entity.SkinLibrary
	err := query.First(&skin).Error
	if err == nil {
		return &skin, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询皮肤库记录失败", true, err)
}

// GetByTextureHash 根据纹理哈希查询皮肤库记录。
func (r *SkinLibraryRepo) GetByTextureHash(ctx context.Context, tx *gorm.DB, textureHash string) (*entity.SkinLibrary, bool, *xError.Error) {
	r.log.Info(ctx, "GetByTextureHash - 根据纹理哈希获取皮肤库记录")

	var skin entity.SkinLibrary
	err := r.pickDB(ctx, tx).Model(&entity.SkinLibrary{}).Where("texture_hash = ?", textureHash).First(&skin).Error
	if err == nil {
		return &skin, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询皮肤库记录失败", true, err)
}

// UpdateNameAndIsPublic 更新皮肤库记录的名称和公开状态。
func (r *SkinLibraryRepo) UpdateNameAndIsPublic(ctx context.Context, tx *gorm.DB, skinID xSnowflake.SnowflakeID, name string, isPublic bool) (*entity.SkinLibrary, *xError.Error) {
	r.log.Info(ctx, "UpdateNameAndIsPublic - 更新皮肤库记录名称和公开状态")

	updates := map[string]interface{}{
		"name":      name,
		"is_public": isPublic,
	}
	if err := r.pickDB(ctx, tx).Model(&entity.SkinLibrary{}).Where("id = ?", skinID).Updates(updates).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新皮肤库记录失败", true, err)
	}

	updatedSkin, found, xErr := r.GetByID(ctx, tx, skinID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "皮肤库记录不存在", true)
	}
	return updatedSkin, nil
}

// DeleteByID 根据皮肤 ID 删除皮肤库记录。
func (r *SkinLibraryRepo) DeleteByID(ctx context.Context, tx *gorm.DB, skinID xSnowflake.SnowflakeID) *xError.Error {
	r.log.Info(ctx, "DeleteByID - 根据皮肤 ID 删除皮肤库记录")

	if err := r.pickDB(ctx, tx).Where("id = ?", skinID).Delete(&entity.SkinLibrary{}).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "删除皮肤库记录失败", true, err)
	}
	return nil
}

// ListPublic 查询公开的皮肤库记录列表。
func (r *SkinLibraryRepo) ListPublic(ctx context.Context, tx *gorm.DB, page int, pageSize int) ([]entity.SkinLibrary, int64, *xError.Error) {
	r.log.Info(ctx, "ListPublic - 查询公开皮肤库记录列表")

	var total int64
	query := r.pickDB(ctx, tx).Model(&entity.SkinLibrary{}).Where("is_public = ?", true)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询皮肤库记录总数失败", true, err)
	}

	var skins []entity.SkinLibrary
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&skins).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询皮肤库记录列表失败", true, err)
	}
	return skins, total, nil
}

// ListByUserID 根据用户 ID 查询皮肤库记录列表。
func (r *SkinLibraryRepo) ListByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]entity.SkinLibrary, int64, *xError.Error) {
	r.log.Info(ctx, "ListByUserID - 根据用户 ID 查询皮肤库记录列表")

	var total int64
	query := r.pickDB(ctx, tx).Model(&entity.SkinLibrary{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询皮肤库记录总数失败", true, err)
	}

	var skins []entity.SkinLibrary
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&skins).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询皮肤库记录列表失败", true, err)
	}
	return skins, total, nil
}

// CountPublicByUserID 统计用户公开皮肤数量。
func (r *SkinLibraryRepo) CountPublicByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountPublicByUserID - 统计用户公开皮肤数量")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.SkinLibrary{}).Where("user_id = ? AND is_public = ?", userID, true).Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户公开皮肤数量失败", true, err)
	}
	return count, nil
}

// CountPrivateByUserID 统计用户私有皮肤数量。
func (r *SkinLibraryRepo) CountPrivateByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountPrivateByUserID - 统计用户私有皮肤数量")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.SkinLibrary{}).Where("user_id = ? AND is_public = ?", userID, false).Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户私有皮肤数量失败", true, err)
	}
	return count, nil
}

func (r *SkinLibraryRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
