package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	xCache "github.com/bamboo-services/bamboo-base-go/major/cache"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
)

// IssueCache 问题缓存（Redis）。
//
// 使用 String 结构存储 JSON 序列化的 Issue 实体，键格式为 fyl:issue:<id>。
type IssueCache xCache.Cache

// issueCacheData 缓存专用结构体，显式声明所有字段以避免 BaseEntity json:"-" 导致 CreatedAt/UpdatedAt 丢失。
type issueCacheData struct {
	ID          xSnowflake.SnowflakeID `json:"id"`
	UserID      xSnowflake.SnowflakeID `json:"user_id"`
	IssueTypeID xSnowflake.SnowflakeID `json:"issue_type_id"`
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Status      string                 `json:"status"`
	Priority    string                 `json:"priority"`
	AdminNote   string                 `json:"admin_note,omitempty"`
	ClosedAt    *time.Time             `json:"closed_at,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// Set 缓存问题详情。
func (c *IssueCache) Set(ctx context.Context, id xSnowflake.SnowflakeID, issue *entity.Issue) error {
	data := issueCacheData{
		ID:          issue.ID,
		UserID:      issue.UserID,
		IssueTypeID: issue.IssueTypeID,
		Title:       issue.Title,
		Content:     issue.Content,
		Status:      string(issue.Status),
		Priority:    string(issue.Priority),
		AdminNote:   issue.AdminNote,
		ClosedAt:    issue.ClosedAt,
		CreatedAt:   issue.CreatedAt,
		UpdatedAt:   issue.UpdatedAt,
	}
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return c.RDB.Set(ctx, bConst.CacheIssue.Get(id.String()).String(), jsonBytes, c.TTL).Err()
}

// Get 获取缓存的问题详情。
//
// 返回值:
//   - *entity.Issue: 问题实体，未命中缓存时返回 nil。
//   - error: 操作过程中发生的错误（redis.Nil 视为未命中，返回 nil, nil）。
func (c *IssueCache) Get(ctx context.Context, id xSnowflake.SnowflakeID) (*entity.Issue, error) {
	data, err := c.RDB.Get(ctx, bConst.CacheIssue.Get(id.String()).String()).Bytes()
	if err != nil {
		if err == redis.Nil { // 缓存未命中
			return nil, nil
		}
		return nil, err
	}
	var cached issueCacheData
	if err := json.Unmarshal(data, &cached); err != nil {
		return nil, err
	}
	return &entity.Issue{
		BaseEntity: xModels.BaseEntity{
			ID:        cached.ID,
			CreatedAt: cached.CreatedAt,
			UpdatedAt: cached.UpdatedAt,
		},
		UserID:      cached.UserID,
		IssueTypeID: cached.IssueTypeID,
		Title:       cached.Title,
		Content:     cached.Content,
		Status:      bConst.IssueStatus(cached.Status),
		Priority:    bConst.IssuePriority(cached.Priority),
		AdminNote:   cached.AdminNote,
		ClosedAt:    cached.ClosedAt,
	}, nil
}

// Del 删除缓存。
func (c *IssueCache) Del(ctx context.Context, id xSnowflake.SnowflakeID) error {
	return c.RDB.Del(ctx, bConst.CacheIssue.Get(id.String()).String()).Err()
}
