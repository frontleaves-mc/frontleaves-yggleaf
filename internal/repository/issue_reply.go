package repository

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"gorm.io/gorm"
)

// IssueReplyRepo 问题回复仓储，负责问题回复数据访问。
type IssueReplyRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewIssueReplyRepo 初始化并返回 IssueReplyRepo 实例。
func NewIssueReplyRepo(db *gorm.DB) *IssueReplyRepo {
	return &IssueReplyRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "IssueReplyRepo"),
	}
}

// Create 创建回复记录。
func (r *IssueReplyRepo) Create(ctx context.Context, tx *gorm.DB, reply *entity.IssueReply) (*entity.IssueReply, *xError.Error) {
	r.log.Info(ctx, "Create - 创建回复")

	if err := r.pickDB(ctx, tx).Create(reply).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建回复失败", true, err)
	}
	return reply, nil
}

// ListByIssueID 分页查询某问题的回复列表（按创建时间正序）。
func (r *IssueReplyRepo) ListByIssueID(ctx context.Context, issueID xSnowflake.SnowflakeID, page, pageSize int) ([]entity.IssueReply, int64, *xError.Error) {
	r.log.Info(ctx, "ListByIssueID - 分页查询回复列表")

	var total int64
	query := r.db.WithContext(ctx).Model(&entity.IssueReply{}).Where("issue_id = ?", issueID)
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询回复总数失败", true, err)
	}

	var list []entity.IssueReply
	offset := (page - 1) * pageSize
	if err := query.Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询回复列表失败", true, err)
	}
	return list, total, nil
}

// CountByIssueID 统计某问题的回复数量。
func (r *IssueReplyRepo) CountByIssueID(ctx context.Context, issueID xSnowflake.SnowflakeID) (int64, *xError.Error) {
	r.log.Info(ctx, "CountByIssueID - 统计回复数量")

	var count int64
	err := r.db.WithContext(ctx).Model(&entity.IssueReply{}).Where("issue_id = ?", issueID).Count(&count).Error
	if err != nil {
		return 0, xError.NewError(ctx, xError.DatabaseError, "统计回复数量失败", true, err)
	}
	return count, nil
}

func (r *IssueReplyRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
