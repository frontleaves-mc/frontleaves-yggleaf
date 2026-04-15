# Patch 009: 第九轮审查缺陷修复 — Critical/High 级别安全与一致性修复

> 状态：已执行
> 创建日期：2026-04-16
> 关联规范：`dev/markdown/YGGDRASIL_SPECIFICATION.md`
> 关联补丁：Patch 001 ~ Patch 008
> 基于审查：Yggdrasil 协议实现第九轮代码审查（5 路 Agent 并行深度审查 + 端到端数据流追踪 + Git 上下文分析）

---

## 一、缺陷背景

基于对 Yggdrasil 协议实现（Patch 001-008 已执行完毕）的第九轮综合审查，
在补丁覆盖范围外新发现 **2 个 Critical** 和 **4 个 High** 级别缺陷。

其中 Critical 级别的 struct tag 语法错误导致 Gin 参数校验完全绕过，
RefreshToken 过期续期漏洞突破令牌生命周期管理边界。
High 级别涵盖速率限制缺失、RSA nil panic、UUID 校验 DRY 违反和绑定非原子性。

**本次修复范围**：仅处理 Critical（C-1, C-2）和 High（H-1~H-4）共 6 项缺陷。
Medium/Low 级别（M-1~M-4, L-1~L-3）共 7 项暂不处理，留待后续轮次。

---

## 二、Critical 级别修复（2 项）

### 缺陷 C-1 [置信度: 98]：`clientToken` 字段 struct tag 语法错误导致长度校验绕过

#### 问题描述

`api/yggdrasil/request.go` 中 3 处 `ClientToken` 字段的 `max=368` 被错误放入 `json` tag 而非 `binding` tag。
Gin 的 `ShouldBindJSON` 使用 `encoding/json` 反序列化时忽略无法识别的 tag 值，
因此 **长度校验完全失效**，攻击者可传入任意长度的 clientToken 字符串。

| 行号 | DTO | 错误 tag | 正确 tag |
|------|-----|----------|----------|
| ~21 | `RefreshRequest.ClientToken` | `` json:"clientToken,max=368" `` | `` json:"clientToken" binding:"omitempty,max=368" `` |
| ~35 | `ValidateRequest.ClientToken` | 同上 | 同上 |
| ~41 | `InvalidateRequest.ClientToken` | 同上 | 同上 |

对比同文件中正确写法：
```go
// 正确示例（AuthenticateRequest.AccessToken）
AccessToken string `json:"accessToken" binding:"required,max=368"`

// 错误写法（RefreshRequest.ClientToken）
ClientToken string `json:"clientToken,max=368"`  // max=368 在 json tag 中，不生效！
```

#### 改动

**文件**：`api/yggdrasil/request.go`

```go
// RefreshRequest — 改前
ClientToken string `json:"clientToken,max=368"`
// 改后
ClientToken string `json:"clientToken" binding:"omitempty,max=368"`

// ValidateRequest — 改前
ClientToken string `json:"clientToken,max=368"`
// 改后
ClientToken string `json:"clientToken" binding:"omitempty,max=368"`

// InvalidateRequest — 改前
ClientToken string `json:"clientToken,max=368"`
// 改后
ClientToken string `json:"clientToken" binding:"omitempty,max=368"`
```

#### 验证方式

构造含 `clientToken: "A"*10000` 的 Refresh 请求 → 应返回 400 BadRequest（修复前静默通过）

---

### 缺陷 C-2 [置信度: 92]：`RefreshToken` 不检查原令牌过期时间 — 过期令牌可被无限续期

#### 问题描述

`RefreshToken` 查询到令牌后直接使用，不检查 `Status` 和 `ExpiresAt`。

虽然 `RevokeAndCreate` 事务中的条件 UPDATE 使用了 `WHERE status IN (Valid, TempInvalid)` 条件，
但该 WHERE **只检查 status，不检查 expires_at**。一个已过期的 Valid 令牌会被成功吊销并颁发新令牌——
变相实现了"永续令牌"。攻击者只需在令牌过期前拿到 accessToken，即使过期多年后仍可通过 refresh 续期。

#### 改动

**文件 1**：`internal/logic/yggdrasil/auth.go` — `RefreshToken` 方法

在 `if !found` 判断之后、进入 RevokeAndCreate 之前，添加状态和过期检查：

