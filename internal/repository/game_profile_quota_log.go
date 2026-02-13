package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/error"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	entityType "github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity/type"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// GameProfileQuotaLogRepo 游戏档案配额日志仓储，负责配额日志数据访问。
type GameProfileQuotaLogRepo struct {
	db  *gorm.DB
	log *xLog.LogNamedLogger
}

// NewGameProfileQuotaLogRepo 初始化并返回 GameProfileQuotaLogRepo 实例。
func NewGameProfileQuotaLogRepo(db *gorm.DB) *GameProfileQuotaLogRepo {
	return &GameProfileQuotaLogRepo{
		db:  db,
		log: xLog.WithName(xLog.NamedREPO, "GameProfileQuotaLogRepo"),
	}
}

// Create 创建一条游戏档案配额日志。
func (r *GameProfileQuotaLogRepo) Create(
	ctx context.Context,
	tx *gorm.DB,
	userID xSnowflake.SnowflakeID,
	opType entityType.ObType,
	delta int32,
	beforeUsed int32,
	beforeTotal int32,
	refProfileID *xSnowflake.SnowflakeID,
	remark *string,
) (*entity.GameProfileQuotaLog, *xError.Error) {
	r.log.Info(ctx, "Create - 创建游戏档案配额日志")

	if !opType.IsValid() {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效操作类型", false)
	}
	if delta < 0 {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效变化量：delta 不能小于 0", false)
	}

	afterUsed := beforeUsed
	if opType.Type == 0 {
		afterUsed = beforeUsed - delta
	} else {
		afterUsed = beforeUsed + delta
	}
	if afterUsed < 0 {
		return nil, xError.NewError(ctx, xError.ParameterError, "无效变化量：变更后已使用额度不能小于 0", false)
	}

	refProfileStr := "0"
	if refProfileID != nil {
		refProfileStr = refProfileID.String()
	}

	idempotencyKey := fmt.Sprintf("%s:%s:%s:%d", opType.String(), userID.String(), refProfileStr, time.Now().UnixNano())
	quotaLog := &entity.GameProfileQuotaLog{
		UserID:         userID,
		OpType:         opType,
		Delta:          delta,
		BeforeUsed:     beforeUsed,
		AfterUsed:      afterUsed,
		BeforeTotal:    beforeTotal,
		AfterTotal:     beforeTotal,
		IdempotencyKey: idempotencyKey,
		RefProfileID:   refProfileID,
		Remark:         remark,
	}

	if err := r.pickDB(ctx, tx).Create(quotaLog).Error; err != nil {
		return nil, xError.NewError(nil, xError.DatabaseError, "创建游戏档案配额日志失败", false, err)
	}
	return quotaLog, nil
}

// GetByID 根据日志 ID 查询游戏档案配额日志。
func (r *GameProfileQuotaLogRepo) GetByID(ctx context.Context, tx *gorm.DB, logID xSnowflake.SnowflakeID, forUpdate bool) (*entity.GameProfileQuotaLog, bool, *xError.Error) {
	r.log.Info(ctx, "GetByID - 根据日志 ID 获取游戏档案配额日志")

	query := r.pickDB(ctx, tx).Model(&entity.GameProfileQuotaLog{}).Where("id = ?", logID)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var quotaLog entity.GameProfileQuotaLog
	err := query.First(&quotaLog).Error
	if err == nil {
		return &quotaLog, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(nil, xError.DatabaseError, "查询游戏档案配额日志失败", false, err)
}

// GetByIdempotencyKey 根据幂等键查询游戏档案配额日志。
func (r *GameProfileQuotaLogRepo) GetByIdempotencyKey(ctx context.Context, tx *gorm.DB, idempotencyKey string, forUpdate bool) (*entity.GameProfileQuotaLog, bool, *xError.Error) {
	r.log.Info(ctx, "GetByIdempotencyKey - 根据幂等键获取游戏档案配额日志")

	query := r.pickDB(ctx, tx).Model(&entity.GameProfileQuotaLog{}).Where("idempotency_key = ?", idempotencyKey)
	if forUpdate {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}

	var quotaLog entity.GameProfileQuotaLog
	err := query.First(&quotaLog).Error
	if err == nil {
		return &quotaLog, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(nil, xError.DatabaseError, "查询游戏档案配额日志失败", false, err)
}

func (r *GameProfileQuotaLogRepo) pickDB(ctx context.Context, tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx.WithContext(ctx)
	}
	return r.db.WithContext(ctx)
}
