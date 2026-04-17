package txn

import (
	"context"
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	"gorm.io/gorm"
)

// IssueTxnRepo 问题事务协调仓储。
type IssueTxnRepo struct {
	db         *gorm.DB
	log        *xLog.LogNamedLogger
	issueRepo  *repository.IssueRepo
	replyRepo  *repository.IssueReplyRepo
	attachRepo *repository.IssueAttachmentRepo
}

func NewIssueTxnRepo(
	db *gorm.DB,
	issueRepo *repository.IssueRepo,
	replyRepo *repository.IssueReplyRepo,
	attachRepo *repository.IssueAttachmentRepo,
) *IssueTxnRepo {
	return &IssueTxnRepo{
		db:         db,
		log:        xLog.WithName(xLog.NamedREPO, "IssueTxnRepo"),
		issueRepo:  issueRepo,
		replyRepo:  replyRepo,
		attachRepo: attachRepo,
	}
}

// CreateIssue 创建问题并设置默认状态为 registered。
func (t *IssueTxnRepo) CreateIssue(ctx context.Context, issue *entity.Issue) (*entity.Issue, *xError.Error) {
	t.log.Info(ctx, "CreateIssue - 事务创建问题")
	issue.Status = bConst.IssueStatusRegistered
	if issue.Priority == "" {
		issue.Priority = bConst.PriorityMedium
	}
	var created *entity.Issue
	var bizErr *xError.Error
	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created, bizErr = t.issueRepo.Create(ctx, tx, issue)
		if bizErr != nil {
			return bizErr
		}
		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建问题事务失败", true, err)
	}
	return created, nil
}

// CreateReplyAndUpdateTimestamp 创建回复并更新问题的 updated_at。
func (t *IssueTxnRepo) CreateReplyAndUpdateTimestamp(
	ctx context.Context,
	reply *entity.IssueReply,
	issueID xSnowflake.SnowflakeID,
) (*entity.IssueReply, *xError.Error) {
	t.log.Info(ctx, "CreateReplyAndUpdateTimestamp - 事务创建回复")
	var created *entity.IssueReply
	var bizErr *xError.Error
	err := t.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created, bizErr = t.replyRepo.Create(ctx, tx, reply)
		if bizErr != nil {
			return bizErr
		}
		now := time.Now()
		if uErr := tx.WithContext(ctx).Model(&entity.Issue{}).Where("id = ?", issueID).Update("updated_at", now).Error; uErr != nil {
			bizErr = xError.NewError(ctx, xError.DatabaseError, "更新问题时间戳失败", true, uErr)
			return bizErr
		}
		return nil
	})
	if bizErr != nil {
		return nil, bizErr
	}
	if err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "创建回复事务失败", true, err)
	}
	return created, nil
}

// UpdateStatusWithCloseTime 更新状态，若目标为 closed 则记录 closed_at。
func (t *IssueTxnRepo) UpdateStatusWithCloseTime(
	ctx context.Context,
	issueID xSnowflake.SnowflakeID,
	status bConst.IssueStatus,
) *xError.Error {
	t.log.Info(ctx, "UpdateStatusWithCloseTime - 事务更新状态")
	var closedAt interface{}
	if status == bConst.IssueStatusClosed {
		tm := time.Now()
		closedAt = &tm
	} else {
		closedAt = nil
	}
	_, bizErr := t.issueRepo.UpdateStatus(ctx, nil, issueID, status, closedAt)
	return bizErr
}
