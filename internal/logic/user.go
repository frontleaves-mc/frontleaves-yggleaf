package logic

import (
	"context"

	"golang.org/x/crypto/bcrypt"
	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xUtil "github.com/bamboo-services/bamboo-base-go/common/utility"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	xModels "github.com/bamboo-services/bamboo-base-go/major/models"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
	"github.com/frontleaves-mc/frontleaves-yggleaf/api/user"
	bSdkModels "github.com/phalanx-labs/beacon-sso-sdk/models"
)

// userRepo 用户数据访问适配器
type userRepo struct {
	user             *repository.UserRepo
	libraryQuotaRepo *repository.LibraryQuotaRepo
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
//   - context: 上下文对象，用于传递请求范围的数据、取消信号和截止时间，同时用于提取基础资源。
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
			user:             repository.NewUserRepo(db, rdb),
			libraryQuotaRepo: repository.NewLibraryQuotaRepo(db),
		},
	}
}

// TakeUser 根据提供的第三方用户信息检索或创建本地用户账号
//
// 该方法充当身份同步的入口点，通常在用户通过 OAuth 等方式登录后调用。
// 它首先尝试通过用户 ID 查找本地用户，若不存在则根据 OAuth 信息创建新用户。
//
// 参数:
//   - context: Gin 上下文对象，用于传递请求范围的数据、控制流和超时取消。
//   - userinfo: 第三方平台返回的原始用户信息对象，用于提取用户标识和基本资料。
//
// 返回值:
//   - *entity.User: 找到的或新建的用户实体对象。
//   - *xError.Error: 用户实体拼装或仓储层操作过程中发生的错误。
func (l *UserLogic) TakeUser(ctx context.Context, userinfo *bSdkModels.OAuthUserinfo) (*entity.User, *xError.Error) {
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

	user, xErr := l.repo.user.Set(ctx, newUser)
	if xErr != nil {
		return nil, xErr
	}

	// 主动为用户初始化资源库配额（不阻断主流程，仅记录警告日志）
	if _, _, quotaErr := l.repo.libraryQuotaRepo.GetByUserID(ctx, nil, snowflakeID, false); quotaErr != nil {
		l.log.Warn(ctx, string("创建用户资源库配额失败: "+quotaErr.ErrorMessage))
	}

	return user, nil
}

// GetUserCurrent 获取当前用户的完整信息（含扩展状态）。
//
// 在 TakeUser 的基础上，额外计算账户完善度信息，
// 构建包含 extend 字段的 UserCurrentResponse DTO。
func (l *UserLogic) GetUserCurrent(ctx context.Context, userinfo *bSdkModels.OAuthUserinfo) (*user.UserCurrentResponse, *xError.Error) {
	l.log.Info(ctx, "GetUserCurrent - 获取用户完整信息")

	// 复用现有 TakeUser 逻辑获取/创建用户
	userEntity, xErr := l.TakeUser(ctx, userinfo)
	if xErr != nil {
		return nil, xErr
	}

	// 构建响应 DTO（含账户完善状态）
	return &user.UserCurrentResponse{
		User:   *userEntity,
		Extend: user.UserExtend{
			AccountReady: l.determineAccountReady(userEntity),
		},
	}, nil
}

// determineAccountReady 根据用户实体判断账户完善状态。
//
// 当前检查项：game_password 是否已填写。
// 未来可在该方法中追加更多检查项。
func (_ *UserLogic) determineAccountReady(userEntity *entity.User) string {
	if userEntity.GamePassword == "" {
		return "game_password"
	}
	return "ready"
}

// UpdateGamePassword 更新当前用户的游戏密码。
//
// 已通过 OAuth2 AT 认证的用户可直接设置/重置 game_password，
// 无需验证旧密码。更新后返回包含最新 account_ready 状态的 UserCurrentResponse。
func (l *UserLogic) UpdateGamePassword(ctx context.Context, userID xSnowflake.SnowflakeID, req *user.UpdateGamePasswordRequest) (*user.UserCurrentResponse, *xError.Error) {
	l.log.Info(ctx, "UpdateGamePassword - 更新游戏密码")

	// 两次密码一致性校验
	if req.NewPassword != req.ConfirmPassword {
		return nil, xError.NewError(ctx, xError.ParameterError, "两次输入的密码不一致", true)
	}

	// bcrypt 加密新密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ServerInternalError, "密码加密失败", true, err)
	}

	// 获取当前用户实体
	userEntity, found, xErr := l.repo.user.Get(ctx, userID.String())
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "用户不存在", true)
	}

	// 更新游戏密码并持久化
	userEntity.GamePassword = string(hashedPassword)
	if _, xErr = l.repo.user.Set(ctx, userEntity); xErr != nil {
		return nil, xErr
	}

	// 构建含最新账户完善状态的响应
	return &user.UserCurrentResponse{
		User:   *userEntity,
		Extend: user.UserExtend{
			AccountReady: l.determineAccountReady(userEntity),
		},
	}, nil
}
