# Patch 004: 令牌一致性、错误分类与事务原子性修复

> 状态：已执行
> 创建日期：2026-04-16
> 关联规范：`dev/markdown/YGGDRASIL_SPECIFICATION.md`
> 关联补丁：Patch 003
> 基于审查：Yggdrasil 协议实现完整代码审查报告

---

## 一、缺陷背景

基于对 Yggdrasil 协议实现（Patch 001-003 已执行完毕）的第三轮深度审查，
在补丁覆盖范围外新发现 7 个缺陷，涉及令牌验证一致性、错误处理分类、
可观测性、事务原子性和并发安全。

---

## 二、缺陷清单与修复

### 缺陷 #1 [Important]：三处令牌验证调用方冗余 Status 检查

#### 问题描述

`ValidateGameToken`（Logic 层）已同时检查 `Status == Valid` 和 `ExpiresAt` 未过期。
当返回 `found=true` 时，token 必然是有效且未过期的。但以下三处调用方
仍手动检查 `gameToken.Status != entity.GameTokenStatusValid`：

| 位置 | 代码 | 风险 |
|------|------|------|
| `yggdrasil_auth.go:54` (中间件) | 冗余 Status 检查 | 未来 ValidateGameToken 行为变更时中间件不会同步 |
| `share.go:201` (validateBearerAuth) | 同上 | 同上 |
| `client.go:191` (Validate Handler) | 同上 | JoinServer 已正确使用 `!found`，三者不一致 |

#### 改动

**3 个文件统一将 `!found \|\| gameToken.Status != entity.GameTokenStatusValid` 改为 `!found`**

- `internal/app/middleware/yggdrasil_auth.go:54` — 移除冗余检查 + 移除 entity import
- `internal/handler/yggdrasil/share/share.go:201` — 移除冗余检查（entity import 保留，返回值类型仍需）
- `internal/handler/yggdrasil/client/client.go:191` — 移除冗余检查 + 移除 entity import

#### 验证标准

- [x] `go build ./...` 编译通过
- [x] 全局搜索 `gameToken.Status != entity.GameTokenStatusValid` 在 handler/middleware 中结果为 0
- [x] 全局搜索同模式仅剩 `auth.go:39`（ValidateGameToken 自身）

---

### 缺陷 #2 [Important]：AuthenticateUser 单角色绑定失败静默忽略

#### 问题描述

单角色用户登录时，`UpdateBoundProfile` 失败后函数仍返回 `selectedProfile`。
客户端认为角色已绑定 → 调用 JoinServer → Handler 检查 `BoundProfileID==nil` → **403 Forbidden**
→ 用户困惑：登录显示已选中的角色，但进服被拒。

#### 改动

**文件**：`internal/logic/yggdrasil/auth.go:112-119`

```go
// 改前：先设 selectedProfile 再绑定（失败也返回）
selectedProfile = &profiles[0]
_, updateErr := l.repo.gameTokenRepo.UpdateBoundProfile(...)
if updateErr != nil { l.log.Warn(...) } // ⚠️ 继续返回 selectedProfile

// 改后：先绑定成功才设置 selectedProfile
_, updateErr := l.repo.gameTokenRepo.UpdateBoundProfile(...)
if updateErr != nil {
    l.log.Error(ctx, fmt.Sprintf("绑定角色到令牌失败: %s", updateErr.ErrorMessage))
    // 不设置 selectedProfile，客户端通过 refresh 选择角色
} else {
    selectedProfile = &profiles[0]
}
```

**行为变更**：单角色用户登录后 `selectedProfile=nil`，客户端需额外发起一次 refresh
请求来选择角色。这是有意为之的安全改进。

#### 验证标准

- [x] `go build ./...` 编译通过
- [x] 绑定失败日志级别为 Error（非 Warn）
- [x] `selectedProfile = &profiles[0]` 仅出现在 else 分支中

---

### 缺陷 #3 [Important]：Signout Handler 未区分内部错误与凭证错误

#### 问题描述

Signout Handler 将所有错误统一映射为 403 ForbiddenOperationException。
数据库故障（返回 ServerInternalError, >=50000）也被映射为 403，
导致运维无法区分"密码错误"和"服务不可用"。

Authenticate Handler（Patch 003 已修复）已正确使用 xError 错误码分类。

#### 改动

**文件**：`internal/handler/yggdrasil/client/client.go:239-242`

```go
// 改前
if xErr != nil {
    apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", string(xErr.ErrorMessage))
    return
}

// 改后（复制 Authenticate 的错误分类模式）
if xErr != nil {
    errMsg := string(xErr.ErrorMessage)
    if xErr.GetErrorCode() != nil && xErr.GetErrorCode().GetCode() >= 50000 {
        apiYgg.AbortYggError(ctx, http.StatusInternalServerError, "InternalServerError", errMsg)
    } else {
        apiYgg.AbortYggError(ctx, http.StatusForbidden, "ForbiddenOperationException", errMsg)
    }
    return
}
```

#### 验证标准

- [x] `go build ./...` 编译通过
- [x] Signout 使用 `xError.GetErrorCode().GetCode() >= 50000` 判断
- [x] 新增 `"fmt"` import（用于 string(xErr.ErrorMessage)）

---

### 缺陷 #4 [Important]：InvalidateToken 静默吞没数据库错误

#### 问题描述

Logic 层 `_, _ = l.repo.gameTokenRepo.UpdateStatus(...)` 完全忽略错误。
Handler 层 `InvalidateToken(...)` 返回值被丢弃。两层静默导致：
- Redis/DB 故障时令牌实际仍然有效
- 运维完全无感知（符合规范 204 响应，但零可观测性）

#### 改动

**文件 A**：`internal/logic/yggdrasil/auth.go:247-256` — 签名变更