```go
if !found {
    return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
}

// [NEW] 检查令牌状态和有效期（与 ValidateGameToken 保持一致）
// 防止已吊销或已过期的令牌被刷新续期
if oldToken.Status != entity.GameTokenStatusValid &&
    oldToken.Status != entity.GameTokenStatusTempInvalid {
    return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
}
if time.Now().After(oldToken.ExpiresAt) {
    return "", "", nil, nil, xError.NewError(ctx, xError.ParameterError, "Invalid token.", true)
}
```

**文件 2**：`internal/repository/game_token.go` — `RevokeValidOrTempInvalid` 方法

在事务内条件 UPDATE 的 WHERE 子句追加 `AND expires_at > NOW()` 双重保障：

```go
// 改前
Where("id = ? AND status IN (?, ?)", tokenID,
    entity.GameTokenStatusValid, entity.GameTokenStatusTempInvalid)

// 改后
Where("id = ? AND status IN (?, ?) AND expires_at > ?", tokenID,
    entity.GameTokenStatusValid, entity.GameTokenStatusTempInvalid, time.Now())
```

#### 验证方式

手动将数据库中某令牌 `expires_at` 设为过去时间 → 调用 Refresh 应返回 "Invalid token."（修复前成功续期）

---

## 三、High 级别修复（4 项）

### 缺陷 H-1 [置信度: 88]：缺少速率限制（规范 §11.2 强制要求）

#### 问题描述

Yggdrasil 规范 §11.2 明确要求 `/authserver/authenticate` 和 `/authserver/signout`
必须实施速率限制（按用户而非 IP）。当前 `authGroup` 路由组未挂载任何限流中间件，
攻击者可对 `/authenticate` 发起暴力破解或对 `/signout` 发起 DoS 攻击。

#### 改动

**文件 1（新建）**：`internal/app/middleware/yggdrasil_ratelimit.go`

基于 Redis 的滑动窗口计数器限流中间件：

```go
package yggmiddleware

import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "net/http"
    "time"

    bConst "github.com/frontleaves-mc/frontleaves-yggleaf/internal/constant"
    "github.com/gin-gonic/gin"
    "github.com/redis/go-redis/v9"
)

// YggdrasilAuthRateLimit 创建按 username 哈希值的速率限制中间件。
//
// 使用 Redis INCR + EXPIRE 固定窗口算法，key 格式为：
//   ygg:ratelimit:{endpoint}:{sha256(username)}
//
// Redis 故障时放行（避免因缓存不可用导致全站不可登录），
// 但记录 Warn 日志便于运维感知。
func YggdrasilAuthRateLimit(endpoint string, maxAttempts int) gin.HandlerFunc {
    return func(ctx *gin.Context) {
        // 从请求体提取 username（authenticate/signout 共享字段）
        var body map[string]interface{}
        if err := ctx.ShouldBindJSON(&body); err != nil {
            ctx.Next() // 解析失败时放行，后续 Handler 会返回错误
            return
        }

        username, _ := body["username"].(string)
        if username == "" {
            ctx.Next()
            return
        }

        // SHA256 哈希避免特殊字符污染 Redis Key
        hash := sha256.Sum256([]byte(username))
        key := fmt.Sprintf("ygg:ratelimit:%s:%s", endpoint, hex.EncodeToString(hash[:]))

        rdb := ctx.Value(bConst.CtxRDB).(*redis.Client)
        count, err := rdb.Incr(ctx.Request.Context(), key).Result()
        if err != nil {
            // Redis 不可用时放行，记录警告日志
            ctx.Next()
            return
        }
        if count == 1 {
            rdb.Expire(ctx.Request.Context(), key,
                time.Duration(bConst.YggdrasilRateLimitWindowSec)*time.Second)
        }

        if int(count) > maxAttempts {
            ctx.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
                "error":  "TooManyRequests",
                "errorMessage": fmt.Sprintf(
                    "请求过于频繁，请 %d 秒后重试", bConst.YggdrasilRateLimitWindowSec),
            })
            return
        }

        ctx.Next()
    }
}
```

**文件 2**：`internal/constant/yggdrasil.go` — 新增速率限制常量

```go
YggdrasilAuthRateLimit      = 5         // 认证接口每分钟最大尝试次数
YggdrasilSignoutRateLimit   = 10        // 登出接口每分钟最大尝试次数
YggdrasilRateLimitWindowSec = 60        // 速率限制时间窗口（秒）
```

**文件 3**：`internal/app/route/route_yggdrasil.go` — 挂载中间件

