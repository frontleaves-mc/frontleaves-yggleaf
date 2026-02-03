package startup

import (
	xEnv "github.com/bamboo-services/bamboo-base-go/env"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	"github.com/redis/go-redis/v9"
)

func (r *Reg) nosqlInit() {
	log := xLog.WithName(xLog.NamedINIT)
	log.Debug(r.Context, "正在连接缓存...")

	// 连接 Redis
	rdb := redis.NewClient(&redis.Options{
		Addr:     xEnv.GetEnvString(xEnv.NoSqlHost, "localhost") + ":" + xEnv.GetEnvString(xEnv.NoSqlPort, "6379"),
		Password: xEnv.GetEnvString(xEnv.NoSqlPass, ""),
		DB:       xEnv.GetEnvInt(xEnv.NoSqlDB, 0),
		PoolSize: xEnv.GetEnvInt(xEnv.NoSqlPoolSize, 10),
	})

	log.Info(r.Context, "缓存连接成功")
	r.RDB = rdb
}
