# FrontLeaves YggLeaf

**锋楪 Minecraft 用户中心微服务** — 管理用户账户、游戏档案、皮肤/披风资源库及资源配额。

## 技术栈

Go 1.25.3 / Gin + GORM / PostgreSQL + Redis / S3 (纹理存储) / OAuth2 SSO

## 项目结构

```
frontleaves-yggleaf/
├── api/                  # 请求/响应 DTO
├── internal/
│   ├── app/              # 路由、中间件、启动初始化
│   ├── constant/         # 常量 (缓存 Key、环境变量、Gene 编号)
│   ├── entity/           # 数据库实体 (User, Role, GameProfile, SkinLibrary, CapeLibrary, Quota...)
│   ├── handler/          # HTTP 处理器
│   ├── logic/            # 业务逻辑 (事务、配额校验、纹理去重)
│   └── repository/       # 数据访问 + Redis 缓存
├── pkg/                  # 公共工具 (Context 辅助等)
├── docs/                 # Swagger 生成文档
└── dev/markdown/         # 项目文档
```

## API 概览 (`/api/v1`)

| 模块 | 端点 | 功能 |
|------|------|------|
| 用户 | `GET /user/info` | 获取当前用户 |
| 档案 | `POST /game-profile` | 创建游戏档案 |
| 档案 | `PATCH /game-profile/:id/username` | 修改档案名 |
| 皮肤 | `CRUD /library/skins` | 皮肤上传/列表/更新/删除 |
| 披风 | `CRUD /library/capes` | 披风上传/列表/更新/删除 |

## 核心机制

- **SSO 认证**: OAuth2 Token → 缓存优先 → SSO 回源 → 本地用户同步
- **资源配额**: 每用户游戏档案/皮肤/披风的公私配额独立管控
- **纹理去重**: Base64 上传 → SHA256 哈希比对 → 去重复用
- **缓存策略**: Redis Hash (TTL 15min)，Key 前缀 `fyl:`
- **ID 方案**: Snowflake 分布式 ID，各实体分配独立 Gene 编号 (32-37)

## 开发

```bash
make dev    # 生成 Swagger + 运行 (推荐)
make swag   # 仅生成 Swagger
make run    # 仅运行
```

端口 `5577`，配置通过 `.env` 文件 (参考 `.env.example`)。
