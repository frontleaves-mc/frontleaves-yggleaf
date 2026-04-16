package bConst

import xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"

const (
	EnvBucketAccessID     xEnv.EnvKey = "BUCKET_ACCESS_ID"      // 定义了对象存储（Bucket）访问密钥 ID 的环境变量键名。
	EnvBucketSecretKey    xEnv.EnvKey = "BUCKET_SECRET_KEY"     // 定义了对象存储访问密钥的环境变量键名。
	EnvBucketHost         xEnv.EnvKey = "BUCKET_HOST"           // 定义了对象存储主机地址的环境变量键名。
	EnvBucketPort         xEnv.EnvKey = "BUCKET_PORT"           // 定义了对象存储端口号的环境变量键名。
	EnvBucketSkinBucketId xEnv.EnvKey = "BUCKET_SKIN_BUCKET_ID" // 定义了皮肤存储桶 ID 的环境变量键名。
	EnvBucketSkinPathId   xEnv.EnvKey = "BUCKET_SKIN_PATH_ID"   // 定义了皮肤存储路径 ID 的环境变量键名。
	EnvBucketCapeBucketId xEnv.EnvKey = "BUCKET_CAPE_BUCKET_ID" // 定义了披风存储桶 ID 的环境变量键名。
	EnvBucketCapePathId   xEnv.EnvKey = "BUCKET_CAPE_PATH_ID"   // 定义了披风存储路径 ID 的环境变量键名。

	EnvYggdrasilPrivateKeyPath xEnv.EnvKey = "YGGDRASIL_PRIVATE_KEY_PATH" // Yggdrasil RSA 私钥文件路径
	EnvYggdrasilPublicKeyPath  xEnv.EnvKey = "YGGDRASIL_PUBLIC_KEY_PATH"  // Yggdrasil RSA 公钥文件路径
	EnvYggdrasilSkinDomainsExtra xEnv.EnvKey = "YGGDRASIL_SKIN_DOMAINS_EXTRA" // 额外皮肤域名（逗号分隔，追加到 skinDomains 白名单）
)
