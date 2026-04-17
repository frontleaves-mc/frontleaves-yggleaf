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

// IssueAttachmentRepo 问题附件仓储，负责附件数据访问。
type IssueAttachmentRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewIssueAttachmentRepo 初始化并返回 IssueAttachmentRepo 实例。
func NewIssueAttachmentRepo(db *gorm.DB) *IssueAttachmentRepo {
	return &IssueAttachmentRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "IssueAttachmentRepo"),
	}
}

// Create 创建附件记录。
func (r *IssueAttachmentRepo) Create(ctx context.Context, tx *gorm.DB, att *entity.IssueAttachment) (*entity.IssueAttachment, *xError.Error) {
	r.log.Info(ctx, "Create - 创建附件记录")

	if err := r.pickDB(ctx, tx).Create(att).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建附件记录失败", true, err)
	}
	return att, nil
}

// GetByID 根据ID查询附件。
func (r *IssueAttachmentRepo) GetByID(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID) (*entity.IssueAttachment, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据ID查询附件")

	var att entity.IssueAttachment
	err := r.pickDB(ctx, tx).Model(&entity.IssueAttachment{}).Where("id = ?", id).First(&att).Error
	if err == nil {
		return &att, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询附件失败", true, err)
}

// ListByIssueID 查询某问题的全部附件列表（按创建时间正序）。
func (r *IssueAttachmentRepo) ListByIssueID(ctx context.Context, issueID xSnowflake.SnowflakeID) ([]entity.IssueAttachment, *xError.Error) {
	r.log.Info(ctx, "ListByIssueID - 查询问题附件列表")

	var list []entity.IssueAttachment
	err := r.db.WithContext(ctx).Model(&entity.IssueAttachment{}).
		Where("issue_id = ?", issueID).
		Order("created_at ASC").
		Find(&list).Error
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "查询附件列表失败", true, err)
	}
	return list, nil
}

// CountByIssueID 统计某问题的附件数量。
func (r *IssueAttachmentRepo) CountByIssueID(ctx context.Context, issueID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountByIssueID - 统计附件数量")

	var count int64
	err := r.db.WithContext(ctx).Model(&entity.IssueAttachment{}).Where("issue_id = ?", issueID).Count(&count).Error
	if err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计附件数量失败", true, err)
	}
	return count, nil
}

// DeleteByID 删除附件记录。
func (r *IssueAttachmentRepo) DeleteByID(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID) *xError.Error {
	r.log.Info(ctx, "DeleteByID - 删除附件记录")

	if err := r.pickDB(ctx, tx).Delete(&entity.IssueAttachment{}, id).Error; err != nil {
		return xError.NewError(ctx, xError.DatabaseError, "删除附件记录失败", true, err)
	}
	return nil
}

func (r *IssueAttachmentRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
