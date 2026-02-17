package bConst

import xSnowflake "github.com/bamboo-services/bamboo-base-go/snowflake"

const (
	GeneForGameProfile         xSnowflake.Gene = 32 // 应用
	GeneForGameProfileQuota    xSnowflake.Gene = 33 // 游戏档案配额
	GeneForGameProfileQuotaLog xSnowflake.Gene = 34 // 游戏档案配额日志
	GeneForSkinLibrary         xSnowflake.Gene = 35 // 皮肤库
	GeneForCapeLibrary         xSnowflake.Gene = 36 // 披风库
	GeneForLibraryQuota        xSnowflake.Gene = 37 // 资源库配额
)
