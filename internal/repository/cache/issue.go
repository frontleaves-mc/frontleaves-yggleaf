package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/redis/go-redis/v9"
)

const (
	issueCachePrefix = "issue:"
	issueCacheTTL    = 15 * time.Minute
)

// IssueCache 问题缓存（Redis）。
//
// 使用 String 结构存储 JSON 序列化的 Issue 实体，键格式为 issue:<id>。
// 缓存默认 TTL 为 15 分钟。
type IssueCache struct {
	rdb *redis.Client
}

// NewIssueCache 创建 IssueCache 实例。
func NewIssueCache(rdb *redis.Client) *IssueCache {
	return &IssueCache{
		rdb: rdb,
	}
}

// key 生成缓存键。
func (c *IssueCache) key(id xSnowflake.SnowflakeID) string {
	return fmt.Sprintf("%s%s", issueCachePrefix, id)
}

// Set 缓存问题详情。
func (c *IssueCache) Set(ctx context.Context, id xSnowflake.SnowflakeID, issue *entity.Issue) error {
	data, err := json.Marshal(issue)
	if err != nil {
		return err
	}
	return c.rdb.Set(ctx, c.key(id), data, issueCacheTTL).Err()
}

// Get 获取缓存的问题详情。
//
// 返回值:
//   - *entity.Issue: 问题实体，未命中缓存时返回 nil。
//   - error: 操作过程中发生的错误（redis.Nil 视为未命中，返回 nil, nil）。
func (c *IssueCache) Get(ctx context.Context, id xSnowflake.SnowflakeID) (*entity.Issue, error) {
	data, err := c.rdb.Get(ctx, c.key(id)).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, err
	}
	var issue entity.Issue
	if err := json.Unmarshal(data, &issue); err != nil {
		return nil, err
	}
	return &issue, nil
}

// Del 删除缓存。
func (c *IssueCache) Del(ctx context.Context, id xSnowflake.SnowflakeID) error {
	return c.rdb.Del(ctx, c.key(id)).Err()
}
