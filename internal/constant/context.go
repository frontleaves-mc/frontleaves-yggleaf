package bConst

import xCtx "github.com/bamboo-services/bamboo-base-go/context"

const (
	CtxBucketKey   xCtx.ContextKey = "business_bucket"   // 是用于在上下文中存储业务桶键的上下文键
	CtxUserinfoKey xCtx.ContextKey = "business_userinfo" // 是用于在上下文中存储用户信息的上下文键
)
