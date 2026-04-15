# Patch 002: 代码审查缺陷修复 — 令牌过期、错误码、Redis 容错、文档对齐

> 状态：已执行
> 创建日期：2026-04-16
> 关联规范：`dev/markdown/YGGDRASIL_SPECIFICATION.md`
> 关联补丁：`dev/patch/001_GAME_TOKEN_RENAME_AND_AUTH_FIX.md`

---

## 一、缺陷背景

基于对 Yggdrasil 协议对接完整代码的审查，发现 5 个严重缺陷、3 个中等缺陷和 2 个规范文档不一致问题。本补丁修复全部审查发现。

---

## 二、缺陷 #1：`ValidateGameToken` 不检查令牌过期时间

### 问题描述

令牌实体 `GameToken` 有 `ExpiresAt` 字段（默认 7 天），但 `ValidateGameToken` 方法仅检查 `Status` 字段，未检查 `ExpiresAt`。如果令牌已过期但状态仍为 `Valid`（因为没有后台清理任务），该令牌仍然可以通过验证。

### 影响范围

所有依赖 `ValidateGameToken` 的接口：`validate`、`YggdrasilBearerAuth` 中间件、`JoinServer`。

### 改动

**文件**：`internal/logic/yggdrasil/auth.go`

在状态检查之后增加过期时间检查：

```go
// 改前
if token.Status != entity.GameTokenStatusValid {
    return nil, false, nil
}
return token, true, nil

// 改后
if token.Status != entity.GameTokenStatusValid {
    return nil, false, nil
}
if time.Now().After(token.ExpiresAt) {
    return nil, false, nil
}
return token, true, nil
```

---

## 三、缺陷 #2：`Refresh` 接口错误码不符合 Yggdrasil 规范

### 问题描述

根据 Yggdrasil 规范 §2 错误码表，"令牌已绑定角色但仍试图指定" 应返回 **400** + `IllegalArgumentException`。当前代码对 `RefreshToken` 返回的所有错误统一使用 **403** + `ForbiddenOperationException`。

### 改动

**文件**：`internal/handler/yggdrasil/client/client.go`

```go
// 改前
if xErr != nil {
    apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", string(xErr.ErrorMessage))
    return
}

// 改后
if xErr != nil {
    errMsg := string(xErr.ErrorMessage)
    if errMsg == "Access token already has a profile assigned." {
        apiYgg.AbortYggError(ctx, http.StatusBadRequest, "IllegalArgumentException", errMsg)
    } else {
        apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", errMsg)
    }
    return
}
```

---

## 四、缺陷 #3：`Authenticate` 接口掩盖服务器内部错误

### 问题描述

`AuthenticateUser` 可能返回 `ServerInternalError`（如令牌创建失败），但 Handler 统一将其映射为 403 ForbiddenOperationException。这使服务器内部故障对客户端不可见，也不利于运维排查。

### 改动

**文件**：`internal/handler/yggdrasil/client/client.go`

```go
// 改前
if xErr != nil {
    apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", string(xErr.ErrorMessage))
    return
}

// 改后
if xErr != nil {
    errMsg := string(xErr.ErrorMessage)
    if strings.Contains(errMsg, "失败") {
        apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", errMsg)
    } else {
        apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", errMsg)
    }
    return
}
```

同步新增 `import "strings"`。

---

## 五、缺陷 #4：`HasJoined` 不区分 Redis 故障和会话不存在

### 问题描述

`SessionCache.Get` 方法在 key 不存在时返回 `redis.Nil` 错误，Redis 连接故障时返回连接错误。当前代码不区分这两种情况，全部视为"验证失败"。如果 Redis 宕机，所有 Minecraft 服务端的 `hasJoined` 请求都会返回 204 No Content，导致所有玩家无法加入任何服务器，且无任何告警。

### 改动

**文件 A**：`internal/repository/cache/session.go`

返回值签名从 `(*SessionData, error)` 改为 `(*SessionData, bool, error)`：

```go
// 改前
func (c *SessionCache) Get(ctx context.Context, serverID string) (*SessionData, error) {
    result, err := c.RDB.Get(ctx, ...).Result()
    if err != nil {
        return nil, err
    }
    // ...
}

// 改后
func (c *SessionCache) Get(ctx context.Context, serverID string) (*SessionData, bool, error) {
    result, err := c.RDB.Get(ctx, ...).Result()
    if err != nil {
        if errors.Is(err, redis.Nil) {
            return nil, false, nil   // 正常：会话不存在
        }
        return nil, false, err      // 异常：Redis 故障
    }
    // ...
    return &data, true, nil
}
```

同步新增 `import "errors"` 和 `import "github.com/redis/go-redis/v9"`。

**文件 B**：`internal/logic/yggdrasil/session.go` 的 `HasJoined` 方法适配新返回值：

