package repository

import (
	"context"
	"errors"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// CapeLibraryRepo 披风库仓储，负责披风库数据访问。
type CapeLibraryRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewCapeLibraryRepo 初始化并返回 CapeLibraryRepo 实例。
func NewCapeLibraryRepo(db *gorm.DB) *CapeLibraryRepo {
	return &CapeLibraryRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "CapeLibraryRepo"),
	}
}

// Create 创建披风库记录。
func (r *CapeLibraryRepo) Create(ctx context.Context, tx *gorm.DB, cape *entity.CapeLibrary) (*entity.CapeLibrary, *xError.Error) {
	r.log.Info(ctx, "Create - 创建披风库记录")

	if err := r.pickDB(ctx, tx).Create(cape).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建披风库记录失败", true, err)
	}
	return cape, nil
}

// GetByID 根据披风 ID 查询披风库记录。
func (r *CapeLibraryRepo) GetByID(ctx context.Context, tx *gorm.DB, capeID xSnowflake.SnowflakeID) (*entity.CapeLibrary, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据披风 ID 获取披风库记录")

	var cape entity.CapeLibrary
	err := r.pickDB(ctx, tx).Model(&entity.CapeLibrary{}).Where("id = ?", capeID).First(&cape).Error
	if err == nil {
		return &cape, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询披风库记录失败", true, err)
}

// GetByIDAndUserID 根据披风 ID 和用户 ID 查询披风库记录。
func (r *CapeLibraryRepo) GetByIDAndUserID(ctx context.Context, tx *gorm.DB, capeID xSnowflake.SnowflakeID, userID xSnowflake.SnowflakeID, forUpdate bool) (*entity.CapeLibrary, bool, *xError.Error) {
	r.log.Info(ctx, "GetByIDAndUserID - 根据披风 ID 与用户 ID 获取披风库记录")

	query := r.pickDB(ctx, tx).Model(&entity.CapeLibrary{}).Where("id = ? AND user_id = ?", capeID, userID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var cape entity.CapeLibrary
	err := query.First(&cape).Error
	if err == nil {
		return &cape, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询披风库记录失败", true, err)
}

// GetByTextureHash 根据纹理哈希查询披风库记录。
func (r *CapeLibraryRepo) GetByTextureHash(ctx context.Context, tx *gorm.DB, textureHash string) (*entity.CapeLibrary, bool, *xError.Error) {
	r.log.Info(ctx, "GetByTextureHash - 根据纹理哈希获取披风库记录")

	var cape entity.CapeLibrary
	err := r.pickDB(ctx, tx).Model(&entity.CapeLibrary{}).Where("texture_hash = ?", textureHash).First(&cape).Error
	if err == nil {
		return &cape, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询披风库记录失败", true, err)
}

// UpdateNameAndIsPublic 更新披风库记录的名称和公开状态。
func (r *CapeLibraryRepo) UpdateNameAndIsPublic(ctx context.Context, tx *gorm.DB, capeID xSnowflake.SnowflakeID, name string, isPublic bool) (*entity.CapeLibrary, *xError.Error) {
	r.log.Info(ctx, "UpdateNameAndIsPublic - 更新披风库记录名称和公开状态")

	updates := map[string]interface{}{
		"name":      name,
		"is_public": isPublic,
	}
	if err := r.pickDB(ctx, tx).Model(&entity.CapeLibrary{}).Where("id = ?", capeID).Updates(updates).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新披风库记录失败", true, err)
	}

	updatedCape, found, xErr := r.GetByID(ctx, tx, capeID)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "披风库记录不存在", true)
	}
	return updatedCape, nil
}

// DeleteByID 根据披风 ID 删除披风库记录。
func (r *CapeLibraryRepo) DeleteByID(ctx context.Context, tx *gorm.DB, capeID xSnowflake.SnowflakeID) *xError.Error {
	r.log.Info(ctx, "DeleteByID - 根据披风 ID 删除披风库记录")

	if err := r.pickDB(ctx, tx).Where("id = ?", capeID).Delete(&entity.CapeLibrary{}).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "删除披风库记录失败", true, err)
	}
	return nil
}

// ListPublic 查询公开的披风库记录列表。
func (r *CapeLibraryRepo) ListPublic(ctx context.Context, tx *gorm.DB, page int, pageSize int) ([]entity.CapeLibrary, int64, *xError.Error) {
	r.log.Info(ctx, "ListPublic - 查询公开披风库记录列表")

	var total int64
	query := r.pickDB(ctx, tx).Model(&entity.CapeLibrary{}).Where("is_public = ?", true)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询披风库记录总数失败", true, err)
	}

	var capes []entity.CapeLibrary
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&capes).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询披风库记录列表失败", true, err)
	}
	return capes, total, nil
}

// ListByUserID 根据用户 ID 查询披风库记录列表。
func (r *CapeLibraryRepo) ListByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]entity.CapeLibrary, int64, *xError.Error) {
	r.log.Info(ctx, "ListByUserID - 根据用户 ID 查询披风库记录列表")

	var total int64
	query := r.pickDB(ctx, tx).Model(&entity.CapeLibrary{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询披风库记录总数失败", true, err)
	}

	var capes []entity.CapeLibrary
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&capes).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询披风库记录列表失败", true, err)
	}
	return capes, total, nil
}

// CountPublicByUserID 统计用户公开披风数量。
func (r *CapeLibraryRepo) CountPublicByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountPublicByUserID - 统计用户公开披风数量")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.CapeLibrary{}).Where("user_id = ? AND is_public = ?", userID, true).Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户公开披风数量失败", true, err)
	}
	return count, nil
}

// CountPrivateByUserID 统计用户私有披风数量。
func (r *CapeLibraryRepo) CountPrivateByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountPrivateByUserID - 统计用户私有披风数量")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.CapeLibrary{}).Where("user_id = ? AND is_public = ?", userID, false).Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户私有披风数量失败", true, err)
	}
	return count, nil
}

func (r *CapeLibraryRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
