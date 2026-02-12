package logic

import (
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

type logic struct {
	db  *gorm.DB             // GORM 数据库实例
	rdb *redis.Client        // Redis 客户端实例
	log *xLog.LogNamedLogger // 日志实例
}
