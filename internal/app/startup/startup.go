package startup

import (
	"context"

	xCtx "github.com/bamboo-services/bamboo-base-go/context"
	xRegNode "github.com/bamboo-services/bamboo-base-go/register/node"
	bSdkStartup "github.com/phalanx/beacon-sso-sdk/startup"
)

// reg 表示应用程序的核心注册结构，包含所有初始化后的组件实例。
type reg struct {
	ctx context.Context
}

// newInit 创建一个新的注册结构实例
func newInit() *reg {
	return &reg{
		ctx: context.Background(),
	}
}

func Init() (context.Context, []xRegNode.RegNodeList) {
	businessReg := newInit()
	var regNode []xRegNode.RegNodeList

	// 初始化注册
	regNode = append(regNode, xRegNode.RegNodeList{Key: xCtx.DatabaseKey, Node: businessReg.databaseInit})
	regNode = append(regNode, xRegNode.RegNodeList{Key: xCtx.RedisClientKey, Node: businessReg.nosqlInit})
	regNode = append(regNode, xRegNode.RegNodeList{Key: xCtx.Nil, Node: businessReg.nosqlInit})
	regNode = append(regNode, xRegNode.RegNodeList{Key: xCtx.Exec, Node: businessReg.businessDataPrepare})

	// 初始化 OAuth2
	regNode = append(regNode, bSdkStartup.NewOAuthConfig()...)

	return businessReg.ctx, regNode
}
