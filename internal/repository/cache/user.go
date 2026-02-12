package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	xCache "github.com/bamboo-services/bamboo-base-go/cache"
	xEnv "github.com/bamboo-services/bamboo-base-go/env"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/redis/go-redis/v9"
)

const (
	userCacheKeyPattern   = "user:entity:%s" // 用户实体缓存键模板
	userCacheKeyPrefix    = "fyl:"           // 用户实体缓存键默认前缀
	userCacheTimeLayout   = time.RFC3339Nano // 时间字段序列化格式
	userCacheFieldID      = "id"
	userCacheFieldUpdated = "updated_at"
	userCacheFieldName    = "username"
	userCacheFieldEmail   = "email"
	userCacheFieldPhone   = "phone"
	userCacheFieldRole    = "role_id"
	userCacheFieldHasBan  = "has_ban"
	userCacheFieldJailed  = "jailed_at"
)

// UserCache 用户实体缓存管理器
//
// 该类型封装了与 Redis 的交互，用于缓存用户实体的核心字段。
// 缓存结构使用 Hash，键格式为 user:entity:<key>，字段采用 JSON 标签命名。
//
// 注意: 该实现非并发安全，不建议在多 goroutine 中共享同一实例操作。
type UserCache xCache.Cache

// Get 从缓存中获取指定字段的值
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 用户缓存键。
//   - field: 要获取的字段名。
//
// 返回值:
//   - *string: 字段值的指针。
//   - bool: 是否命中缓存（true 表示命中，false 表示未命中）。
//   - error: 操作过程中发生的错误。
func (c *UserCache) Get(ctx context.Context, key string, field string) (*string, bool, error) {
	if key == "" {
		return nil, false, fmt.Errorf("用户标识为空")
	}
	if field == "" {
		return nil, false, fmt.Errorf("字段为空")
	}

	value, err := c.RDB.HGet(ctx, c.buildKey(key), field).Result()
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
//   - key: 用户缓存键。
//   - field: 要设置的字段名。
//   - value: 字段值的指针。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *UserCache) Set(ctx context.Context, key string, field string, value *string) error {
	if key == "" {
		return fmt.Errorf("用户标识为空")
	}
	if field == "" {
		return fmt.Errorf("字段为空")
	}
	if value == nil {
		return fmt.Errorf("缓存值为空")
	}

	if err := c.RDB.HSet(ctx, c.buildKey(key), field, *value).Err(); err != nil {
		return err
	}
	return c.RDB.Expire(ctx, c.buildKey(key), c.TTL).Err()
}

// GetAll 获取哈希表中的所有字段和值
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 用户缓存键。
//
// 返回值:
//   - map[string]string: 所有字段和值的映射。
//   - error: 操作过程中发生的错误。
func (c *UserCache) GetAll(ctx context.Context, key string) (map[string]string, error) {
	if key == "" {
		return nil, fmt.Errorf("用户标识为空")
	}

	return c.RDB.HGetAll(ctx, c.buildKey(key)).Result()
}

// GetAllStruct 获取哈希表中的所有字段和值，并映射为用户实体
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 用户缓存键。
//
// 返回值:
//   - *entity.User: 用户实体对象，未命中缓存时返回 nil。
//   - error: 操作过程中发生的错误。
func (c *UserCache) GetAllStruct(ctx context.Context, key string) (*entity.User, error) {
	if key == "" {
		return nil, fmt.Errorf("用户标识为空")
	}

	result, err := c.RDB.HGetAll(ctx, c.buildKey(key)).Result()
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
//   - key: 用户缓存键。
//   - fields: 字段名到值指针的映射。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *UserCache) SetAll(ctx context.Context, key string, fields map[string]*string) error {
	if key == "" {
		return fmt.Errorf("用户标识为空")
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

	if err := c.RDB.HSet(ctx, c.buildKey(key), values).Err(); err != nil {
		return err
	}
	return c.RDB.Expire(ctx, c.buildKey(key), c.TTL).Err()
}

// SetAllStruct 批量设置用户实体的缓存字段
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 用户缓存键。
//   - value: 用户实体指针。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *UserCache) SetAllStruct(ctx context.Context, key string, value *entity.User) error {
	if key == "" {
		return fmt.Errorf("用户标识为空")
	}
	if value == nil {
		return fmt.Errorf("缓存值为空")
	}

	fields := buildUserFields(value)
	if len(fields) == 0 {
		return nil
	}

	if err := c.RDB.HSet(ctx, c.buildKey(key), fields).Err(); err != nil {
		return err
	}
	return c.RDB.Expire(ctx, c.buildKey(key), c.TTL).Err()
}

