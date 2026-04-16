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

// UserCapeLibraryRepo 用户披风关联仓储，负责用户与披风资源关联的数据访问。
type UserCapeLibraryRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewUserCapeLibraryRepo 初始化并返回 UserCapeLibraryRepo 实例。
func NewUserCapeLibraryRepo(db *gorm.DB) *UserCapeLibraryRepo {
	return &UserCapeLibraryRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "UserCapeLibraryRepo"),
	}
}

// Create 创建用户披风关联记录。
func (r *UserCapeLibraryRepo) Create(ctx context.Context, tx *gorm.DB, association *entity.UserCapeLibrary) (*entity.UserCapeLibrary, *xError.Error) {
	r.log.Info(ctx, "Create - 创建用户披风关联记录")

	if err := r.pickDB(ctx, tx).Create(association).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建用户披风关联记录失败", true, err)
	}
	return association, nil
}

// GetByUserAndCape 根据用户 ID 和披风库 ID 查询关联记录。
func (r *UserCapeLibraryRepo) GetByUserAndCape(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, capeLibraryID xSnowflake.SnowflakeID) (*entity.UserCapeLibrary, bool, *xError.Error) {
	r.log.Info(ctx, "GetByUserAndCape - 根据用户 ID 与披风库 ID 获取关联记录")

	var association entity.UserCapeLibrary
	err := r.pickDB(ctx, tx).Model(&entity.UserCapeLibrary{}).Where("user_id = ? AND cape_library_id = ?", userID, capeLibraryID).First(&association).Error
	if err == nil {
		return &association, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询用户披风关联记录失败", true, err)
}

// ExistsByUserAndCape 判断用户是否关联指定披风。
func (r *UserCapeLibraryRepo) ExistsByUserAndCape(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, capeLibraryID xSnowflake.SnowflakeID) (bool, *xError.Error) {
	r.log.Info(ctx, "ExistsByUserAndCape - 判断用户是否关联指定披风")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.UserCapeLibrary{}).Where("user_id = ? AND cape_library_id = ?", userID, capeLibraryID).Count(&count).Error; err != nil {
		return false, xError.NewError(ctx, xError.DatabaseError, "查询用户披风关联记录失败", true, err)
	}
	return count > 0, nil
}

// DeleteByUserAndCape 根据用户 ID 和披风库 ID 删除关联记录。
func (r *UserCapeLibraryRepo) DeleteByUserAndCape(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, capeLibraryID xSnowflake.SnowflakeID) *xError.Error {
	r.log.Info(ctx, "DeleteByUserAndCape - 根据用户 ID 与披风库 ID 删除关联记录")

	if err := r.pickDB(ctx, tx).Where("user_id = ? AND cape_library_id = ?", userID, capeLibraryID).Delete(&entity.UserCapeLibrary{}).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "删除用户披风关联记录失败", true, err)
	}
	return nil
}

// ListByUserID 根据用户 ID 查询关联记录列表（Preload CapeLibrary）。
func (r *UserCapeLibraryRepo) ListByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID, page int, pageSize int) ([]entity.UserCapeLibrary, int64, *xError.Error) {
	r.log.Info(ctx, "ListByUserID - 根据用户 ID 查询披风关联列表")

	var total int64
	query := r.pickDB(ctx, tx).Model(&entity.UserCapeLibrary{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询用户披风关联总数失败", true, err)
	}

	var associations []entity.UserCapeLibrary
	offset := (page - 1) * pageSize
	if err := query.Preload("CapeLibrary").Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&associations).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询用户披风关联列表失败", true, err)
	}
	return associations, total, nil
}

// ListAllByUserID 根据用户 ID 查询所有关联记录（Preload CapeLibrary，不分页）。
//
// 用于披风选择器等只需要 ID 和名称的场景，避免不必要的分页和纹理解析。
func (r *UserCapeLibraryRepo) ListAllByUserID(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) ([]entity.UserCapeLibrary, *xError.Error) {
	r.log.Info(ctx, "ListAllByUserID - 查询用户所有披风关联（不分页）")

	var associations []entity.UserCapeLibrary
	if err := r.pickDB(ctx, tx).
		Model(&entity.UserCapeLibrary{}).
		Where("user_id = ?", userID).
		Preload("CapeLibrary").
		Order("created_at DESC").
		Find(&associations).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "查询用户披风关联列表失败", true, err)
	}
	return associations, nil
}

// CountReferences 统计指定披风库被多少用户关联。
func (r *UserCapeLibraryRepo) CountReferences(ctx context.Context, tx *gorm.DB, capeLibraryID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountReferences - 统计披风库关联引用数")

	var count int64
	if err := r.pickDB(ctx, tx).Model(&entity.UserCapeLibrary{}).Where("cape_library_id = ?", capeLibraryID).Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计披风库关联引用数失败", true, err)
	}
	return count, nil
}

// CountNormalPublicByUser JOIN CapeLibrary 统计用户 normal+public 类型关联数。
func (r *UserCapeLibraryRepo) CountNormalPublicByUser(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountNormalPublicByUser - 统计用户 normal+public 披风关联数")

	var count int64
	if err := r.pickDB(ctx, tx).
		Model(&entity.UserCapeLibrary{}).
		Joins("JOIN fyl_cape_library ON fyl_cape_library.id = fyl_user_cape_library.cape_library_id").
		Where("fyl_user_cape_library.user_id = ? AND fyl_user_cape_library.assignment_type = ? AND fyl_cape_library.is_public = ?",
			userID, entityType.AssignmentTypeNormal, true).
		Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户 normal+public 披风关联数失败", true, err)
	}
	return count, nil
}

// CountNormalPrivateByUser JOIN CapeLibrary 统计用户 normal+private 类型关联数。
func (r *UserCapeLibraryRepo) CountNormalPrivateByUser(ctx context.Context, tx *gorm.DB, userID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountNormalPrivateByUser - 统计用户 normal+private 披风关联数")

	var count int64
	if err := r.pickDB(ctx, tx).
		Model(&entity.UserCapeLibrary{}).
		Joins("JOIN fyl_cape_library ON fyl_cape_library.id = fyl_user_cape_library.cape_library_id").
		Where("fyl_user_cape_library.user_id = ? AND fyl_user_cape_library.assignment_type = ? AND fyl_cape_library.is_public = ?",
			userID, entityType.AssignmentTypeNormal, false).
		Count(&count).Error; err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计用户 normal+private 披风关联数失败", true, err)
	}
	return count, nil
}

func (r *UserCapeLibraryRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
