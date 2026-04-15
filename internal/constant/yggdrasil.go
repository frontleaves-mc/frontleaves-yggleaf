package bConst

import "crypto/rsa"

// Yggdrasil 协议相关常量配置。

// RSAKeyPair 持有 Yggdrasil 协议的 RSA 签名密钥对。
//
// 该结构体通过框架节点注册机制注入到上下文中，供 YggdrasilLogic 在签名时使用。
type RSAKeyPair struct {
	PrivKey   *rsa.PrivateKey // RSA 私钥（用于 textures 属性签名）
	PubKeyPEM string          // RSA 公钥 PEM 字符串（用于 API 元数据响应）
}

const (
	// Yggdrasil 服务元数据
	YggdrasilServerName         = "FrontLeaves YggLeaf"     // 服务名称
	YggdrasilImplementationName = "frontleaves-yggleaf"     // 实现名称
	YggdrasilImplementationVer  = "1.0.0"                   // 实现版本
	YggdrasilHomepageURL        = "https://yggleaf.frontleaves.com/"  // 主页地址
	YggdrasilRegisterURL        = "https://sso.frontleaves.com/register" // 注册地址

	// Yggdrasil 皮肤域名（用于 skinDomains 配置）
	YggdrasilSkinDomainMain   = "yggleaf.frontleaves.com" // 主皮肤域名
	YggdrasilSkinDomainSuffix = ".frontleaves.com"        // 皮肤域名后缀（匹配所有子域名）

	// Yggdrasil 材质 URL 模板（需配合 TextureHash 使用）
	// 完整 URL 格式: https://{纹理域名}/textures/{TextureHash}
	YggdrasilTextureURLTemplate = "https://yggleaf.frontleaves.com/textures/%s"

	// Yggdrasil 令牌配置
	YggdrasilTokenMaxPerUser   = 10          // 每用户最大有效令牌数
	YggdrasilTokenExpireHours  = 168         // 令牌过期时间（小时，默认 7 天）
	YggdrasilSessionExpireSec  = 30          // 会话缓存过期时间（秒）

	// Yggdrasil 批量查询限制
	YggdrasilBatchLookupMaxNames = 10        // 批量角色查询最大名称数量（spec §5.10 防 CC 攻击）

	// Yggdrasil 速率限制配置（spec §11.2 强制要求：按用户而非 IP 限流）
	YggdrasilAuthRateLimit      = 5         // 认证接口每分钟最大尝试次数（authenticate）
	YggdrasilSignoutRateLimit   = 10        // 登出接口每分钟最大尝试次数（signout）
	YggdrasilRateLimitWindowSec = 60        // 速率限制时间窗口（秒）

	// Yggdrasil API 路由前缀
	YggdrasilAPIPrefix = "/api/v1/yggdrasil"

	// Yggdrasil ALI (API Location Indication) 响应头
	YggdrasilALIHeader = "X-Authlib-Injector-API-Location"
	YggdrasilALIPath   = "/api/v1/yggdrasil/"
)
