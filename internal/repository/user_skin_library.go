package repository

import (
	"context"
	"errors"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"gorm.io/gorm"
)

// UserSkinLibraryRepo 用户皮肤关联仓储，负责用户与皮肤资源关联的数据访问。
type UserSkinLibraryRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewUserSkinLibraryRepo 初始化并返回 UserSkinLibraryRepo 实例。
func NewUserSkinLibraryRepo(db *gorm.DB) *UserSkinLibraryRepo {
	return &UserSkinLibraryRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "UserSkinLibraryRepo"),
	}
}

// Create 创建用户皮肤关联记录。
func (r *UserSkinLibraryRepo) Create(ctx context.Context, tx *gorm.DB, association *entity.UserSkinLibrary) (*entity.UserSkinLibrary, *xError.Error) {
	r.log.Info(ctx, "Create - 创建用户皮肤关联记录")

	if err := r.pickDB(ctx, tx).Create(association).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建用户皮肤关联记录失败", true, err)
	}
	return association, nil
}

// GetByUserAndSkin 根据用户 ID 和皮肤库 ID 查询关联记录。
func (r *UserSkinLibraryRepo) GetByUserAndSkin(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, skinLibraryID xSnowflake.SnowflakeID) (*entity.UserSkinLibrary, bool, *xError.Error) {
	r.log.Info(ctx, "GetByUserAndSkin - 根据用户 ID 与皮肤库 ID 获取关联记录")

	var association entity.UserSkinLibrary
	err := r.pickDB(ctx, tx).Model(&entity.UserSkinLibrary{}).Where("user_id = ? AND skin_library_id = ?", userID, skinLibraryID).First(&association).Error
	if err == nil {
		return &association, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询用户皮肤关联记录失败", true, err)
}

// ExistsByUserAndSkin 判断用户是否关联指定皮肤。
func (r *UserSkinLibraryRepo) ExistsByUserAndSkin(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, skinLibraryID xSnowflake.SnowflakeID) (bool, *xError.Error) {
	r.log.Info(ctx, "ExistsByUserAndSkin - 判断用户是否关联指定皮肤")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.UserSkinLibrary{}).Where("user_id = ? AND skin_library_id = ?", userID, skinLibraryID).Count(&count).Error; err != nil {
		return false, xError.NewError(ctx, xError.DatabaseError, "查询用户皮肤关联记录失败", true, err)
	}
	return count > 0, nil
}

// DeleteByUserAndSkin 根据用户 ID 和皮肤库 ID 删除关联记录。
func (r *UserSkinLibraryRepo) DeleteByUserAndSkin(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, skinLibraryID xSnowflake.SnowflakeID) *xError.Error {
	r.log.Info(ctx, "DeleteByUserAndSkin - 根据用户 ID 与皮肤库 ID 删除关联记录")

	if err := r.pickDB(ctx, tx).Where("user_id = ? AND skin_library_id = ?", userID, skinLibraryID).Delete(&entity.UserSkinLibrary{}).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "删除用户皮肤关联记录失败", true, err)
	}
	return nil
}

// ListByUserID 根据用户 ID 查询关联记录列表（Preload SkinLibrary）。
func (r *UserSkinLibraryRepo) ListByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]entity.UserSkinLibrary, int64, *xError.Error) {
	r.log.Info(ctx, "ListByUserID - 根据用户 ID 查询皮肤关联列表")

	var total int64
	query := r.pickDB(ctx, tx).Model(&entity.UserSkinLibrary{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询用户皮肤关联总数失败", true, err)
	}

	var associations []entity.UserSkinLibrary
	offset := (page - 1) * pageSize
	if err := query.Preload("SkinLibrary").Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&associations).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询用户皮肤关联列表失败", true, err)
	}
	return associations, total, nil
}

// CountReferences 统计指定皮肤库被多少用户关联。
func (r *UserSkinLibraryRepo) CountReferences(ctx context.Context, tx *gorm.DB, skinLibraryID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountReferences - 统计皮肤库关联引用数")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.UserSkinLibrary{}).Where("skin_library_id = ?", skinLibraryID).Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计皮肤库关联引用数失败", true, err)
	}
	return count, nil
}

// CountNormalPublicByUser JOIN SkinLibrary 统计用户 normal+public 类型关联数。
func (r *UserSkinLibraryRepo) CountNormalPublicByUser(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountNormalPublicByUser - 统计用户 normal+public 皮肤关联数")

	var count int64
	if err := r.pickDB(ctx, tx).
		Model(&entity.UserSkinLibrary{}).
		Joins("JOIN fyl_skin_library ON fyl_skin_library.id = fyl_user_skin_library.skin_library_id").
		Where("fyl_user_skin_library.user_id = ? AND fyl_user_skin_library.assignment_type = ? AND fyl_skin_library.is_public = ?",
			userID, entityType.AssignmentTypeNormal, true).
		Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户 normal+public 皮肤关联数失败", true, err)
	}
	return count, nil
}

// CountNormalPrivateByUser JOIN SkinLibrary 统计用户 normal+private 类型关联数。
func (r *UserSkinLibraryRepo) CountNormalPrivateByUser(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountNormalPrivateByUser - 统计用户 normal+private 皮肤关联数")

	var count int64
	if err := r.pickDB(ctx, tx).
		Model(&entity.UserSkinLibrary{}).
		Joins("JOIN fyl_skin_library ON fyl_skin_library.id = fyl_user_skin_library.skin_library_id").
		Where("fyl_user_skin_library.user_id = ? AND fyl_user_skin_library.assignment_type = ? AND fyl_skin_library.is_public = ?",
			userID, entityType.AssignmentTypeNormal, false).
		Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户 normal+private 皮肤关联数失败", true, err)
	}
	return count, nil
}

func (r *UserSkinLibraryRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