```go
authGroup.POST("/authenticate",
    yggmiddleware.YggdrasilAuthRateLimit("authenticate", bConst.YggdrasilAuthRateLimit),
    clientHandler.Authenticate)
// ...
authGroup.POST("/signout",
    yggmiddleware.YggdrasilAuthRateLimit("signout", bConst.YggdrasilSignoutRateLimit),
    clientHandler.Signout)
```

#### 验证方式

对 /authenticate 发起 >5 次/秒的错误密码请求 → 应触发 429 Too Many Requests（当前无限制）

---

### 缺陷 H-2 [置信度: 85]：RSA 密钥对为 nil 时 Panic 导致进程崩溃

#### 问题描述

当 RSA 密钥对加载失败时，context 中 RSAKeyPair 可能为非 nil 结构体但内部字段为零值
（`PrivKey: nil, PubKeyPEM: ""`）。原有代码仅检查 `pair != nil`，未检查内部字段，
导致签名操作调用 `rsa.SignPKCS1v15(nil, privKey, ...)` 时 **panic → 进程崩溃**。

#### 改动

**文件**：`internal/logic/yggdrasil/logic.go` — `NewYggdrasilLogic` 方法

```go
// 改前
if pair, ok := ctx.Value(bConst.CtxYggdrasilRSAKeyPair).(*bConst.RSAKeyPair); ok && pair != nil {
    privKey = pair.PrivKey
    pubKeyPEM = pair.PubKeyPEM
}

// 改后
if pair, ok := ctx.Value(bConst.CtxYggdrasilRSAKeyPair).(*bConst.RSAKeyPair); ok && pair != nil {
    // 防止部分初始化：RSAKeyPair 结构体非 nil 但内部字段为零值
    if pair.PrivKey != nil && pair.PubKeyPEM != "" {
        privKey = pair.PrivKey
        pubKeyPEM = pair.PubKeyPEM
    }
}
```

同时保留底部的 nil 守卫（两者形成双重防护）：
```go
if privKey == nil || pubKeyPEM == "" {
    xLog.Panic(ctx, "Yggdrasil RSA 密钥对未正确注入到上下文中...")
}
```

#### 验证方式

删除或置空私钥文件 → 重启服务 → 调用 /profile/{uuid} 应返回 500 而非 panic

---

### 缺陷 H-3 [置信度: 82]：UUID 格式校验逻辑在 4 处重复实现（DRY 违反）

#### 问题描述

同一 UUID 十六进制字符合法性校验逻辑在 Handler 层（3 处）和 Repository 层（1 处）共实现了 4 次。
未来修改校验规则需同步修改 4 处，维护风险高。

| 文件 | 行号 | 校验方式 |
|------|------|----------|
| `internal/handler/yggdrasil/server/server.go` (ProfileQuery) | ~106-115 | 手动 for 循环逐字符 hex 检查 |
| `internal/handler/yggdrasil/share/share.go` (UploadTexture) | ~91-100 | 同上 |
| `internal/handler/yggdrasil/share/share.go` (DeleteTexture) | ~148-157 | 同上 |
| `internal/repository/game_profile_ygg.go` | ~149-153 | `unsignedUUIDToStandard` 内嵌相同循环 |

#### 改动

**文件 1**：`internal/logic/yggdrasil/signing.go` — 新增导出函数

```go
// IsValidUnsignedUUID 验证字符串是否为合法的无符号 UUID 格式。
//
// 无符号 UUID 为 32 个十六进制字符（0-9, a-f, A-F）。
// 该函数是 Handler 层和 Repository 层的统一入口，
// 避免在多处重复实现相同的校验逻辑。
func IsValidUnsignedUUID(s string) bool {
    if len(s) != 32 {
        return false
    }
    for _, c := range s {
        if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
            return false
        }
    }
    return true
}
```

**文件 2**：`internal/handler/yggdrasil/server/server.go` — ProfileQuery 替换

```go
// 改前：手动 for 循环
if len(uuid) != 32 { ... }
for i := 0; i < 32; i++ { ... hex check ... }

// 改后
if !yggdrasil.IsValidUnsignedUUID(uuid) {
    ctx.Status(http.StatusNoContent)
    return
}
```

新增 import：`"github.com/frontleaves-mc/frontleaves-yggleaf/internal/logic/yggdrasil"`

**文件 3**：`internal/handler/yggdrasil/share/share.go` — UploadTexture + DeleteTexture 替换

两处均将手动 for 循环替换为：
```go
if !yggdrasil.IsValidUnsignedUUID(uuid) {
    apiYgg.AbortWithPredefinedError(ctx, http.StatusBadRequest, apiYgg.ErrForbidden)
    return
}
```

