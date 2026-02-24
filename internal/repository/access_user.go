package repository

import (
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/cache"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// AccessUserRepo 访问令牌用户缓存仓储，负责通过 AccessToken 摘要管理用户实体的缓存读写。
//
// 该类型为纯缓存仓储，无数据库依赖。通过封装 AccessUserCache 提供统一的数据访问接口，
// 供上层业务逻辑层使用。
//
// 字段说明:
//   - cache: 访问令牌用户缓存管理器，处理 Redis Hash 结构的读写。
//   - log: 带命名空间的结构化日志记录器。
type AccessUserRepo struct {
	cache *cache.AccessUserCache
	log   *xLog.LogNamedLogger
}

// NewAccessUserRepo 初始化并返回一个 AccessUserRepo 仓储实例
//
// 该工厂函数通过组装 Redis 客户端和日志记录器，构建一个具备缓存管理和日志追踪能力的
// AccessUserRepo 仓储对象。缓存组件默认配置为 15 分钟过期时间。
//
// 参数说明:
//   - rdb: 已初始化的 Redis 客户端实例，用于构建缓存策略。
//
// 返回值:
//   - *AccessUserRepo: 配置完成的 AccessUserRepo 仓储实例指针，可直接用于业务逻辑层。
func NewAccessUserRepo(rdb *redis.Client) *AccessUserRepo {
	return &AccessUserRepo{
		cache: &cache.AccessUserCache{
			RDB: rdb,
			TTL: time.Minute * 15,
		},
		log: xLog.WithName(xLog.NamedREPO, "AccessUserRepo"),
	}
}

// Get 从缓存中获取指定 AccessToken 摘要对应的用户实体
//
// 该方法通过 tokenMD5 作为键，从 Redis Hash 缓存中读取完整的用户实体。
// 缓存未命中时返回 (nil, false, nil)，调用方可据此判断是否需要回退到远端获取。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - tokenMD5: AccessToken 的 MD5 摘要，作为缓存键。
//
// 返回值:
//   - *entity.User: 命中的用户实体，未命中时返回 nil。
//   - bool: 是否命中缓存（true 表示命中，false 表示未命中）。
//   - *xError.Error: 缓存读取过程中的错误。
func (r *AccessUserRepo) Get(ctx *gin.Context, tokenMD5 string) (*entity.User, bool, *xError.Error) {
	r.log.Info(ctx, "Get - 从缓存获取 AccessToken 用户信息")

	user, err := r.cache.GetAllStruct(ctx, tokenMD5)
	if err != nil {
		return nil, false, xError.NewError(ctx, xError.CacheError, "获取 AccessToken 用户缓存失败", true, err)
	}
	if user != nil {
		return user, true, nil
	}
	return nil, false, nil
}

// Set 将用户实体写入 AccessToken 摘要对应的缓存
//
// 该方法通过 tokenMD5 作为键，将完整的用户实体写入 Redis Hash 缓存。
// 缓存写入失败仅记录警告日志，不影响上层业务返回。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - tokenMD5: AccessToken 的 MD5 摘要，作为缓存键。
//   - user: 要缓存的用户实体指针。
//
// 返回值:
//   - *xError.Error: 缓存写入过程中的错误。
func (r *AccessUserRepo) Set(ctx *gin.Context, tokenMD5 string, user *entity.User) *xError.Error {
	r.log.Info(ctx, "Set - 写入 AccessToken 用户缓存")

	if err := r.cache.SetAllStruct(ctx, tokenMD5, user); err != nil {
		r.log.Warn(ctx, err.Error())
		return xError.NewError(ctx, xError.CacheError, "写入 AccessToken 用户缓存失败", true, err)
	}
	return nil
}
