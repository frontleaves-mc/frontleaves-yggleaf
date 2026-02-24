package context

import (
	"context"

	xError "github.com/bamboo-services/bamboo-base-go/common/error"
	xCtxUtil "github.com/bamboo-services/bamboo-base-go/common/utility/context"
	bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
	bBucket "github.com/phalanx-labs/beacon-bucket-sdk"
)

// MustGetBucket 从上下文中获取存储桶客户端实例
//
// 该函数从传入的 `ctx` 中提取与 `bConst.CtxBucketKey` 关联的 `*bBucket.BucketClient` 值。
// 如果上下文中不存在该键或类型断言失败，将触发 panic。
//
// 参数说明:
//   - ctx: 包含存储桶客户端的上下文对象
//
// 返回值:
//   - *bBucket.BucketClient: 存储桶客户端实例
//
// 注意: 此函数为 Must 风格，调用前需确保上下文已正确注入存储桶客户端，否则会导致程序崩溃。
func MustGetBucket(ctx context.Context) *bBucket.BucketClient {
	return xCtxUtil.MustGet[*bBucket.BucketClient](ctx, bConst.CtxBucketKey)
}

// GetBucket 从上下文中获取 BucketClient 实例
//
// 该函数用于从给定的 `context.Context` 中提取预存的 `BucketClient` 实例。
// 它通过 `bConst.CtxBucketKey` 作为键来检索上下文中的值。
//
// 参数说明:
//   - ctx: 包含 BucketClient 实例的上下文对象。
//
// 返回值:
//   - *bBucket.BucketClient: 成功时返回存储桶客户端实例。
//   - *xError.Error: 当上下文中不存在该键或类型断言失败时返回错误。
func GetBucket(ctx context.Context) (*bBucket.BucketClient, *xError.Error) {
	return xCtxUtil.Get[*bBucket.BucketClient](ctx, bConst.CtxBucketKey)
}