```go
// 改前：无返回值，_, _ 忽略错误
func (l *YggdrasilLogic) InvalidateToken(ctx context.Context, accessToken string) { ... }

// 改后：返回 *xError.Error
func (l *YggdrasilLogic) InvalidateToken(ctx context.Context, accessToken string) *xError.Error { ...
    return updateErr // 返回查询/更新错误
}
```

**文件 B**：`internal/handler/yggdrasil/client/client.go:220` — 捕获并记录

```go
// 改前
h.Service.Logic().InvalidateToken(ctx.Request.Context(), req.AccessToken)

// 改后
invErr := h.Service.Logic().InvalidateToken(ctx.Request.Context(), req.AccessToken)
if invErr != nil {
    h.Log.Warn(ctx, fmt.Sprintf("吊销令牌异常（客户端不可见）: %s", invErr.ErrorMessage))
}
```

HTTP 响应始终为 204 No Content，符合 Yggdrasil 规范。

#### 验证标准

- [x] `go build ./...` 编译通过
- [x] `_, _ = l.repo.gameTokenRepo.UpdateStatus` 全局搜索结果为 0
- [x] Handler 层有 Warn 日志捕获 invalidate 异常

---

### 缺陷 #5 [Important]：RefreshToken 吊销+创建非原子

#### 问题描述

两步操作不在同一事务中：

```go
UpdateStatus(ctx, nil, oldToken.ID, Invalid)  // Step 1: 成功
CreateGameToken(ctx, oldToken.UserID, ...)       // Step 2: 失败！
// 结果：旧令牌已被标记 Invalid，用户失去所有有效令牌
```

#### 改动

**新建文件**：`internal/repository/txn/game_token.go`（事务协调层）

新增 `GameTokenTxnRepo.RevokeAndCreate()` 方法：GORM `Transaction()` 内原子执行
吊销旧令牌 + 创建新令牌。任一步骤失败自动回滚。

**修改文件**：`internal/logic/yggdrasil/auth.go:173-184`

RefreshToken 不再调用 CreateGameToken（避免触发配额检查），改为：
1. 内联生成 accessToken UUID
2. 构建 newTokenEntity
3. 调用 `l.repo.gameTokenTxnRepo.RevokeAndCreate(ctx, oldToken.ID, newTokenEntity)` 原子执行

> 设计决策：Refresh 是"以旧换新"语义，不应受配额限制。

#### 验证标准

- [x] `go build ./...` 编译通过
- [x] `RevokeAndCreate` 出现在 txn/game_token.go + auth.go(RefreshToken)
- [x] logic.go 注入 `gameTokenTxnRepo *txn.GameTokenTxnRepo`

---

### 缺陷 #6 [Important]：CreateGameToken 配额竞态条件

#### 问题描述

四步操作不在同一事务中：查询 count → 检查上限 → RevokeOldest → Create。
高并发下两个请求可能同时看到 count==10，都触发 RevokeOldest，最终超出上限。

#### 改动

**复用缺陷 #5 新建的 `txn/game_token.go`**

新增 `GameTokenTxnRepo.CreateWithQuotaCheck()` 方法：GORM `Transaction()` 内原子执行
COUNT 有效令牌 → 超限则 RevokeOldest → Create。

**修改文件**：`internal/logic/yggdrasil/auth.go:309-370`（CreateGameToken 方法体）

原有四步非原子操作替换为单次 `l.repo.gameTokenTxnRepo.CreateWithQuotaCheck(...)` 调用。

#### 验证标准

- [x] `go build ./...` 编译通过
- [x] `CreateWithQuotaCheck` 出现在 txn/game_token.go + auth.go(CreateGameToken)
- [x] 原 `GetValidTokensByUserID` + `RevokeOldestByUserID` 在 CreateGameToken 中不再直接调用

---

### 缺陷 #7 [Low]：批量查询静默截断

#### 问题描述

`ProfilesBatchLookup` 超过 10 个名称时静默截断无日志，排查困难。

#### 改动

**文件**：`internal/handler/yggdrasil/server/server.go:143-145`

```go
// 改前
if len(names) > 10 { names = names[:10] }

// 改后
if len(names) > 10 {
    h.Log.Warn(ctx, fmt.Sprintf("批量查询名称数量超限: 请求 %d 个，截断至 10 个", len(names)))
    names = names[:10]
}
```

新增 `"fmt"` import。

#### 验证标准

- [x] `go build ./...` 编译通过
- [x] 截断处有 Warn 日志

---

## 三、新建文件清单

| 文件路径 | 用途 |
|----------|------|
| `internal/repository/txn/game_token.go` | GameTokenTxnRepo 事务协调层（RevokeAndCreate + CreateWithQuotaCheck） |

## 四、修改文件清单

| 文件路径 | 缺陷 |
|----------|------|
| `internal/app/middleware/yggdrasil_auth.go` | #1 C-2.1 |
| `internal/handler/yggdrasil/share/share.go` | #1 C-2.2 |
| `internal/handler/yggdrasil/client/client.go` | #1 C-2.3 + #3 I-2 + #4 I-3b |
| `internal/logic/yggdrasil/auth.go` | #2 I-1 + #4 I-3a + #5 I-4 + #6 I-5 |
| `internal/logic/yggdrasil/logic.go` | #5/#6 I-4/I-5 依赖注入 |
| `internal/handler/yggdrasil/server/server.go` | #7 I-6 |

## 五、执行顺序建议

1. 阶段一（纯逻辑修正，无依赖）：缺陷 #1 ~ #4、#7
2. 阶段二（新建事务层）：新建 `txn/game_token.go`
3. 阶段三（依赖阶段二）：logic.go 注入 + 缺陷 #5、#6 重写
