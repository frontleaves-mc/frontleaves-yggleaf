package bConst

import (
	"fmt"

	xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"
)

type RedisKey string

const (
	CacheUserinfo         RedisKey = "user:info:%s"            // CacheUserinfo 用户实体缓存（UserCache 使用）
	CacheUserAccess       RedisKey = "user:access:%s"          // CacheUserAccess AccessToken→User 缓存（AccessUserCache 使用，%s = MD5(token)）
	CacheYggdrasilSession RedisKey = "yggdrasil:session:%s"    // CacheYggdrasilSession Yggdrasil 会话缓存（%s = serverId，已通过 JoinServerRequest.ServerID 的 max=256 binding tag 限制长度）
)

// Get 返回一个格式化后的 `RedisKey`，根据输入参数对原始键进行格式化并生成新的键。
func (k RedisKey) Get(args ...interface{}) RedisKey {
	validKey := xEnv.GetEnvString(xEnv.NoSqlPrefix, "fyl:") + string(k)
	return RedisKey(fmt.Sprintf(validKey, args...))
}

// String 返回 `RedisKey` 的字符串表示形式，主要用于将自定义键类型转换为其底层字符串值。
func (k RedisKey) String() string {
	return string(k)
}
