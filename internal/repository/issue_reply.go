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
	if err := query.Preload("User").Order("created_at ASC").Offset(offset).Limit(pageSize).Find(&list).Error; err != nil {
		return nil, 0, xError.NewError(ctx, xError.DatabaseError, "查询回复列表失败", true, err)
	}
	return list, total, nil
}

// GetLatestAdminReply 获取指定 Issue 中最近一条管理员回复。
func (r *IssueReplyRepo) GetLatestAdminReply(ctx context.Context, issueID xSnowflake.SnowflakeID) (*entity.IssueReply, bool, *xError.Error) {
	r.log.Info(ctx, "GetLatestAdminReply - 获取最近管理员回复")

	var reply entity.IssueReply
	err := r.db.WithContext(ctx).
		Where("issue_id = ?", issueID).
		Where("is_admin_reply = ?", true).
		Preload("User").
		Order("created_at DESC").
		Limit(1).
		First(&reply).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, nil
		}
		return nil, false, xError.NewError(ctx, xError.DatabaseError, "获取管理员回复失败", true, err)
	}

	return &reply, true, nil
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

// CountBatchByIssueIDs 批量统计多个问题的回复数量，返回 issueID → count 映射。
func (r *IssueReplyRepo) CountBatchByIssueIDs(ctx context.Context, issueIDs []xSnowflake.SnowflakeID) (map[xSnowflake.SnowflakeID]int64, *xError.Error) {
	r.log.Info(ctx, "CountBatchByIssueIDs - 批量统计回复数量")

	if len(issueIDs) == 0 {
		return make(map[xSnowflake.SnowflakeID]int64), nil
	}

	var results []struct {
		IssueID xSnowflake.SnowflakeID
		Count   int64
	}
	err := r.db.WithContext(ctx).Model(&entity.IssueReply{}).
		Select("issue_id, COUNT(*) as count").
		Where("issue_id IN ?", issueIDs).
		Group("issue_id").
		Find(&results).Error
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "批量统计回复数量失败", true, err)
	}

	countMap := make(map[xSnowflake.SnowflakeID]int64, len(issueIDs))
	for _, res := range results {
		countMap[res.IssueID] = res.Count
	}
	return countMap, nil
}

func (r *IssueReplyRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
