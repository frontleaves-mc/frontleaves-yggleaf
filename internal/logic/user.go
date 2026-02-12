package logic

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/error"
	xLog "github.com/bamboo-services/bamboo-base-go/log"
	xModels "github.com/bamboo-services/bamboo-base-go/models"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/utility"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/utility/ctxutil"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	"github.com/gin-gonic/gin"
	bSdkModels "github.com/phalanx/beacon-sso-sdk/models"
)

// userRepo 用户数据访问适配器
type userRepo struct {
	user *repository.UserRepo
}

// UserLogic 用户业务逻辑处理者
//
// 封装了用户相关的核心业务逻辑。它通过嵌入匿名的 `logic` 结构体，
// 继承了 GORM 数据库实例 (`db`)、Redis 客户端 (`rdb`) 和日志记录器 (`log`)，
// 用于处理用户数据的持久化、缓存管理和日志记录。
type UserLogic struct {
	logic
	repo userRepo
}

// NewUserLogic 创建用户业务逻辑实例
//
// 该函数用于初始化并返回一个 `UserLogic` 结构体指针。它会尝试从传入的上下文 (context.Context)
// 中获取必需的依赖项（数据库连接、Redis 连接和日志组件）。
//
// 参数说明:
//   - ctx: 上下文对象，用于传递请求范围的数据、取消信号和截止时间，同时用于提取基础资源。
//
// 返回值:
//   - *UserLogic: 初始化完成的用户业务逻辑实例指针。
//
// 注意: 该函数依赖于 `xCtxUtil.MustGetDB` 和 `xCtxUtil.MustGetRDB`。如果上下文中缺少
// 必要的数据库或 Redis 连接，这些辅助函数会触发 panic。请确保上下文已通过中间件正确注入了这些资源。
func NewUserLogic(ctx context.Context) *UserLogic {
	db := xCtxUtil.MustGetDB(ctx)
	rdb := xCtxUtil.MustGetRDB(ctx)
	return &UserLogic{
		logic: logic{
			db:  db,
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "UserLogic"),
		},
		repo: userRepo{
			user: repository.NewUserRepo(db, rdb),
		},
	}
}

// TakeUser 根据提供的第三方用户信息检索或创建本地用户账号
//
// 该方法充当身份同步的入口点，通常在用户通过 OAuth 等方式登录后调用。
// 它首先尝试通过用户 ID 查找本地用户，若不存在则根据 OAuth 信息创建新用户。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据、控制流和超时取消。
//   - userinfo: 第三方平台返回的原始用户信息对象，用于提取用户标识和基本资料。
//
// 返回值:
//   - *entity.User: 找到的或新建的用户实体对象。
//   - *xError.Error: 用户实体拼装或仓储层操作过程中发生的错误。
func (l *UserLogic) TakeUser(ctx *gin.Context, userinfo *bSdkModels.OAuthUserinfo) (*entity.User, *xError.Error) {
	l.log.Info(ctx, "TakeUser - 获取用户信息或创建用户")

	// 尝试获取已存在用户
	user, found, err := l.repo.user.Get(ctx, userinfo.Sub)
	if err != nil {
		return nil, err
	}
	if found {
		return user, nil
	}

	// 用户不存在，解析 ID 并构建新用户实体
	snowflakeID, parseErr := xSnowflake.ParseSnowflakeID(userinfo.Sub)
	if parseErr != nil {
		return nil, xError.NewError(ctx, xError.ParameterError, "解析用户 ID 失败", true, parseErr)
	}

	newUser := &entity.User{
		BaseEntity: xModels.BaseEntity{
			ID: snowflakeID,
		},
		Username: userinfo.Nickname,
		Email:    xUtil.Ptr(userinfo.Email),
		Phone:    xUtil.Ptr(userinfo.Phone),
		RoleName: xUtil.Ptr(entity.RolePlayer.String()),
	}

	return l.repo.user.Set(ctx, newUser)
}
