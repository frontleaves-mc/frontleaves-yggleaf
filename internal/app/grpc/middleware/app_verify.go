package middleware

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xGrpcConst "github.com/bamboo-services/bamboo-base-go/plugins/grpc/constant"
	xGrpcUtil "github.com/bamboo-services/bamboo-base-go/plugins/grpc/utility"
	xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"
	"google.golang.org/grpc"
)

// UnaryAppVerify 创建 App 级认证中间件
//
// 验证调用方的 app-access-id 和 app-secret-key。
// plugin 服务调用 yggleaf gRPC 接口时，必须在 metadata 中携带这两个字段。
func UnaryAppVerify(ctx context.Context) grpc.UnaryServerInterceptor {
	log := xLog.WithName(xLog.NamedMIDE, "UnaryAppVerify")

	expectedAccessID := xEnv.GetEnvString("PLUGIN_APP_ACCESS_ID", "")
	expectedSecretKey := xEnv.GetEnvString("PLUGIN_APP_SECRET_KEY", "")

	return func(
		ctx context.Context, req interface{},
		info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
	) (interface{}, error) {
		log.Info(ctx, "验证 App 身份")

		accessID, xErr := xGrpcUtil.ExtractMetadata(ctx, xGrpcConst.MetadataAppAccessID)
		if xErr != nil {
			return nil, xError.NewError(ctx, xError.Unauthorized, "缺少 app-access-id", true)
		}

		secretKey, xErr := xGrpcUtil.ExtractMetadata(ctx, xGrpcConst.MetadataAppSecretKey)
		if xErr != nil {
			return nil, xError.NewError(ctx, xError.Unauthorized, "缺少 app-secret-key", true)
		}

		if accessID != expectedAccessID || secretKey != expectedSecretKey {
			log.Warn(ctx, "App 凭证无效")
			return nil, xError.NewError(ctx, xError.PermissionDenied, "App 凭证无效", true)
		}

		return handler(ctx, req)
	}
}
