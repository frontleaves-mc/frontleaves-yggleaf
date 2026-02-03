package startup

import (
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/startup/prepare"
)

// businessDataPrepare 初始化业务数据。
//
// 该方法用于在系统启动时，对必要的业务数据进行预加载或初始化操作，确保后续功能正常运行。
// 通常包括数据缓存填充、配置同步或第三方服务预热等逻辑。
//
// 注意: 确保在数据库与缓存初始化完成后调用此方法，以避免关键依赖未准备好。
func (r *Reg) businessDataPrepare() {
	log := xLog.WithName(xLog.NamedINIT)

	// 数据预加载
	prepare.New(log, r.DB, r.Context).Prepare()
}
