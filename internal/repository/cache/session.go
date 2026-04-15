package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	xCache "github.com/bamboo-services/bamboo-base-go/major/cache"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/redis/go-redis/v9"
)

// SessionCache Yggdrasil 会话缓存管理器
//
// 该类型封装了与 Redis 的交互，用于缓存 Yggdrasil 协议的会话数据。
// 缓存结构使用 String，键格式为 yggdrasil:session:<serverID>，值为 JSON 序列化的 SessionData。
//
// 注意: 该实现非并发安全，不建议在多 goroutine 中共享同一实例操作。
type SessionCache xCache.Cache

// SessionData Yggdrasil 会话数据结构，用于存储 hasJoined 验证所需的会话信息。
type SessionData struct {
	AccessToken string `json:"access_token"` // 访问令牌
	ProfileUUID string `json:"profile_uuid"` // 游戏档案 UUID
	ClientIP    string `json:"client_ip"`    // 客户端 IP 地址
}

// Set 将会话数据写入缓存，使用 serverID 作为键。
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - serverID: 会话标识（Minecraft 客户端发送的 serverId）。
//   - data: 会话数据指针。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *SessionCache) Set(ctx context.Context, serverID string, data *SessionData) error {
	if serverID == "" {
		return fmt.Errorf("会话标识为空")
	}
	if data == nil {
		return fmt.Errorf("会话数据为空")
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化会话数据失败: %w", err)
	}

	redisKey := bConst.CacheYggdrasilSession.Get(serverID).String()
	if err := c.RDB.Set(ctx, redisKey, jsonData, time.Duration(bConst.YggdrasilSessionExpireSec)*time.Second).Err(); err != nil {
		return err
	}
	return nil
}

// Get 从缓存中获取指定 serverID 的会话数据。
//
// 返回值:
//   - *SessionData: 会话数据，未命中缓存时返回 nil。
//   - bool: 是否存在（true 表示存在，false 表示不存在或 Redis 查询故障）。
//   - error: Redis 连接故障等非预期错误。redis.Nil 已转换为 found=false, error=nil。
func (c *SessionCache) Get(ctx context.Context, serverID string) (*SessionData, bool, error) {
	if serverID == "" {
		return nil, false, fmt.Errorf("会话标识为空")
	}

	result, err := c.RDB.Get(ctx, bConst.CacheYggdrasilSession.Get(serverID).String()).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			// 正常：会话不存在或已过期
			return nil, false, nil
		}
		// 异常：Redis 连接故障等
		return nil, false, err
	}

	var data SessionData
	if err := json.Unmarshal([]byte(result), &data); err != nil {
		return nil, false, fmt.Errorf("反序列化会话数据失败: %w", err)
	}
	return &data, true, nil
}

// Delete 删除指定 serverID 的会话缓存数据。
//
// 参数:
//   - ctx: 上下文对象，用于传递请求上下文。
//   - serverID: 会话标识（Minecraft 客户端发送的 serverId）。
//
// 返回值:
//   - error: 操作过程中发生的错误。
func (c *SessionCache) Delete(ctx context.Context, serverID string) error {
	if serverID == "" {
		return fmt.Errorf("会话标识为空")
	}

	return c.RDB.Del(ctx, bConst.CacheYggdrasilSession.Get(serverID).String()).Err()
}
