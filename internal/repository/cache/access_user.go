package cache

import (
	"context"
	"errors"
	"fmt"

	xCache "github.com/bamboo-services/bamboo-base-go/cache"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/redis/go-redis/v9"
)

// AccessUserCache 访问令牌用户缓存管理器
//
// 该类型封装了与 Redis 的交互，用于通过 AccessToken（MD5 摘要）缓存用户实体。
// 缓存结构使用 Hash，键格式为 user:access:<md5(token)>，字段与 UserCache 共用。
//
// 注意: 该实现非并发安全，不建议在多 goroutine 中共享同一实例操作。
type AccessUserCache xCache.Cache

// Get 从缓存中获取指定字段的值
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//   - field: 要获取的字段名。
//
// 返回值:
//   - *string: 字段值的指针。
//   - bool: 是否命中缓存（true 表示命中，false 表示未命中）。
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) Get(ctx context.Context, key string, field string) (*string, bool, error) {
	if key == "" {
		return nil, false, fmt.Errorf("访问令牌标识为空")
	}
	if field == "" {
		return nil, false, fmt.Errorf("字段为空")
	}

	value, err := c.RDB.HGet(ctx, bConst.CacheUserAccess.Get(key).String(), field).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return &value, true, nil
}

// Set 设置指定字段的值
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//   - field: 要设置的字段名。
//   - value: 字段值的指针。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) Set(ctx context.Context, key string, field string, value *string) error {
	if key == "" {
		return fmt.Errorf("访问令牌标识为空")
	}
	if field == "" {
		return fmt.Errorf("字段为空")
	}
	if value == nil {
		return fmt.Errorf("缓存值为空")
	}

	redisKey := bConst.CacheUserAccess.Get(key).String()
	if err := c.RDB.HSet(ctx, redisKey, field, *value).Err(); err != nil {
		return err
	}
	return c.RDB.Expire(ctx, redisKey, c.TTL).Err()
}

// GetAll 获取哈希表中的所有字段和值
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//
// 返回值:
//   - map[string]string: 所有字段和值的映射。
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) GetAll(ctx context.Context, key string) (map[string]string, error) {
	if key == "" {
		return nil, fmt.Errorf("访问令牌标识为空")
	}

	return c.RDB.HGetAll(ctx, bConst.CacheUserAccess.Get(key).String()).Result()
}

// GetAllStruct 获取哈希表中的所有字段和值，并映射为用户实体
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//
// 返回值:
//   - *entity.User: 用户实体对象，未命中缓存时返回 nil。
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) GetAllStruct(ctx context.Context, key string) (*entity.User, error) {
	if key == "" {
		return nil, fmt.Errorf("访问令牌标识为空")
	}

	result, err := c.RDB.HGetAll(ctx, bConst.CacheUserAccess.Get(key).String()).Result()
	if err != nil {
		return nil, err
	}
	if len(result) == 0 {
		return nil, nil
	}

	return parseUserFields(result)
}

// SetAll 批量设置多个字段的值
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//   - fields: 字段名到值指针的映射。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) SetAll(ctx context.Context, key string, fields map[string]*string) error {
	if key == "" {
		return fmt.Errorf("访问令牌标识为空")
	}
	if len(fields) == 0 {
		return nil
	}

	values := make(map[string]any, len(fields))
	for field, value := range fields {
		if field == "" {
			return fmt.Errorf("字段为空")
		}
		if value == nil {
			return fmt.Errorf("缓存值为空")
		}
		values[field] = *value
	}

	redisKey := bConst.CacheUserAccess.Get(key).String()
	if err := c.RDB.HSet(ctx, redisKey, values).Err(); err != nil {
		return err
	}
	return c.RDB.Expire(ctx, redisKey, c.TTL).Err()
}

// SetAllStruct 批量设置用户实体的缓存字段
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//   - value: 用户实体指针。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) SetAllStruct(ctx context.Context, key string, value *entity.User) error {
	if key == "" {
		return fmt.Errorf("访问令牌标识为空")
	}
	if value == nil {
		return fmt.Errorf("缓存值为空")
	}

	fields := buildUserFields(value)
	if len(fields) == 0 {
		return nil
	}

	redisKey := bConst.CacheUserAccess.Get(key).String()
	if err := c.RDB.HSet(ctx, redisKey, fields).Err(); err != nil {
		return err
	}
	return c.RDB.Expire(ctx, redisKey, c.TTL).Err()
}

// Exists 检查指定字段是否存在
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//   - field: 要检查的字段名。
//
// 返回值:
//   - bool: 字段是否存在。
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) Exists(ctx context.Context, key string, field string) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("访问令牌标识为空")
	}
	if field == "" {
		return false, fmt.Errorf("字段为空")
	}

	return c.RDB.HExists(ctx, bConst.CacheUserAccess.Get(key).String(), field).Result()
}

// Remove 从缓存中移除指定的字段
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//   - fields: 要移除的字段名列表。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) Remove(ctx context.Context, key string, fields ...string) error {
	if key == "" {
		return fmt.Errorf("访问令牌标识为空")
	}
	if len(fields) == 0 {
		return nil
	}

	return c.RDB.HDel(ctx, bConst.CacheUserAccess.Get(key).String(), fields...).Err()
}

// Delete 删除访问令牌用户缓存数据
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 访问令牌的 MD5 摘要。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *AccessUserCache) Delete(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("访问令牌标识为空")
	}

	return c.RDB.Del(ctx, bConst.CacheUserAccess.Get(key).String()).Err()
}
