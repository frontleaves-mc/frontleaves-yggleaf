package repository

import (
	"errors"
	"time"

	xError "github.com/bamboo-services/bamboo-base-go/error"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository/cache"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

// UserRepo 用户领域仓储，负责用户数据的持久化、缓存管理与日志记录。
//
// 该类型封装了底层数据库交互和缓存策略，为上层业务逻辑提供统一的数据访问接口。
//
// 字段说明:
//   - db: 指向 GORM 数据库实例的指针，用于执行 SQL 操作。
//   - cache: OAuth 2.0 相关的用户缓存管理器，处理临时状态存储。
//   - log: 带命名空间的结构化日志记录器。
type UserRepo struct {
	db    *gorm.DB
	cache *cache.UserCache
	log   *xLog.LogNamedLogger
}

// NewUserRepo 初始化并返回一个 UserRepo 领域仓储实例
//
// 该工厂函数通过组装 GORM 数据库实例、Redis 客户端和日志记录器，构建一个具备持久化、
// 缓存管理和日志追踪能力的 UserRepo 仓储对象。缓存组件默认配置为 15 分钟过期时间。
//
// 参数说明:
//   - db: 已初始化的 GORM 数据库连接实例，用于执行底层数据库操作。
//   - rdb: 已初始化的 Redis 客户端实例，用于构建缓存策略。
//
// 返回值:
//   - *UserRepo: 配置完成的 UserRepo 仓储实例指针，可直接用于业务逻辑层。
func NewUserRepo(db *gorm.DB, rdb *redis.Client) *UserRepo {
	return &UserRepo{
		db: db,
		cache: &cache.UserCache{
			RDB: rdb,
			TTL: time.Minute * 15,
		},
		log: xLog.WithName(xLog.NamedREPO, "UserRepo"),
	}
}

// Get 根据 ID 获取用户实体
//
// 该方法采用缓存优先策略：首先尝试从缓存读取用户实体，未命中时回退数据库查询。
// 若数据库命中则更新缓存并返回。缓存更新失败仅记录日志，不影响业务返回。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - id: 用户雪花 ID。
//
// 返回值:
//   - *entity.User: 命中的用户实体，未命中时返回 nil。
//   - bool: 是否命中（true 表示命中，false 表示未命中）。
//   - *xError.Error: 缓存读取或数据库查询过程中的错误。
func (r *UserRepo) Get(ctx *gin.Context, id string) (*entity.User, bool, *xError.Error) {
	r.log.Info(ctx, "Get - 获取用户信息")

	// 检查缓存是否存在
	getUser, err := r.cache.GetAllStruct(ctx, id)
	if err != nil {
		return nil, false, xError.NewError(ctx, xError.CacheError, "获取用户信息失败", true, err)
	}
	if getUser != nil {
		return getUser, true, nil
	}

	// 缓存不存在，数据库获取
	var user entity.User
	dbErr := r.db.WithContext(ctx).Model(&entity.User{}).Where("id = ?", id).First(&user).Error
	if dbErr == nil {
		// 存在则更新缓存
		if err := r.cache.SetAllStruct(ctx, id, &user); err != nil {
			r.log.Warn(ctx, err.Error())
		}
		return &user, true, nil
	}
	if errors.Is(dbErr, gorm.ErrRecordNotFound) {
		return nil, false, nil
	}
	return nil, false, xError.NewError(ctx, xError.DatabaseError, "查询用户失败", true, dbErr)
}

// Set 创建或更新用户实体
//
// 该方法将用户实体持久化到数据库，并在成功后更新缓存。
// 缓存更新失败仅记录日志，不影响业务返回。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - user: 要创建或更新的用户实体。
//
// 返回值:
//   - *entity.User: 持久化后的用户实体。
//   - *xError.Error: 数据库操作过程中的错误。
func (r *UserRepo) Set(ctx *gin.Context, user *entity.User) (*entity.User, *xError.Error) {
	r.log.Info(ctx, "Set - 创建或更新用户信息")

	if err := r.db.WithContext(ctx).Save(user).Error; err != nil {
		return nil, xError.NewError(ctx, xError.DatabaseError, "保存用户失败", true, err)
	}

	// 保存成功后更新缓存
	if err := r.cache.SetAllStruct(ctx, user.ID.String(), user); err != nil {
		r.log.Warn(ctx, err.Error())
	}
	return user, nil
}