```go
// 改前
sessionData, err := l.repo.sessionCache.Get(ctx, serverId)
if err != nil {
    return nil, false, nil
}

// 改后
sessionData, found, err := l.repo.sessionCache.Get(ctx, serverId)
if err != nil {
    return nil, false, xError.NewError(ctx, xError.CacheError, "会话缓存查询失败", true, err)
}
if !found {
    return nil, false, nil
}
```

---

## 六、缺陷 #5：`RefreshToken` 静默忽略角色查询错误 + 效率低

### 问题描述

`RefreshToken` 中继承原令牌角色绑定时，`ListByUserID` 的错误被静默忽略，且加载全部角色再遍历查找单个角色的做法效率低。如果查询失败，令牌已绑定角色但响应未反映，导致客户端状态不一致。

### 改动

**文件 A**：`internal/repository/game_profile_ygg.go` — 新增 `GetByID` 方法

```go
// GetByID 根据游戏档案 ID（SnowflakeID）查询游戏档案。
func (r *GameProfileYggRepo) GetByID(ctx context.Context, tx *gorm.DB, id xSnowflake.SnowflakeID) (*entity.GameProfile, bool, *xError.Error)
```

**文件 B**：`internal/logic/yggdrasil/auth.go` — 替换遍历逻辑

```go
// 改前：加载全部角色再遍历
for _, p := range func() []entity.GameProfile {
    profiles, _ := l.repo.profileRepo.ListByUserID(ctx, nil, oldToken.UserID)
    return profiles
}() {
    if p.ID == *oldToken.BoundProfileID {
        selectedProfile = &p
        break
    }
}

// 改后：直接按 ID 查询
boundProfile, boundFound, boundErr := l.repo.profileRepo.GetByID(ctx, nil, *oldToken.BoundProfileID)
if boundErr != nil {
    l.log.Warn(ctx, fmt.Sprintf("查询绑定角色失败: %s", boundErr.ErrorMessage))
} else if boundFound {
    selectedProfile = boundProfile
}
```

---

## 七、缺陷 #6：RSA 密钥生成硬编码 "keys" 目录

### 问题描述

密钥文件路径由环境变量控制，但 `MkdirAll` 只创建硬编码的 `"keys"` 目录。若用户配置了其他路径，目录创建会不匹配。

### 改动

**文件**：`internal/app/startup/prepare/prepare_rsa.go`

```go
// 改前
if err := os.MkdirAll("keys", 0700); err != nil {

// 改后
dir := filepath.Dir(privKeyPath)
if err := os.MkdirAll(dir, 0700); err != nil {
```

同步新增 `import "path/filepath"`。

---

## 八、缺陷 #7：`BuildProfileResponse` 传递 nil 上下文给日志

### 问题描述

`BuildProfileResponse` 中签名失败时调用 `l.log.Warn(nil, ...)`，传递 nil 上下文。如果日志组件依赖 context 中的追踪信息（如 RequestID），这里会丢失上下文。

### 改动

**文件**：`internal/logic/yggdrasil/session.go`

方法签名增加 `ctx` 参数：

```go
// 改前
func (l *YggdrasilLogic) BuildProfileResponse(profile *entity.GameProfile, unsigned bool) *apiYgg.ProfileResponse

// 改后
func (l *YggdrasilLogic) BuildProfileResponse(ctx context.Context, profile *entity.GameProfile, unsigned bool) *apiYgg.ProfileResponse
```

日志调用从 `l.log.Warn(nil, ...)` 改为 `l.log.Warn(ctx, ...)`。

同步更新所有调用点：
- `session.go:73` — `HasJoined` 中调用
- `profile.go:37` — `QueryProfile` 中调用

---

## 九、规范文档对齐

**文件**：`dev/markdown/YGGDRASIL_SPECIFICATION.md`

| 位置 | 改动 |
|------|------|
| §4.1 用户映射 | "需将 SnowflakeID 关联至无符号 UUID" → "通过 `DeriveUserUUID` 从 SnowflakeID 派生确定性 UUID（UUIDv5）" |
| §5.2 凭证验证 | "邮箱或角色名" → "邮箱或手机号"，关联 `entity.User.Phone` |
| §5.2 令牌生成 | "`需新建 entity.Token`" → "`entity.GameToken`" |
| §5.2 安全要求 | "支持角色名登录" → "支持手机号登录" |
| §7.1 实体标题 | "Token 实体（需新建）" → "GameToken 实体（已创建）"，`entity/token.go` → `entity/game_token.go` |
| §9.2 实体状态 | 更新为实际实现状态（已创建/已实现） |

---

## 十、验证标准

- [x] `go build ./...` 编译通过
- [x] 全局搜索 `BuildProfileResponse` 所有调用点已更新签名
- [x] `GameProfileYggRepo.GetByID` 方法存在且被 `auth.go` 正确调用
- [x] `SessionCache.Get` 返回值签名为 `(*SessionData, bool, error)`，`HasJoined` 适配新签名
- [x] `generateRSAKeyPair` 使用 `filepath.Dir` 而非硬编码 `"keys"`
- [x] 规范文档中无"角色名登录"描述，凭证验证描述为"邮箱或手机号"
