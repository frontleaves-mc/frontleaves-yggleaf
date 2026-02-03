package startup

import (
	"context"

	xReg "github.com/bamboo-services/bamboo-base-go/register"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// Reg 表示应用程序的核心注册结构，包含所有初始化后的组件实例。
type Reg struct {
	Context context.Context // 上下文，用于控制取消和超时
	Serve   *gin.Engine     // Gin 引擎实例
	DB      *gorm.DB        // GORM 数据库实例
	RDB     *redis.Client   // Redis 客户端实例
}

// newRegister 创建一个新的注册结构实例
func newRegister(reg *xReg.Reg) *Reg {
	return &Reg{
		Context: reg.Context,
		Serve:   reg.Serve,
	}
}

func Register(reg *xReg.Reg) *Reg {
	businessReg := newRegister(reg)

	// 初始化注册
	businessReg.databaseInit()
	businessReg.nosqlInit()

	businessReg.businessContextInit()

	// 初始化数据
	businessReg.businessDataPrepare()

	return businessReg
}
