package handler

import (
	"context"
	"slices"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xSnowflake "github.com/bamboo-services/bamboo-base-go/common/snowflake"
	xGrpcMiddle "github.com/bamboo-services/bamboo-base-go/plugins/grpc/middleware"
	xGrpcResult "github.com/bamboo-services/bamboo-base-go/plugins/grpc/result"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/app/grpc/middleware"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/entity"
	"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic"
	authpb "github.com/frontleaves-mc/frontleaves-yggleaf/proto/auth"
	bSdkLogic "github.com/phalanx-labs/beacon-sso-sdk/logic"
	"google.golang.org/grpc"
)

// GRPCAuthHandler 认证服务 gRPC Handler
type GRPCAuthHandler struct {
	log     *xLog.LogNamedLogger
	service *grpcAuthService
	authpb.UnimplementedAuthServiceServer
}

// grpcAuthService gRPC 认证服务的业务依赖
type grpcAuthService struct {
	userLogic       *logic.UserLogic
	accessUserLogic *logic.AccessUserLogic
	oauthLogic      *bSdkLogic.BusinessLogic
}

// NewGRPCAuthHandler 创建认证服务 gRPC Handler
func NewGRPCAuthHandler(ctx context.Context, server grpc.ServiceRegistrar) *GRPCAuthHandler {
	h := &GRPCAuthHandler{
		log: xLog.WithName(xLog.NamedGRPC, "GRPCAuthHandler"),
		service: &grpcAuthService{
			userLogic:       logic.NewUserLogic(ctx),
			accessUserLogic: logic.NewAccessUserLogic(ctx),
			oauthLogic:      bSdkLogic.NewBusiness(ctx),
		},
	}

	authpb.RegisterAuthServiceServer(server, h)
	xGrpcMiddle.UseUnary(authpb.AuthService_ServiceDesc, middleware.UnaryAppVerify(ctx))

	return h
}

// ValidateToken 验证 AccessToken 并返回用户信息
func (h *GRPCAuthHandler) ValidateToken(
	ctx context.Context, req *authpb.ValidateTokenRequest,
) (*authpb.ValidateTokenResponse, error) {
	h.log.Info(ctx, "ValidateToken - 验证 AccessToken")

	accessToken := req.GetAccessToken()
	if accessToken == "" {
		return nil, xError.NewError(ctx, xError.ParameterEmpty, "access_token 不能为空", true)
	}

	// 缓存优先
	cachedUser, xErr := h.service.accessUserLogic.GetUserByAT(ctx, accessToken)
	if xErr != nil {
		return nil, xErr
	}

	var takeUser *entity.User
	if cachedUser != nil {
		takeUser = cachedUser
	} else {
		// SSO 远端获取
		getUser, xErr := h.service.oauthLogic.Userinfo(ctx, accessToken)
		if xErr != nil {
			return nil, xErr
		}

		// 获取/创建本地用户
		takeUser, xErr = h.service.userLogic.TakeUser(ctx, getUser)
		if xErr != nil {
			return nil, xErr
		}

		// 回写缓存
		_ = h.service.accessUserLogic.SetUserByAT(ctx, accessToken, takeUser)
	}

	// 检查封禁
	if takeUser.HasBan {
		return nil, xError.NewError(ctx, xError.Forbidden, "用户已被封禁", true)
	}

	// 构建响应
	resp := xGrpcResult.SuccessWith[*authpb.ValidateTokenResponse](ctx, "验证成功")
	resp.UserId = takeUser.ID.String()
	resp.Username = takeUser.Username
	if takeUser.RoleName != nil {
		resp.RoleName = *takeUser.RoleName
	}
	resp.HasBan = takeUser.HasBan

	return resp, nil
}

// GetUserRole 根据 user_id 获取角色
func (h *GRPCAuthHandler) GetUserRole(
	ctx context.Context, req *authpb.GetUserRoleRequest,
) (*authpb.GetUserRoleResponse, error) {
	h.log.Info(ctx, "GetUserRole - 获取用户角色")

	userIDStr := req.GetUserId()
	if userIDStr == "" {
		return nil, xError.NewError(ctx, xError.ParameterEmpty, "user_id 不能为空", true)
	}

	userID, err := xSnowflake.ParseSnowflakeID(userIDStr)
	if err != nil {
		return nil, xError.NewError(ctx, xError.ParameterError, "user_id 格式无效", true, err)
	}

	// 通过 UserRepo 查询用户（复用 UserLogic 内部的 repo）
	// 此处直接用 userLogic.TakeUser 的方式太重，直接用 DB 查询
	userLogic := h.service.userLogic
	userEntity, found, xErr := userLogic.GetByID(ctx, userID.String())
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "用户不存在", true)
	}

	resp := xGrpcResult.SuccessWith[*authpb.GetUserRoleResponse](ctx, "查询成功")
	if userEntity.RoleName != nil {
		resp.RoleName = *userEntity.RoleName
	}
	resp.HasBan = userEntity.HasBan

	return resp, nil
}

// CheckPermission 检查用户权限
func (h *GRPCAuthHandler) CheckPermission(
	ctx context.Context, req *authpb.CheckPermissionRequest,
) (*authpb.CheckPermissionResponse, error) {
	h.log.Info(ctx, "CheckPermission - 检查用户权限")

	userIDStr := req.GetUserId()
	if userIDStr == "" {
		return nil, xError.NewError(ctx, xError.ParameterEmpty, "user_id 不能为空", true)
	}

	allowedRoles := req.GetAllowedRoles()
	if len(allowedRoles) == 0 {
		return nil, xError.NewError(ctx, xError.ParameterEmpty, "allowed_roles 不能为空", true)
	}

	userLogic := h.service.userLogic
	userEntity, found, xErr := userLogic.GetByID(ctx, userIDStr)
	if xErr != nil {
		return nil, xErr
	}
	if !found {
		return nil, xError.NewError(ctx, xError.ResourceNotFound, "用户不存在", true)
	}

	resp := xGrpcResult.SuccessWith[*authpb.CheckPermissionResponse](ctx, "权限检查完成")
	if userEntity.RoleName != nil {
		resp.RoleName = *userEntity.RoleName
		resp.Allowed = slices.Contains(allowedRoles, *userEntity.RoleName)
	} else {
		resp.Allowed = false
	}

	return resp, nil
}
