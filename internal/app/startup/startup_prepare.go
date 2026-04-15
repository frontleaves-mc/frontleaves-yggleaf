package startup

import (
	"context"

	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/startup/prepare"
)

// businessDataPrepare 初始化业务数据。
//
// 该方法用于在系统启动时，对必要的业务数据进行预加载或初始化操作，确保后续功能正常运行。
// 通常包括数据缓存填充、配置同步或第三方服务预热等逻辑。
//
// 注意: 确保在数据库与缓存初始化完成后调用此方法，以避免关键依赖未准备好。
func (r *reg) businessDataPrepare(ctx context.Context) (any, error) {
	log := xLog.WithName(xLog.NamedINIT)

	// 数据预加载
	prepare.New(log, ctx).Prepare()

	return nil, nil
}

// yggdrasilRSAKeyInit 初始化 Yggdrasil RSA 签名密钥对。
//
// 该方法通过框架节点注册机制执行，返回加载完成的密钥对实例。
// 框架会将返回值通过 context.WithValue 注入到上下文中，
// 供后续 NewYggdrasilLogic 通过 CtxYggdrasilRSAKeyPair 键获取。
//
// 密钥加载逻辑：
//  1. 从环境变量读取密钥文件路径
//  2. 若密钥文件不存在，自动生成 RSA-2048 密钥对
//  3. 加载私钥并导出公钥 PEM
//
// 返回值:
//   - *bConst.RSAKeyPair: 加载完成的密钥对（框架存入上下文）
//   - error: 加载或生成过程中的错误
func (r *reg) yggdrasilRSAKeyInit(ctx context.Context) (any, error) {
	log := xLog.WithName(xLog.NamedINIT)
	log.Info(ctx, "正在初始化 Yggdrasil RSA 密钥对...")

	keyPair, err := prepare.LoadRSAKeyPair()
	if err != nil {
		log.Error(ctx, "Yggdrasil RSA 密钥对初始化失败: "+err.Error())
		return nil, err
	}

	log.Info(ctx, "Yggdrasil RSA 密钥对加载成功")
	return keyPair, nil
}
