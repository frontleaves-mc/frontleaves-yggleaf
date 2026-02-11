package startup

import (
	"context"

	xEnv "github.com/bamboo-services/bamboo-base-go/env"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	"github.com/redis/go-redis/v9"
)

// nosqlInit 初始化并连接 NoSQL (Redis) 客户端。
//
// 该方法通过读取环境变量配置 Redis 连接参数（地址、密码、DB、池大小等），
// 创建一个新的 Redis 客户端实例。
//
// 参数说明:
//   - ctx: 用于日志追踪和上下文控制。
//
// 返回值:
//   - any: 初始化成功的 `*redis.Client` 实例。
//   - error: 连接过程中的错误（当前实现总是返回 nil）。
//
// 注意: 此函数仅负责建立客户端对象，不进行 Ping 健康检查。请确保环境变量
// (如 NoSqlHost, NoSqlPort) 已正确配置。
func (r *reg) nosqlInit(ctx context.Context) (any, error) {
	log := xLog.WithName(xLog.NamedINIT)
	log.Debug(ctx, "正在连接缓存...")

	// 连接 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     xEnv.GetEnvString(xEnv.NoSqlHost, "localhost") + ":" + xEnv.GetEnvString(xEnv.NoSqlPort, "6379"),
		Password: xEnv.GetEnvString(xEnv.NoSqlPass, ""),
		DB:       xEnv.GetEnvInt(xEnv.NoSqlDatabase, 0),
		PoolSize: xEnv.GetEnvInt(xEnv.NoSqlPoolSize, 10),
	})

	log.Info(ctx, "缓存连接成功")
	return rdb, nil
}