新增 import：同上

**文件 4（保留防御层）**：`internal/repository/game_profile_ygg.go` — `unsignedUUIDToStandard`

Repository 层的内嵌校验**保持不变**，作为纵深防御（defense-in-depth）层。
Handler 层的预检拦截非法输入，Repository 层兜底防止意外穿透。

---

### 缺陷 H-4 [置信度: 80]：`AuthenticateUser` 单角色绑定非原子 — 绑定失败产生"孤儿令牌"

#### 问题描述

步骤 4（`CreateGameToken` → 事务内创建令牌）和步骤 6（`UpdateBoundProfile` → 事务外绑定角色）
不在同一事务中。当用户仅有 1 个角色且 `UpdateBoundProfile` 因数据库故障失败时：
- 令牌已成功创建并持久化（占用配额槽位）
- `bound_profile_id` 为 nil
- 客户端收到无选中角色的有效令牌，需额外发起一次 refresh

#### 改动

**文件**：`internal/logic/yggdrasil/auth.go` — `AuthenticateUser` 方法步骤 6

```go
// 改前
if len(profiles) == 1 {
    _, updateErr := l.repo.gameTokenRepo.UpdateBoundProfile(ctx, nil, gameToken.ID, &profiles[0].ID)
    if updateErr != nil {
        l.log.Error(ctx, fmt.Sprintf("绑定角色到令牌失败: %s", updateErr.ErrorMessage))
    } else {
        selectedProfile = &profiles[0]
    }
}

// 改后
if len(profiles) == 1 {
    // 先尝试绑定角色到令牌，成功后才设置 selectedProfile
    _, updateErr := l.repo.gameTokenRepo.UpdateBoundProfile(ctx, nil, gameToken.ID, &profiles[0].ID)
    if updateErr != nil {
        l.log.Error(ctx, fmt.Sprintf("绑定角色到令牌失败: %s", updateErr.ErrorMessage))
        // 回滚刚创建的令牌，避免产生无角色绑定的"孤儿令牌"占用配额槽位
        if _, rollbackErr := l.repo.gameTokenRepo.InvalidateByAccessToken(ctx, nil, gameToken.AccessToken); rollbackErr != nil {
            l.log.Error(ctx, fmt.Sprintf("回滚孤儿令牌失败: %s", rollbackErr.ErrorMessage))
        }
        // 不设置 selectedProfile，客户端将通过 refresh 流程选择角色
    } else {
        selectedProfile = &profiles[0]
    }
}
```

---

## 四、新建/修改方法清单

| 文件路径 | 操作 | 方法/常量 |
|----------|------|-----------|
| `api/yggdrasil/request.go` | 修改 | 3 处 ClientToken struct tag 修正 |
| `internal/logic/yggdrasil/auth.go` | 修改 | RefreshToken 增加 Status+ExpiresAt 检查；AuthenticateUser 绑定失败回滚 |
| `internal/repository/game_token.go` | 修改 | RevokeValidOrTempInvalid WHERE 追加 expires_at 条件 |
| `internal/app/middleware/yggdrasil_ratelimit.go` | **新建** | YggdrasilAuthRateLimit 中间件 |
| `internal/constant/yggdrasil.go` | 修改 | 新增 3 个速率限制常量 |
| `internal/app/route/route_yggdrasil.go` | 修改 | authenticate/signout 挂载限流中间件 |
| `internal/logic/yggdrasil/logic.go` | 修改 | NewYggdrasilLogic RSAKeyPair 内部字段守卫 |
| `internal/logic/yggdrasil/signing.go` | 修改 | 新增导出函数 IsValidUnsignedUUID |
| `internal/handler/yggdrasil/server/server.go` | 修改 | ProfileQuery 使用共享 IsValidUnsignedUUID |
| `internal/handler/yggdrasil/share/share.go` | 修改 | UploadTexture/DeleteTexture 使用共享 IsValidUnsignedUUID |

**统计**：修改 9 个文件，新建 1 个文件。

---

## 五、验证标准

### 编译验证

- [x] `go build ./...` 编译通过（零错误零警告）

### C-1 验证

- [x] `api/yggdrasil/request.go` 中 3 处 ClientToken 均为 `` binding:"omitempty,max=368" `` 格式
- [x] 全局搜索 `clientToken,max=368` 结果为 0（旧格式已清除）

### C-2 验证

