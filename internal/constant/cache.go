package bConst

import (
	"fmt"

	xEnv "github.com/bamboo-services/bamboo-base-go/env"
)

type RedisKey string

const (
	CacheUserinfo RedisKey = "user:info:%s" // CacheUserinfo 用于存储用户详细信息的 Redis 键名模板
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
