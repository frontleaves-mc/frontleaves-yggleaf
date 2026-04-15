// Package yggdrasil 提供 Yggdrasil 外置登录协议的 HTTP 请求处理器基础设施。
//
// 该包定义了 Yggdrasil Handler 的基类和共享服务，供 server、client、share
// 三个子包中的具体处理器复用。与现有 handler 包中的 OAuth2/SSO 认证体系完全隔离，
// 使用独立的令牌管理和中间件。
package yggdrasil

import (
	"context"

	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic/yggdrasil"
)

// yggdrasilService Yggdrasil 协议核心业务逻辑服务。
//
// 封装了 YggdrasilLogic 实例，作为 Handler 与 Logic 层之间的桥梁。
// 该服务与现有 handler 包中的 service 结构体完全独立，两套认证体系互不干扰。
type yggdrasilService struct {
	yggdrasilLogic *yggdrasil.YggdrasilLogic
}

// YggdrasilBase Yggdrasil Handler 的基类结构体。
//
// 作为 server/client/share 三个子包中具体 Handler 的嵌入基础，
// 提供统一的日志记录和 Yggdrasil 业务逻辑调用能力。
//
// 设计说明：该结构体独立于现有 handler 包中的 handler 基类，
// 因为 Go 子包无法访问父包的未导出类型。两套 Handler 体系
// 通过不同的路由组和中间件完全隔离。
type YggdrasilBase struct {
	Name    string               // 处理器名称标识
	Log     *xLog.LogNamedLogger // 日志实例
	Service *yggdrasilService    // 服务实例
}

// NewYggdrasilBase 创建 Yggdrasil Handler 基类实例。
//
// 通过传入的上下文初始化 YggdrasilLogic 及其所有关联的 Repository 和缓存实例。
// 该函数是所有 Yggdrasil Handler 子包的统一入口点。
//
// 参数:
//   - ctx: 应用上下文，需包含数据库、Redis 和 RSA 密钥对等依赖
//   - name: 处理器名称标识，用于日志记录
//
// 返回值:
//   - *YggdrasilBase: 初始化完成的基类实例，可传递给各子包的构造函数
func NewYggdrasilBase(ctx context.Context, name string) *YggdrasilBase {
	return &YggdrasilBase{
		Name: name,
		Log:  xLog.WithName(xLog.NamedCONT, name),
		Service: &yggdrasilService{
			yggdrasilLogic: yggdrasil.NewYggdrasilLogic(ctx),
		},
	}
}

// Logic 返回 YggdrasilLogic 实例，供子包 Handler 调用业务逻辑。
//
// 由于 yggdrasilLogic 是未导出字段，子包无法直接访问，
// 通过此导出方法提供对 Logic 实例的安全访问。
func (s *yggdrasilService) Logic() *yggdrasil.YggdrasilLogic {
	return s.yggdrasilLogic
}
