package logic

import (
	"context"
	"crypto/md5"
	"encoding/hex"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/repository"
)

// accessUserRepo 访问令牌用户数据访问适配器
type accessUserRepo struct {
	accessUser *repository.AccessUserRepo
}

// AccessUserLogic 访问令牌用户缓存业务逻辑处理者
//
// 封装了通过 AccessToken 缓存用户实体的核心业务逻辑。它通过嵌入匿名的 `logic` 结构体，
// 继承了 Redis 客户端 (`rdb`) 和日志记录器 (`log`)，用于处理 AccessToken → User 的
// 缓存读写操作，减少对 SSO 远端接口的重复调用。
type AccessUserLogic struct {
	logic
	repo accessUserRepo
}

// NewAccessUserLogic 创建访问令牌用户缓存业务逻辑实例
//
// 该函数用于初始化并返回一个 `AccessUserLogic` 结构体指针。它会尝试从传入的上下文
// (context.Context) 中获取必需的 Redis 连接依赖项。
//
// 参数说明:
//   - ctx: 上下文对象，用于传递请求范围的数据、取消信号和截止时间，同时用于提取基础资源。
//
// 返回值:
//   - *AccessUserLogic: 初始化完成的访问令牌用户缓存业务逻辑实例指针。
//
// 注意: 该函数依赖于 `xCtxUtil.MustGetRDB`。如果上下文中缺少必要的 Redis 连接，
// 该辅助函数会触发 panic。请确保上下文已通过中间件正确注入了该资源。
func NewAccessUserLogic(ctx context.Context) *AccessUserLogic {
	rdb := xCtxUtil.MustGetRDB(ctx)
	return &AccessUserLogic{
		logic: logic{
			rdb: rdb,
			log: xLog.WithName(xLog.NamedLOGC, "AccessUserLogic"),
		},
		repo: accessUserRepo{
			accessUser: repository.NewAccessUserRepo(rdb),
		},
	}
}

// GetUserByAT 根据 AccessToken 从缓存获取用户实体
//
// 该方法将 AccessToken 转为 MD5 摘要后，从 Redis 缓存中检索对应的用户实体。
// 缓存命中时直接返回用户，未命中时返回 (nil, nil)，调用方可据此决定是否走远端获取流程。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - accessToken: 原始访问令牌字符串。
//
// 返回值:
//   - *entity.User: 缓存命中的用户实体，未命中时返回 nil。
//   - *xError.Error: 缓存读取过程中发生的错误。
func (l *AccessUserLogic) GetUserByAT(ctx context.Context, accessToken string) (*entity.User, *xError.Error) {
	l.log.Info(ctx, "GetUserByAT - 根据 AccessToken 从缓存获取用户信息")

	tokenMD5 := md5Hex(accessToken)
	user, found, err := l.repo.accessUser.Get(ctx, tokenMD5)
	if err != nil {
		return nil, err
	}
	if found {
		return user, nil
	}
	return nil, nil
}

// SetUserByAT 将用户实体写入 AccessToken 缓存
//
// 该方法将 AccessToken 转为 MD5 摘要后，将用户实体写入 Redis 缓存。
// 用于在首次远端获取用户信息后回写缓存，加速后续请求。
//
// 参数:
//   - ctx: Gin 上下文对象，用于传递请求范围的数据与控制流程。
//   - accessToken: 原始访问令牌字符串。
//   - user: 要缓存的用户实体指针。
//
// 返回值:
//   - *xError.Error: 缓存写入过程中发生的错误。
func (l *AccessUserLogic) SetUserByAT(ctx context.Context, accessToken string, user *entity.User) *xError.Error {
	l.log.Info(ctx, "SetUserByAT - 写入 AccessToken 用户缓存")

	tokenMD5 := md5Hex(accessToken)
	return l.repo.accessUser.Set(ctx, tokenMD5, user)
}

// md5Hex 计算字符串的 MD5 摘要并返回十六进制编码
func md5Hex(s string) string {
	sum := md5.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}