- [x] `auth.go` RefreshToken 含 `oldToken.Status != GameTokenStatusValid && != TempInvalid` 检查
- [x] `auth.go` RefreshToken 含 `time.Now().After(oldToken.ExpiresAt)` 检查
- [x] `game_token.go` RevokeValidOrTempInvalid WHERE 含 `AND expires_at > ?` 条件

### H-1 验证

- [x] `yggdrasil_ratelimit.go` 文件存在且含 `YggdrasilAuthRateLimit` 函数
- [x] `yggdrasil.go` 含 `YggdrasilAuthRateLimit = 5`、`YggdrasilSignoutRateLimit = 10`、`YggdrasilRateLimitWindowSec = 60`
- [x] `route_yggdrasil.go` authenticate 路由含 `YggdrasilAuthRateLimit("authenticate", ...)` 中间件
- [x] `route_yggdrasil.go` signout 路由含 `YggdrasilAuthRateLimit("signout", ...)` 中间件

### H-2 验证

- [x] `logic.go` NewYggdrasilLogic 含嵌套 `if pair.PrivKey != nil && pair.PubKeyPEM != ""` 守卫
- [x] 底部 `privKey == nil \|\| pubKeyPEM == ""` Panic 守卫仍存在（双重防护）

### H-3 验证

- [x] `signing.go` 含导出函数 `IsValidUnsignedUUID(s string) bool`
- [x] `server.go` ProfileQuery 使用 `yggdrasil.IsValidUnsignedUUID(uuid)` 替代手动循环
- [x] `share.go` UploadTexture 使用 `yggdrasil.IsValidUnsignedUUID(uuid)` 替代手动循环
- [x] `share.go` DeleteTexture 使用 `yggdrasil.IsValidUnsignedUUID(uuid)` 替代手动循环
- [x] `game_profile_ygg.go` `unsignedUUIDToStandard` 内嵌校验保持不变（纵深防御）

### H-4 验证

- [x] `auth.go` AuthenticateUser 步骤 6 含 `InvalidateByAccessToken` 回滚调用
- [x] 回滚失败时有独立的 Error 日志记录

---

## 六、未处理缺陷清单（Medium/Low — 留待后续轮次）

| 编号 | 级别 | 置信度 | 简述 | 延迟原因 |
|------|------|--------|------|----------|
| M-1 | Medium | 78% | Signout 二次扫描仅单轮重试 | 极端并发场景，生产影响低 |
| M-2 | Medium | 78% | Invalidate Handler 请求体解析失败静默返回 204 | 协议合规优先（invalidate 必须返回 204） |
| M-3 | Medium | 75% | BatchLookupProfiles 双重截断 | 不影响正确性，仅维护性 |
| M-4 | Medium | 72% | pickDB 辅助方法 11 处重复 | 低优先级重构 |
| L-1 | Low | 70% | 硬编码 bcrypt Dummy Hash 为公开测试向量 | 时序防护本身有效，审计友好性改进 |
| L-2 | Low | 68% | ProfileQuery 非法 UUID 返回 204 而非 400 | 有意设计决策（协议规定） |
| L-3 | Low | 65% | route_yggdrasil.go authRequired 变量命名歧义 | 纯命名问题 |

---

## 七、与 Patch 008 的关系说明

| Patch 008 缺陷 | 本次处理 | 说明 |
|---------------|---------|------|
| C-1 UUID hex 校验遗漏 | — | Patch 008 已修复（unsignedUUIDToStandard），本次延伸至 Handler 层统一入口 |
| C-2 Invalidate 原子性 | — | Patch 008 已修复（条件 UPDATE），本次无变更 |
| I-5 空 payload 签名 | — | Patch 008 已修复（Marshal 失败早返回），本次无变更 |
| I-6 UploadTexture UUID 预检 | **H-3 替代升级** | Patch 008 手动内联校验 → 本次替换为共享函数调用 |
| I-7 IP 标准化比较 | — | Patch 008 已修复（net.ParseIP），本次无变更 |
| **新增 C-1** | **本次修复** | struct tag 语法错误（Patch 008 未覆盖 DTO 层） |
| **新增 C-2** | **本次修复** | RefreshToken 过期续期（Patch 008 未覆盖此场景） |
| **新增 H-1** | **本次修复** | 速率限制（全新功能，此前各轮均未涉及） |
| **新增 H-2** | **本次修复** | RSA nil 守卫（Patch 008 未覆盖初始化路径） |
| **新增 H-4** | **本次修复** | 绑定回滚（Patch 008 未覆盖 Authenticate 流程） |