// Exists 检查指定字段是否存在
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 用户缓存键。
//   - field: 要检查的字段名。
//
// 返回值:
//   - bool: 字段是否存在。
//   - error: 操作过程中发生的错误。
func (c *UserCache) Exists(ctx context.Context, key string, field string) (bool, error) {
	if key == "" {
		return false, fmt.Errorf("用户标识为空")
	}
	if field == "" {
		return false, fmt.Errorf("字段为空")
	}

	return c.RDB.HExists(ctx, c.buildKey(key), field).Result()
}

// Remove 从缓存中移除指定的字段
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 用户缓存键。
//   - fields: 要移除的字段名列表。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *UserCache) Remove(ctx context.Context, key string, fields ...string) error {
	if key == "" {
		return fmt.Errorf("用户标识为空")
	}
	if len(fields) == 0 {
		return nil
	}

	return c.RDB.HDel(ctx, c.buildKey(key), fields...).Err()
}

// Delete 删除用户缓存数据
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - key: 用户缓存键。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *UserCache) Delete(ctx context.Context, key string) error {
	if key == "" {
		return fmt.Errorf("用户标识为空")
	}

	return c.RDB.Del(ctx, c.buildKey(key)).Err()
}

func (c *UserCache) buildKey(key string) string {
	prefix := xEnv.GetEnvString(xEnv.NoSqlPrefix, userCacheKeyPrefix)
	return fmt.Sprintf(prefix+userCacheKeyPattern, key)
}

func buildUserFields(user *entity.User) map[string]any {
	fields := map[string]any{
		userCacheFieldName:   user.Username,
		userCacheFieldHasBan: strconv.FormatBool(user.HasBan),
	}
	if !user.ID.IsZero() {
		fields[userCacheFieldID] = user.ID.String()
	}
	if !user.UpdatedAt.IsZero() {
		fields[userCacheFieldUpdated] = user.UpdatedAt.Format(userCacheTimeLayout)
	}
	if user.Email != nil {
		fields[userCacheFieldEmail] = *user.Email
	}
	if user.Phone != nil {
		fields[userCacheFieldPhone] = *user.Phone
	}
	if user.RoleName != nil {
		fields[userCacheFieldRole] = *user.RoleName
	}
	if user.JailedAt != nil {
		fields[userCacheFieldJailed] = user.JailedAt.Format(userCacheTimeLayout)
	}
	return fields
}

func parseUserFields(fields map[string]string) (*entity.User, error) {
	user := &entity.User{}
	if value, ok := fields[userCacheFieldID]; ok && value != "" {
		id, err := xSnowflake.ParseSnowflakeID(value)
		if err != nil {
			return nil, fmt.Errorf("字段 %s 解析失败: %w", userCacheFieldID, err)
		}
		user.ID = id
	}
	if value, ok := fields[userCacheFieldUpdated]; ok && value != "" {
		parsed, err := parseTime(value)
		if err != nil {
			return nil, fmt.Errorf("字段 %s 解析失败: %w", userCacheFieldUpdated, err)
		}
		user.UpdatedAt = parsed
	}
	if value, ok := fields[userCacheFieldName]; ok {
		user.Username = value
	}
	if value, ok := fields[userCacheFieldEmail]; ok {
		email := value
		user.Email = &email
	}
	if value, ok := fields[userCacheFieldPhone]; ok {
		phone := value
		user.Phone = &phone
	}
	if value, ok := fields[userCacheFieldRole]; ok {
		roleName := value
		user.RoleName = &roleName
	}
	if value, ok := fields[userCacheFieldHasBan]; ok && value != "" {
		flag, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("字段 %s 解析失败: %w", userCacheFieldHasBan, err)
		}
		user.HasBan = flag
	}
	if value, ok := fields[userCacheFieldJailed]; ok && value != "" {
		parsed, err := parseTime(value)
		if err != nil {
			return nil, fmt.Errorf("字段 %s 解析失败: %w", userCacheFieldJailed, err)
		}
		user.JailedAt = &parsed
	}
	return user, nil
}

func parseTime(value string) (time.Time, error) {
	parsed, err := time.Parse(userCacheTimeLayout, value)
	if err == nil {
		return parsed, nil
	}
	return time.Parse(time.RFC3339, value)
}
