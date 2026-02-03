package prepare

import (
	"context"

	xLog "github.com/bamboo-services/bamboo-base-go/log"
	"gorm.io/gorm"
)

type Prepare struct {
	log *xLog.LogNamedLogger // 日志实例
	db  *gorm.DB             // GORM 数据库实例
	ctx context.Context      // 上下文实例
}

func New(log *xLog.LogNamedLogger, db *gorm.DB, ctx context.Context) *Prepare {
	return &Prepare{
		log: log,
		db:  db,
		ctx: ctx,
	}
}

func (p *Prepare) Prepare() {
	p.prepareRole()
}
