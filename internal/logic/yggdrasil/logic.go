// Package yggdrasil 提供 Yggdrasil 外置登录协议的业务逻辑编排。
//
// 该包负责承载 Yggdrasil 协议的所有核心业务规则和用例编排，充当 Handler 层
// 与 Repository 层之间的桥梁。本层不直接操作数据库或缓存，所有持久化操作
// 均通过 Repository 层完成。
//
// 分层定位:
//   - 接收来自 Handler 层的结构化输入
//   - 执行业务校验、参数归一化、领域规则判断
//   - 组合调用多个 Repository 完成业务流程
//   - 返回标准化的领域实体或业务错误
//
// 功能模块:
//   - auth.go: 认证服务（登录、刷新、验证、吊销、登出）
//   - session.go: 会话管理（加入服务器、验证加入）
//   - profile.go: 角色查询（单查、批查）
//   - texture.go: 材质管理（上传、删除）
//   - signing.go: 数字签名、UUID 转换、材质 JSON 组装
package yggdrasil

import (
	"context"
	"crypto/rsa"

	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/cache"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/txn"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// logic Yggdrasil 业务逻辑基结构体。
//
// 作为 YggdrasilLogic 的嵌入基类，提供通用的基础设施依赖：
// GORM 数据库实例、Redis 客户端和日志记录器。
//
// 注意：`db` 字段仅用于向下传递给 Repository 构造函数，Logic 层本身不应
// 直接使用该字段执行任何数据库操作或事务管理。
type logic struct {
	db  *gorm.DB             // GORM 数据库实例（传递给 Repository 使用）
	rdb *redis.Client        // Redis 客户端实例（传递给 Cache/Repository 使用）
	log *xLog.LogNamedLogger // 日志实例
}

// yggdrasilRepo Yggdrasil 数据访问适配器。
//
// 聚合 Yggdrasil 协议相关的各仓储实例，包括游戏令牌管理、用户查询、角色查询和会话缓存，
// 供 YggdrasilLogic 统一调用。
type yggdrasilRepo struct {
	gameTokenRepo    *repository.GameTokenRepo       // 游戏令牌仓储
	gameTokenTxnRepo *txn.GameTokenTxnRepo         // 游戏令牌事务协调仓储
	userRepo         *repository.UserRepo            // 用户仓储
	profileRepo      *repository.GameProfileYggRepo  // Yggdrasil 角色查询仓储
	sessionCache     *cache.SessionCache             // 会话缓存
}

// YggdrasilLogic Yggdrasil 协议业务逻辑处理者。
//
// 封装了 Yggdrasil 外置登录协议相关的核心业务逻辑，包括认证服务、会话管理、
// 角色查询、材质管理和数字签名。所有 12 个协议端点共享此逻辑实例，
// 以便统一管理 RSA 密钥对、令牌 CRUD 和会话缓存等基础设施。
//
// 设计约束：本层不直接操作数据库事务，所有涉及多表写入的事务性操作
// 均委托给 Repository 层完成。
type YggdrasilLogic struct {
	logic                                // 嵌入基类 (db, rdb, log)
	repo      yggdrasilRepo
	privKey   *rsa.PrivateKey            // RSA 私钥（用于 textures 属性签名）
	pubKeyPEM string                     // RSA 公钥 PEM 字符串（用于 API 元数据响应）
}

// NewYggdrasilLogic 创建 Yggdrasil 业务逻辑实例。
//
// 从上下文中提取数据库连接、Redis 连接和 RSA 密钥对，初始化所有关联的 Repository 实例。
//
// 参数:
//   - ctx: 上下文对象，需包含数据库、Redis 和 RSA 密钥对等依赖
//
// 返回值:
//   - *YggdrasilLogic: 初始化完成的 Yggdrasil 业务逻辑实例
func NewYggdrasilLogic(ctx context.Context) *YggdrasilLogic {
	db := xCtxUtil.MustGetDB(ctx)
	rdb := xCtxUtil.MustGetRDB(ctx)

	// 从上下文中提取 RSA 密钥对（由框架节点注册机制注入）
	var privKey *rsa.PrivateKey
	var pubKeyPEM string
	if pair, ok := ctx.Value(bConst.CtxYggdrasilRSAKeyPair).(*bConst.RSAKeyPair); ok && pair != nil {
		// 防止部分初始化：RSAKeyPair 结构体非 nil 但内部字段为零值
		if pair.PrivKey != nil && pair.PubKeyPEM != "" {
			privKey = pair.PrivKey
			pubKeyPEM = pair.PubKeyPEM
		}
	}
	if privKey == nil || pubKeyPEM == "" {
		xLog.Panic(ctx, "Yggdrasil RSA 密钥对未正确注入到上下文中（privKey=nil 或 pubKeyPEM=空），请检查 yggdrasilRSAKeyInit 节点是否正常注册")
	}

	return &YggdrasilLogic{
		logic: logic{
			db:  db,
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "YggdrasilLogic"),
		},
		repo: yggdrasilRepo{
			gameTokenRepo:    repository.NewGameTokenRepo(db),
			gameTokenTxnRepo: txn.NewGameTokenTxnRepo(db, repository.NewGameTokenRepo(db)),
			userRepo:         repository.NewUserRepo(db, rdb),
			profileRepo:      repository.NewGameProfileYggRepo(db),
			sessionCache:     &cache.SessionCache{RDB: rdb},
		},
		privKey:   privKey,
		pubKeyPEM: pubKeyPEM,
	}
}

// GetPubKeyPEM 返回 RSA 公钥的 PEM 格式字符串。
//
// 用于 API 元数据响应中的 signaturePublickey 字段。
//
// 返回值:
//   - string: RSA 公钥的 PEM 格式字符串
func (l *YggdrasilLogic) GetPubKeyPEM() string {
	return l.pubKeyPEM
}
