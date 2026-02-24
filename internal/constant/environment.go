package bConst

import xEnv "github.com/bamboo-services/bamboo-base-go/defined/env"

const (
	EnvBucketAccessID  xEnv.EnvKey = "BUCKET_ACCESS_ID"  // 定义了对象存储（Bucket）访问密钥 ID 的环境变量键名。
	EnvBucketSecretKey xEnv.EnvKey = "BUCKET_SECRET_KEY" // 定义了对象存储访问密钥的环境变量键名。
	EnvBucketHost      xEnv.EnvKey = "BUCKET_HOST"       // 定义了对象存储主机地址的环境变量键名。
	EnvBucketPort      xEnv.EnvKey = "BUCKET_PORT"       // 定义了对象存储端口号的环境变量键名。
)
