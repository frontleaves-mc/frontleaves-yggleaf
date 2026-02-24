package startup

import (
	"context"
	"fmt"

	xLog "github.com/bamboo-services/bamboo-base-go/common/log"
	xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	bBucket "github.com/phalanx-labs/beacon-bucket-sdk"
)

// bucketInit 初始化并配置对象存储 (Bucket) 客户端。
//
// 该方法通过读取环境变量 (`BUCKET_HOST`, `BUCKET_PORT` 等) 来配置连接参数
// 和应用凭证，创建一个可用的 Bucket 客户端实例。
//
// 参数说明:
//   - ctx: 用于日志追踪的上下文。
//
// 返回值:
//   - any: 初始化成功的 `bBucket.Client` 实例。
//   - error: 当前实现总是返回 nil。
//
// 注意: 请确保所有相关环境变量已正确设置。
func (r *reg) bucketInit(ctx context.Context) (any, error) {
	log := xLog.WithName(xLog.NamedINIT)
	log.Debug(ctx, "初始化 BucketClient...")

	var (
		bucketHost   = xEnv.GetEnvString(bConst.EnvBucketHost, "")
		bucketPort   = xEnv.GetEnvString(bConst.EnvBucketPort, "")
		appAccessID  = xEnv.GetEnvString(bConst.EnvBucketAccessID, "")
		appAccessKey = xEnv.GetEnvString(bConst.EnvBucketSecretKey, "")
	)

	if bucketHost == "" || bucketPort == "" || appAccessID == "" || appAccessKey == "" {
		return nil, fmt.Errorf(
			"缺少必要的 Bucket 配置环境变量: BUCKET_HOST=%s, BUCKET_PORT=%s, BUCKET_ACCESS_ID=%s, BUCKET_SECRET_KEY=%s",
			bucketHost, bucketPort, appAccessID, appAccessKey,
		)
	}

	bucketClient := bBucket.NewClient(
		bBucket.WithConnect(bucketHost, bucketPort),
		bBucket.WithAppAccess(appAccessID, appAccessKey),
	)

	return bucketClient, nil
}
