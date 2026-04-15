package bConst

import xCtx "github.com/bamboo-services/bamboo-base-go/defined/context"

const (
	CtxBucketKey            xCtx.ContextKey = "business_bucket"            // 是用于在上下文中存储业务桶键的上下文键
	CtxUserinfoKey          xCtx.ContextKey = "business_userinfo"          // 是用于在上下文中存储用户信息的上下文键
	CtxYggdrasilGameToken   xCtx.ContextKey = "yggdrasil_game_token"      // 是用于在上下文中存储 Yggdrasil 游戏令牌实体的上下文键
	CtxYggdrasilRSAKeyPair  xCtx.ContextKey = "yggdrasil_rsa_key_pair"    // 是用于在上下文中存储 Yggdrasil RSA 密钥对的上下文键
)
