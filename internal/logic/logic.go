// Package logic 提供业务逻辑编排层。
//
// 该包负责承载应用程序的核心业务规则和用例编排，充当 Handler（接口适配层）
// 与 Repository（数据访问层）之间的桥梁。本层不直接操作数据库或缓存，
// 所有持久化操作均通过 Repository 层完成，所有事务管理均由 Repository 层的
// 事务协调仓储（TxnRepo）负责。
//
// 分层定位:
//   - 接收来自 Handler 层的结构化输入
//   - 执行业务校验、参数归一化、领域规则判断
//   - 组合调用多个 Repository 完成业务流程
//   - 返回标准化的领域实体或业务错误
//
// 设计原则:
//   - 不直接操作 GORM Session 或数据库事务
//   - 不直接执行 Redis 命令（缓存操作通过 Repository 的 Cache 层完成）
//   - 事务边界由 Repository 层管理，Logic 层仅做纯逻辑编排
package logic

import (
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// logic 业务逻辑基结构体。
//
// 作为各具体 Logic（GameProfileLogic、LibraryLogic、UserLogic 等）的嵌入基类，
// 提供通用的基础设施依赖：GORM 数据库实例、Redis 客户端和日志记录器。
//
// 注意：`db` 字段仅用于向下传递给 Repository 构造函数，Logic 层本身不应
// 直接使用该字段执行任何数据库操作或事务管理。
type logic struct {
	db  *gorm.DB             // GORM 数据库实例（传递给 Repository 使用）
	rdb *redis.Client        // Redis 客户端实例（传递给 Cache/Repository 使用）
	log *xLog.LogNamedLogger // 日志实例
}
