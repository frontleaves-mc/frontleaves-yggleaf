package repository

import (
	"context"
	"errors"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
)

// IssueTypeRepo 问题类型仓储，负责问题类型数据访问。
type IssueTypeRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewIssueTypeRepo 初始化并返回 IssueTypeRepo 实例。
func NewIssueTypeRepo(db *gorm.DB) *IssueTypeRepo {
	return &IssueTypeRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "IssueTypeRepo"),
	}
}

// Create 创建问题类型记录。
func (r *IssueTypeRepo) Create(ctx context.Context, tx *gorm.DB, it *entity.IssueType) (*entity.IssueType, *xError.Error) {
	r.log.Info(ctx, "Create - 创建问题类型")

	if err := r.pickDB(ctx, tx).Create(it).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建问题类型失败", true, err)
	}
	return it, nil
}

// GetByID 根据ID查询问题类型。
func (r *IssueTypeRepo) GetByID(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID) (*entity.IssueType, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据ID查询问题类型")

	var it entity.IssueType
	err := r.pickDB(ctx, tx).Model(&entity.IssueType{}).Where("id = ?", id).First(&it).Error
	if err == nil {
		return &it, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询问题类型失败", true, err)
}

// ListEnabled 获取启用的类型列表。
func (r *IssueTypeRepo) ListEnabled(ctx context.Context) ([]entity.IssueType, *xError.Error) {
	r.log.Info(ctx, "ListEnabled - 获取启用的类型列表")

	var list []entity.IssueType
	err := r.db.WithContext(ctx).Model(&entity.IssueType{}).
		Where("is_enabled = ?", true).
		Order("sort_order ASC, id ASC").
		Find(&list).Error
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "查询问题类型列表失败", true, err)
	}
	return list, nil
}

// Update 更新问题类型记录。
func (r *IssueTypeRepo) Update(ctx context.Context, tx *gorm.DB, it *entity.IssueType) (*entity.IssueType, *xError.Error) {
	r.log.Info(ctx, "Update - 更新问题类型")

	if err := r.pickDB(ctx, tx).Save(it).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新问题类型失败", true, err)
	}
	return it, nil
}

// DeleteByID 删除问题类型记录。
func (r *IssueTypeRepo) DeleteByID(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID) *xError.Error {
	r.log.Info(ctx, "DeleteByID - 删除问题类型")

	if err := r.pickDB(ctx, tx).Delete(&entity.IssueType{}, id).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "删除问题类型失败", true, err)
	}
	return nil
}

func (r *IssueTypeRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
