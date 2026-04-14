# FrontLeaves YggLeaf — Yggdrasil 协议对接文档

> 参考规范：[yushijinhun/authlib-injector Wiki — Yggdrasil 服务端技术规范](https://github.com/yushijinhun/authlib-injector/wiki/Yggdrasil-%E6%9C%8D%E5%8A%A1%E7%AB%AF%E6%8A%80%E6%9C%AF%E8%A7%84%E8%8C%83)

## 一、概述

本文档描述 FrontLeaves YggLeaf 作为 **Yggdrasil 外置登录服务端**时，需要实现的所有协议接口及其与本项目现有架构的映射关系。

### 核心概念

Yggdrasil 协议围绕三个核心模型：

| 模型 | 说明 | 本项目对应实体 |
|------|------|--------------|
| **用户 (User)** | 系统账号，含 ID / 邮箱 / 密码 | `entity.User` |
| **角色 (Profile)** | Minecraft 玩家角色，含 UUID / 名称 / 材质 | `entity.GameProfile` |
| **令牌 (Token)** | 登录凭证，含 accessToken / clientToken | **需新建** `entity.Token` |

### 数据格式约定

- 字符编码：UTF-8
- 请求/响应格式：JSON (`Content-Type: application/json; charset=utf-8`)
- UUID 格式：**无符号 UUID**（去掉所有 `-` 字符）
- 所有 API 应使用 HTTPS 协议

---

## 二、错误信息格式

所有业务异常响应应符合以下格式：

```json
{
  "error": "错误的简要描述（机器可读）",
  "errorMessage": "错误的详细信息（人类可读）",
  "cause": "该错误的原因（可选）"
}
```

### 标准错误码表

| 异常情况 | HTTP 状态码 | Error | Error Message |
|----------|------------|-------|---------------|
| 令牌无效 | 403 | `ForbiddenOperationException` | `Invalid token.` |
| 密码错误/多次登录失败被禁 | 403 | `ForbiddenOperationException` | `Invalid credentials. Invalid username or password.` |
| 令牌已绑定角色但仍试图指定 | 400 | `IllegalArgumentException` | `Access token already has a profile assigned.` |
| 绑定不属于该用户的角色 | 403 | `ForbiddenOperationException` | _未定义_ |
| 使用错误角色加入服务器 | 403 | `ForbiddenOperationException` | `Invalid token.` |
| 一般 HTTP 异常 (404 等) | 对应状态码 | HTTP Reason Phrase | _未定义_ |

---

## 三、协议接口总览

### 3.1 接口分类与路由规划

Yggdrasil 协议要求的服务端接口按功能分为 **五大模块**。

> **重要：路由前缀说明**
>
> Yggdrasil 规范定义的路径是以 API 根路径为基准的相对路径。在本项目中，**根路径 `/` 留给前端页面**，
> 所有 Yggdrasil API 遵循项目规范统一挂载在 `/api/v1/yggdrasil` 前缀下，
> 并通过 ALI（API 地址指示）机制让 authlib-injector 自动发现真正的 API 位置。
>
> 即：用户在启动器输入 `yggleaf.frontleaves.com` → authlib-injector 访问根路径 →
> 读取 `X-Authlib-Injector-API-Location: /api/v1/yggdrasil/` 响应头 → 所有后续请求发往 `/api/v1/yggdrasil/*`。

```
/                                          → 前端页面（非 Yggdrasil）
/api/v1/yggdrasil/                            → API 元数据
/api/v1/yggdrasil/authserver/*                → 认证服务
/api/v1/yggdrasil/sessionserver/session/minecraft/* → 会话服务
/api/v1/yggdrasil/api/*                       → 角色查询 + 材质上传
```

### 3.2 完整接口清单

| # | 方法 | 路径 | 功能 | 消费者 | 状态 |
|---|------|------|------|--------|------|
| 1 | `GET` | `/api/v1/yggdrasil/` | API 元数据获取 | authlib-injector / 启动器 | **待实现** |
| 2 | `POST` | `/api/v1/yggdrasil/authserver/authenticate` | 密码登录认证 | **启动器 (客户端)** | **待实现** |
| 3 | `POST` | `/api/v1/yggdrasil/authserver/refresh` | 刷新令牌 | **启动器 (客户端)** | **待实现** |
| 4 | `POST` | `/api/v1/yggdrasil/authserver/validate` | 验证令牌有效性 | **启动器 (客户端)** | **待实现** |
| 5 | `POST` | `/api/v1/yggdrasil/authserver/invalidate` | 吊销指定令牌 | **启动器 (客户端)** | **待实现** |
| 6 | `POST` | `/api/v1/yggdrasil/authserver/signout` | 吊销用户所有令牌 | **启动器 (客户端)** | **待实现** |
| 7 | `POST` | `/api/v1/yggdrasil/sessionserver/session/minecraft/join` | 客户端加入服务器 | **Minecraft 客户端** | **待实现** |
| 8 | `GET` | `/api/v1/yggdrasil/sessionserver/session/minecraft/hasJoined` | 服务端验证客户端 | **Minecraft 服务端** | **待实现** |
| 9 | `GET` | `/api/v1/yggdrasil/sessionserver/session/minecraft/profile/{uuid}` | 查询角色属性 | **Minecraft 服务端/客户端** | **待实现** |
| 10 | `POST` | `/api/v1/yggdrasil/api/profiles/minecraft` | 按名称批量查询角色 | **Minecraft 服务端** | **待实现** |
| 11 | `PUT` | `/api/v1/yggdrasil/api/user/profile/{uuid}/{textureType}` | 上传材质 | **启动器 (客户端)** | **待实现** |
| 12 | `DELETE` | `/api/v1/yggdrasil/api/user/profile/{uuid}/{textureType}` | 清除材质 | **启动器 (客户端)** | **待实现** |

> **消费者说明**：
> - **启动器 (客户端)**：玩家使用的启动器程序，如 HMCL、BakaXL 等
> - **Minecraft 客户端**：玩家运行的游戏本体
> - **Minecraft 服务端**：玩家加入的游戏服务器
> - **authlib-injector**：注入 Minecraft 的外置登录代理

---

## 四、模型序列化格式

### 4.1 用户信息序列化

```json
{
  "id": "用户的无符号UUID",
  "properties": [
    {
      "name": "preferredLanguage",
      "value": "zh_cn"
    }
  ]
}
```

**本项目映射**：`entity.User` → 需将 SnowflakeID 关联至一个无符号 UUID 作为用户 ID。

### 4.2 角色信息序列化

```json
{
  "id": "角色的无符号UUID",
  "name": "角色名称",
  "properties": [
    {
      "name": "textures",
      "value": "Base64编码的材质JSON",
      "signature": "数字签名（Base64，仅特定情况包含）"
    }
  ]
}
```

**本项目映射**：`entity.GameProfile` → `UUID` 字段直接可用，`Name` 字段直接可用。

### 4.3 `textures` 材质信息属性

将以下 JSON 进行 **Base64 编码**后作为 `textures` 属性的 `value`：

```json
{
  "timestamp": 1700000000000,
  "profileId": "无符号UUID",
  "profileName": "角色名称",
  "textures": {
    "SKIN": {
      "url": "https://纹理域名/textures/SHA256哈希值",
      "metadata": {
        "model": "slim"
      }
    },
    "CAPE": {
      "url": "https://纹理域名/textures/SHA256哈希值"
    }
  }
}
```

**本项目映射**：
- `entity.SkinLibrary.TextureHash` → SKIN 的 SHA256 哈希（已有，可直接作为 URL 文件名）
- `entity.SkinLibrary.Model` → 映射为 `"default"` (ModelTypeClassic=1) 或 `"slim"` (ModelTypeSlim=2)
- `entity.CapeLibrary.TextureHash` → CAPE 的 SHA256 哈希
- 纹理 URL 格式：`https://{纹理域名}/textures/{TextureHash}`

### 4.4 材质 URL 规范

- URL 最后一个 `/` 之后的字符串即为材质 hash
- Minecraft 客户端会直接将 URL 文件名作为 hash 进行缓存
- 纹理文件响应的 `Content-Type` 必须为 `image/png`
- Hash 计算建议使用 SHA-256（本项目已使用 `TextureHash` 字段存储）

---

## 五、各接口详细规范

### 5.1 API 元数据获取

```
GET /api/v1/yggdrasil/
```

**消费者**：authlib-injector（自动发现）、启动器

**请求**：无

**响应**：

```json
{
  "meta": {
    "serverName": "FrontLeaves YggLeaf",
    "implementationName": "frontleaves-yggleaf",
    "implementationVersion": "1.0.0",
    "links": {
      "homepage": "https://yggleaf.frontleaves.com/",
      "register": "https://sso.frontleaves.com/register"
    },
    "feature.non_email_login": true
  },
  "skinDomains": [
    "yggleaf.frontleaves.com",
    ".frontleaves.com"
  ],
  "signaturePublickey": "-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----\n"
}
```

**本项目实现要点**：
- 需要生成 RSA 签名密钥对，公钥 PEM 格式写入 `signaturePublickey`
- `skinDomains` 应包含本项目 S3 纹理服务域名
- `feature.non_email_login` 设为 `true` 以支持角色名登录
- 建议配置化 `meta` 中的服务端信息

---

### 5.2 登录认证

```
POST /api/v1/yggdrasil/authserver/authenticate
```

**消费者**：启动器 (客户端)

**请求**：

```json
{
  "username": "邮箱或角色名",
  "password": "游戏账户密码",
  "clientToken": "客户端指定的令牌标识（可选）",
  "requestUser": true,
  "agent": {
    "name": "Minecraft",
    "version": 1
  }
}
```

**响应**：

```json
{
  "accessToken": "服务端生成的访问令牌",
  "clientToken": "与请求中相同的 clientToken",
  "availableProfiles": [
    {
      "id": "无符号UUID",
      "name": "角色名称"
    }
  ],
  "selectedProfile": {
    "id": "无符号UUID",
    "name": "自动选中的角色（可选）"
  },
  "user": {
    "id": "用户无符号UUID",
    "properties": []
  }
}
```

**本项目实现要点**：

| 步骤 | 说明 | 关联 |
|------|------|------|
| 1. 凭证验证 | 接受邮箱或角色名作为 `username` | `entity.User.Email` / `entity.GameProfile.Name` |
| 2. 密码校验 | 验证 `password` 与 `entity.User.GamePassword` | 需密码加密存储（bcrypt 推荐） |
| 3. 令牌生成 | 生成 accessToken + clientToken | **需新建** `entity.Token` |
| 4. 角色绑定 | 单角色自动绑定，多角色返回空 | `entity.GameProfile` 关联 `UserID` |
| 5. 封禁检查 | `entity.User.HasBan` 为 true 则拒绝 | 已有字段 |

**安全要求**：
- 必须实施速率限制（按用户而非 IP）
- 支持角色名登录（`feature.non_email_login`）

---

### 5.3 刷新令牌

```
POST /api/v1/yggdrasil/authserver/refresh
```

**消费者**：启动器 (客户端)

**请求**：

```json
{
  "accessToken": "当前令牌",
  "clientToken": "客户端令牌（可选）",
  "requestUser": false,
  "selectedProfile": {
    "id": "要选择的角色UUID",
    "name": "角色名称"
  }
}
```

**响应**：

```json
{
  "accessToken": "新令牌",
  "clientToken": "与原令牌相同",
  "selectedProfile": {
    "id": "无符号UUID",
    "name": "角色名称"
  }
}
```

**本项目实现要点**：
- 吊销原令牌，颁发新令牌
- 携带 `selectedProfile` 时为角色选择操作（要求原令牌未绑定角色）
- 暂时失效状态的令牌也可以执行刷新
- 新令牌的 `clientToken` 与原令牌相同

---

### 5.4 验证令牌

```
POST /api/v1/yggdrasil/authserver/validate
```

**消费者**：启动器 (客户端)

**请求**：

```json
{
  "accessToken": "令牌",
  "clientToken": "客户端令牌（可选）"
}
```

**响应**：
- 有效：`204 No Content`（无响应体）
- 无效：返回错误信息（见错误码表）

---

### 5.5 吊销令牌

```
POST /api/v1/yggdrasil/authserver/invalidate
```

**消费者**：启动器 (客户端)

**请求**：

```json
{
  "accessToken": "令牌",
  "clientToken": "客户端令牌（可选）"
}
```

**响应**：无论是否成功，均返回 `204 No Content`

**说明**：仅检查 `accessToken`，忽略 `clientToken`。

---

### 5.6 登出

```
POST /api/v1/yggdrasil/authserver/signout
```

**消费者**：启动器 (客户端)

**请求**：

```json
{
  "username": "邮箱",
  "password": "密码"
}
```

**响应**：
- 成功：`204 No Content`
- 密码错误：标准错误响应

**安全要求**：需与登录接口相同的速率限制。

---

### 5.7 客户端加入服务器

```
POST /api/v1/yggdrasil/sessionserver/session/minecraft/join
```

**消费者**：Minecraft 客户端

**请求**：

```json
{
  "accessToken": "令牌的 accessToken",
  "selectedProfile": "令牌绑定角色的无符号UUID",
  "serverId": "服务端生成的随机字符串"
}
```

**响应**：
- 成功：`204 No Content`
- 失败：标准错误响应

**本项目实现要点**：
- 验证 accessToken 有效且 selectedProfile 与令牌绑定角色一致
- 在 **Redis** 中记录以下信息（设 30 秒过期时间）：
  - `serverId` → `{ accessToken, selectedProfile, clientIP }`
- `serverId` 的随机性使其可作为 Redis Key

**Redis Key 设计**：
```
fyl:yggdrasil:session:{serverId} → { accessToken, profileUUID, clientIP }  TTL=30s
```

---

### 5.8 服务端验证客户端

```
GET /api/v1/yggdrasil/sessionserver/session/minecraft/hasJoined?username={username}&serverId={serverId}&ip={ip}
```

**消费者**：Minecraft 服务端

**请求参数**：

| 参数 | 说明 |
|------|------|
| `username` | 角色名称 |
| `serverId` | 服务端发送给客户端的标识 |
| `ip` | _(可选)_ 客户端 IP，仅在 `prevent-proxy-connections` 开启时携带 |

**响应**：
- 成功：返回角色完整信息（**含属性和数字签名**）

```json
{
  "id": "无符号UUID",
  "name": "角色名称",
  "properties": [
    {
      "name": "textures",
      "value": "Base64编码的材质信息",
      "signature": "数字签名（Base64）"
    }
  ]
}
```

- 失败：`204 No Content`

**本项目实现要点**：
- 从 Redis 中查询 `serverId` 对应的会话记录
- 验证 `username` 与令牌绑定角色的名称相同
- 可选验证客户端 IP（`ip` 参数）
- **必须包含 `textures` 属性和数字签名**（此处 `unsigned` 默认为 `false`）
- 数字签名使用 SHA1withRSA 算法

---

### 5.9 查询角色属性

```
GET /api/v1/yggdrasil/sessionserver/session/minecraft/profile/{uuid}?unsigned={unsigned}
```

**消费者**：Minecraft 服务端 / 客户端

**请求参数**：

| 参数 | 说明 |
|------|------|
| `uuid` | 角色的无符号 UUID |
| `unsigned` | `true`(默认) 不含签名 / `false` 含签名 |

**响应**：
- 存在：角色信息（含属性，按 unsigned 决定是否含签名）

```json
{
  "id": "无符号UUID",
  "name": "角色名称",
  "properties": [
    {
      "name": "textures",
      "value": "Base64编码的材质信息",
      "signature": "数字签名（仅 unsigned=false 时包含）"
    }
  ]
}
```

- 不存在：`204 No Content`

**本项目实现要点**：
- `uuid` 参数为无符号 UUID，查询时需转回标准 UUID 格式
- `entity.GameProfile.UUID` 字段可直接用于查询
- 需要组装 `textures` 属性（见 §4.3）
- 需要实现数字签名（见 §六）

---

### 5.10 按名称批量查询角色

```
POST /api/v1/yggdrasil/api/profiles/minecraft
```

**消费者**：Minecraft 服务端

**请求**：

```json
["角色名称1", "角色名称2"]
```

**响应**：

```json
[
  {
    "id": "无符号UUID",
    "name": "角色名称"
  }
]
```

**说明**：
- 不存在的角色不包含在响应中
- 响应中**不包含角色属性**（即无 `properties`）
- 单次查询数量需设最大值（至少 2），防 CC 攻击

**本项目实现要点**：
- 通过 `entity.GameProfile.Name` 批量查询
- 仅返回 `id`（无符号 UUID）和 `name`

---

### 5.11 上传材质

```
PUT /api/v1/yggdrasil/api/user/profile/{uuid}/{textureType}
```

**消费者**：启动器 (客户端)

**认证**：`Authorization: Bearer {accessToken}`

**请求**：`Content-Type: multipart/form-data`

| 字段 | 说明 |
|------|------|
| `model` | _(仅皮肤)_ `slim` 或空字符串（普通皮肤） |
| `file` | PNG 图片，`Content-Type: image/png` |

**响应**：
- 成功：`204 No Content`
- 未认证：`401 Unauthorized`

**本项目实现要点**：
- 从 `Authorization` 头获取 accessToken，验证令牌有效性
- 验证 `{uuid}` 对应的角色属于该令牌的用户
- 皮肤需检查尺寸（64x32 或 64x64 的整数倍）
- 披风需检查尺寸（64x32 或 22x17 的整数倍）
- **安全：必须去除 PNG 中非位图数据，防止远程代码执行**
- 计算图片 SHA-256 作为 hash → 存入 `TextureHash`
- 图片上传至 S3，URL 格式：`/textures/{hash}`
- 去重：已有相同 hash 则复用

---

### 5.12 清除材质

```
DELETE /api/v1/yggdrasil/api/user/profile/{uuid}/{textureType}
```

**消费者**：启动器 (客户端)

**认证**：`Authorization: Bearer {accessToken}`

**响应**：
- 成功：`204 No Content`
- 未认证：`401 Unauthorized`

**本项目实现要点**：
- 将 `GameProfile` 的 `SkinLibraryID` 或 `CapeLibraryID` 置为 NULL
- 验证令牌和角色归属关系

---

## 六、数字签名机制

### 6.1 签名密钥对

Yggdrasil 协议要求使用 **RSA 密钥对** 对角色属性进行数字签名。

| 项目 | 说明 |
|------|------|
| 算法 | SHA1withRSA (PKCS #1) |
| 公钥格式 | PEM 格式，在 API 元数据中暴露 |
| 私钥用途 | 签名 `textures` 属性的 `value` 值 |
| 签名编码 | Base64 |

### 6.2 签名流程

```
1. 组装 textures JSON（见 §4.3）
2. 将 JSON 字符串进行 Base64 编码 → value
3. 使用私钥对 value 进行 SHA1withRSA 签名
4. 将签名结果进行 Base64 编码 → signature
5. 返回 { name: "textures", value, signature }
```

### 6.3 本项目实现建议

- 启动时生成或从配置加载 RSA 密钥对
- 私钥缓存于内存，用于签名
- 公钥 PEM 写入 API 元数据响应
- 建议新增 `internal/constant/` 中的配置项管理密钥路径

---

## 七、需要新建的实体与数据结构

### 7.1 Token 实体（需新建）

```go
// entity/token.go
type Token struct {
    ID              int64     // 主键
    AccessToken     string    // 服务端生成的访问令牌（UUID 或 JWT）
    ClientToken     string    // 客户端提供的令牌标识
    UserID          int64     // 关联用户 ID（Snowflake）
    BoundProfileID *int64     // 绑定的游戏档案 ID（可选）
    Status          uint8     // 令牌状态：1=有效, 2=暂时失效, 3=无效
    IssuedAt        time.Time // 颁发时间
    ExpiresAt       time.Time // 过期时间
}
```

**令牌状态机**：

```
有效 (1) ──→ 暂时失效 (2) ──→ 无效 (3)
  │                                ↑
  └────────────────────────────────┘
```

**Gene 编号建议**：`38`（接续现有编号 32-37）

### 7.2 会话缓存结构（Redis，无需持久化实体）

```
Key:    fyl:yggdrasil:session:{serverId}
Value:  { accessToken, profileUUID, clientIP }
TTL:    30s
```

### 7.3 签名密钥配置

```
YGGDRASIL_PRIVATE_KEY_PATH=/path/to/private_key.pem
YGGDRASIL_PUBLIC_KEY_PATH=/path/to/public_key.pem
```

---

## 八、路由注册规划

在 `internal/app/route/route.go` 的 `NewRoute` 中新增 Yggdrasil 路由组：

```go
// Yggdrasil 协议路由（挂载在 /api/v1/yggdrasil 前缀下）
{
    yggRouter := r.engine.Group("/api/v1/yggdrasil")
    yggHandler := handler.NewHandler[handler.YggdrasilHandler](r.context, "YggdrasilHandler")

    // API 元数据
    yggRouter.GET("/", yggHandler.APIMetadata)

    // 认证服务
    authGroup := yggRouter.Group("/authserver")
    authGroup.POST("/authenticate", yggHandler.Authenticate)
    authGroup.POST("/refresh", yggHandler.Refresh)
    authGroup.POST("/validate", yggHandler.Validate)
    authGroup.POST("/invalidate", yggHandler.Invalidate)
    authGroup.POST("/signout", yggHandler.Signout)

    // 会话服务
    sessionGroup := yggRouter.Group("/sessionserver/session/minecraft")
    sessionGroup.POST("/join", yggHandler.JoinServer)
    sessionGroup.GET("/hasJoined", yggHandler.HasJoinedServer)

    // 角色查询
    yggRouter.GET("/sessionserver/session/minecraft/profile/:uuid", yggHandler.ProfileLookup)
    yggRouter.POST("/api/profiles/minecraft", yggHandler.ProfilesBatchLookup)

    // 材质上传/清除
    yggRouter.PUT("/api/user/profile/:uuid/:textureType", yggHandler.UploadTexture)
    yggRouter.DELETE("/api/user/profile/:uuid/:textureType", yggHandler.DeleteTexture)
}
```

> **注意**：Yggdrasil 路由挂载在 `/api/v1/yggdrasil` 前缀下，与现有的 `/api/v1` 管理路由（用户、档案、资源库）互不冲突。前端页面通过 ALI 响应头指向此前缀。

---

## 九、现有实体映射关系

### 9.1 已有实体可直接复用

| 现有实体 | Yggdrasil 用途 | 字段映射 |
|----------|---------------|---------|
| `entity.User` | 用户模型 | `Email` → 登录凭证, `GamePassword` → 密码验证, `HasBan` → 封禁判断 |
| `entity.GameProfile` | 角色模型 | `UUID` → 角色 UUID, `Name` → 角色名称 |
| `entity.SkinLibrary` | 皮肤材质 | `TextureHash` → 材质 URL, `Model` → default/slim, `IsPublic` → 可见性 |
| `entity.CapeLibrary` | 披风材质 | `TextureHash` → 材质 URL, `IsPublic` → 可见性 |
| `entity.Role` | 权限角色 | 可用于令牌权限分级 |

### 9.2 需新增的字段或实体

| 实体/字段 | 说明 |
|-----------|------|
| `entity.Token` | **新建**：令牌管理（accessToken, clientToken, 绑定角色, 状态, 过期时间） |
| `User` 无符号 UUID | 需确定方案：可复用 User 的 SnowflakeID 转 UUID，或新增 UUID 字段 |
| 纹理 URL 生成 | 需根据 S3 配置域名 + TextureHash 组装完整 URL |
| RSA 密钥对 | 启动时加载，用于签名和验证 |

---

## 十、进服验证流程图

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Minecraft      │     │  Minecraft      │     │  FrontLeaves    │
│  客户端         │     │  服务端         │     │  YggLeaf        │
└────────┬────────┘     └────────┬────────┘     └────────┬────────┘
         │                       │                        │
         │  ① 共同生成 serverId   │                        │
         │◄──────────────────────►│                        │
         │                       │                        │
         │  ② POST /sessionserver/session/minecraft/join  │
         │───────────────────────────────────────────────►│
         │  { accessToken, selectedProfile, serverId }    │
         │                       │       ③ Redis 记录会话   │
         │                       │                        │
         │                       │  ④ GET /sessionserver/session/minecraft/hasJoined
         │                       │───────────────────────►│
         │                       │  ?username=xx&serverId=xx
         │                       │                        │
         │                       │  ⑤ 返回角色信息(含签名)  │
         │                       │◄───────────────────────│
         │                       │                        │
         │  ⑥ 验证通过，玩家进服  │                        │
         │◄──────────────────────►│                        │
```

---

## 十一、安全注意事项

### 11.1 材质上传安全

- **必须**去除 PNG 中非位图数据，防止远程代码执行 (RCE)
- **必须**在读取前检查图像大小，防止 PNG Bomb (DoS)
- 皮肤尺寸验证：64x32 或 64x64 的整数倍
- 披风尺寸验证：64x32 或 22x17 的整数倍
- 响应 `Content-Type` 必须为 `image/png`，防止 MIME Sniffing

### 11.2 认证接口安全

- `/authserver/authenticate` 和 `/authserver/signout` 必须实施**速率限制**
- 速率限制应**针对用户**而非客户端 IP
- 令牌数量应设上限（建议 10 个），超出时吊销最旧令牌

### 11.3 会话安全

- 会话记录存储于 Redis，TTL 30 秒
- `hasJoined` 验证可包含 IP 比对（防代理）

---

## 十二、实现优先级建议

| 阶段 | 接口 | 说明 |
|------|------|------|
| **P0 基础** | API 元数据 (`GET /`) | authlib-injector 发现服务的入口 |
| **P0 基础** | 登录认证 (`POST /authserver/authenticate`) | 启动器登录 |
| **P0 基础** | 令牌验证 (`POST /authserver/validate`) | 启动器保活 |
| **P0 基础** | Token 实体 + 签名密钥对 | 基础设施 |
| **P1 核心** | 客户端加入 (`POST .../join`) | 进服必要 |
| **P1 核心** | 服务端验证 (`GET .../hasJoined`) | 进服必要 |
| **P1 核心** | 查询角色属性 (`GET .../profile/{uuid}`) | 材质加载 |
| **P1 核心** | 批量查询角色 (`POST /api/v1/yggdrasil/api/profiles/minecraft`) | 服务端用 |
| **P2 完善** | 刷新令牌 (`POST /authserver/refresh`) | 多角色选择 |
| **P2 完善** | 吊销/登出 (`invalidate` / `signout`) | 安全管理 |
| **P3 扩展** | 材质上传/清除 | 皮肤管理（已有管理接口可复用） |

---

## 十三、API 地址指示 (ALI)

authlib-injector 支持 API 地址指示（API Location Indication），通过 HTTP 响应头 `X-Authlib-Injector-API-Location` 实现服务发现。

**建议配置**：在项目首页或全站响应中添加：

```
X-Authlib-Injector-API-Location: /api/v1/yggdrasil/
```

这样用户只需在启动器中输入 `yggleaf.frontleaves.com`，authlib-injector 会自动发现 `/api/v1/yggdrasil/` 作为 API 根路径，无需记忆完整路径。

---

## 参考链接

- [Yggdrasil 服务端技术规范](https://github.com/yushijinhun/authlib-injector/wiki/Yggdrasil-%E6%9C%8D%E5%8A%A1%E7%AB%AF%E6%8A%80%E6%9C%AF%E8%A7%84%E8%8C%83)
- [启动器技术规范](https://github.com/yushijinhun/authlib-injector/wiki/%E5%90%AF%E5%8A%A8%E5%99%A8%E6%8A%80%E6%9C%AF%E8%A7%84%E8%8C%83)
- [authlib-injector 仓库](https://github.com/yushijinhun/authlib-injector)
- [yggdrasil-mock 参考实现](https://github.com/yushijinhun/yggdrasil-mock)
- [wiki.vg Authentication](http://wiki.vg/Authentication)
