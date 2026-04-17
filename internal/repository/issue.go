package repository

import (
	"context"
	"errors"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
)

// IssueRepo 问题仓储，负责问题数据访问。
type IssueRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewIssueRepo 初始化并返回 IssueRepo 实例。
func NewIssueRepo(db *gorm.DB) *IssueRepo {
	return &IssueRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "IssueRepo"),
	}
}

// Create 创建问题记录。
func (r *IssueRepo) Create(ctx context.Context, tx *gorm.DB, issue *entity.Issue) (*entity.Issue, *xError.Error) {
	r.log.Info(ctx, "Create - 创建问题")

	if err := r.pickDB(ctx, tx).Create(issue).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建问题失败", true, err)
	}
	return issue, nil
}

// GetByID 根据ID查询问题。
func (r *IssueRepo) GetByID(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID) (*entity.Issue, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据ID查询问题")

	var issue entity.Issue
	err := r.pickDB(ctx, tx).Model(&entity.Issue{}).Where("id = ?", id).First(&issue).Error
	if err == nil {
		return &issue, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询问题失败", true, err)
}

// GetByIDAndUserID 根据ID与用户ID查询问题（确保归属校验）。
func (r *IssueRepo) GetByIDAndUserID(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID, userID xSnowflake.SnowflakeID) (*entity.Issue, bool, *xError.Error) {
	r.log.Info(ctx, "GetByIDAndUserID - 根据ID与用户ID查询问题")

	var issue entity.Issue
	err := r.pickDB(ctx, tx).Model(&entity.Issue{}).
		Where("id = ? AND user_id = ?", id, userID).
		First(&issue).Error
	if err == nil {
		return &issue, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询问题失败", true, err)
}

// ListByUserID 分页查询用户问题列表。
func (r *IssueRepo) ListByUserID(ctx context.Context, userID xSnowflake.SnowflakeID, page, pageSize int) ([]entity.Issue, int64, *xError.Error) {
	r.log.Info(ctx, "ListByUserID - 分页查询用户问题列表")

	var total int64
	query := r.db.WithContext(ctx).Model(&entity.Issue{}).Where("user_id = ?", userID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询问题总数失败", true, err)
	}

	var list []entity.Issue
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询问题列表失败", true, err)
	}
	return list, total, nil
}

// ListAdmin 管理员全量分页查询，支持状态、优先级、类型及关键词过滤。
func (r *IssueRepo) ListAdmin(
	ctx context.Context,
	page, pageSize int,
	status *bConst.IssueStatus,
	priority *bConst.IssuePriority,
	issueTypeID *xSnowflake.SnowflakeID,
	keyword string,
) ([]entity.Issue, int64, *xError.Error) {
	r.log.Info(ctx, "ListAdmin - 管理员全量分页查询")

	query := r.db.WithContext(ctx).Model(&entity.Issue{})
	if status != nil {
		query = query.Where("status = ?", *status)
	}
	if priority != nil {
		query = query.Where("priority = ?", *priority)
	}
	if issueTypeID != nil {
		query = query.Where("issue_type_id = ?", *issueTypeID)
	}
	if keyword != "" {
		query = query.Where("title ILIKE ?", "%"+keyword+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询问题总数失败", true, err)
	}

	var list []entity.Issue
	offset := (page - 1) * pageSize
	if err := query.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询问题列表失败", true, err)
	}
	return list, total, nil
}

// UpdateStatus 更新问题状态，可选传入关闭时间。
func (r *IssueRepo) UpdateStatus(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID, status bConst.IssueStatus, closedAt interface{}) (*entity.Issue, *xError.Error) {
	r.log.Info(ctx, "UpdateStatus - 更新问题状态")

	updates := map[string]interface{}{
		"status": status,
	}
	if closedAt != nil {
		updates["closed_at"] = closedAt
	}

	var issue entity.Issue
	if err := r.pickDB(ctx, tx).Model(&issue).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "更新问题状态失败", true, err)
	}
	return &issue, nil
}

// UpdatePriority 更新问题优先级。
func (r *IssueRepo) UpdatePriority(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID, priority bConst.IssuePriority) *xError.Error {
	r.log.Info(ctx, "UpdatePriority - 更新优先级")

	if err := r.pickDB(ctx, tx).Model(&entity.Issue{}).Where("id = ?", id).Update("priority", priority).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "更新优先级失败", true, err)
	}
	return nil
}

// UpdateAdminNote 更新内部备注。
func (r *IssueRepo) UpdateAdminNote(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID, note string) *xError.Error {
	r.log.Info(ctx, "UpdateAdminNote - 更新内部备注")

	if err := r.pickDB(ctx, tx).Model(&entity.Issue{}).Where("id = ?", id).Update("admin_note", note).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "更新内部备注失败", true, err)
	}
	return nil
}

func (r *IssueRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
